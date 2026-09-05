package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/go-playground/validator/v10"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/config"
	"terrion-backend/internal/planning"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/usecase"
	"terrion-backend/internal/weather"
)

func main() {
	cooperativeID := flag.String("cooperative", "", "cooperative id")
	seasonLabel := flag.String("season", "", "season label, e.g. \"MT I 2026/2027\"")
	flag.Parse()

	if *cooperativeID == "" || *seasonLabel == "" {
		flag.Usage()
		os.Exit(2)
	}

	cfg := config.NewConfig()
	log := config.NewLogger(cfg)
	db := config.NewDatabase(cfg, log)

	weatherUseCase := usecase.NewWeatherUseCase(
		db, log, &repository.WeatherRepository{}, weather.NewClient())
	projection := usecase.NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	planner := usecase.NewPlanningUseCase(db, log, validator.New(),
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.MemberRepository{}, &repository.CommodityRepository{},
		&repository.VarietyRepository{}, &repository.CooperativeRepository{},
		&repository.ReferencePriceRepository{}, &repository.SupplyRequestRepository{},
		&repository.SeasonPlanRepository{}, projection, weatherUseCase, nil)

	proposal, err := planner.Propose(
		context.Background(), *cooperativeID, *seasonLabel, timeNow())
	if err != nil {
		log.Fatalf("proposing a plan: %v", err)
	}

	report(proposal)
}

func timeNow() time.Time {
	return time.Now().UTC()
}

func report(proposal usecase.Proposal) {
	fmt.Printf("Musim   : %s (%s .. %s)\n", proposal.Season.Label,
		agronomy.ToISODate(proposal.Season.Start),
		agronomy.ToISODate(proposal.Season.End))
	fmt.Printf("Panen tercatat yang melatih model: %d\n", proposal.YieldObservations)
	fmt.Printf("Lahan dilewati: %d\n\n", len(proposal.Skipped))

	for _, plan := range proposal.Plans {
		fmt.Printf("== Rencana %s ==\n", plan.Objective)
		fmt.Printf("  Puncak harapan   : %.2f ton/minggu\n", plan.Metrics.PeakTonnesExpected)
		fmt.Printf("  Puncak terburuk  : %.2f ton/minggu\n", plan.Metrics.PeakTonnesWorst)
		fmt.Printf("  Total proyeksi   : %.2f ton\n", plan.Metrics.TotalTonnesMid)
		if plan.Metrics.GrossValue == nil {
			fmt.Printf("  Nilai panen      : -\n")
		} else {
			fmt.Printf("  Nilai panen      : Rp %.0f\n", *plan.Metrics.GrossValue)
		}
		fmt.Printf("  Permintaan ditutup: %.0f kg\n", plan.Metrics.DemandCoveredKg)
		fmt.Printf("  Minggu tertandai : %d\n", len(plan.Flagged))
		fmt.Printf("  Evaluasi         : %d\n", plan.Evaluations)
		printAssignments(plan)
		fmt.Println()
	}
}

func printAssignments(plan planning.Plan) {
	for _, assignment := range plan.Assignments {
		fmt.Printf("  %-24s %-18s %5.2f ha  %-12s tanam %s  panen %s..%s  %.2f-%.2f t\n",
			assignment.MemberName, assignment.PlotName, assignment.AreaHa,
			assignment.VarietyName,
			agronomy.ToISODate(assignment.PlantingDate),
			agronomy.ToISODate(assignment.Window.Start),
			agronomy.ToISODate(assignment.Window.End),
			assignment.TonnesLow, assignment.TonnesHigh)
	}
}
