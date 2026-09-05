package usecase

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
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
	catalog *CatalogUseCase,
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

	return Proposal{
		Season: season,
		Plans: planning.Search(planning.Input{
			Season:     season,
			Plots:      candidates,
			PricePerKg: prices,
			Demand:     demand,
			Capacity:   capacity,
		}),
		Skipped:           skipped,
		YieldObservations: projection.Yield.NObservations,
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
