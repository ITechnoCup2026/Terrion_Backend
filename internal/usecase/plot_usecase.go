package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/weather"
)

var (
	ErrNoCooperative = errors.New("account is not linked to a cooperative")
	ErrPlotNotFound  = errors.New("plot not found")
	ErrAreaTooLarge  = errors.New("plantings exceed the maximum plot area")
)

// Why a harvest could not be recorded. Sentinels rather than a refusal struct
// like plots.SplitRefusal, because none of these needs to hand a number back:
// the code alone says what happened.
var (
	ErrHarvestBlockGone       = errors.New("block is not in this cooperative")
	ErrHarvestAlreadyRecorded = errors.New("block already has a recorded harvest")
	ErrHarvestBeforePlanting  = errors.New("harvest date precedes the planting date")
	ErrHarvestInFuture        = errors.New("harvest date is in the future")
	ErrPaymentBeforeHarvest   = errors.New("payment date precedes the harvest date")
)

type PlotUseCase struct {
	DB                  *gorm.DB
	Log                 *logrus.Logger
	Validate            *validator.Validate
	PlotRepository      *repository.PlotRepository
	BlockRepository     *repository.BlockRepository
	MemberRepository    *repository.MemberRepository
	CommodityRepository *repository.CommodityRepository
	VarietyRepository   *repository.VarietyRepository
	// For the price panel: prices are published per province, and only the
	// cooperative knows which province a plot sits in.
	CooperativeRepository    *repository.CooperativeRepository
	ReferencePriceRepository *repository.ReferencePriceRepository
	Projection               *ProjectionUseCase
	Weather                  *WeatherUseCase
}

func NewPlotUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	memberRepository *repository.MemberRepository,
	commodityRepository *repository.CommodityRepository,
	varietyRepository *repository.VarietyRepository,
	cooperativeRepository *repository.CooperativeRepository,
	referencePriceRepository *repository.ReferencePriceRepository,
	projection *ProjectionUseCase, weatherUseCase *WeatherUseCase,
) *PlotUseCase {
	return &PlotUseCase{
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
		Projection:               projection,
		Weather:                  weatherUseCase,
	}
}

type CreatedPlot struct {
	PlotID   string
	PublicID string
}

type PlotDetail struct {
	Plot        entity.Plot
	MemberName  string
	Blocks      []entity.Block
	Windows     map[string]agronomy.HarvestWindow
	Tonnes      map[string]float64
	Commodities map[string]entity.Commodity
	Varieties   map[string]entity.Variety
	// By block ID, and absent for a block whose commodity the province's price
	// panel does not cover.
	Prices             map[string]agronomy.PriceBenchmark
	HasHarvestedBlocks bool
	Degraded           bool
}

func (u *PlotUseCase) List(
	ctx context.Context, cooperativeID string, now time.Time,
) ([]plots.PlotSummary, error) {
	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return nil, err
	}

	members, err := u.memberNamesOf(ctx, projection.Plots)
	if err != nil {
		return nil, err
	}

	rows := make([]plots.PlotRow, len(projection.Plots))
	for i, plot := range projection.Plots {
		rows[i] = plots.PlotRow{
			ID:         plot.ID,
			Name:       plot.Name,
			PublicID:   plot.PublicID,
			AreaHa:     plot.AreaHa,
			MemberName: members[plot.MemberID],
		}
	}

	return plots.SummarisePlots(rows, projection.Projections, projection.Windows), nil
}

func (u *PlotUseCase) Get(
	ctx context.Context, cooperativeID, plotID string, now time.Time,
) (PlotDetail, error) {
	db := u.DB.WithContext(ctx)

	plot, err := u.PlotRepository.FindInCooperative(db, plotID, cooperativeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return PlotDetail{}, ErrPlotNotFound
		}
		return PlotDetail{}, fmt.Errorf("reading plot %s: %w", plotID, err)
	}

	allBlocks, err := u.BlockRepository.FindByPlotID(db, plot.ID)
	if err != nil {
		return PlotDetail{}, fmt.Errorf("reading blocks of plot %s: %w", plotID, err)
	}

	detail := PlotDetail{
		Plot:    *plot,
		Windows: map[string]agronomy.HarvestWindow{},
		Tonnes:  map[string]float64{},
	}
	for _, block := range allBlocks {
		if block.ActualHarvestDate != nil {
			detail.HasHarvestedBlocks = true
			continue
		}
		detail.Blocks = append(detail.Blocks, block)
	}

	member := new(entity.Member)
	if err := u.MemberRepository.FindById(db, member, plot.MemberID); err == nil {
		detail.MemberName = member.Name
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return PlotDetail{}, fmt.Errorf("reading member of plot %s: %w", plotID, err)
	}

	if err := u.attachReference(db, &detail); err != nil {
		return PlotDetail{}, err
	}

	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return PlotDetail{}, err
	}
	for _, block := range projection.Projections {
		if block.PlotID != plot.ID {
			continue
		}
		detail.Tonnes[block.BlockID] = block.ExpectedTonnes
		if window, known := projection.Windows[block.BlockID]; known {
			detail.Windows[block.BlockID] = window
		}
	}

	if err := u.attachPrices(db, cooperativeID, &detail); err != nil {
		return PlotDetail{}, err
	}

	cell := weather.GridCell{GridLat: plot.GridLat, GridLng: plot.GridLng}
	cellWeather, err := u.Weather.LoadWeatherFor(ctx, cell, time.Time{}, now)
	if err != nil {
		return PlotDetail{}, err
	}
	detail.Degraded = len(cellWeather.Normals) == 0

	return detail, nil
}

func (u *PlotUseCase) Create(
	ctx context.Context, user *entity.AppUser, request *model.CreatePlotRequest,
) (CreatedPlot, error) {
	if err := u.Validate.Struct(request); err != nil {
		return CreatedPlot{}, err
	}
	if user.CooperativeID == nil {
		return CreatedPlot{}, ErrNoCooperative
	}
	cooperativeID := *user.CooperativeID

	areas := make([]float64, len(request.Plantings))
	plantingDates := make([]time.Time, len(request.Plantings))
	for i, planting := range request.Plantings {
		areas[i] = planting.AreaHa

		date, err := agronomy.UTCDate(planting.PlantingDate)
		if err != nil {
			return CreatedPlot{}, err
		}
		plantingDates[i] = date
	}

	areaHa := plots.PlotAreaHa(areas)
	if areaHa > constants.MaxPlotHa {
		return CreatedPlot{}, ErrAreaTooLarge
	}

	plot := &entity.Plot{
		ID:            uuid.NewString(),
		CooperativeID: cooperativeID,
		PublicID:      uuid.NewString()[:constants.PublicIDLength],
		Name:          request.PlotName,
		AreaHa:        areaHa,
		Lat:           *request.Lat,
		Lng:           *request.Lng,
		Decorations:   json.RawMessage("[]"),
	}

	blocks := make([]entity.Block, len(request.Plantings))
	for i, planting := range request.Plantings {
		blocks[i] = entity.Block{
			ID:           uuid.NewString(),
			PlotID:       plot.ID,
			Label:        plots.BlockLabel(i),
			AreaHa:       planting.AreaHa,
			OrderIndex:   i,
			CommodityID:  planting.CommodityID,
			VarietyID:    planting.VarietyID,
			PlantingDate: plantingDates[i],
		}
	}

	if err := u.persistPlot(ctx, cooperativeID, request.MemberName, plot, blocks); err != nil {
		return CreatedPlot{}, err
	}

	u.backfillInBackground(plot.ID, weather.SnapToGrid(plot.Lat, plot.Lng))

	return CreatedPlot{PlotID: plot.ID, PublicID: plot.PublicID}, nil
}

func (u *PlotUseCase) persistPlot(
	ctx context.Context, cooperativeID, memberName string,
	plot *entity.Plot, blocks []entity.Block,
) error {
	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	memberID, err := u.findOrCreateMember(tx, cooperativeID, memberName)
	if err != nil {
		return err
	}
	plot.MemberID = memberID

	if err := u.PlotRepository.Create(tx, plot); err != nil {
		return fmt.Errorf("creating plot for cooperative %s: %w", cooperativeID, err)
	}
	if err := tx.Create(&blocks).Error; err != nil {
		return fmt.Errorf("creating blocks of plot %s: %w", plot.ID, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing plot %s: %w", plot.ID, err)
	}
	return nil
}

func (u *PlotUseCase) findOrCreateMember(
	tx *gorm.DB, cooperativeID, name string,
) (string, error) {
	existing, err := u.MemberRepository.FindByNameInCooperative(tx, cooperativeID, name)
	if err != nil {
		return "", fmt.Errorf("looking up member %q: %w", name, err)
	}
	if existing != nil {
		return existing.ID, nil
	}

	member := &entity.Member{
		ID:            uuid.NewString(),
		CooperativeID: cooperativeID,
		Name:          name,
	}
	if err := u.MemberRepository.Create(tx, member); err != nil {
		return "", fmt.Errorf("creating member %q: %w", name, err)
	}
	return member.ID, nil
}

func (u *PlotUseCase) backfillInBackground(plotID string, cell weather.GridCell) {
	go func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				u.Log.Errorf("weather backfill panicked for plot %s: %v", plotID, recovered)
			}
		}()

		ctx, cancel := context.WithTimeout(
			context.Background(), constants.WeatherBackfillTimeout)
		defer cancel()

		if _, err := u.Weather.BackfillGrid(ctx, cell, time.Now()); err != nil {
			u.Log.Errorf("weather backfill failed for plot %s: %v", plotID, err)
		}
	}()
}

type SplitBlockResult struct {
	PlotID  string
	BlockID string
}

func (u *PlotUseCase) SplitBlock(
	ctx context.Context, user *entity.AppUser, blockID string, request *model.SplitBlockRequest,
) (SplitBlockResult, error) {
	if err := u.Validate.Struct(request); err != nil {
		return SplitBlockResult{}, err
	}
	if user.CooperativeID == nil {
		return SplitBlockResult{}, ErrNoCooperative
	}

	plantingDate, err := agronomy.UTCDate(request.PlantingDate)
	if err != nil {
		return SplitBlockResult{}, err
	}

	db := u.DB.WithContext(ctx)

	block, err := u.BlockRepository.FindInCooperative(db, blockID, *user.CooperativeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return SplitBlockResult{}, &plots.SplitRefusal{Code: constants.SplitBlockAlreadyGone}
		}
		return SplitBlockResult{}, fmt.Errorf("reading block %s: %w", blockID, err)
	}
	if block.ActualHarvestDate != nil {
		return SplitBlockResult{}, &plots.SplitRefusal{Code: constants.SplitBlockHarvested}
	}

	plan, refusal := plots.PlanSplit(block.AreaHa, request.AreaHa)
	if refusal != nil {
		return SplitBlockResult{}, refusal
	}

	nextIndex, err := u.BlockRepository.NextOrderIndex(db, block.PlotID)
	if err != nil {
		return SplitBlockResult{}, fmt.Errorf("reading block order of plot %s: %w", block.PlotID, err)
	}

	planted := &entity.Block{
		ID:           uuid.NewString(),
		PlotID:       block.PlotID,
		Label:        plots.BlockLabel(nextIndex),
		AreaHa:       plan.TakenHa,
		OrderIndex:   nextIndex,
		CommodityID:  request.CommodityID,
		VarietyID:    request.VarietyID,
		PlantingDate: plantingDate,
	}

	tx := db.Begin()
	defer tx.Rollback()

	if err := tx.Model(&entity.Block{}).Where("id = ?", block.ID).
		Update("area_ha", plan.KeptHa).Error; err != nil {
		return SplitBlockResult{}, fmt.Errorf("shrinking block %s: %w", block.ID, err)
	}
	if err := u.BlockRepository.Create(tx, planted); err != nil {
		return SplitBlockResult{}, fmt.Errorf("creating block on plot %s: %w", block.PlotID, err)
	}
	if err := tx.Commit().Error; err != nil {
		return SplitBlockResult{}, fmt.Errorf("committing split of block %s: %w", block.ID, err)
	}

	return SplitBlockResult{PlotID: block.PlotID, BlockID: planted.ID}, nil
}

func (u *PlotUseCase) memberNamesOf(
	ctx context.Context, rows []entity.Plot,
) (map[string]*string, error) {
	names := map[string]*string{}
	if len(rows) == 0 {
		return names, nil
	}

	ids := make([]string, 0, len(rows))
	for _, plot := range rows {
		ids = append(ids, plot.MemberID)
	}

	members := []entity.Member{}
	if err := u.DB.WithContext(ctx).Where("id IN ?", ids).Find(&members).Error; err != nil {
		return nil, fmt.Errorf("reading plot members: %w", err)
	}

	for _, member := range members {
		name := member.Name
		names[member.ID] = &name
	}
	return names, nil
}

func (u *PlotUseCase) attachReference(db *gorm.DB, detail *PlotDetail) error {
	commodityIDs := map[string]bool{}
	varietyIDs := map[string]bool{}
	for _, block := range detail.Blocks {
		commodityIDs[block.CommodityID] = true
		varietyIDs[block.VarietyID] = true
	}

	commodities, err := u.CommodityRepository.FindByIDs(db, keysOf(commodityIDs))
	if err != nil {
		return fmt.Errorf("reading commodities of plot %s: %w", detail.Plot.ID, err)
	}
	detail.Commodities = make(map[string]entity.Commodity, len(commodities))
	for _, commodity := range commodities {
		detail.Commodities[commodity.ID] = commodity
	}

	varieties, err := u.VarietyRepository.FindByIDs(db, keysOf(varietyIDs))
	if err != nil {
		return fmt.Errorf("reading varieties of plot %s: %w", detail.Plot.ID, err)
	}
	detail.Varieties = make(map[string]entity.Variety, len(varieties))
	for _, variety := range varieties {
		detail.Varieties[variety.ID] = variety
	}

	return nil
}

// The province's price panel for whatever this plot is growing.
//
// Called after the projection rather than alongside the rest of the reference
// data, because the seasonal price is chosen by the week a harvest window
// opens -- there is nothing to look up until the windows exist.
//
// A plot whose cooperative has no province, or whose commodities nobody
// publishes a price for, simply comes back with no benchmarks. That is not an
// error: only Jawa Barat is seeded, so it is the common case today.
func (u *PlotUseCase) attachPrices(db *gorm.DB, cooperativeID string, detail *PlotDetail) error {
	detail.Prices = map[string]agronomy.PriceBenchmark{}
	if len(detail.Blocks) == 0 {
		return nil
	}

	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(db, cooperative, cooperativeID); err != nil {
		return fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	commodityIDs := map[string]bool{}
	for _, block := range detail.Blocks {
		commodityIDs[block.CommodityID] = true
	}

	// ponytail: pulls the province's whole weekly history for these commodities
	// and picks the two rows per block in Go, reusing the query impactOf
	// already relies on. Roughly 150 small rows per commodity, so a six-block
	// plot is under a thousand. If the panel ever holds a decade, narrow this
	// to a query for the latest week plus the seasonal ones.
	rows, err := u.ReferencePriceRepository.FindForCommodities(
		db, cooperative.Province, keysOf(commodityIDs))
	if err != nil {
		return fmt.Errorf("reading reference prices for %s: %w", cooperative.Province, err)
	}

	prices := make([]agronomy.ReferencePrice, len(rows))
	for i, row := range rows {
		prices[i] = agronomy.ReferencePrice{
			CommodityID: row.CommodityID,
			WeekStart:   row.WeekStart,
			PricePerKg:  row.PricePerKg,
			Source:      row.Source,
		}
	}

	for _, block := range detail.Blocks {
		// Zero when the block has no window, which BenchmarkFor reads as "no
		// seasonal week to ask about" and still answers with the latest price.
		var windowStart time.Time
		if window, known := detail.Windows[block.ID]; known {
			windowStart = window.Start
		}
		if found := agronomy.BenchmarkFor(prices, block.CommodityID, windowStart); found != nil {
			detail.Prices[block.ID] = *found
		}
	}
	return nil
}

func (u *PlotUseCase) Catalogue(
	ctx context.Context,
) ([]entity.Commodity, []entity.Variety, error) {
	db := u.DB.WithContext(ctx)

	commodities, err := u.CommodityRepository.FindAll(db)
	if err != nil {
		return nil, nil, fmt.Errorf("reading commodities: %w", err)
	}

	varieties, err := u.VarietyRepository.FindAll(db)
	if err != nil {
		return nil, nil, fmt.Errorf("reading varieties: %w", err)
	}

	return commodities, varieties, nil
}

type RecordHarvestResult struct {
	PlotID    string
	BlockID   string
	VarietyID string
	// Nil when there was nothing to fit -- see RefitCalibration.
	Calibration *entity.Calibration
	// Carried so the response can name what the model learned about, rather
	// than handing the browser a pair of ids to look up again.
	VarietyName   string
	CommodityName string
}

// RecordHarvest closes the loop the predictor has always been reading from.
//
// Until this existed, block.actual_harvest_date could only be set by seed
// data, so the calibration table was permanently empty and every prediction
// ran on the base model for ever. One kader typing what actually came off a
// field is what turns the projection from a fixed formula into something that
// gets better where it is used.
//
// The refit runs after the write and inside no transaction, deliberately: the
// harvest is a fact the cooperative reported and must survive, whereas the
// calibration is derived and can be recomputed from those same facts on the
// next recording. Rolling back a farmer's entry because a weather fetch timed
// out would be the wrong way round.
func (u *PlotUseCase) RecordHarvest(
	ctx context.Context, user *entity.AppUser, blockID string,
	request *model.RecordHarvestRequest,
) (RecordHarvestResult, error) {
	if err := u.Validate.Struct(request); err != nil {
		return RecordHarvestResult{}, err
	}
	if user.CooperativeID == nil {
		return RecordHarvestResult{}, ErrNoCooperative
	}

	harvestDate, err := agronomy.UTCDate(request.ActualHarvestDate)
	if err != nil {
		return RecordHarvestResult{}, err
	}

	var paymentDate *time.Time
	if request.PaymentReceivedDate != nil {
		parsed, err := agronomy.UTCDate(*request.PaymentReceivedDate)
		if err != nil {
			return RecordHarvestResult{}, err
		}
		paymentDate = &parsed
	}

	db := u.DB.WithContext(ctx)

	block, err := u.BlockRepository.FindInCooperative(db, blockID, *user.CooperativeID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return RecordHarvestResult{}, ErrHarvestBlockGone
		}
		return RecordHarvestResult{}, fmt.Errorf("reading block %s: %w", blockID, err)
	}

	// Recording twice would put a second observation of the same field into the
	// calibration and let one harvest count as two.
	if block.ActualHarvestDate != nil {
		return RecordHarvestResult{}, ErrHarvestAlreadyRecorded
	}
	if harvestDate.Before(block.PlantingDate) {
		return RecordHarvestResult{}, ErrHarvestBeforePlanting
	}

	now := time.Now().UTC()
	if harvestDate.After(now) {
		return RecordHarvestResult{}, ErrHarvestInFuture
	}
	if paymentDate != nil && paymentDate.Before(harvestDate) {
		return RecordHarvestResult{}, ErrPaymentBeforeHarvest
	}

	if err := db.Model(&entity.Block{}).Where("id = ?", block.ID).Updates(map[string]any{
		"actual_harvest_date":   harvestDate,
		"actual_yield_kg":       request.ActualYieldKg,
		"actual_price_per_kg":   request.ActualPricePerKg,
		"payment_received_date": paymentDate,
	}).Error; err != nil {
		return RecordHarvestResult{}, fmt.Errorf("recording harvest of block %s: %w", block.ID, err)
	}

	result := RecordHarvestResult{
		PlotID:    block.PlotID,
		BlockID:   block.ID,
		VarietyID: block.VarietyID,
	}

	// After the write, so this harvest is one of the observations it fits over.
	calibration, err := u.Projection.RefitCalibration(
		ctx, *user.CooperativeID, block.VarietyID, now)
	if err != nil {
		// Logged and swallowed. The harvest IS recorded, and telling the kader
		// otherwise would have them enter it again.
		u.Log.Errorf("refitting calibration of variety %s: %v", block.VarietyID, err)
		return result, nil
	}

	result.Calibration = calibration
	if calibration != nil {
		result.VarietyName, result.CommodityName = u.namesOfVariety(db, block.VarietyID)
	}
	return result, nil
}

// What a variety and its commodity are called, best effort.
//
// Only ever used to label a calibration the reader is being shown. A missing
// name leaves the label thinner, which is a far better outcome than failing a
// harvest that is already recorded.
func (u *PlotUseCase) namesOfVariety(db *gorm.DB, varietyID string) (string, string) {
	variety := new(entity.Variety)
	if err := u.VarietyRepository.FindById(db, variety, varietyID); err != nil {
		u.Log.Warnf("naming variety %s: %v", varietyID, err)
		return "", ""
	}

	commodity := new(entity.Commodity)
	if err := u.CommodityRepository.FindById(db, commodity, variety.CommodityID); err != nil {
		u.Log.Warnf("naming commodity %s: %v", variety.CommodityID, err)
		return variety.Name, ""
	}
	return variety.Name, commodity.Name
}
