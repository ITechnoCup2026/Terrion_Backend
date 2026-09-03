package usecase

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

func plotUseCase(t *testing.T, db *gorm.DB) *PlotUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	unreachable := weather.NewClient()
	unreachable.ArchiveURL = "http://127.0.0.1:1/archive"
	unreachable.ForecastURL = "http://127.0.0.1:1/forecast"

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, unreachable)
	projection := NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{}, weatherUseCase)

	return NewPlotUseCase(db, log, validator.New(),
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.MemberRepository{}, &repository.CommodityRepository{},
		&repository.VarietyRepository{}, projection, weatherUseCase)
}

func plotFixture(t *testing.T) (*gorm.DB, *entity.AppUser) {
	t.Helper()

	db := seedProjectionFixture(t)
	if err := db.AutoMigrate(&entity.Member{}, &entity.Commodity{}); err != nil {
		t.Fatalf("migrating member and commodity: %v", err)
	}
	if err := db.Create(&entity.Member{
		ID: "member-plot-home", CooperativeID: homeCoop, Name: "Pak Asep",
	}).Error; err != nil {
		t.Fatalf("seeding member: %v", err)
	}
	if err := db.Exec(`UPDATE plot SET member_id = ? WHERE id = ?`,
		"member-plot-home", "plot-home").Error; err != nil {
		t.Fatalf("linking member to plot: %v", err)
	}
	if err := db.Create(&entity.Commodity{
		ID: maizeCommodity, Slug: "jagung", Name: "Jagung", SpriteRow: 1,
	}).Error; err != nil {
		t.Fatalf("seeding commodity: %v", err)
	}

	cooperativeID := homeCoop
	return db, &entity.AppUser{
		ID: "user-1", Role: constants.RoleKader, CooperativeID: &cooperativeID,
	}
}

func TestPlotListSummarisesOnlyTheCooperativesPlots(t *testing.T) {
	db, user := plotFixture(t)

	summaries, err := plotUseCase(t, db).
		List(context.Background(), *user.CooperativeID, projectionNow)
	if err != nil {
		t.Fatalf("List: %v", err)
	}

	if len(summaries) != 1 {
		t.Fatalf("len(summaries) = %d, want 1", len(summaries))
	}
	if summaries[0].ID != "plot-home" {
		t.Errorf("ID = %q, want plot-home", summaries[0].ID)
	}
	if summaries[0].MemberName == nil || *summaries[0].MemberName != "Pak Asep" {
		t.Errorf("MemberName = %v, want Pak Asep", summaries[0].MemberName)
	}
	if summaries[0].BlockCount != 1 {
		t.Errorf("BlockCount = %d, want 1: the harvested block is not standing crop",
			summaries[0].BlockCount)
	}
}

func TestPlotGetRefusesAPlotOfAnotherCooperative(t *testing.T) {
	db, user := plotFixture(t)

	_, err := plotUseCase(t, db).
		Get(context.Background(), *user.CooperativeID, "plot-other", projectionNow)

	if !errors.Is(err, ErrPlotNotFound) {
		t.Errorf("err = %v, want ErrPlotNotFound", err)
	}
}

func TestPlotGetReturnsStandingBlocksAndFlagsHarvestedOnes(t *testing.T) {
	db, user := plotFixture(t)

	detail, err := plotUseCase(t, db).
		Get(context.Background(), *user.CooperativeID, "plot-home", projectionNow)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(detail.Blocks) != 1 || detail.Blocks[0].ID != "block-growing" {
		t.Fatalf("Blocks = %+v, want only block-growing", detail.Blocks)
	}
	if !detail.HasHarvestedBlocks {
		t.Error("HasHarvestedBlocks = false, want true so the page can tell an empty plot from a finished season")
	}
	if detail.MemberName != "Pak Asep" {
		t.Errorf("MemberName = %q, want Pak Asep", detail.MemberName)
	}
	if detail.Degraded {
		t.Error("Degraded = true, want false: this cell has stored normals")
	}
	if _, projected := detail.Windows["block-growing"]; !projected {
		t.Error("Windows has no entry for block-growing")
	}
}

func createPlotRequest() *model.CreatePlotRequest {
	lat := -6.25
	lng := 107.75
	return &model.CreatePlotRequest{
		MemberName: "Bu Sri",
		PlotName:   "Sawah Selatan",
		Lat:        &lat,
		Lng:        &lng,
		Plantings: []model.PlantingRequest{
			{
				CommodityID:  "3f4b1a2c-1111-4111-8111-111111111111",
				VarietyID:    "3f4b1a2c-2222-4222-8222-222222222222",
				PlantingDate: "2026-09-01",
				AreaHa:       0.3,
			},
			{
				CommodityID:  "3f4b1a2c-1111-4111-8111-111111111111",
				VarietyID:    "3f4b1a2c-2222-4222-8222-222222222222",
				PlantingDate: "2026-09-05",
				AreaHa:       0.42,
			},
		},
	}
}

func TestPlotCreateStoresThePlotItsBlocksAndTheMember(t *testing.T) {
	db, user := plotFixture(t)

	created, err := plotUseCase(t, db).Create(context.Background(), user, createPlotRequest())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if len(created.PublicID) != constants.PublicIDLength {
		t.Errorf("PublicID = %q, want %d characters", created.PublicID, constants.PublicIDLength)
	}

	stored := new(entity.Plot)
	if err := db.Where("id = ?", created.PlotID).Take(stored).Error; err != nil {
		t.Fatalf("reading back plot: %v", err)
	}
	if stored.AreaHa != 0.72 {
		t.Errorf("AreaHa = %v, want 0.72: the plot's area is the sum of its plantings",
			stored.AreaHa)
	}
	if stored.CooperativeID != *user.CooperativeID {
		t.Errorf("CooperativeID = %q, want the caller's", stored.CooperativeID)
	}

	blocks := []entity.Block{}
	if err := db.Where("plot_id = ?", created.PlotID).Order("order_index").
		Find(&blocks).Error; err != nil {
		t.Fatalf("reading back blocks: %v", err)
	}
	if len(blocks) != 2 {
		t.Fatalf("len(blocks) = %d, want 2", len(blocks))
	}
	if blocks[0].Label != "BLOK A" || blocks[1].Label != "BLOK B" {
		t.Errorf("labels = %q, %q, want BLOK A, BLOK B", blocks[0].Label, blocks[1].Label)
	}

	member := new(entity.Member)
	if err := db.Where("id = ?", stored.MemberID).Take(member).Error; err != nil {
		t.Fatalf("reading back member: %v", err)
	}
	if member.Name != "Bu Sri" {
		t.Errorf("member name = %q, want Bu Sri", member.Name)
	}
}

func TestPlotCreateReusesAMemberWhateverTheCase(t *testing.T) {
	db, user := plotFixture(t)
	useCase := plotUseCase(t, db)

	request := createPlotRequest()
	request.MemberName = "pak asep"

	created, err := useCase.Create(context.Background(), user, request)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	stored := new(entity.Plot)
	if err := db.Where("id = ?", created.PlotID).Take(stored).Error; err != nil {
		t.Fatalf("reading back plot: %v", err)
	}
	if stored.MemberID != "member-plot-home" {
		t.Errorf("MemberID = %q, want the existing member rather than a duplicate",
			stored.MemberID)
	}
}

func TestPlotCreateRejectsAnAccountWithNoCooperative(t *testing.T) {
	db, _ := plotFixture(t)
	buyer := &entity.AppUser{ID: "buyer-1", Role: constants.RoleBuyer}

	_, err := plotUseCase(t, db).Create(context.Background(), buyer, createPlotRequest())

	if !errors.Is(err, ErrNoCooperative) {
		t.Errorf("err = %v, want ErrNoCooperative", err)
	}
}

func TestPlotCreateRejectsAnInvalidRequest(t *testing.T) {
	db, user := plotFixture(t)
	useCase := plotUseCase(t, db)

	tests := []struct {
		name   string
		mutate func(*model.CreatePlotRequest)
	}{
		{"no plantings", func(r *model.CreatePlotRequest) { r.Plantings = nil }},
		{"latitude outside Indonesia", func(r *model.CreatePlotRequest) {
			outside := 48.85
			r.Lat = &outside
		}},
		{"missing latitude", func(r *model.CreatePlotRequest) { r.Lat = nil }},
		{"member name too short", func(r *model.CreatePlotRequest) { r.MemberName = "A" }},
		{"planting below the minimum area", func(r *model.CreatePlotRequest) {
			r.Plantings[0].AreaHa = 0.004
		}},
		{"malformed planting date", func(r *model.CreatePlotRequest) {
			r.Plantings[0].PlantingDate = "01-09-2026"
		}},
	}

	for _, test := range tests {
		request := createPlotRequest()
		test.mutate(request)

		if _, err := useCase.Create(context.Background(), user, request); err == nil {
			t.Errorf("%s: Create returned nil error, want a rejection", test.name)
		}
	}
}

func splitRequest() *model.SplitBlockRequest {
	return &model.SplitBlockRequest{
		AreaHa:       0.3,
		CommodityID:  "3f4b1a2c-1111-4111-8111-111111111111",
		VarietyID:    "3f4b1a2c-2222-4222-8222-222222222222",
		PlantingDate: "2026-09-20",
	}
}

func TestSplitBlockShrinksTheOriginalAndPlantsTheRemainder(t *testing.T) {
	db, user := plotFixture(t)

	result, err := plotUseCase(t, db).
		SplitBlock(context.Background(), user, "block-growing", splitRequest())
	if err != nil {
		t.Fatalf("SplitBlock: %v", err)
	}

	original := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(original).Error; err != nil {
		t.Fatalf("reading back the original block: %v", err)
	}
	if original.AreaHa != 0.7 {
		t.Errorf("original AreaHa = %v, want 0.7", original.AreaHa)
	}

	planted := new(entity.Block)
	if err := db.Where("id = ?", result.BlockID).Take(planted).Error; err != nil {
		t.Fatalf("reading back the new block: %v", err)
	}
	if planted.AreaHa != 0.3 {
		t.Errorf("new AreaHa = %v, want 0.3", planted.AreaHa)
	}
	if planted.OrderIndex != 2 {
		t.Errorf("OrderIndex = %d, want 2: a harvested season's slot must not be reused",
			planted.OrderIndex)
	}
	if planted.Label != "BLOK C" {
		t.Errorf("Label = %q, want BLOK C", planted.Label)
	}
}

func TestSplitBlockRefusals(t *testing.T) {
	tests := []struct {
		name    string
		blockID string
		areaHa  float64
		want    string
	}{
		{"block of another cooperative", "block-other-coop", 0.3, constants.SplitBlockAlreadyGone},
		{"block that does not exist", "block-ghost", 0.3, constants.SplitBlockAlreadyGone},
		{"block already harvested", "block-harvested", 0.3, constants.SplitBlockHarvested},
		{"taking the whole block", "block-growing", 1.0, constants.SplitLeavesTooLittle},
	}

	for _, test := range tests {
		db, user := plotFixture(t)
		request := splitRequest()
		request.AreaHa = test.areaHa

		_, err := plotUseCase(t, db).
			SplitBlock(context.Background(), user, test.blockID, request)

		var refusal *plots.SplitRefusal
		if !errors.As(err, &refusal) {
			t.Errorf("%s: err = %v, want a *plots.SplitRefusal", test.name, err)
			continue
		}
		if refusal.Code != test.want {
			t.Errorf("%s: Code = %q, want %q", test.name, refusal.Code, test.want)
		}
	}
}

func TestSplitBlockLeavesTheBlockAloneWhenRefused(t *testing.T) {
	db, user := plotFixture(t)
	request := splitRequest()
	request.AreaHa = 1.0

	if _, err := plotUseCase(t, db).
		SplitBlock(context.Background(), user, "block-growing", request); err == nil {
		t.Fatal("SplitBlock returned nil error, want a refusal")
	}

	original := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(original).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	if original.AreaHa != 1 {
		t.Errorf("AreaHa = %v, want the untouched 1", original.AreaHa)
	}
}

// Yesterday, as an ISO date. Derived from the clock rather than hard-coded so
// the "not in the future" rule is exercised against the same `now` the use case
// reads, whenever the suite happens to run.
func recentDate(daysAgo int) string {
	return time.Now().UTC().AddDate(0, 0, -daysAgo).Format(constants.ISODateLayout)
}

func harvestRequest() *model.RecordHarvestRequest {
	price := 5200.0
	return &model.RecordHarvestRequest{
		ActualHarvestDate: recentDate(1),
		ActualYieldKg:     7400,
		ActualPricePerKg:  &price,
	}
}

func TestRecordHarvestStoresWhatCameOffTheField(t *testing.T) {
	db, user := plotFixture(t)

	result, err := plotUseCase(t, db).
		RecordHarvest(context.Background(), user, "block-growing", harvestRequest())
	if err != nil {
		t.Fatalf("RecordHarvest: %v", err)
	}
	if result.BlockID != "block-growing" || result.PlotID != "plot-home" {
		t.Errorf("result = %+v, want block-growing on plot-home", result)
	}

	block := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(block).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	if block.ActualHarvestDate == nil {
		t.Fatal("ActualHarvestDate is still nil: the harvest was not recorded")
	}
	if got := block.ActualHarvestDate.Format(constants.ISODateLayout); got != recentDate(1) {
		t.Errorf("ActualHarvestDate = %s, want %s", got, recentDate(1))
	}
	if block.ActualYieldKg == nil || *block.ActualYieldKg != 7400 {
		t.Errorf("ActualYieldKg = %v, want 7400", block.ActualYieldKg)
	}
	if block.ActualPricePerKg == nil || *block.ActualPricePerKg != 5200 {
		t.Errorf("ActualPricePerKg = %v, want 5200", block.ActualPricePerKg)
	}
}

// The weather client in this harness is deliberately unreachable, so the refit
// cannot run. The harvest must still be recorded: it is the fact the farmer
// reported, and the calibration is derived from it rather than the other way
// round.
func TestRecordHarvestSurvivesAFailedRefit(t *testing.T) {
	db, user := plotFixture(t)

	result, err := plotUseCase(t, db).
		RecordHarvest(context.Background(), user, "block-growing", harvestRequest())
	if err != nil {
		t.Fatalf("RecordHarvest: %v", err)
	}

	block := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(block).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	if block.ActualHarvestDate == nil {
		t.Error("a failed refit rolled back the harvest, which must never happen")
	}
	_ = result
}

func TestRecordHarvestRefusals(t *testing.T) {
	tests := []struct {
		name    string
		blockID string
		mutate  func(*model.RecordHarvestRequest)
		want    error
	}{
		{"block of another cooperative", "block-other-coop", nil, ErrHarvestBlockGone},
		{"block that does not exist", "block-ghost", nil, ErrHarvestBlockGone},
		{"already recorded", "block-harvested", nil, ErrHarvestAlreadyRecorded},
		{
			"before the planting date", "block-growing",
			func(r *model.RecordHarvestRequest) { r.ActualHarvestDate = "2026-01-05" },
			ErrHarvestBeforePlanting,
		},
		{
			"in the future", "block-growing",
			func(r *model.RecordHarvestRequest) {
				r.ActualHarvestDate = time.Now().UTC().AddDate(0, 0, 3).
					Format(constants.ISODateLayout)
			},
			ErrHarvestInFuture,
		},
		{
			"paid before it was cut", "block-growing",
			func(r *model.RecordHarvestRequest) {
				earlier := recentDate(30)
				r.PaymentReceivedDate = &earlier
			},
			ErrPaymentBeforeHarvest,
		},
	}

	for _, test := range tests {
		db, user := plotFixture(t)
		request := harvestRequest()
		if test.mutate != nil {
			test.mutate(request)
		}

		_, err := plotUseCase(t, db).
			RecordHarvest(context.Background(), user, test.blockID, request)

		if !errors.Is(err, test.want) {
			t.Errorf("%s: err = %v, want %v", test.name, err, test.want)
		}
	}
}

func TestRecordHarvestLeavesTheBlockAloneWhenRefused(t *testing.T) {
	db, user := plotFixture(t)
	request := harvestRequest()
	request.ActualHarvestDate = "2026-01-05"

	if _, err := plotUseCase(t, db).
		RecordHarvest(context.Background(), user, "block-growing", request); err == nil {
		t.Fatal("RecordHarvest accepted a date before the planting date")
	}

	block := new(entity.Block)
	if err := db.Where("id = ?", "block-growing").Take(block).Error; err != nil {
		t.Fatalf("reading back the block: %v", err)
	}
	if block.ActualHarvestDate != nil || block.ActualYieldKg != nil {
		t.Error("a refused harvest still wrote to the block")
	}
}
