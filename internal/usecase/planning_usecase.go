package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

var plannableCommoditySlugs = []string{"padi", "jagung"}

type PlanRefusal struct {
	Code string
}

func (r *PlanRefusal) Error() string {
	return r.Code
}

type SkippedPlot struct {
	PlotID     string
	PlotName   string
	MemberName string
	Reason     string
}

type Proposal struct {
	Season            planning.Season
	Plans             []planning.Plan
	Skipped           []SkippedPlot
	YieldObservations int
	Engine            constants.PlanEngine
}

type PlanningUseCase struct {
	DB                       *gorm.DB
	Log                      *logrus.Logger
	Validate                 *validator.Validate
	PlotRepository           *repository.PlotRepository
	BlockRepository          *repository.BlockRepository
	MemberRepository         *repository.MemberRepository
	CommodityRepository      *repository.CommodityRepository
	VarietyRepository        *repository.VarietyRepository
	CooperativeRepository    *repository.CooperativeRepository
	ReferencePriceRepository *repository.ReferencePriceRepository
	SupplyRequestRepository  *repository.SupplyRequestRepository
	SeasonPlanRepository     *repository.SeasonPlanRepository
	Projection               *ProjectionUseCase
	Weather                  *WeatherUseCase
	Catalog                  *CatalogUseCase
	AI                       *aiclient.Client
	Redis                    *redis.Client
}

func NewPlanningUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	memberRepository *repository.MemberRepository,
	commodityRepository *repository.CommodityRepository,
	varietyRepository *repository.VarietyRepository,
	cooperativeRepository *repository.CooperativeRepository,
	referencePriceRepository *repository.ReferencePriceRepository,
	supplyRequestRepository *repository.SupplyRequestRepository,
	seasonPlanRepository *repository.SeasonPlanRepository,
	projection *ProjectionUseCase, weatherUseCase *WeatherUseCase,
	catalog *CatalogUseCase, ai *aiclient.Client, cache *redis.Client,
) *PlanningUseCase {
	return &PlanningUseCase{
		DB:                       db,
		Log:                      log,
		Validate:                 validate,
		PlotRepository:           plotRepository,
		BlockRepository:          blockRepository,
		MemberRepository:         memberRepository,
		CommodityRepository:      commodityRepository,
		VarietyRepository:        varietyRepository,
		CooperativeRepository:    cooperativeRepository,
		ReferencePriceRepository: referencePriceRepository,
		SupplyRequestRepository:  supplyRequestRepository,
		SeasonPlanRepository:     seasonPlanRepository,
		Projection:               projection,
		Weather:                  weatherUseCase,
		Catalog:                  catalog,
		AI:                       ai,
		Redis:                    cache,
	}
}

func (u *PlanningUseCase) Propose(
	ctx context.Context, cooperativeID, seasonLabel string, now time.Time,
) (Proposal, error) {
	season, open := planning.SeasonByLabel(seasonLabel, now)
	if !open {
		return Proposal{}, &PlanRefusal{Code: constants.PlanSeasonClosed}
	}

	dates := planning.CandidatePlantingDates(season, now)
	if len(dates) == 0 {
		return Proposal{}, &PlanRefusal{Code: constants.PlanSeasonClosed}
	}

	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return Proposal{}, err
	}
	if len(projection.Plots) == 0 {
		return Proposal{}, &PlanRefusal{Code: constants.PlanNoPlots}
	}

	normals, err := u.normalsFor(ctx, projection.Plots, now)
	if err != nil {
		return Proposal{}, err
	}

	varieties, commodityOfVariety, err := u.plannableVarieties(ctx)
	if err != nil {
		return Proposal{}, err
	}
	if len(varieties) == 0 {
		return Proposal{}, &PlanRefusal{Code: constants.PlanNoEligiblePlots}
	}

	names, err := u.memberNames(ctx, cooperativeID)
	if err != nil {
		return Proposal{}, err
	}

	candidates, skipped, err := u.buildCandidates(
		projection, normals, varieties, commodityOfVariety, names, dates)
	if err != nil {
		return Proposal{}, err
	}
	if len(candidates) == 0 {
		return Proposal{}, &PlanRefusal{Code: constants.PlanNoEligiblePlots}
	}

	prices, err := u.seasonalPrices(ctx, cooperativeID, commodityOfVariety, season)
	if err != nil {
		return Proposal{}, err
	}

	demand, err := u.historicalDemand(ctx, cooperativeID, season)
	if err != nil {
		return Proposal{}, err
	}

	capacity, err := u.capacityOf(u.DB.WithContext(ctx), cooperativeID)
	if err != nil {
		return Proposal{}, err
	}

	input := planning.Input{
		Season:     season,
		Plots:      candidates,
		PricePerKg: prices,
		Demand:     demand,
		Capacity:   capacity,
	}
	plans, engine := u.solve(ctx, input, season, now)

	return Proposal{
		Season:            season,
		Plans:             plans,
		Skipped:           skipped,
		YieldObservations: projection.Yield.NObservations,
		Engine:            engine,
	}, nil
}

func (u *PlanningUseCase) normalsFor(
	ctx context.Context, plots []entity.Plot, now time.Time,
) (map[weather.GridCell][]agronomy.ClimateNormal, error) {
	normals := map[weather.GridCell][]agronomy.ClimateNormal{}

	for _, plot := range plots {
		cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
		if _, loaded := normals[cell]; loaded {
			continue
		}

		cellWeather, err := u.Weather.LoadWeatherFor(ctx, cell, time.Time{}, now)
		if err != nil {
			return nil, err
		}
		if len(cellWeather.Normals) == 0 {
			return nil, &PlanRefusal{Code: constants.PlanNoClimateNormals}
		}
		normals[cell] = cellWeather.Normals
	}
	return normals, nil
}

func (u *PlanningUseCase) plannableVarieties(
	ctx context.Context,
) ([]entity.Variety, map[string]string, error) {
	db := u.DB.WithContext(ctx)

	commodities := []entity.Commodity{}
	if err := db.Where("slug IN ?", plannableCommoditySlugs).
		Order("slug").Find(&commodities).Error; err != nil {
		return nil, nil, fmt.Errorf("reading plannable commodities: %w", err)
	}

	ids := make([]string, len(commodities))
	for i, commodity := range commodities {
		ids[i] = commodity.ID
	}

	varieties := []entity.Variety{}
	if len(ids) > 0 {
		if err := db.Where("commodity_id IN ?", ids).
			Order("commodity_id, name, id").Find(&varieties).Error; err != nil {
			return nil, nil, fmt.Errorf("reading plannable varieties: %w", err)
		}
	}

	commodityOfVariety := make(map[string]string, len(varieties))
	for _, variety := range varieties {
		commodityOfVariety[variety.ID] = variety.CommodityID
	}
	return varieties, commodityOfVariety, nil
}

func (u *PlanningUseCase) memberNames(
	ctx context.Context, cooperativeID string,
) (map[string]string, error) {
	members, err := u.MemberRepository.FindByCooperativeID(u.DB.WithContext(ctx), cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading members of cooperative %s: %w", cooperativeID, err)
	}

	names := make(map[string]string, len(members))
	for _, member := range members {
		names[member.ID] = member.Name
	}
	return names, nil
}

func (u *PlanningUseCase) buildCandidates(
	projection Projection,
	normals map[weather.GridCell][]agronomy.ClimateNormal,
	varieties []entity.Variety,
	commodityOfVariety map[string]string,
	names map[string]string,
	dates []time.Time,
) ([]planning.PlotCandidate, []SkippedPlot, error) {
	simulators := map[weather.GridCell]*planning.Simulator{}
	for cell, cellNormals := range normals {
		simulators[cell] = planning.NewSimulator(cellNormals)
	}

	occupiedUntil := map[string]time.Time{}
	for _, block := range projection.Blocks {
		if block.ActualHarvestDate != nil {
			continue
		}
		until := block.PlantingDate
		if window, known := projection.Windows[block.ID]; known {
			until = window.End
		}
		if current, seen := occupiedUntil[block.PlotID]; !seen || until.After(current) {
			occupiedUntil[block.PlotID] = until
		}
	}

	plots := append([]entity.Plot{}, projection.Plots...)
	sort.SliceStable(plots, func(i, j int) bool {
		left, right := names[plots[i].MemberID], names[plots[j].MemberID]
		if left != right {
			return left < right
		}
		if plots[i].Name != plots[j].Name {
			return plots[i].Name < plots[j].Name
		}
		return plots[i].ID < plots[j].ID
	})

	candidates := []planning.PlotCandidate{}
	skipped := []SkippedPlot{}

	for _, plot := range plots {
		cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
		simulator := simulators[cell]

		options := []planning.Assignment{}
		for _, variety := range varieties {
			for _, plantingDate := range dates {
				if free, occupied := occupiedUntil[plot.ID]; occupied &&
					!plantingDate.After(free) {
					continue
				}

				option, usable, err := u.optionFor(
					simulator, projection.Yield, plot, variety,
					commodityOfVariety[variety.ID], names[plot.MemberID], plantingDate)
				if err != nil {
					return nil, nil, err
				}
				if usable {
					options = append(options, option)
				}
			}
		}

		if len(options) == 0 {
			skipped = append(skipped, SkippedPlot{
				PlotID:     plot.ID,
				PlotName:   plot.Name,
				MemberName: names[plot.MemberID],
				Reason:     constants.PlanNoEligiblePlots,
			})
			continue
		}

		candidates = append(candidates, planning.PlotCandidate{
			PlotID:     plot.ID,
			PlotName:   plot.Name,
			MemberID:   plot.MemberID,
			MemberName: names[plot.MemberID],
			AreaHa:     plot.AreaHa,
			Options:    options,
		})
	}
	return candidates, skipped, nil
}

func (u *PlanningUseCase) optionFor(
	simulator *planning.Simulator, model agronomy.YieldModel,
	plot entity.Plot, row entity.Variety,
	commodityID, memberName string, plantingDate time.Time,
) (planning.Assignment, bool, error) {
	variety := agronomy.Variety{
		GddRequirement:   row.GddRequirement,
		BaseTempC:        row.BaseTempC,
		DaysToHarvestMin: row.DaysToHarvestMin,
		DaysToHarvestMax: row.DaysToHarvestMax,
		YieldPerHaMin:    row.YieldPerHaMin,
		YieldPerHaMax:    row.YieldPerHaMax,
	}

	window, plausibility, err := simulator.Window(row.ID, variety, nil, plantingDate)
	if err != nil {
		return planning.Assignment{}, false, fmt.Errorf(
			"simulating plot %s with variety %s: %w", plot.ID, row.ID, err)
	}
	if plausibility == constants.PlausibilityImplausible {
		return planning.Assignment{}, false, nil
	}

	low, mid, high := simulator.YieldPerHaRange(
		model, variety, plantingDate, window.End, plot.AreaHa)

	return planning.Assignment{
		PlotID:       plot.ID,
		PlotName:     plot.Name,
		MemberID:     plot.MemberID,
		MemberName:   memberName,
		AreaHa:       plot.AreaHa,
		CommodityID:  commodityID,
		VarietyID:    row.ID,
		VarietyName:  row.Name,
		PlantingDate: plantingDate,
		Window:       window,
		Plausibility: plausibility,
		TonnesLow:    low * plot.AreaHa,
		TonnesMid:    mid * plot.AreaHa,
		TonnesHigh:   high * plot.AreaHa,
	}, true, nil
}

func (u *PlanningUseCase) seasonalPrices(
	ctx context.Context, cooperativeID string,
	commodityOfVariety map[string]string, season planning.Season,
) (map[string]float64, error) {
	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(
		u.DB.WithContext(ctx), cooperative, cooperativeID); err != nil {
		return nil, fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	ids := []string{}
	seen := map[string]bool{}
	for _, commodityID := range commodityOfVariety {
		if !seen[commodityID] {
			seen[commodityID] = true
			ids = append(ids, commodityID)
		}
	}
	sort.Strings(ids)

	rows, err := u.ReferencePriceRepository.FindForCommodities(
		u.DB.WithContext(ctx), cooperative.Province, ids)
	if err != nil {
		return nil, fmt.Errorf("reading reference prices: %w", err)
	}

	published := make([]agronomy.ReferencePrice, len(rows))
	for i, row := range rows {
		published[i] = agronomy.ReferencePrice{
			CommodityID: row.CommodityID,
			WeekStart:   row.WeekStart,
			PricePerKg:  row.PricePerKg,
			Source:      row.Source,
		}
	}

	middle := agronomy.AddDays(season.Start,
		agronomy.DaysBetween(season.Start, season.End)/2)

	prices := map[string]float64{}
	for _, commodityID := range ids {
		benchmark := agronomy.BenchmarkFor(published, commodityID, middle)
		if benchmark == nil || benchmark.Seasonal == nil {
			continue
		}
		prices[commodityID] = benchmark.Seasonal.PricePerKg
	}
	return prices, nil
}

func (u *PlanningUseCase) historicalDemand(
	ctx context.Context, cooperativeID string, season planning.Season,
) ([]planning.Demand, error) {
	rows, err := u.SupplyRequestRepository.FindForCooperative(
		u.DB.WithContext(ctx), cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading supply requests of %s: %w", cooperativeID, err)
	}

	requests := make([]planning.HistoricalRequest, len(rows))
	for i, row := range rows {
		requests[i] = planning.HistoricalRequest{
			CommodityID: row.CommodityID,
			VolumeKg:    row.VolumeKg,
			WindowStart: row.WindowStart,
		}
	}
	return planning.DemandByWeek(requests, season), nil
}

func (u *PlanningUseCase) capacityOf(
	db *gorm.DB, cooperativeID string,
) (map[string]float64, error) {
	rows, err := u.CooperativeRepository.FindCapacity(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading capacity of cooperative %s: %w", cooperativeID, err)
	}

	capacity := make(map[string]float64, len(rows))
	for _, row := range rows {
		capacity[row.CommodityID] = row.TonnesPerWeek
	}
	return capacity, nil
}

type AppliedPlan struct {
	PlanID string
	Blocks int
}

func (u *PlanningUseCase) Apply(
	ctx context.Context, user *entity.AppUser,
	request *model.ApplySeasonPlanRequest, now time.Time,
) (AppliedPlan, error) {
	if user.CooperativeID == nil {
		return AppliedPlan{}, ErrNoCooperative
	}
	if err := u.Validate.Struct(request); err != nil {
		return AppliedPlan{}, err
	}
	cooperativeID := *user.CooperativeID

	season, open := planning.SeasonByLabel(request.SeasonLabel, now)
	if !open {
		return AppliedPlan{}, &PlanRefusal{Code: constants.PlanSeasonClosed}
	}

	db := u.DB.WithContext(ctx)
	if _, err := u.SeasonPlanRepository.FindActiveByLabel(
		db, cooperativeID, season.Label); err == nil {
		return AppliedPlan{}, &PlanRefusal{Code: constants.PlanAlreadyApplied}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return AppliedPlan{}, fmt.Errorf("reading active plan of %s: %w", cooperativeID, err)
	}

	offered, err := u.offeredOptions(ctx, cooperativeID, season, now)
	if err != nil {
		return AppliedPlan{}, err
	}

	assignments := make([]planning.Assignment, len(request.Assignments))
	for i, wanted := range request.Assignments {
		matched, found := offered[optionKey(
			wanted.PlotID, wanted.VarietyID, wanted.PlantingDate)]
		if !found {
			return AppliedPlan{}, &PlanRefusal{Code: constants.PlanAssignmentRejected}
		}
		assignments[i] = matched
	}

	plan := &entity.SeasonPlan{
		ID:            uuid.NewString(),
		CooperativeID: cooperativeID,
		SeasonLabel:   season.Label,
		SeasonStart:   season.Start,
		SeasonEnd:     season.End,
		Objective:     constants.PlanningObjective(request.Objective),
		Status:        constants.PlanApplied,
		CreatedBy:     user.ID,
		CreatedAt:     now,
	}

	if err := u.persistPlan(ctx, plan, assignments); err != nil {
		return AppliedPlan{}, err
	}

	if u.Catalog != nil {
		u.Catalog.Invalidate(ctx, now)
	}

	return AppliedPlan{PlanID: plan.ID, Blocks: len(assignments)}, nil
}

func optionKey(plotID, varietyID, plantingDate string) string {
	return plotID + "|" + varietyID + "|" + plantingDate
}

func (u *PlanningUseCase) offeredOptions(
	ctx context.Context, cooperativeID string, season planning.Season, now time.Time,
) (map[string]planning.Assignment, error) {
	dates := planning.CandidatePlantingDates(season, now)
	if len(dates) == 0 {
		return nil, &PlanRefusal{Code: constants.PlanSeasonClosed}
	}

	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return nil, err
	}
	if len(projection.Plots) == 0 {
		return nil, &PlanRefusal{Code: constants.PlanNoPlots}
	}

	normals, err := u.normalsFor(ctx, projection.Plots, now)
	if err != nil {
		return nil, err
	}

	varieties, commodityOfVariety, err := u.plannableVarieties(ctx)
	if err != nil {
		return nil, err
	}

	names, err := u.memberNames(ctx, cooperativeID)
	if err != nil {
		return nil, err
	}

	candidates, _, err := u.buildCandidates(
		projection, normals, varieties, commodityOfVariety, names, dates)
	if err != nil {
		return nil, err
	}

	offered := map[string]planning.Assignment{}
	for _, candidate := range candidates {
		for _, option := range candidate.Options {
			offered[optionKey(option.PlotID, option.VarietyID,
				agronomy.ToISODate(option.PlantingDate))] = option
		}
	}
	return offered, nil
}

func (u *PlanningUseCase) persistPlan(
	ctx context.Context, plan *entity.SeasonPlan, assignments []planning.Assignment,
) error {
	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.SeasonPlanRepository.Create(tx, plan); err != nil {
		return fmt.Errorf("creating season plan for %s: %w", plan.CooperativeID, err)
	}

	for _, assignment := range assignments {
		nextIndex, err := u.BlockRepository.NextOrderIndex(tx, assignment.PlotID)
		if err != nil {
			return fmt.Errorf("reading block order of plot %s: %w", assignment.PlotID, err)
		}

		planID := plan.ID
		block := &entity.Block{
			ID:           uuid.NewString(),
			PlotID:       assignment.PlotID,
			Label:        plots.BlockLabel(nextIndex),
			AreaHa:       assignment.AreaHa,
			OrderIndex:   nextIndex,
			CommodityID:  assignment.CommodityID,
			VarietyID:    assignment.VarietyID,
			PlantingDate: assignment.PlantingDate,
			SeasonPlanID: &planID,
		}
		if err := u.BlockRepository.Create(tx, block); err != nil {
			return fmt.Errorf("creating plan block on plot %s: %w", assignment.PlotID, err)
		}

		blockID := block.ID
		item := &entity.SeasonPlanItem{
			ID:                   uuid.NewString(),
			PlanID:               plan.ID,
			PlotID:               assignment.PlotID,
			MemberID:             assignment.MemberID,
			CommodityID:          assignment.CommodityID,
			VarietyID:            assignment.VarietyID,
			PlantingDate:         assignment.PlantingDate,
			AreaHa:               assignment.AreaHa,
			ExpectedTonnesLow:    assignment.TonnesLow,
			ExpectedTonnesMid:    assignment.TonnesMid,
			ExpectedTonnesHigh:   assignment.TonnesHigh,
			ExpectedHarvestStart: assignment.Window.Start,
			ExpectedHarvestEnd:   assignment.Window.End,
			Plausibility:         string(assignment.Plausibility),
			BlockID:              &blockID,
		}
		if err := tx.Create(item).Error; err != nil {
			return fmt.Errorf("creating plan item for plot %s: %w", assignment.PlotID, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing season plan %s: %w", plan.ID, err)
	}
	return nil
}

type CancelledPlan struct {
	PlanID string
	Blocks int
}

func (u *PlanningUseCase) Cancel(
	ctx context.Context, user *entity.AppUser, planID string, now time.Time,
) (CancelledPlan, error) {
	if user.CooperativeID == nil {
		return CancelledPlan{}, ErrNoCooperative
	}
	db := u.DB.WithContext(ctx)

	plan, err := u.SeasonPlanRepository.FindInCooperative(db, planID, *user.CooperativeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return CancelledPlan{}, &PlanRefusal{Code: constants.PlanNotFound}
	}
	if err != nil {
		return CancelledPlan{}, fmt.Errorf("reading plan %s: %w", planID, err)
	}
	if plan.Status == constants.PlanCancelled {
		return CancelledPlan{}, &PlanRefusal{Code: constants.PlanAlreadyCancelled}
	}

	blocks, err := u.BlockRepository.FindByPlanID(db, plan.ID)
	if err != nil {
		return CancelledPlan{}, fmt.Errorf("reading blocks of plan %s: %w", plan.ID, err)
	}
	for _, block := range blocks {
		if recorded(block) || !block.PlantingDate.After(agronomy.StartOfDay(now)) {
			return CancelledPlan{}, &PlanRefusal{Code: constants.PlanPartiallyCancellable}
		}
	}

	if err := u.releasePlan(ctx, plan, now); err != nil {
		return CancelledPlan{}, err
	}

	if u.Catalog != nil {
		u.Catalog.Invalidate(ctx, now)
	}

	return CancelledPlan{PlanID: plan.ID, Blocks: len(blocks)}, nil
}

func recorded(block entity.Block) bool {
	return block.ActualHarvestDate != nil ||
		block.ActualYieldKg != nil ||
		block.ActualPricePerKg != nil ||
		block.PaymentReceivedDate != nil
}

func (u *PlanningUseCase) releasePlan(
	ctx context.Context, plan *entity.SeasonPlan, now time.Time,
) error {
	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := tx.Model(&entity.SeasonPlanItem{}).Where("plan_id = ?", plan.ID).
		Update("block_id", nil).Error; err != nil {
		return fmt.Errorf("releasing items of plan %s: %w", plan.ID, err)
	}

	if err := tx.Where("season_plan_id = ?", plan.ID).
		Delete(&entity.Block{}).Error; err != nil {
		return fmt.Errorf("deleting blocks of plan %s: %w", plan.ID, err)
	}

	cancelled := now
	plan.Status = constants.PlanCancelled
	plan.CancelledAt = &cancelled
	if err := u.SeasonPlanRepository.Update(tx, plan); err != nil {
		return fmt.Errorf("cancelling plan %s: %w", plan.ID, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing the cancellation of plan %s: %w", plan.ID, err)
	}
	return nil
}

type StoredPlan struct {
	Plan           entity.SeasonPlan
	Items          []entity.SeasonPlanItem
	MemberNames    map[string]string
	PlotNames      map[string]string
	CommodityNames map[string]string
	VarietyNames   map[string]string
}

func (u *PlanningUseCase) List(
	ctx context.Context, user *entity.AppUser,
) ([]entity.SeasonPlan, error) {
	if user.CooperativeID == nil {
		return nil, ErrNoCooperative
	}

	plans, err := u.SeasonPlanRepository.FindByCooperativeID(
		u.DB.WithContext(ctx), *user.CooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading plans of %s: %w", *user.CooperativeID, err)
	}
	return plans, nil
}

func (u *PlanningUseCase) Get(
	ctx context.Context, user *entity.AppUser, planID string,
) (StoredPlan, error) {
	if user.CooperativeID == nil {
		return StoredPlan{}, ErrNoCooperative
	}
	db := u.DB.WithContext(ctx)

	plan, err := u.SeasonPlanRepository.FindInCooperative(db, planID, *user.CooperativeID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return StoredPlan{}, &PlanRefusal{Code: constants.PlanNotFound}
	}
	if err != nil {
		return StoredPlan{}, fmt.Errorf("reading plan %s: %w", planID, err)
	}

	items, err := u.SeasonPlanRepository.FindItemsByPlanID(db, plan.ID)
	if err != nil {
		return StoredPlan{}, fmt.Errorf("reading items of plan %s: %w", plan.ID, err)
	}

	memberNames, err := u.memberNames(ctx, *user.CooperativeID)
	if err != nil {
		return StoredPlan{}, err
	}

	plotNames, err := u.plotNames(db, *user.CooperativeID)
	if err != nil {
		return StoredPlan{}, err
	}

	commodityNames, varietyNames, err := u.catalogueNames(db, items)
	if err != nil {
		return StoredPlan{}, err
	}

	return StoredPlan{
		Plan:           *plan,
		Items:          items,
		MemberNames:    memberNames,
		PlotNames:      plotNames,
		CommodityNames: commodityNames,
		VarietyNames:   varietyNames,
	}, nil
}

func (u *PlanningUseCase) plotNames(
	db *gorm.DB, cooperativeID string,
) (map[string]string, error) {
	rows, err := u.PlotRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}

	names := make(map[string]string, len(rows))
	for _, plot := range rows {
		names[plot.ID] = plot.Name
	}
	return names, nil
}

func (u *PlanningUseCase) catalogueNames(
	db *gorm.DB, items []entity.SeasonPlanItem,
) (map[string]string, map[string]string, error) {
	commodityIDs := []string{}
	varietyIDs := []string{}
	seenCommodity := map[string]bool{}
	seenVariety := map[string]bool{}

	for _, item := range items {
		if !seenCommodity[item.CommodityID] {
			seenCommodity[item.CommodityID] = true
			commodityIDs = append(commodityIDs, item.CommodityID)
		}
		if !seenVariety[item.VarietyID] {
			seenVariety[item.VarietyID] = true
			varietyIDs = append(varietyIDs, item.VarietyID)
		}
	}

	commodities, err := u.CommodityRepository.FindByIDs(db, commodityIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("reading commodities of a plan: %w", err)
	}
	varieties, err := u.VarietyRepository.FindByIDs(db, varietyIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("reading varieties of a plan: %w", err)
	}

	commodityNames := make(map[string]string, len(commodities))
	for _, commodity := range commodities {
		commodityNames[commodity.ID] = commodity.Name
	}
	varietyNames := make(map[string]string, len(varieties))
	for _, variety := range varieties {
		varietyNames[variety.ID] = variety.Name
	}
	return commodityNames, varietyNames, nil
}

func (u *PlanningUseCase) solve(
	ctx context.Context, input planning.Input, season planning.Season, now time.Time,
) ([]planning.Plan, constants.PlanEngine) {
	if u.AI == nil {
		return planning.Search(input), constants.PlanEngineFallback
	}

	plans, err := u.askAIService(ctx, input, season, now)
	if err == nil {
		return plans, constants.PlanEngineAIService
	}

	if !errors.Is(err, aiclient.ErrBreakerOpen) {
		u.Log.WithError(err).Warn("layanan AI tidak terpakai, memakai solver lokal")
	}
	return planning.Search(input), constants.PlanEngineFallback
}

func (u *PlanningUseCase) askAIService(
	ctx context.Context, input planning.Input, season planning.Season, now time.Time,
) ([]planning.Plan, error) {
	request, options := buildAIRequest(input, season, now)
	if len(options) == 0 {
		return nil, fmt.Errorf("tidak ada kandidat untuk dikirim ke layanan AI")
	}

	fingerprint := aiclient.Fingerprint(request)
	if response, found := u.cachedPlan(ctx, fingerprint); found {
		return translateAIPlans(response, input, options)
	}

	response, err := u.AI.Propose(ctx, request)
	if err != nil {
		return nil, err
	}

	u.cachePlan(ctx, fingerprint, response)
	return translateAIPlans(response, input, options)
}

func buildAIRequest(
	input planning.Input, season planning.Season, now time.Time,
) (aiclient.Request, map[string]planning.Assignment) {
	table := aiclient.NewRefTable()
	options := map[string]planning.Assignment{}
	candidates := []aiclient.Candidate{}

	for _, plot := range input.Plots {
		for _, option := range plot.Options {
			id := fmt.Sprintf("c%03d", len(candidates)+1)
			options[id] = option

			candidates = append(candidates, aiclient.Candidate{
				ID:           id,
				PlotRef:      table.Plot(option.PlotID),
				AreaHa:       option.AreaHa,
				CommodityRef: table.Commodity(option.CommodityID),
				VarietyRef:   table.Variety(option.VarietyID),
				PlantingDate: agronomy.ToISODate(option.PlantingDate),
				HarvestStart: agronomy.ToISODate(option.Window.Start),
				HarvestEnd:   agronomy.ToISODate(option.Window.End),
				TonnesLow:    option.TonnesLow,
				TonnesMid:    option.TonnesMid,
				TonnesHigh:   option.TonnesHigh,
				Plausibility: contractPlausibility(option.Plausibility),
				PricePerKg:   priceOf(input.PricePerKg, option.CommodityID),
			})
		}
	}

	demand := make([]aiclient.DemandRow, 0, len(input.Demand))
	for _, row := range input.Demand {
		demand = append(demand, aiclient.DemandRow{
			CommodityRef: table.Commodity(row.CommodityID),
			ISOWeek:      agronomy.ToISODate(row.WeekStart),
			Kg:           int64(math.Round(row.Kg)),
		})
	}

	return aiclient.Request{
		ContractVersion: constants.AIContractVersion,
		RequestID:       uuid.NewString(),
		Seed:            agronomy.StartOfDay(now).Unix() / constants.AISeedSecondsPerDay,
		Season: aiclient.Season{
			Label: season.Label,
			Start: agronomy.ToISODate(season.Start),
			End:   agronomy.ToISODate(season.End),
		},
		Objectives: []string{
			string(constants.ObjectiveSafe),
			string(constants.ObjectiveIncome),
			string(constants.ObjectiveMarket),
		},
		CapacityTonnesPerWeek: soleCapacity(input.Capacity),
		Candidates:            candidates,
		Demand:                demand,
	}, options
}

func translateAIPlans(
	response *aiclient.Response, input planning.Input,
	options map[string]planning.Assignment,
) ([]planning.Plan, error) {
	if len(response.Plans) == 0 {
		return nil, fmt.Errorf("layanan AI mengembalikan nol rencana")
	}

	plans := make([]planning.Plan, 0, len(response.Plans))

	for _, result := range response.Plans {
		assignments := make([]planning.Assignment, 0, len(result.CandidateIDs))
		claimed := map[string]bool{}

		for _, id := range result.CandidateIDs {
			option, known := options[id]
			if !known || claimed[option.PlotID] {
				continue
			}
			claimed[option.PlotID] = true
			assignments = append(assignments, option)
		}

		if len(assignments) == 0 {
			return nil, fmt.Errorf("rencana %q tidak memuat satu pun kandidat yang dikenal",
				result.Objective)
		}

		plans = append(plans, planning.Plan{
			Objective:   constants.PlanningObjective(result.Objective),
			Assignments: assignments,
			Metrics:     planning.Measure(assignments, input.PricePerKg, input.Demand),
			Flagged: agronomy.DetectCollisions(
				planning.Projections(assignments), input.Capacity).Flagged,
			Evaluations: response.Diagnostics.Evaluations,
			Narrative:   result.Narrative,
		})
	}

	return plans, nil
}

func contractPlausibility(plausibility constants.Plausibility) string {
	if plausibility == constants.PlausibilityOk {
		return constants.AIPlausible
	}
	return string(plausibility)
}

func priceOf(prices map[string]float64, commodityID string) *float64 {
	price, published := prices[commodityID]
	if !published {
		return nil
	}
	return &price
}

func soleCapacity(capacity map[string]float64) *float64 {
	if len(capacity) != 1 {
		return nil
	}
	for _, tonnes := range capacity {
		return &tonnes
	}
	return nil
}

func (u *PlanningUseCase) cachedPlan(
	ctx context.Context, fingerprint string,
) (*aiclient.Response, bool) {
	if u.Redis == nil || fingerprint == "" {
		return nil, false
	}

	raw, err := u.Redis.Get(ctx, constants.AIPlanCacheKey+fingerprint).Bytes()
	if err != nil {
		return nil, false
	}

	var response aiclient.Response
	if err := json.Unmarshal(raw, &response); err != nil {
		return nil, false
	}
	return &response, true
}

func (u *PlanningUseCase) cachePlan(
	ctx context.Context, fingerprint string, response *aiclient.Response,
) {
	if u.Redis == nil || fingerprint == "" {
		return
	}

	raw, err := json.Marshal(response)
	if err != nil {
		return
	}

	if err := u.Redis.Set(
		ctx, constants.AIPlanCacheKey+fingerprint, raw, constants.AIPlanCacheTTL,
	).Err(); err != nil {
		u.Log.WithError(err).Warn("gagal menyimpan cache rencana")
	}
}
