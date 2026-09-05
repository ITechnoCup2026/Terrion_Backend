package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/rdkk"
	"terrion-backend/internal/repository"
)

var ErrNothingToOrder = errors.New(constants.RdkkNothingToOrder)

type RdkkUseCase struct {
	DB                       *gorm.DB
	Log                      *logrus.Logger
	CooperativeRepository    *repository.CooperativeRepository
	PlotRepository           *repository.PlotRepository
	BlockRepository          *repository.BlockRepository
	MemberRepository         *repository.MemberRepository
	FertiliserRateRepository *repository.FertiliserRateRepository
	InputOrderRepository     *repository.InputOrderRepository
}

func NewRdkkUseCase(
	db *gorm.DB, log *logrus.Logger,
	cooperativeRepository *repository.CooperativeRepository,
	plotRepository *repository.PlotRepository,
	blockRepository *repository.BlockRepository,
	memberRepository *repository.MemberRepository,
	fertiliserRateRepository *repository.FertiliserRateRepository,
	inputOrderRepository *repository.InputOrderRepository,
) *RdkkUseCase {
	return &RdkkUseCase{
		DB:                       db,
		Log:                      log,
		CooperativeRepository:    cooperativeRepository,
		PlotRepository:           plotRepository,
		BlockRepository:          blockRepository,
		MemberRepository:         memberRepository,
		FertiliserRateRepository: fertiliserRateRepository,
		InputOrderRepository:     inputOrderRepository,
	}
}

type Season struct {
	Label string
	Start time.Time
	End   time.Time
}

func DefaultSeason(now time.Time) Season {
	end := agronomy.StartOfDay(now)
	return Season{
		Label: constants.RdkkDefaultLabel,
		Start: agronomy.AddDays(end, -constants.RdkkSeasonDays),
		End:   end,
	}
}

type CreatedInputOrder struct {
	OrderID string
	Lines   int
}

func (u *RdkkUseCase) LoadSeason(
	ctx context.Context, cooperativeID string, season Season,
) (rdkk.Document, error) {
	aggregate, err := u.aggregateSeason(ctx, cooperativeID, season)
	if err != nil {
		return rdkk.Document{}, err
	}

	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(
		u.DB.WithContext(ctx), cooperative, cooperativeID); err != nil {
		return rdkk.Document{}, fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	return rdkk.BuildDocument(aggregate, rdkk.DocumentMeta{
		CooperativeName: cooperative.Name,
		Village:         cooperative.Village,
		District:        cooperative.District,
		Province:        cooperative.Province,
		SeasonLabel:     season.Label,
		PrintedAt:       time.Now(),
	}), nil
}

func (u *RdkkUseCase) CreateInputOrder(
	ctx context.Context, user *entity.AppUser, season Season,
) (CreatedInputOrder, error) {
	if user.CooperativeID == nil {
		return CreatedInputOrder{}, ErrNoCooperative
	}
	cooperativeID := *user.CooperativeID

	aggregate, err := u.aggregateSeason(ctx, cooperativeID, season)
	if err != nil {
		return CreatedInputOrder{}, err
	}

	drafts := rdkk.ToOrderLines(aggregate.Totals)
	if len(drafts) == 0 {
		return CreatedInputOrder{}, ErrNothingToOrder
	}

	order := &entity.InputOrder{
		ID:            uuid.NewString(),
		CooperativeID: cooperativeID,
		SeasonLabel:   season.Label,
		Status:        constants.OrderDraft,
	}

	lines := make([]entity.InputOrderLine, len(drafts))
	for i, draft := range drafts {
		lines[i] = entity.InputOrderLine{
			ID:           uuid.NewString(),
			InputOrderID: order.ID,
			Item:         draft.Item,
			Quantity:     draft.Quantity,
			Unit:         draft.Unit,
		}
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.InputOrderRepository.Create(tx, order); err != nil {
		return CreatedInputOrder{},
			fmt.Errorf("creating input order for cooperative %s: %w", cooperativeID, err)
	}
	if err := tx.Create(&lines).Error; err != nil {
		return CreatedInputOrder{},
			fmt.Errorf("creating lines of input order %s: %w", order.ID, err)
	}
	if err := tx.Commit().Error; err != nil {
		return CreatedInputOrder{},
			fmt.Errorf("committing input order %s: %w", order.ID, err)
	}

	return CreatedInputOrder{OrderID: order.ID, Lines: len(lines)}, nil
}

type InputOrderWithLines struct {
	Order entity.InputOrder
	Lines []entity.InputOrderLine
}

func (u *RdkkUseCase) ListInputOrders(
	ctx context.Context, user *entity.AppUser,
) ([]InputOrderWithLines, error) {
	if user.CooperativeID == nil {
		return nil, ErrNoCooperative
	}
	db := u.DB.WithContext(ctx)

	orders, err := u.InputOrderRepository.FindByCooperativeID(db, *user.CooperativeID)
	if err != nil {
		return nil, fmt.Errorf(
			"reading input orders of cooperative %s: %w", *user.CooperativeID, err)
	}
	if len(orders) == 0 {
		return []InputOrderWithLines{}, nil
	}

	orderIDs := make([]string, len(orders))
	for i, order := range orders {
		orderIDs[i] = order.ID
	}
	lines, err := u.InputOrderRepository.FindLinesByOrderIDs(db, orderIDs)
	if err != nil {
		return nil, fmt.Errorf("reading input order lines: %w", err)
	}
	linesByOrder := make(map[string][]entity.InputOrderLine, len(orders))
	for _, line := range lines {
		linesByOrder[line.InputOrderID] = append(linesByOrder[line.InputOrderID], line)
	}

	result := make([]InputOrderWithLines, len(orders))
	for i, order := range orders {
		result[i] = InputOrderWithLines{Order: order, Lines: linesByOrder[order.ID]}
	}
	return result, nil
}

func (u *RdkkUseCase) aggregateSeason(
	ctx context.Context, cooperativeID string, season Season,
) (rdkk.Aggregate, error) {
	db := u.DB.WithContext(ctx)

	plots, err := u.PlotRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return rdkk.Aggregate{},
			fmt.Errorf("reading plots of cooperative %s: %w", cooperativeID, err)
	}
	if len(plots) == 0 {
		return rdkk.Aggregate{}, nil
	}

	memberOfPlot := make(map[string]string, len(plots))
	for _, plot := range plots {
		memberOfPlot[plot.ID] = plot.MemberID
	}

	members, err := u.MemberRepository.FindByCooperativeID(db, cooperativeID)
	if err != nil {
		return rdkk.Aggregate{},
			fmt.Errorf("reading members of cooperative %s: %w", cooperativeID, err)
	}
	nameOfMember := make(map[string]string, len(members))
	for _, member := range members {
		nameOfMember[member.ID] = member.Name
	}

	blocks, err := u.BlockRepository.FindPlantedInSeason(
		db, plotIDsOf(plots), season.Start, season.End)
	if err != nil {
		return rdkk.Aggregate{},
			fmt.Errorf("reading blocks planted in %s: %w", season.Label, err)
	}

	planted := []rdkk.PlantedBlock{}
	for _, block := range blocks {
		memberID, known := memberOfPlot[block.PlotID]
		if !known {
			continue
		}
		name, named := nameOfMember[memberID]
		if !named {
			name = constants.MemberWithoutName
		}

		planted = append(planted, rdkk.PlantedBlock{
			BlockID:     block.ID,
			MemberID:    memberID,
			MemberName:  name,
			CommodityID: block.CommodityID,
			AreaHa:      block.AreaHa,
		})
	}

	rateRows, err := u.FertiliserRateRepository.FindAll(db)
	if err != nil {
		return rdkk.Aggregate{}, fmt.Errorf("reading fertiliser rates: %w", err)
	}
	rates := make([]rdkk.FertiliserRate, len(rateRows))
	for i, row := range rateRows {
		rates[i] = rdkk.FertiliserRate{
			CommodityID: row.CommodityID,
			InputItem:   row.InputItem,
			KgPerHa:     row.KgPerHa,
			Source:      row.Source,
		}
	}

	return rdkk.AggregateInputs(planted, rates), nil
}
