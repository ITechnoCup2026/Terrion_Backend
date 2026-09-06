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
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/rdkk"
	"terrion-backend/internal/repository"
)

var (
	ErrNothingToOrder         = errors.New(constants.RdkkNothingToOrder)
	ErrOrderNotFound          = errors.New(constants.OrderNotFound)
	ErrOrderTransitionInvalid = errors.New(constants.OrderTransitionInvalid)
	ErrOrderAlreadyFinal      = errors.New(constants.OrderAlreadyFinal)
	ErrOrderSeasonAlreadyOpen = errors.New(constants.OrderSeasonAlreadyOpen)
	ErrOrderLineUnknown       = errors.New(constants.OrderLineUnknown)
	ErrOrderLinesEmpty        = errors.New(constants.OrderLinesEmpty)
)

type RdkkUseCase struct {
	DB                       *gorm.DB
	Log                      *logrus.Logger
	Validate                 *validator.Validate
	CooperativeRepository    *repository.CooperativeRepository
	PlotRepository           *repository.PlotRepository
	BlockRepository          *repository.BlockRepository
	MemberRepository         *repository.MemberRepository
	FertiliserRateRepository *repository.FertiliserRateRepository
	InputOrderRepository     *repository.InputOrderRepository
}

func NewRdkkUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
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
		Validate:                 validate,
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
	request *model.CreateInputOrderRequest,
) (CreatedInputOrder, error) {
	if request != nil {
		if err := u.Validate.Struct(request); err != nil {
			return CreatedInputOrder{}, err
		}
	}
	if user.CooperativeID == nil {
		return CreatedInputOrder{}, ErrNoCooperative
	}
	cooperativeID := *user.CooperativeID
	db := u.DB.WithContext(ctx)

	_, err := u.InputOrderRepository.FindOpenBySeason(db, cooperativeID, season.Label)
	if err == nil {
		return CreatedInputOrder{}, ErrOrderSeasonAlreadyOpen
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return CreatedInputOrder{},
			fmt.Errorf("checking open orders for %s: %w", season.Label, err)
	}

	aggregate, err := u.aggregateSeason(ctx, cooperativeID, season)
	if err != nil {
		return CreatedInputOrder{}, err
	}

	drafts := rdkk.ToOrderLines(aggregate.Totals)
	if len(drafts) == 0 {
		return CreatedInputOrder{}, ErrNothingToOrder
	}

	adjusted, err := applyLineAdjustments(drafts, request)
	if err != nil {
		return CreatedInputOrder{}, err
	}
	if len(adjusted) == 0 {
		return CreatedInputOrder{}, ErrOrderLinesEmpty
	}

	order := &entity.InputOrder{
		ID:            uuid.NewString(),
		CooperativeID: cooperativeID,
		SeasonLabel:   season.Label,
		Status:        constants.OrderDraft,
		CreatedByID:   &user.ID,
		CreatedByName: &user.FullName,
	}

	lines := make([]entity.InputOrderLine, len(adjusted))
	for i, line := range adjusted {
		lines[i] = entity.InputOrderLine{
			ID:           uuid.NewString(),
			InputOrderID: order.ID,
			Item:         line.Item,
			Quantity:     line.Quantity,
			Unit:         line.Unit,
			QuantityRdkk: line.QuantityRdkk,
		}
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.InputOrderRepository.Create(tx, order); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			return CreatedInputOrder{}, ErrOrderSeasonAlreadyOpen
		}
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

type adjustedOrderLine struct {
	Item         string
	Quantity     float64
	Unit         string
	QuantityRdkk *float64
}

func applyLineAdjustments(
	drafts []rdkk.OrderLineDraft, request *model.CreateInputOrderRequest,
) ([]adjustedOrderLine, error) {
	adjustmentOf := map[string]float64{}
	if request != nil {
		for _, line := range request.Lines {
			adjustmentOf[line.Item] = line.Quantity
		}
	}

	adjusted := make([]adjustedOrderLine, 0, len(drafts))
	seen := map[string]bool{}
	for _, draft := range drafts {
		quantity := draft.Quantity
		var quantityRdkk *float64
		if requestedQuantity, requested := adjustmentOf[draft.Item]; requested {
			seen[draft.Item] = true
			if requestedQuantity != draft.Quantity {
				original := draft.Quantity
				quantityRdkk = &original
			}
			quantity = requestedQuantity
		}
		if quantity == 0 {
			continue
		}
		adjusted = append(adjusted, adjustedOrderLine{
			Item: draft.Item, Quantity: quantity, Unit: draft.Unit, QuantityRdkk: quantityRdkk,
		})
	}

	if request != nil {
		for _, line := range request.Lines {
			if !seen[line.Item] {
				return nil, ErrOrderLineUnknown
			}
		}
	}

	return adjusted, nil
}

func (u *RdkkUseCase) UpdateInputOrderStatus(
	ctx context.Context, user *entity.AppUser, orderID string,
	request *model.UpdateInputOrderStatusRequest, now time.Time,
) error {
	if err := u.Validate.Struct(request); err != nil {
		return err
	}
	if user.CooperativeID == nil {
		return ErrNoCooperative
	}

	stored := new(entity.InputOrder)
	if err := u.InputOrderRepository.FindById(u.DB.WithContext(ctx), stored, orderID); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOrderNotFound
		}
		return fmt.Errorf("reading input order %s: %w", orderID, err)
	}
	if stored.CooperativeID != *user.CooperativeID {
		return ErrOrderNotFound
	}
	if stored.Status == constants.OrderCompleted || stored.Status == constants.OrderCancelled {
		return ErrOrderAlreadyFinal
	}
	if !rdkk.CanTransitionOrder(stored.Status, request.Status) {
		return ErrOrderTransitionInvalid
	}

	changedAt := now.UTC()

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	result := tx.Model(&entity.InputOrder{}).
		Where("id = ? AND cooperative_id = ? AND status = ?", orderID, *user.CooperativeID, stored.Status).
		Updates(map[string]any{
			"status":                 request.Status,
			"status_changed_at":      changedAt,
			"status_changed_by_id":   user.ID,
			"status_changed_by_name": user.FullName,
		})
	if result.Error != nil {
		return fmt.Errorf("updating input order %s: %w", orderID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOrderTransitionInvalid
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing input order %s status change: %w", orderID, err)
	}
	return nil
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
