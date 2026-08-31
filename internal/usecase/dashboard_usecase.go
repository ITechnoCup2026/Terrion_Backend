package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/dashboard"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

type DashboardUseCase struct {
	DB                       *gorm.DB
	Log                      *logrus.Logger
	CooperativeRepository    *repository.CooperativeRepository
	BlockRepository          *repository.BlockRepository
	CommodityRepository      *repository.CommodityRepository
	MemberRepository         *repository.MemberRepository
	ReferencePriceRepository *repository.ReferencePriceRepository
	InputOrderRepository     *repository.InputOrderRepository
	Projection               *ProjectionUseCase
}

func NewDashboardUseCase(
	db *gorm.DB, log *logrus.Logger,
	cooperativeRepository *repository.CooperativeRepository,
	blockRepository *repository.BlockRepository,
	commodityRepository *repository.CommodityRepository,
	memberRepository *repository.MemberRepository,
	referencePriceRepository *repository.ReferencePriceRepository,
	inputOrderRepository *repository.InputOrderRepository,
	projection *ProjectionUseCase,
) *DashboardUseCase {
	return &DashboardUseCase{
		DB:                       db,
		Log:                      log,
		CooperativeRepository:    cooperativeRepository,
		BlockRepository:          blockRepository,
		CommodityRepository:      commodityRepository,
		MemberRepository:         memberRepository,
		ReferencePriceRepository: referencePriceRepository,
		InputOrderRepository:     inputOrderRepository,
		Projection:               projection,
	}
}

type Dashboard struct {
	Weeks          []dashboard.ProjectionWeek
	Flagged        []dashboard.CollisionWeek
	Lead           *dashboard.CollisionWeek
	Suggestions    []agronomy.StaggerSuggestion
	Upcoming       []dashboard.UpcomingHarvest
	UpcomingTonnes float64
	Impact         agronomy.ImpactFigures
	Commodities    map[string]string
}

func (u *DashboardUseCase) Load(
	ctx context.Context, cooperativeID string, now time.Time,
) (Dashboard, error) {
	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return Dashboard{}, err
	}

	db := u.DB.WithContext(ctx)

	capacity, err := u.capacityOf(db, cooperativeID)
	if err != nil {
		return Dashboard{}, err
	}

	collisions := agronomy.DetectCollisions(projection.Projections, capacity)

	commodities, err := u.commodityNames(db)
	if err != nil {
		return Dashboard{}, err
	}

	plots, err := u.plotRefs(db, projection.Plots)
	if err != nil {
		return Dashboard{}, err
	}

	upcoming := dashboard.UpcomingHarvests(
		projection.Projections,
		now, agronomy.AddDays(now, constants.UpcomingDays),
		plots, commodities, constants.UpcomingLimit)

	flagged := withPlotCounts(collisions.Flagged, projection.Projections)

	impact, err := u.impactOf(db, cooperativeID, projection.Projections, capacity)
	if err != nil {
		return Dashboard{}, err
	}

	return Dashboard{
		Weeks: dashboard.WeeklyProjection(
			projection.Projections, now, constants.DefaultHorizonWeeks),
		Flagged:        flagged,
		Lead:           dashboard.SelectLeadCollision(flagged),
		Suggestions:    collisions.Suggestions,
		Upcoming:       upcoming,
		UpcomingTonnes: dashboard.UpcomingTonnes(upcoming),
		Impact:         impact,
		Commodities:    commodities,
	}, nil
}

func withPlotCounts(
	flagged []agronomy.FlaggedWeek, projections []agronomy.BlockProjection,
) []dashboard.CollisionWeek {
	plotOfBlock := make(map[string]string, len(projections))
	for _, projection := range projections {
		plotOfBlock[projection.BlockID] = projection.PlotID
	}

	weeks := make([]dashboard.CollisionWeek, len(flagged))
	for i, week := range flagged {
		plots := map[string]bool{}
		for _, blockID := range week.ContributingBlockIDs {
			if plotID, known := plotOfBlock[blockID]; known {
				plots[plotID] = true
			}
		}
		weeks[i] = dashboard.CollisionWeek{FlaggedWeek: week, PlotCount: len(plots)}
	}
	return weeks
}

func (u *DashboardUseCase) capacityOf(
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

func (u *DashboardUseCase) commodityNames(db *gorm.DB) (map[string]string, error) {
	rows, err := u.CommodityRepository.FindAll(db)
	if err != nil {
		return nil, fmt.Errorf("reading commodities: %w", err)
	}

	names := make(map[string]string, len(rows))
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

func (u *DashboardUseCase) plotRefs(
	db *gorm.DB, rows []entity.Plot,
) (map[string]dashboard.PlotRef, error) {
	refs := map[string]dashboard.PlotRef{}
	if len(rows) == 0 {
		return refs, nil
	}

	memberIDs := make([]string, 0, len(rows))
	for _, plot := range rows {
		memberIDs = append(memberIDs, plot.MemberID)
	}

	members := []entity.Member{}
	if err := db.Where("id IN ?", memberIDs).Find(&members).Error; err != nil {
		return nil, fmt.Errorf("reading plot members: %w", err)
	}
	nameOf := map[string]string{}
	for _, member := range members {
		nameOf[member.ID] = member.Name
	}

	for _, plot := range rows {
		ref := dashboard.PlotRef{Name: plot.Name}
		if name, known := nameOf[plot.MemberID]; known {
			ref.MemberName = &name
		}
		refs[plot.ID] = ref
	}
	return refs, nil
}

func (u *DashboardUseCase) impactOf(
	db *gorm.DB, cooperativeID string,
	projections []agronomy.BlockProjection, capacity map[string]float64,
) (agronomy.ImpactFigures, error) {
	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(db, cooperative, cooperativeID); err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	plots := []entity.Plot{}
	if err := db.Model(&entity.Plot{}).Select("id").
		Where("cooperative_id = ?", cooperativeID).Find(&plots).Error; err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}
	plotIDs := plotIDsOf(plots)

	harvested, err := u.BlockRepository.FindHarvestedByPlotIDs(db, plotIDs)
	if err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading harvested blocks of cooperative %s: %w", cooperativeID, err)
	}

	blocks := make([]agronomy.HarvestedBlock, len(harvested))
	commodityIDs := map[string]bool{}
	for i, block := range harvested {
		commodityIDs[block.CommodityID] = true
		blocks[i] = agronomy.HarvestedBlock{
			BlockID:             block.ID,
			CommodityID:         block.CommodityID,
			ActualHarvestDate:   block.ActualHarvestDate,
			ActualYieldKg:       block.ActualYieldKg,
			ActualPricePerKg:    block.ActualPricePerKg,
			PaymentReceivedDate: block.PaymentReceivedDate,
		}
	}

	priceRows, err := u.ReferencePriceRepository.FindForCommodities(
		db, cooperative.Province, keysOf(commodityIDs))
	if err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading reference prices for %s: %w", cooperative.Province, err)
	}
	prices := make([]agronomy.ReferencePrice, len(priceRows))
	for i, row := range priceRows {
		prices[i] = agronomy.ReferencePrice{
			CommodityID: row.CommodityID,
			WeekStart:   row.WeekStart,
			PricePerKg:  row.PricePerKg,
		}
	}

	orders, err := u.InputOrderRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading input orders of cooperative %s: %w", cooperativeID, err)
	}
	statusOf := make(map[string]constants.OrderStatus, len(orders))
	orderIDs := make([]string, len(orders))
	for i, order := range orders {
		statusOf[order.ID] = order.Status
		orderIDs[i] = order.ID
	}

	lineRows, err := u.InputOrderRepository.FindLinesByOrderIDs(db, orderIDs)
	if err != nil {
		return agronomy.ImpactFigures{},
			fmt.Errorf("reading input order lines of cooperative %s: %w", cooperativeID, err)
	}
	lines := []agronomy.OrderLine{}
	for _, row := range lineRows {
		status, known := statusOf[row.InputOrderID]
		if !known {
			continue
		}
		lines = append(lines, agronomy.OrderLine{
			Quantity:           row.Quantity,
			RetailPricePerUnit: row.RetailPricePerUnit,
			BulkPricePerUnit:   row.BulkPricePerUnit,
			Status:             status,
		})
	}

	return agronomy.ComputeImpact(agronomy.ImpactInput{
		Blocks:          blocks,
		ReferencePrices: prices,
		OrderLines:      lines,
		StaggerApplied:  parseStaggerLog(cooperative.StaggerApplied),
		Projections:     projections,
		Capacity:        capacity,
	}), nil
}

type staggerLogEntry struct {
	SeasonLabel  string `json:"season_label"`
	BlockID      string `json:"block_id"`
	OriginalDate string `json:"original_date"`
	ShiftedDate  string `json:"shifted_date"`
}

func parseStaggerLog(raw json.RawMessage) []agronomy.StaggerRecord {
	if len(raw) == 0 {
		return nil
	}

	entries := []staggerLogEntry{}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil
	}

	records := []agronomy.StaggerRecord{}
	for _, entry := range entries {
		if entry.BlockID == "" {
			continue
		}
		originalDate, err := agronomy.UTCDate(entry.OriginalDate)
		if err != nil {
			continue
		}
		shiftedDate, err := agronomy.UTCDate(entry.ShiftedDate)
		if err != nil {
			continue
		}

		records = append(records, agronomy.StaggerRecord{
			SeasonLabel:  entry.SeasonLabel,
			BlockID:      entry.BlockID,
			OriginalDate: originalDate,
			ShiftedDate:  shiftedDate,
		})
	}
	return records
}
