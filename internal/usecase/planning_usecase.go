package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
	"terrion-backend/internal/rdkk"
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

// ProposedPlan adalah rencana apa adanya dari optimizer, ditambah kebutuhan
// pupuk yang menyertainya. Perhitungan pupuknya tinggal di sini dan bukan di
// paket planning supaya paket itu tetap tidak tahu-menahu soal RDKK.
type ProposedPlan struct {
	planning.Plan
	Fertiliser rdkk.Aggregate
}

type Proposal struct {
	Season planning.Season
	// Musim sejenis setahun lalu, atau nil kalau koperasi ini belum punya
	// riwayatnya. nil berarti "belum ada pembandingnya", bukan "nol ton".
	PreviousSeason    *planning.SeasonSummary
	Plans             []ProposedPlan
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
	FertiliserRateRepository *repository.FertiliserRateRepository
	PlanShareTokenRepository *repository.PlanShareTokenRepository
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
	fertiliserRateRepository *repository.FertiliserRateRepository,
	planShareTokenRepository *repository.PlanShareTokenRepository,
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
		FertiliserRateRepository: fertiliserRateRepository,
		PlanShareTokenRepository: planShareTokenRepository,
		Projection:               projection,
		Weather:                  weatherUseCase,
		Catalog:                  catalog,
		AI:                       ai,
		Redis:                    cache,
	}
}

// Propose menyusun tiga rencana calon untuk satu musim.
//
// `goal` adalah kalimat bebas pengurus ("musim depan jangan menumpuk", "utamakan
// pabrik yang tahun lalu kami tolak"). Ia diteruskan apa adanya ke layanan AI,
// yang menerjemahkannya menjadi bobot solver — dan tidak pernah menjadi angka.
// Kosong berarti bobot bawaan dan nol panggilan model.
func (u *PlanningUseCase) Propose(
	ctx context.Context, cooperativeID, seasonLabel, goal string, now time.Time,
) (Proposal, error) {
	goal, err := trimmedGoal(goal)
	if err != nil {
		return Proposal{}, err
	}

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

	normals, err := u.normalsFor(ctx, projection.Plots)
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
	plans, engine := u.solve(ctx, input, season, goal, now)

	proposed, err := u.withFertiliser(ctx, plans)
	if err != nil {
		return Proposal{}, err
	}

	previous := planning.PreviousSeason(season)

	return Proposal{
		Season: season,
		PreviousSeason: planning.SummariseSeason(
			projection.Projections, previous, previous.Label),
		Plans:             proposed,
		Skipped:           skipped,
		YieldObservations: projection.Yield.NObservations,
		Engine:            engine,
	}, nil
}

// withFertiliser melengkapi tiap rencana dengan kebutuhan pupuknya.
//
// Inilah yang membuat janji "RDKK terbit sebelum musim" berdiri: angka pupuk
// dan penandaan batas subsidi 2 ha dihitung dari rencana, bukan menunggu
// benihnya masuk tanah. Tarifnya dibaca sekali untuk ketiga rencana — ia sama
// untuk semuanya, dan tiga kueri yang identik hanya membebani anggaran waktu.
func (u *PlanningUseCase) withFertiliser(
	ctx context.Context, plans []planning.Plan,
) ([]ProposedPlan, error) {
	rates, err := u.fertiliserRates(ctx)
	if err != nil {
		return nil, err
	}

	proposed := make([]ProposedPlan, len(plans))
	for i, plan := range plans {
		planted := make([]rdkk.PlantedBlock, len(plan.Assignments))
		for j, assignment := range plan.Assignments {
			planted[j] = rdkk.PlantedBlock{
				BlockID:     assignment.PlotID,
				MemberID:    assignment.MemberID,
				MemberName:  assignment.MemberName,
				CommodityID: assignment.CommodityID,
				AreaHa:      assignment.AreaHa,
			}
		}
		proposed[i] = ProposedPlan{
			Plan:       plan,
			Fertiliser: rdkk.AggregateInputs(planted, rates),
		}
	}
	return proposed, nil
}

func (u *PlanningUseCase) fertiliserRates(ctx context.Context) ([]rdkk.FertiliserRate, error) {
	if u.FertiliserRateRepository == nil {
		return nil, nil
	}

	rows, err := u.FertiliserRateRepository.FindAll(u.DB.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("reading fertiliser rates: %w", err)
	}

	rates := make([]rdkk.FertiliserRate, len(rows))
	for i, row := range rows {
		rates[i] = rdkk.FertiliserRate{
			CommodityID: row.CommodityID,
			InputItem:   row.InputItem,
			KgPerHa:     row.KgPerHa,
			Source:      row.Source,
		}
	}
	return rates, nil
}

func (u *PlanningUseCase) normalsFor(
	ctx context.Context, plots []entity.Plot,
) (map[weather.GridCell][]agronomy.ClimateNormal, error) {
	cells := []weather.GridCell{}
	seen := map[weather.GridCell]bool{}
	for _, plot := range plots {
		cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
		if seen[cell] {
			continue
		}
		seen[cell] = true
		cells = append(cells, cell)
	}

	stored, err := u.Weather.Repository.FindNormalsForCells(u.DB.WithContext(ctx), cells)
	if err != nil {
		return nil, fmt.Errorf("reading climate normals of %d cells: %w", len(cells), err)
	}

	normals := make(map[weather.GridCell][]agronomy.ClimateNormal, len(cells))
	for _, cell := range cells {
		rows := stored[cell]
		if len(rows) == 0 {
			return nil, &PlanRefusal{Code: constants.PlanNoClimateNormals}
		}

		cellNormals := make([]agronomy.ClimateNormal, len(rows))
		for i, row := range rows {
			cellNormals[i] = agronomy.ClimateNormal{
				DayOfYear: row.DayOfYear,
				MeanC:     row.MeanC,
				SdC:       row.SdC,
			}
		}
		normals[cell] = cellNormals
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

	normals, err := u.normalsFor(ctx, projection.Plots)
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
	plotIDs := make([]string, 0, len(assignments))
	seen := map[string]bool{}
	for _, assignment := range assignments {
		if seen[assignment.PlotID] {
			continue
		}
		seen[assignment.PlotID] = true
		plotIDs = append(plotIDs, assignment.PlotID)
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.SeasonPlanRepository.Create(tx, plan); err != nil {
		return fmt.Errorf("creating season plan for %s: %w", plan.CooperativeID, err)
	}

	nextIndex, err := u.BlockRepository.NextOrderIndexes(tx, plotIDs)
	if err != nil {
		return fmt.Errorf("reading block order of plan %s: %w", plan.ID, err)
	}

	blocks := make([]entity.Block, 0, len(assignments))
	items := make([]entity.SeasonPlanItem, 0, len(assignments))
	tokens := make([]entity.PlanShareToken, 0, len(assignments))
	seenMember := map[string]bool{}
	for _, assignment := range assignments {
		order := nextIndex[assignment.PlotID]
		nextIndex[assignment.PlotID] = order + 1

		planID := plan.ID
		blockID := uuid.NewString()
		blocks = append(blocks, entity.Block{
			ID:           blockID,
			PlotID:       assignment.PlotID,
			Label:        plots.BlockLabel(order),
			AreaHa:       assignment.AreaHa,
			OrderIndex:   order,
			CommodityID:  assignment.CommodityID,
			VarietyID:    assignment.VarietyID,
			PlantingDate: assignment.PlantingDate,
			SeasonPlanID: &planID,
		})
		items = append(items, entity.SeasonPlanItem{
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
		})

		if !seenMember[assignment.MemberID] {
			seenMember[assignment.MemberID] = true
			tokens = append(tokens, entity.PlanShareToken{
				ID:       uuid.NewString(),
				PlanID:   plan.ID,
				MemberID: assignment.MemberID,
			})
		}
	}

	if len(blocks) > 0 {
		if err := tx.Create(&blocks).Error; err != nil {
			return fmt.Errorf("creating plan blocks of %s: %w", plan.ID, err)
		}
		if err := tx.Create(&items).Error; err != nil {
			return fmt.Errorf("creating plan items of %s: %w", plan.ID, err)
		}
		if err := tx.Create(&tokens).Error; err != nil {
			return fmt.Errorf("creating share tokens of %s: %w", plan.ID, err)
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
	ShareTokens    []entity.PlanShareToken
}

var ErrPlanShareNotFound = errors.New("plan share not found")

type MemberPlanShareItem struct {
	PlotName      string
	CommodityName string
	VarietyName   string
	PlantingDate  time.Time
	HarvestStart  time.Time
	HarvestEnd    time.Time
	AreaHa        float64
	TonnesLow     float64
	TonnesMid     float64
	TonnesHigh    float64
	Plausibility  string
}

type MemberSubsidyCap struct {
	PlantedHa float64
	ExcessHa  float64
}

type MemberPlanShare struct {
	MemberName      string
	CooperativeName string
	SeasonLabel     string
	PlanStatus      string
	Items           []MemberPlanShareItem
	Fertiliser      []rdkk.RequirementLine
	OverSubsidyCap  *MemberSubsidyCap
}

func (u *PlanningUseCase) ViewShare(
	ctx context.Context, token string, now time.Time,
) (MemberPlanShare, error) {
	db := u.DB.WithContext(ctx)

	share := new(entity.PlanShareToken)
	if err := u.PlanShareTokenRepository.FindById(db, share, token); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return MemberPlanShare{}, ErrPlanShareNotFound
		}
		return MemberPlanShare{}, fmt.Errorf("reading plan share token %s: %w", token, err)
	}

	plan := new(entity.SeasonPlan)
	if err := u.SeasonPlanRepository.FindById(db, plan, share.PlanID); err != nil {
		return MemberPlanShare{}, fmt.Errorf("reading plan %s: %w", share.PlanID, err)
	}

	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(db, cooperative, plan.CooperativeID); err != nil {
		return MemberPlanShare{}, fmt.Errorf("reading cooperative %s: %w", plan.CooperativeID, err)
	}

	member := new(entity.Member)
	if err := u.MemberRepository.FindById(db, member, share.MemberID); err != nil {
		return MemberPlanShare{}, fmt.Errorf("reading member %s: %w", share.MemberID, err)
	}

	items, err := u.SeasonPlanRepository.FindItemsByPlanID(db, plan.ID)
	if err != nil {
		return MemberPlanShare{}, fmt.Errorf("reading items of plan %s: %w", plan.ID, err)
	}

	commodityNames, varietyNames, err := u.catalogueNames(db, items)
	if err != nil {
		return MemberPlanShare{}, err
	}
	plotNames, err := u.plotNames(db, plan.CooperativeID)
	if err != nil {
		return MemberPlanShare{}, err
	}

	memberItems := []MemberPlanShareItem{}
	planted := []rdkk.PlantedBlock{}
	for _, item := range items {
		if item.MemberID != share.MemberID {
			continue
		}
		memberItems = append(memberItems, MemberPlanShareItem{
			PlotName:      plotNames[item.PlotID],
			CommodityName: commodityNames[item.CommodityID],
			VarietyName:   varietyNames[item.VarietyID],
			PlantingDate:  item.PlantingDate,
			HarvestStart:  item.ExpectedHarvestStart,
			HarvestEnd:    item.ExpectedHarvestEnd,
			AreaHa:        item.AreaHa,
			TonnesLow:     item.ExpectedTonnesLow,
			TonnesMid:     item.ExpectedTonnesMid,
			TonnesHigh:    item.ExpectedTonnesHigh,
			Plausibility:  item.Plausibility,
		})
		planted = append(planted, rdkk.PlantedBlock{
			BlockID:     item.ID,
			MemberID:    item.MemberID,
			MemberName:  member.Name,
			CommodityID: item.CommodityID,
			AreaHa:      item.AreaHa,
		})
	}

	rates, err := u.fertiliserRates(ctx)
	if err != nil {
		return MemberPlanShare{}, err
	}
	aggregate := rdkk.AggregateInputs(planted, rates)

	result := MemberPlanShare{
		MemberName:      member.Name,
		CooperativeName: cooperative.Name,
		SeasonLabel:     plan.SeasonLabel,
		PlanStatus:      string(plan.Status),
		Items:           memberItems,
		Fertiliser:      []rdkk.RequirementLine{},
	}
	if len(aggregate.Members) > 0 {
		result.Fertiliser = aggregate.Members[0].Lines
		if aggregate.Members[0].OverSubsidyCap {
			result.OverSubsidyCap = &MemberSubsidyCap{
				PlantedHa: aggregate.Members[0].PlantedHa,
				ExcessHa:  aggregate.Members[0].ExcessHa,
			}
		}
	}

	if err := u.PlanShareTokenRepository.MarkViewed(db, token, now); err != nil {
		return MemberPlanShare{}, fmt.Errorf("marking plan share %s viewed: %w", token, err)
	}

	return result, nil
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

	shareTokens, err := u.PlanShareTokenRepository.FindByPlanID(db, plan.ID)
	if err != nil {
		return StoredPlan{}, fmt.Errorf("reading share tokens of plan %s: %w", plan.ID, err)
	}

	return StoredPlan{
		Plan:           *plan,
		Items:          items,
		MemberNames:    memberNames,
		PlotNames:      plotNames,
		CommodityNames: commodityNames,
		VarietyNames:   varietyNames,
		ShareTokens:    shareTokens,
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

// trimmedGoal merapikan tujuan pengurus, atau menolaknya kalau kepanjangan.
//
// Batasnya ditegakkan di sini, bukan dibiarkan menjadi penolakan kontrak dari
// layanan AI:
// satu kalimat kepanjangan tidak boleh membuat pengurus kehilangan seluruh
// rencananya. Panjangnya dihitung dalam aksara, sama seperti kontrak Python.
func trimmedGoal(goal string) (string, error) {
	goal = strings.TrimSpace(goal)
	if utf8.RuneCountInString(goal) > constants.PlanGoalMaxChars {
		return "", &PlanRefusal{Code: constants.PlanGoalTooLong}
	}
	return goal, nil
}

func (u *PlanningUseCase) solve(
	ctx context.Context, input planning.Input, season planning.Season,
	goal string, now time.Time,
) ([]planning.Plan, constants.PlanEngine) {
	if u.AI == nil {
		return planning.Search(input), constants.PlanEngineFallback
	}

	plans, err := u.askAIService(ctx, input, season, goal, now)
	if err == nil {
		return plans, constants.PlanEngineAIService
	}

	if !errors.Is(err, aiclient.ErrBreakerOpen) {
		u.Log.WithError(err).Warn("layanan AI tidak terpakai, memakai solver lokal")
	}
	return planning.Search(input), constants.PlanEngineFallback
}

func (u *PlanningUseCase) askAIService(
	ctx context.Context, input planning.Input, season planning.Season,
	goal string, now time.Time,
) ([]planning.Plan, error) {
	request, options := buildAIRequest(input, season, goal, now)
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
	input planning.Input, season planning.Season, goal string, now time.Time,
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
		Goal:                  goal,
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

		projections := planning.Projections(assignments)
		plans = append(plans, planning.Plan{
			Objective:   constants.PlanningObjective(result.Objective),
			Assignments: assignments,
			Metrics:     planning.Measure(assignments, input.PricePerKg, input.Demand),
			Flagged:     agronomy.DetectCollisions(projections, input.Capacity).Flagged,
			Thresholds:  agronomy.ThresholdsFor(projections, input.Capacity),
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
