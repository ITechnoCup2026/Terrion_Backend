package usecase

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

var (
	planningNow = time.Date(2026, 9, 5, 0, 0, 0, 0, time.UTC)

	riceVariety   = "55555555-5555-4555-8555-555555555555"
	riceCommodity = "66666666-6666-4666-8666-666666666666"
	maizeVariety  = "77777777-7777-4777-8777-777777777777"

	planPlotHome  = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	planPlotTwo   = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	planPlotOther = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

func planningDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("opening in-memory sqlite: %v", err)
	}
	pool, err := db.DB()
	if err != nil {
		t.Fatalf("reading sqlite connection pool: %v", err)
	}
	pool.SetMaxOpenConns(1)

	if err := db.AutoMigrate(
		&entity.Plot{}, &entity.Block{}, &entity.Variety{}, &entity.Calibration{},
		&entity.WeatherDaily{}, &entity.WeatherNormal{}, &entity.Cooperative{},
		&entity.CooperativeCapacity{}, &entity.ReferencePrice{},
		&entity.SupplyContractRequest{}, &entity.SeasonPlan{},
	); err != nil {
		t.Fatalf("migrating planning tables: %v", err)
	}
	return db
}

func seedPlanningFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db := planningDB(t)
	seedProjectionWeather(t, db, homeCell)
	seedPlot(t, db, planPlotHome, homeCoop, homeCell)
	seedPlot(t, db, planPlotTwo, homeCoop, homeCell)
	seedPlot(t, db, planPlotOther, otherCoop, homeCell)

	if err := db.Create(&entity.Cooperative{
		ID: homeCoop, Name: "KUD Uji", Village: "Desa", District: "Kec",
		Province: "Jawa Barat", CreatedAt: planningNow,
	}).Error; err != nil {
		t.Fatalf("seeding cooperative: %v", err)
	}

	varieties := []entity.Variety{
		{
			ID: maizeVariety, CommodityID: maizeCommodity, Name: "Bisi-18",
			GddRequirement: 1400, BaseTempC: 10,
			DaysToHarvestMin: 90, DaysToHarvestMax: 110,
			YieldPerHaMin: 7, YieldPerHaMax: 9.5,
		},
		{
			ID: riceVariety, CommodityID: riceCommodity, Name: "Inpari-32",
			GddRequirement: 1650, BaseTempC: 12,
			DaysToHarvestMin: 100, DaysToHarvestMax: 120,
			YieldPerHaMin: 5, YieldPerHaMax: 7,
		},
	}
	if err := db.Create(&varieties).Error; err != nil {
		t.Fatalf("seeding varieties: %v", err)
	}

	if err := db.Create(&entity.CooperativeCapacity{
		CooperativeID: homeCoop, CommodityID: maizeCommodity, TonnesPerWeek: 6,
	}).Error; err != nil {
		t.Fatalf("seeding capacity: %v", err)
	}

	prices := []entity.ReferencePrice{}
	for week := 0; week < 40; week++ {
		prices = append(prices, entity.ReferencePrice{
			CommodityID: maizeCommodity, Province: "Jawa Barat",
			WeekStart:  agronomy.AddDays(planningNow, week*7),
			PricePerKg: 4100, Source: "SINTETIS — ganti dengan panel harga Badan Pangan Nasional",
		})
	}
	if err := db.CreateInBatches(prices, 100).Error; err != nil {
		t.Fatalf("seeding reference prices: %v", err)
	}

	return db
}

func planningUseCase(t *testing.T, db *gorm.DB) *PlanningUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewPlanningUseCase(db, log, validator.New(),
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.ReferencePriceRepository{},
		&repository.CooperativeRepository{}, &repository.SupplyRequestRepository{},
		&repository.SeasonPlanRepository{},
		projectionUseCase(t, db), nil)
}

func seasonRequest() *model.ProposePlanRequest {
	return &model.ProposePlanRequest{
		SeasonLabel: "MT I 2026/2027",
		SeasonStart: "2026-10-01",
		SeasonEnd:   "2027-03-31",
	}
}

func propose(t *testing.T, db *gorm.DB) *model.ProposePlanResponse {
	t.Helper()

	proposal, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, seasonRequest(), planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}
	return proposal
}

func TestProposeFallsBackToTheGoSolverWithoutAnAiService(t *testing.T) {
	proposal := propose(t, seedPlanningFixture(t))

	if proposal.Engine != EngineFallback {
		t.Fatalf("tanpa AI_SERVICE_URL mesinnya harus fallback, dapat %q", proposal.Engine)
	}
	if len(proposal.Plans) != 3 {
		t.Fatalf("harus tiga rencana, dapat %d", len(proposal.Plans))
	}
}

func TestProposeStoresNothing(t *testing.T) {
	db := seedPlanningFixture(t)

	propose(t, db)

	var blocks, plans int64
	db.Model(&entity.Block{}).Count(&blocks)
	db.Model(&entity.SeasonPlan{}).Count(&plans)

	if blocks != 0 || plans != 0 {
		t.Fatalf("usulan tidak boleh menyentuh basis data: %d blok, %d rencana", blocks, plans)
	}
}

func TestProposeOnlyEverPlansThisCooperativesLand(t *testing.T) {
	proposal := propose(t, seedPlanningFixture(t))

	for _, plan := range proposal.Plans {
		for _, assignment := range plan.Assignments {
			if assignment.PlotID == planPlotOther {
				t.Fatal("rencana memuat lahan koperasi lain")
			}
		}
	}
}

func TestProposePlantsEachPlotAtMostOnce(t *testing.T) {
	proposal := propose(t, seedPlanningFixture(t))

	for _, plan := range proposal.Plans {
		seen := map[string]bool{}
		for _, assignment := range plan.Assignments {
			if seen[assignment.PlotID] {
				t.Fatalf("%s menanami %s dua kali", plan.Objective, assignment.PlotID)
			}
			seen[assignment.PlotID] = true
		}
	}
}

func TestProposeReportsNoQuantilesWithoutTheAiService(t *testing.T) {
	proposal := propose(t, seedPlanningFixture(t))

	for _, plan := range proposal.Plans {
		if plan.Metrics.PeakTonnesP50 != nil || plan.Metrics.PeakTonnesP90 != nil {
			t.Fatalf("%s melaporkan kuantil Monte Carlo yang tidak pernah dihitung", plan.Objective)
		}
		if plan.Metrics.WorstCasePeakTonnes < plan.Metrics.ExpectedPeakTonnes {
			t.Fatalf("%s kasus terburuk di bawah kasus harapan", plan.Objective)
		}
	}
}

func TestProposeCarriesThePriceProvenanceToTheScreen(t *testing.T) {
	proposal := propose(t, seedPlanningFixture(t))

	for _, plan := range proposal.Plans {
		if plan.Metrics.GrossValue == nil {
			continue
		}
		if plan.Metrics.GrossValueSource == nil || *plan.Metrics.GrossValueSource == "" {
			t.Fatalf("%s melaporkan rupiah tanpa menyebut asal panel harganya", plan.Objective)
		}
	}
}

func TestProposeIsDeterministic(t *testing.T) {
	db := seedPlanningFixture(t)

	first := propose(t, db)
	second := propose(t, db)

	for i := range first.Plans {
		a := first.Plans[i].Assignments
		b := second.Plans[i].Assignments
		if len(a) != len(b) {
			t.Fatalf("%s berubah jumlah penugasannya antar dua panggilan", first.Plans[i].Objective)
		}
		for j := range a {
			if a[j].CandidateID != b[j].CandidateID {
				t.Fatalf("%s berubah antar dua panggilan pada posisi %d", first.Plans[i].Objective, j)
			}
		}
	}
}

func TestProposeRefusesACooperativeWithoutLand(t *testing.T) {
	db := planningDB(t)
	if err := db.Create(&entity.Cooperative{
		ID: homeCoop, Name: "Kosong", Province: "Jawa Barat", CreatedAt: planningNow,
	}).Error; err != nil {
		t.Fatalf("seeding cooperative: %v", err)
	}

	_, err := planningUseCase(t, db).
		Propose(context.Background(), homeCoop, seasonRequest(), planningNow)

	if !errors.Is(err, ErrNoPlots) {
		t.Fatalf("koperasi tanpa lahan harus ditolak dengan jelas, dapat %v", err)
	}
}

func TestProposeRefusesABackwardsSeason(t *testing.T) {
	backwards := seasonRequest()
	backwards.SeasonStart, backwards.SeasonEnd = backwards.SeasonEnd, backwards.SeasonStart

	_, err := planningUseCase(t, seedPlanningFixture(t)).
		Propose(context.Background(), homeCoop, backwards, planningNow)

	if !errors.Is(err, ErrSeasonInvalid) {
		t.Fatalf("musim terbalik harus ditolak, dapat %v", err)
	}
}

func applyRequestFrom(proposal *model.ProposePlanResponse) *model.ApplyPlanRequest {
	plan := proposal.Plans[0]
	assignments := make([]model.ApplyPlanAssignment, 0, len(plan.Assignments))
	for _, assignment := range plan.Assignments {
		assignments = append(assignments, model.ApplyPlanAssignment{
			PlotID:       assignment.PlotID,
			CommodityID:  assignment.CommodityID,
			VarietyID:    assignment.VarietyID,
			AreaHa:       assignment.AreaHa,
			PlantingDate: assignment.PlantingDate,
		})
	}

	return &model.ApplyPlanRequest{
		SeasonLabel: proposal.Season.Label,
		SeasonStart: proposal.Season.Start,
		SeasonEnd:   proposal.Season.End,
		Objective:   plan.Objective,
		Engine:      proposal.Engine,
		Assignments: assignments,
	}
}

func TestApplyTurnsAProposalIntoBlocks(t *testing.T) {
	db := seedPlanningFixture(t)
	request := applyRequestFrom(propose(t, db))

	applied, err := planningUseCase(t, db).Apply(context.Background(), homeCoop, request)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if applied.BlocksCreated != len(request.Assignments) {
		t.Fatalf("%d penugasan menghasilkan %d blok", len(request.Assignments), applied.BlocksCreated)
	}

	var stamped int64
	db.Model(&entity.Block{}).Where("season_plan_id = ?", applied.PlanID).Count(&stamped)
	if int(stamped) != applied.BlocksCreated {
		t.Fatalf("hanya %d blok yang membawa season_plan_id", stamped)
	}
}

func TestApplyingASecondPlanReplacesTheFirst(t *testing.T) {
	db := seedPlanningFixture(t)
	request := applyRequestFrom(propose(t, db))
	useCase := planningUseCase(t, db)

	first, err := useCase.Apply(context.Background(), homeCoop, request)
	if err != nil {
		t.Fatalf("Apply pertama: %v", err)
	}
	second, err := useCase.Apply(context.Background(), homeCoop, request)
	if err != nil {
		t.Fatalf("Apply kedua: %v", err)
	}

	if !second.ReplacedExisting {
		t.Fatal("rencana kedua harus menggantikan yang pertama, bukan menumpuk")
	}

	var active int64
	db.Model(&entity.SeasonPlan{}).Where("status = ?", planStatusApplied).Count(&active)
	if active != 1 {
		t.Fatalf("harus tepat satu rencana berlaku, dapat %d", active)
	}

	var orphaned int64
	db.Model(&entity.Block{}).Where("season_plan_id = ?", first.PlanID).Count(&orphaned)
	if orphaned != 0 {
		t.Fatalf("%d blok rencana lama tertinggal", orphaned)
	}
}

func TestApplyRefusesAPlotFromAnotherCooperative(t *testing.T) {
	db := seedPlanningFixture(t)
	request := applyRequestFrom(propose(t, db))
	request.Assignments[0].PlotID = planPlotOther

	_, err := planningUseCase(t, db).Apply(context.Background(), homeCoop, request)

	if err == nil {
		t.Fatal("menanami lahan koperasi lain harus ditolak")
	}

	var blocks int64
	db.Model(&entity.Block{}).Count(&blocks)
	if blocks != 0 {
		t.Fatalf("penolakan meninggalkan %d blok; transaksi tidak berputar balik", blocks)
	}
}

func TestCancelRemovesTheBlocksItCreated(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	applied, err := useCase.Apply(context.Background(), homeCoop, applyRequestFrom(propose(t, db)))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := useCase.Cancel(context.Background(), homeCoop, applied.PlanID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	var blocks int64
	db.Model(&entity.Block{}).Where("season_plan_id = ?", applied.PlanID).Count(&blocks)
	if blocks != 0 {
		t.Fatalf("%d blok tertinggal setelah pembatalan", blocks)
	}

	plan := new(entity.SeasonPlan)
	db.Where("id = ?", applied.PlanID).Take(plan)
	if plan.Status != planStatusCancelled {
		t.Fatalf("status rencana %q, seharusnya dibatalkan", plan.Status)
	}
}

func TestCancelKeepsBlocksWhoseHarvestWasRecorded(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	applied, err := useCase.Apply(context.Background(), homeCoop, applyRequestFrom(propose(t, db)))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	harvested := agronomy.AddDays(planningNow, 120)
	yieldKg := 5200.0
	if err := db.Model(&entity.Block{}).
		Where("season_plan_id = ?", applied.PlanID).Limit(1).
		Updates(map[string]any{"actual_harvest_date": harvested, "actual_yield_kg": yieldKg}).
		Error; err != nil {
		t.Fatalf("mencatat panen: %v", err)
	}

	if err := useCase.Cancel(context.Background(), homeCoop, applied.PlanID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	var remaining int64
	db.Model(&entity.Block{}).Where("season_plan_id = ? AND actual_harvest_date IS NOT NULL",
		applied.PlanID).Count(&remaining)
	if remaining == 0 {
		t.Fatal("pembatalan rencana tidak boleh menghapus panen yang sudah tercatat")
	}
}

func TestCancelRefusesAPlanOfAnotherCooperative(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	applied, err := useCase.Apply(context.Background(), homeCoop, applyRequestFrom(propose(t, db)))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := useCase.Cancel(context.Background(), otherCoop, applied.PlanID); !errors.Is(err, ErrPlanNotFound) {
		t.Fatalf("membatalkan rencana koperasi lain harus ditolak, dapat %v", err)
	}
}

func TestAcceptedDemandReachesThePlanner(t *testing.T) {
	db := seedPlanningFixture(t)

	requests := []entity.SupplyContractRequest{
		{
			ID: "req-accepted", CooperativeID: homeCoop, BuyerID: "buyer-1",
			BuyerName: "Pembeli", CommodityID: maizeCommodity, VolumeKg: 4000,
			WindowStart: agronomy.AddDays(planningNow, 120),
			WindowEnd:   agronomy.AddDays(planningNow, 134),
			Status:      constants.RequestAccepted, CreatedAt: planningNow,
		},
		{
			ID: "req-pending", CooperativeID: homeCoop, BuyerID: "buyer-2",
			BuyerName: "Pembeli", CommodityID: maizeCommodity, VolumeKg: 9000,
			WindowStart: agronomy.AddDays(planningNow, 120),
			WindowEnd:   agronomy.AddDays(planningNow, 134),
			Status:      constants.RequestPending, CreatedAt: planningNow,
		},
	}
	if err := db.Create(&requests).Error; err != nil {
		t.Fatalf("seeding supply requests: %v", err)
	}

	demand, err := planningUseCase(t, db).demandFor(db, homeCoop,
		agronomy.AddDays(planningNow, 26), agronomy.AddDays(planningNow, 207))
	if err != nil {
		t.Fatalf("demandFor: %v", err)
	}

	if len(demand) != 1 || demand[0].Kg != 4000 {
		t.Fatalf("hanya permintaan yang diterima yang boleh menjadi permintaan pasar: %+v", demand)
	}
}
