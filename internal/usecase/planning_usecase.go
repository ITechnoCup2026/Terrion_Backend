package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/aiclient"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

const (
	EngineAIService = "ai-service"
	EngineFallback  = "fallback"

	planStatusApplied   = "applied"
	planStatusCancelled = "cancelled"
)

var (
	ErrNoPlots       = errors.New("planning_no_plots")
	ErrNoCandidates  = errors.New("planning_no_candidates")
	ErrSeasonInvalid = errors.New("planning_season_invalid")
	ErrPlanNotFound  = errors.New("planning_plan_not_found")
)

var defaultObjectives = []planning.Objective{
	planning.ObjectiveSafe, planning.ObjectiveIncome, planning.ObjectiveMarket,
}

type PlanningUseCase struct {
	DB                       *gorm.DB
	Log                      *logrus.Logger
	Validate                 *validator.Validate
	PlotRepository           *repository.PlotRepository
	BlockRepository          *repository.BlockRepository
	VarietyRepository        *repository.VarietyRepository
	ReferencePriceRepository *repository.ReferencePriceRepository
	CooperativeRepository    *repository.CooperativeRepository
	SupplyRequestRepository  *repository.SupplyRequestRepository
	SeasonPlanRepository     *repository.SeasonPlanRepository
	Projection               *ProjectionUseCase
	AI                       *aiclient.Client
}

func NewPlanningUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	varietyRepository *repository.VarietyRepository,
	referencePriceRepository *repository.ReferencePriceRepository,
	cooperativeRepository *repository.CooperativeRepository,
	supplyRequestRepository *repository.SupplyRequestRepository,
	seasonPlanRepository *repository.SeasonPlanRepository,
	projection *ProjectionUseCase,
	ai *aiclient.Client,
) *PlanningUseCase {
	return &PlanningUseCase{
		DB: db, Log: log, Validate: validate,
		PlotRepository: plotRepository, BlockRepository: blockRepository,
		VarietyRepository:        varietyRepository,
		ReferencePriceRepository: referencePriceRepository,
		CooperativeRepository:    cooperativeRepository,
		SupplyRequestRepository:  supplyRequestRepository,
		SeasonPlanRepository:     seasonPlanRepository,
		Projection:               projection, AI: ai,
	}
}

type planningInputs struct {
	season      planning.Season
	plots       []entity.Plot
	plotNames   map[string]string
	candidates  []planning.Candidate
	demand      []planning.DemandRow
	capacity    *float64
	priceSource *string
}

func (u *PlanningUseCase) Propose(
	ctx context.Context, cooperativeID string, request *model.ProposePlanRequest, now time.Time,
) (*model.ProposePlanResponse, error) {
	if err := u.Validate.Struct(request); err != nil {
		return nil, err
	}

	inputs, err := u.gather(ctx, cooperativeID, request, now)
	if err != nil {
		return nil, err
	}

	objectives := requestedObjectives(request.Objectives)
	problem := planning.NewProblem(inputs.candidates, inputs.demand, inputs.capacity)

	plans, engine, diagnostics := u.solve(ctx, problem, inputs, objectives)

	return &model.ProposePlanResponse{
		Season: model.SeasonResponse{
			Label: inputs.season.Label,
			Start: agronomy.ToISODate(inputs.season.Start),
			End:   agronomy.ToISODate(inputs.season.End),
		},
		Engine:      engine,
		Plans:       plans,
		Diagnostics: diagnostics,
	}, nil
}

func requestedObjectives(requested []string) []planning.Objective {
	if len(requested) == 0 {
		return defaultObjectives
	}
	objectives := make([]planning.Objective, 0, len(requested))
	for _, name := range requested {
		objectives = append(objectives, planning.Objective(name))
	}
	return objectives
}

func (u *PlanningUseCase) solve(
	ctx context.Context,
	problem *planning.Problem,
	inputs planningInputs,
	objectives []planning.Objective,
) ([]model.PlanResponse, string, model.PlanDiagnosticsResponse) {
	diagnostics := model.PlanDiagnosticsResponse{
		CandidateCount: len(inputs.candidates),
		Degraded:       []string{},
	}

	if u.AI.Enabled() {
		plans, degraded, evaluations, err := u.solveWithAI(ctx, problem, inputs, objectives)
		if err == nil {
			diagnostics.Evaluations = evaluations
			diagnostics.Degraded = degraded
			return plans, EngineAIService, diagnostics
		}
		u.Log.WithField("reason", err.Error()).Warn("planning_ai_fallback")
		diagnostics.Degraded = append(diagnostics.Degraded, "ai:"+err.Error())
	}

	searched, evaluations := planning.Search(problem, objectives)
	diagnostics.Evaluations = evaluations

	plans := make([]model.PlanResponse, 0, len(searched))
	for _, plan := range searched {
		plans = append(plans, u.describe(problem, inputs, plan.Objective, plan.CandidateIDs, nil, nil, nil, "none"))
	}
	return plans, EngineFallback, diagnostics
}

func (u *PlanningUseCase) solveWithAI(
	ctx context.Context,
	problem *planning.Problem,
	inputs planningInputs,
	objectives []planning.Objective,
) ([]model.PlanResponse, []string, int, error) {
	names := make([]string, 0, len(objectives))
	for _, objective := range objectives {
		names = append(names, string(objective))
	}

	request, mapping := aiclient.BuildRequest(aiclient.RequestInput{
		RequestID:  uuid.NewString(),
		Seed:       inputs.season.Start.Unix(),
		Season:     inputs.season,
		Objectives: objectives,
		Candidates: inputs.candidates,
		Demand:     inputs.demand,
		Capacity:   inputs.capacity,
	})

	response, err := u.AI.Propose(ctx, request)
	if err != nil {
		return nil, nil, 0, err
	}
	if err := aiclient.ValidatePlans(response, mapping, names); err != nil {
		return nil, nil, 0, err
	}

	plans := make([]model.PlanResponse, 0, len(response.Plans))
	for _, plan := range response.Plans {
		p50, p90 := trustedQuantiles(problem, plan, inputs.capacity)
		plans = append(plans, u.describe(
			problem, inputs, planning.Objective(plan.Objective), plan.CandidateIDs,
			p50, p90, plan.Narrative, plan.NarrativeSource,
		))
	}

	return plans, response.Diagnostics.Degraded, response.Diagnostics.Evaluations, nil
}

// The risk quantiles are the only numbers Go cannot recompute: they need the
// Monte Carlo that lives in the AI service. They are accepted only when they
// are internally consistent and bounded by figures Go derived itself, and
// dropped to nil otherwise. A plan is still returned either way.
func trustedQuantiles(
	problem *planning.Problem, plan aiclient.PlanResult, capacity *float64,
) (*float64, *float64) {
	chosen := problem.Select(plan.CandidateIDs)
	if len(chosen) == 0 {
		return nil, nil
	}

	own := problem.Measure(chosen, false)
	ceiling := problem.Measure(chosen, true).Peak
	p50, p90 := plan.Metrics.PeakTonnesP50, plan.Metrics.PeakTonnesP90

	if p50 <= 0 || p90 < p50 || p90 > ceiling*1.5 || p50 > own.TotalTonnes {
		return nil, nil
	}
	return &p50, &p90
}

func (u *PlanningUseCase) describe(
	problem *planning.Problem,
	inputs planningInputs,
	objective planning.Objective,
	candidateIDs []string,
	p50, p90 *float64,
	narrative *string,
	narrativeSource string,
) model.PlanResponse {
	chosen := problem.Select(candidateIDs)
	expected := problem.Measure(chosen, false)
	worst := problem.Measure(chosen, true)

	assignments := make([]model.PlanAssignmentResponse, 0, len(chosen))
	for _, candidate := range chosen {
		assignments = append(assignments, model.PlanAssignmentResponse{
			CandidateID:  candidate.ID,
			PlotID:       candidate.PlotID,
			PlotName:     inputs.plotNames[candidate.PlotID],
			CommodityID:  candidate.CommodityID,
			VarietyID:    candidate.VarietyID,
			AreaHa:       candidate.AreaHa,
			PlantingDate: agronomy.ToISODate(candidate.PlantingDate),
			HarvestStart: agronomy.ToISODate(candidate.HarvestStart),
			HarvestEnd:   agronomy.ToISODate(candidate.HarvestEnd),
			TonnesLow:    candidate.TonnesLow,
			TonnesMid:    candidate.TonnesMid,
			TonnesHigh:   candidate.TonnesHigh,
			Plausibility: string(candidate.Plausibility),
		})
	}

	source := inputs.priceSource
	if expected.GrossValue == nil {
		source = nil
	}

	return model.PlanResponse{
		Objective:   string(objective),
		Assignments: assignments,
		Metrics: model.PlanMetricsResponse{
			ExpectedPeakTonnes:    expected.Peak,
			WorstCasePeakTonnes:   worst.Peak,
			PeakTonnesP50:         p50,
			PeakTonnesP90:         p90,
			TotalTonnes:           expected.TotalTonnes,
			GrossValue:            expected.GrossValue,
			GrossValueSource:      source,
			DemandCoveredKg:       expected.CoverageKg,
			CapacityTonnesPerWeek: inputs.capacity,
		},
		Narrative:       narrative,
		NarrativeSource: narrativeSource,
	}
}

func (u *PlanningUseCase) gather(
	ctx context.Context, cooperativeID string, request *model.ProposePlanRequest, now time.Time,
) (planningInputs, error) {
	db := u.DB.WithContext(ctx)

	start, err := agronomy.UTCDate(request.SeasonStart)
	if err != nil {
		return planningInputs{}, ErrSeasonInvalid
	}
	end, err := agronomy.UTCDate(request.SeasonEnd)
	if err != nil || !end.After(start) {
		return planningInputs{}, ErrSeasonInvalid
	}

	plots, err := u.PlotRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return planningInputs{}, fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}
	if len(plots) == 0 {
		return planningInputs{}, ErrNoPlots
	}

	varietyRows, err := u.VarietyRepository.FindAll(db)
	if err != nil {
		return planningInputs{}, fmt.Errorf("reading varieties: %w", err)
	}
	varieties := make([]planning.Variety, 0, len(varietyRows))
	byID := make(map[string]agronomy.Variety, len(varietyRows))
	for _, row := range varietyRows {
		reference := agronomy.Variety{
			GddRequirement:   row.GddRequirement,
			BaseTempC:        row.BaseTempC,
			DaysToHarvestMin: row.DaysToHarvestMin,
			DaysToHarvestMax: row.DaysToHarvestMax,
			YieldPerHaMin:    row.YieldPerHaMin,
			YieldPerHaMax:    row.YieldPerHaMax,
		}
		varieties = append(varieties, planning.Variety{
			ID: row.ID, CommodityID: row.CommodityID, Agronomy: reference,
		})
		byID[row.ID] = reference
	}

	blocks, err := u.BlockRepository.FindByPlotIDs(db, plotIDsOf(plots))
	if err != nil {
		return planningInputs{}, fmt.Errorf("reading blocks of cooperative %s: %w", cooperativeID, err)
	}

	since := start
	if planted := earliestPlanting(blocks); !planted.IsZero() && planted.Before(since) {
		since = planted
	}
	weatherByCell, err := u.Projection.weatherFor(ctx, plots, since, now)
	if err != nil {
		return planningInputs{}, err
	}

	cellOfPlot := make(map[string]weather.GridCell, len(plots))
	for _, plot := range plots {
		cellOfPlot[plot.ID] = weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
	}
	weatherOf := func(plotID string) (CellWeather, bool) {
		cell, known := cellOfPlot[plotID]
		if !known {
			return CellWeather{}, false
		}
		cellWeather, stored := weatherByCell[cell]
		return cellWeather, stored
	}

	yieldModel := agronomy.FitYieldModel(harvestObservations(blocks, byID, weatherOf))

	calibrations, err := u.Projection.calibrationsFor(db, cooperativeID)
	if err != nil {
		return planningInputs{}, err
	}
	fitted := make(map[string]agronomy.Calibration, len(calibrations))
	for varietyID, calibration := range calibrations {
		if calibration != nil {
			fitted[varietyID] = *calibration
		}
	}

	prices, priceSource, err := u.pricePanel(db, cooperativeID, varietyRows, start, end)
	if err != nil {
		return planningInputs{}, err
	}

	season := planning.Season{Label: request.SeasonLabel, Start: start, End: end}
	today := agronomy.ToISODate(now)

	groups := [][]planning.Candidate{}
	for cell, cellWeather := range weatherByCell {
		inCell := []planning.Plot{}
		for _, plot := range plots {
			if cellOfPlot[plot.ID] == cell {
				inCell = append(inCell, planning.Plot{ID: plot.ID, AreaHa: plot.AreaHa})
			}
		}
		if len(inCell) == 0 {
			continue
		}

		observed, forecast := splitOnToday(cellWeather.Observed, today)
		groups = append(groups, planning.BuildCandidates(planning.CandidateInput{
			Season:       season,
			Plots:        inCell,
			Varieties:    varieties,
			Observed:     observed,
			Forecast:     forecast,
			Climatology:  cellWeather.Normals,
			Calibrations: fitted,
			YieldModel:   yieldModel,
			PricePerKg:   prices,
		}))
	}

	candidates := planning.MergeCandidates(groups...)
	if len(candidates) == 0 {
		return planningInputs{}, ErrNoCandidates
	}

	capacity, err := u.capacityFor(db, cooperativeID)
	if err != nil {
		return planningInputs{}, err
	}

	demand, err := u.demandFor(db, cooperativeID, start, end)
	if err != nil {
		return planningInputs{}, err
	}

	plotNames := make(map[string]string, len(plots))
	for _, plot := range plots {
		plotNames[plot.ID] = plot.Name
	}

	return planningInputs{
		season: season, plots: plots, plotNames: plotNames,
		candidates: candidates, demand: demand, capacity: capacity, priceSource: priceSource,
	}, nil
}

// One reference price per commodity, averaged across the season.
//
// Deliberately not the price of the week a harvest window opens. The panel in
// the database is still a synthetic annual sine wave, and optimising against
// its weekly shape would be optimising the shape of a curve we invented. The
// panel's own source string travels back to the screen so nobody reads these
// rupiah as budgetable.
func (u *PlanningUseCase) pricePanel(
	db *gorm.DB, cooperativeID string, varieties []entity.Variety, start, end time.Time,
) (map[string]*float64, *string, error) {
	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(db, cooperative, cooperativeID); err != nil {
		return nil, nil, fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	commodityIDs := map[string]bool{}
	for _, variety := range varieties {
		commodityIDs[variety.CommodityID] = true
	}

	rows, err := u.ReferencePriceRepository.FindForCommodities(
		db, cooperative.Province, keysOf(commodityIDs))
	if err != nil {
		return nil, nil, fmt.Errorf("reading reference prices for %s: %w", cooperative.Province, err)
	}

	totals := map[string]float64{}
	counts := map[string]int{}
	var source *string
	for _, row := range rows {
		if row.WeekStart.Before(start) || row.WeekStart.After(end) {
			continue
		}
		totals[row.CommodityID] += row.PricePerKg
		counts[row.CommodityID]++
		if source == nil && row.Source != "" {
			value := row.Source
			source = &value
		}
	}

	prices := make(map[string]*float64, len(totals))
	for commodityID, total := range totals {
		mean := total / float64(counts[commodityID])
		prices[commodityID] = &mean
	}
	return prices, source, nil
}

func (u *PlanningUseCase) capacityFor(db *gorm.DB, cooperativeID string) (*float64, error) {
	rows, err := u.CooperativeRepository.FindCapacity(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading capacity of cooperative %s: %w", cooperativeID, err)
	}
	if len(rows) == 0 {
		return nil, nil
	}

	total := 0.0
	for _, row := range rows {
		total += row.TonnesPerWeek
	}
	return &total, nil
}

func (u *PlanningUseCase) demandFor(
	db *gorm.DB, cooperativeID string, start, end time.Time,
) ([]planning.DemandRow, error) {
	requests, err := u.SupplyRequestRepository.FindForCooperative(db, cooperativeID)
	if err != nil {
		return nil, fmt.Errorf("reading supply requests of cooperative %s: %w", cooperativeID, err)
	}

	demand := []planning.DemandRow{}
	for _, request := range requests {
		if request.Status != constants.RequestAccepted {
			continue
		}
		if request.WindowStart.After(end) || request.WindowEnd.Before(start) {
			continue
		}
		demand = append(demand, planning.DemandRow{
			CommodityID: request.CommodityID,
			Week:        request.WindowStart,
			Kg:          int(request.VolumeKg),
		})
	}
	return demand, nil
}

// Turns an approved proposal into real blocks.
//
// A proposal touches nothing until a human posts it back here. Applying a
// second plan for the same season cancels the first rather than stacking on
// top of it: the unique index allows one applied plan per cooperative per
// season, and blocks already harvested are never removed.
func (u *PlanningUseCase) Apply(
	ctx context.Context, cooperativeID string, request *model.ApplyPlanRequest,
) (*model.ApplyPlanResponse, error) {
	if err := u.Validate.Struct(request); err != nil {
		return nil, err
	}

	start, err := agronomy.UTCDate(request.SeasonStart)
	if err != nil {
		return nil, ErrSeasonInvalid
	}
	end, err := agronomy.UTCDate(request.SeasonEnd)
	if err != nil || !end.After(start) {
		return nil, ErrSeasonInvalid
	}

	response := &model.ApplyPlanResponse{
		Objective:   request.Objective,
		SeasonLabel: request.SeasonLabel,
		SeasonStart: request.SeasonStart,
	}

	err = u.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plots, err := u.PlotRepository.FindByCooperativeID(tx, cooperativeID)
		if err != nil {
			return fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
		}
		owned := make(map[string]bool, len(plots))
		for _, plot := range plots {
			owned[plot.ID] = true
		}
		for _, assignment := range request.Assignments {
			if !owned[assignment.PlotID] {
				return ErrPlanNotFound
			}
		}

		if existing, err := u.SeasonPlanRepository.FindActive(tx, cooperativeID, start); err == nil {
			if err := u.SeasonPlanRepository.DeleteBlocksOfPlan(tx, existing.ID); err != nil {
				return fmt.Errorf("clearing blocks of plan %s: %w", existing.ID, err)
			}
			if err := u.SeasonPlanRepository.Cancel(tx, existing.ID); err != nil {
				return fmt.Errorf("cancelling plan %s: %w", existing.ID, err)
			}
			response.ReplacedExisting = true
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("reading active plan of cooperative %s: %w", cooperativeID, err)
		}

		plan := &entity.SeasonPlan{
			ID:            uuid.NewString(),
			CooperativeID: cooperativeID,
			Label:         request.SeasonLabel,
			SeasonStart:   start,
			SeasonEnd:     end,
			Objective:     request.Objective,
			Engine:        request.Engine,
			Status:        planStatusApplied,
			CreatedAt:     time.Now().UTC(),
		}
		if err := u.SeasonPlanRepository.Create(tx, plan); err != nil {
			return fmt.Errorf("creating season plan: %w", err)
		}
		response.PlanID = plan.ID

		for _, assignment := range request.Assignments {
			plantingDate, err := agronomy.UTCDate(assignment.PlantingDate)
			if err != nil {
				return ErrSeasonInvalid
			}

			orderIndex, err := u.BlockRepository.NextOrderIndex(tx, assignment.PlotID)
			if err != nil {
				return fmt.Errorf("reading next order index for plot %s: %w", assignment.PlotID, err)
			}

			label := assignment.Label
			if label == "" {
				label = fmt.Sprintf("%s %s", request.SeasonLabel, request.Objective)
			}

			block := &entity.Block{
				ID:           uuid.NewString(),
				PlotID:       assignment.PlotID,
				Label:        label,
				AreaHa:       assignment.AreaHa,
				OrderIndex:   orderIndex,
				CommodityID:  assignment.CommodityID,
				VarietyID:    assignment.VarietyID,
				PlantingDate: plantingDate,
				SeasonPlanID: &plan.ID,
			}
			if err := u.BlockRepository.Create(tx, block); err != nil {
				return fmt.Errorf("creating block on plot %s: %w", assignment.PlotID, err)
			}
			response.BlocksCreated++
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// Withdraws an applied plan and removes the blocks it created.
//
// Blocks whose harvest was already recorded stay: a plan being cancelled does
// not unmake a harvest that happened.
func (u *PlanningUseCase) Cancel(ctx context.Context, cooperativeID, planID string) error {
	return u.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan := new(entity.SeasonPlan)
		if err := u.SeasonPlanRepository.FindById(tx, plan, planID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrPlanNotFound
			}
			return fmt.Errorf("reading plan %s: %w", planID, err)
		}
		if plan.CooperativeID != cooperativeID || plan.Status != planStatusApplied {
			return ErrPlanNotFound
		}

		if err := u.SeasonPlanRepository.DeleteBlocksOfPlan(tx, planID); err != nil {
			return fmt.Errorf("clearing blocks of plan %s: %w", planID, err)
		}
		return u.SeasonPlanRepository.Cancel(tx, planID)
	})
}

func (u *PlanningUseCase) List(
	ctx context.Context, cooperativeID string,
) ([]entity.SeasonPlan, error) {
	return u.SeasonPlanRepository.FindByCooperativeID(u.DB.WithContext(ctx), cooperativeID)
}
