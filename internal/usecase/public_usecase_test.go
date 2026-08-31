package usecase

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

func publicFixture(t *testing.T) *gorm.DB {
	t.Helper()

	db := dashboardFixture(t)
	err := db.Exec(`CREATE VIEW public_plot AS
		SELECT p.public_id, p.name, p.area_ha, p.tile_size_m2,
		       m.name AS member_name, c.village, c.district, p.terrain_seed
		FROM plot p
		JOIN member m ON m.id = p.member_id
		JOIN cooperative c ON c.id = p.cooperative_id`).Error
	if err != nil {
		t.Fatalf("creating the public_plot view: %v", err)
	}
	return db
}

func publicUseCase(t *testing.T, db *gorm.DB) *PublicUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	weatherUseCase := NewWeatherUseCase(db, log, &repository.WeatherRepository{}, nil)

	return NewPublicUseCase(db, log,
		&repository.PublicPlotRepository{}, &repository.PlotRepository{},
		&repository.BlockRepository{}, &repository.CommodityRepository{},
		&repository.VarietyRepository{}, &repository.CooperativeRepository{},
		weatherUseCase)
}

func atlasUseCase(t *testing.T, db *gorm.DB) *AtlasUseCase {
	t.Helper()

	log := logrus.New()
	log.SetOutput(io.Discard)

	return NewAtlasUseCase(db, log,
		&repository.CooperativeRepository{}, &repository.PlotRepository{},
		&repository.PublicPlotRepository{}, &repository.BlockRepository{},
		&repository.CommodityRepository{})
}

func TestLoadPublicPlotShowsOnlyWhatIsStandingOnIt(t *testing.T) {
	db := publicFixture(t)

	plot, err := publicUseCase(t, db).
		LoadPlot(context.Background(), "pub-plot-home", projectionNow)
	if err != nil {
		t.Fatalf("LoadPlot: %v", err)
	}

	if plot.View.MemberName != "Pak Asep" || plot.View.Village != "Jalancagak" {
		t.Errorf("view = %+v, want the member and village from the view", plot.View)
	}
	if len(plot.Blocks) != 1 || plot.Blocks[0].ID != "block-growing" {
		t.Fatalf("Blocks = %+v, want only the standing block: a harvested season is long gone",
			plot.Blocks)
	}
	if plot.Blocks[0].CommodityName != "Jagung" || plot.Blocks[0].VarietyName != "Bisi-18" {
		t.Errorf("block = %+v, want the commodity and variety named", plot.Blocks[0])
	}
	if plot.Degraded {
		t.Error("Degraded = true, want false: this cell has stored normals")
	}
	if plot.Blocks[0].Window == nil {
		t.Error("Window = nil, want a predicted window")
	}
}

func TestLoadPublicPlotReportsAYieldRangeNotAPointEstimate(t *testing.T) {
	db := publicFixture(t)

	plot, err := publicUseCase(t, db).
		LoadPlot(context.Background(), "pub-plot-home", projectionNow)
	if err != nil {
		t.Fatalf("LoadPlot: %v", err)
	}

	yieldRange := plot.Blocks[0].YieldRange
	if yieldRange == nil {
		t.Fatal("YieldRange = nil, want the variety's published range")
	}
	if yieldRange.MinTonnes != 7 || yieldRange.MaxTonnes != 9.5 {
		t.Errorf("YieldRange = %+v, want 7..9.5 for one hectare", yieldRange)
	}
}

func TestLoadPublicPlotNamesTheCooperativeAndItsNeighbours(t *testing.T) {
	db := publicFixture(t)

	plot, err := publicUseCase(t, db).
		LoadPlot(context.Background(), "pub-plot-home", projectionNow)
	if err != nil {
		t.Fatalf("LoadPlot: %v", err)
	}

	if plot.CooperativeName == nil || *plot.CooperativeName != "KUD Subang" {
		t.Errorf("CooperativeName = %v, want KUD Subang", plot.CooperativeName)
	}
	if plot.Neighbours.Position != 1 || plot.Neighbours.Total != 1 {
		t.Errorf("neighbours = %d of %d, want 1 of 1",
			plot.Neighbours.Position, plot.Neighbours.Total)
	}
	if plot.Neighbours.Previous != nil || plot.Neighbours.Next != nil {
		t.Errorf("neighbours = %+v, want none either side of the only plot", plot.Neighbours)
	}
}

func TestLoadPublicPlotOfAnUnknownCodeIsNotFound(t *testing.T) {
	db := publicFixture(t)

	_, err := publicUseCase(t, db).
		LoadPlot(context.Background(), "pub-nothing", projectionNow)

	if !errors.Is(err, ErrPublicPlotNotFound) {
		t.Errorf("err = %v, want ErrPublicPlotNotFound", err)
	}
}

func TestAtlasCooperativesTalliesPlotsAndHectares(t *testing.T) {
	db := publicFixture(t)

	cooperatives, err := atlasUseCase(t, db).Cooperatives(context.Background())
	if err != nil {
		t.Fatalf("Cooperatives: %v", err)
	}

	if len(cooperatives) != 1 {
		t.Fatalf("len(cooperatives) = %d, want 1", len(cooperatives))
	}
	if cooperatives[0].Name != "KUD Subang" {
		t.Errorf("Name = %q, want KUD Subang", cooperatives[0].Name)
	}
	if cooperatives[0].PlotCount != 1 || cooperatives[0].Hectares != 1 {
		t.Errorf("tally = %d plots / %v ha, want 1 / 1",
			cooperatives[0].PlotCount, cooperatives[0].Hectares)
	}
	if cooperatives[0].Lat != -6.25 || cooperatives[0].Lng != 107.75 {
		t.Errorf("pin = %v/%v, want the cooperative's own coordinates",
			cooperatives[0].Lat, cooperatives[0].Lng)
	}
}

func TestAtlasFarmListsPlotsAndWhatIsGrowingOnThem(t *testing.T) {
	db := publicFixture(t)

	farm, err := atlasUseCase(t, db).Farm(context.Background(), homeCoop)
	if err != nil {
		t.Fatalf("Farm: %v", err)
	}

	if len(farm.Plots) != 1 {
		t.Fatalf("len(Plots) = %d, want 1", len(farm.Plots))
	}
	if farm.Plots[0].PublicID != "pub-plot-home" || farm.Plots[0].MemberName != "Pak Asep" {
		t.Errorf("plot = %+v, want the public id and member from the view", farm.Plots[0])
	}
	if len(farm.Plots[0].Crops) != 1 || farm.Plots[0].Crops[0] != "Jagung" {
		t.Errorf("Crops = %v, want only what is standing", farm.Plots[0].Crops)
	}
	if farm.TotalHectares != 1 {
		t.Errorf("TotalHectares = %v, want 1", farm.TotalHectares)
	}
}

func TestAtlasFarmOfAnUnknownCooperativeIsNotFound(t *testing.T) {
	db := publicFixture(t)

	_, err := atlasUseCase(t, db).Farm(context.Background(), otherCoop)

	if !errors.Is(err, ErrCooperativeNotFound) {
		t.Errorf("err = %v, want ErrCooperativeNotFound", err)
	}
}

func TestPublicPlotViewCarriesNoCoordinate(t *testing.T) {
	db := publicFixture(t)

	view := new(entity.PublicPlot)
	if err := db.Table("public_plot").Where("public_id = ?", "pub-plot-home").
		Take(view).Error; err != nil {
		t.Fatalf("reading the view: %v", err)
	}

	columns := []string{}
	if err := db.Raw(`SELECT name FROM pragma_table_info('public_plot')`).
		Scan(&columns).Error; err != nil {
		t.Fatalf("reading the view columns: %v", err)
	}

	for _, column := range columns {
		switch column {
		case "lat", "lng", "grid_lat", "grid_lng", "nik_hash":
			t.Errorf("public_plot exposes %q; the public read path must have no column to leak",
				column)
		}
	}
}
