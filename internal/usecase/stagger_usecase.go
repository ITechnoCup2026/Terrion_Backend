package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

var ErrSuggestionStale = errors.New(constants.StaggerSuggestionStale)

type StaggerRefusal struct {
	Code           string
	AlreadyPlanted int
	WouldBeInPast  int
}

func (r *StaggerRefusal) Error() string {
	return r.Code
}

type StaggerUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	Validate              *validator.Validate
	CooperativeRepository *repository.CooperativeRepository
	BlockRepository       *repository.BlockRepository
	Projection            *ProjectionUseCase
}

func NewStaggerUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	cooperativeRepository *repository.CooperativeRepository,
	blockRepository *repository.BlockRepository,
	projection *ProjectionUseCase,
) *StaggerUseCase {
	return &StaggerUseCase{
		DB:                    db,
		Log:                   log,
		Validate:              validate,
		CooperativeRepository: cooperativeRepository,
		BlockRepository:       blockRepository,
		Projection:            projection,
	}
}

type AppliedStagger struct {
	Shifted int
}

func (u *StaggerUseCase) Apply(
	ctx context.Context, user *entity.AppUser,
	request *model.ApplyStaggerRequest, now time.Time,
) (AppliedStagger, error) {
	if err := u.Validate.Struct(request); err != nil {
		return AppliedStagger{}, err
	}
	if !agronomy.IsISOWeekKey(request.ISOWeek) {
		return AppliedStagger{}, ErrSuggestionStale
	}
	if user.CooperativeID == nil {
		return AppliedStagger{}, ErrNoCooperative
	}
	cooperativeID := *user.CooperativeID

	suggestion, err := u.suggestionFor(ctx, cooperativeID, request, now)
	if err != nil {
		return AppliedStagger{}, err
	}

	candidates, err := u.ownedBlocks(ctx, cooperativeID, suggestion.BlockIDs)
	if err != nil {
		return AppliedStagger{}, err
	}

	plan := agronomy.PlanStagger(agronomy.StaggerPlanInput{
		BlockIDs:  suggestion.BlockIDs,
		ShiftDays: suggestion.ShiftDays,
		Blocks:    candidates,
		Today:     now,
	})
	if len(plan.Shifts) == 0 {
		return AppliedStagger{}, refusalFrom(plan.Refused)
	}

	if err := u.persistShifts(ctx, cooperativeID, plan.Shifts); err != nil {
		return AppliedStagger{}, err
	}

	return AppliedStagger{Shifted: len(plan.Shifts)}, nil
}

func (u *StaggerUseCase) suggestionFor(
	ctx context.Context, cooperativeID string,
	request *model.ApplyStaggerRequest, now time.Time,
) (agronomy.StaggerSuggestion, error) {
	projection, err := u.Projection.ProjectCooperative(ctx, cooperativeID, now)
	if err != nil {
		return agronomy.StaggerSuggestion{}, err
	}

	capacityRows, err := u.CooperativeRepository.FindCapacity(
		u.DB.WithContext(ctx), cooperativeID)
	if err != nil {
		return agronomy.StaggerSuggestion{},
			fmt.Errorf("reading capacity of cooperative %s: %w", cooperativeID, err)
	}
	capacity := make(map[string]float64, len(capacityRows))
	for _, row := range capacityRows {
		capacity[row.CommodityID] = row.TonnesPerWeek
	}

	for _, suggestion := range agronomy.DetectCollisions(
		projection.Projections, capacity).Suggestions {
		if suggestion.ISOWeek == request.ISOWeek &&
			suggestion.CommodityID == request.CommodityID {
			return suggestion, nil
		}
	}

	return agronomy.StaggerSuggestion{}, ErrSuggestionStale
}

func (u *StaggerUseCase) ownedBlocks(
	ctx context.Context, cooperativeID string, blockIDs []string,
) ([]agronomy.ShiftCandidate, error) {
	blocks := []entity.Block{}
	err := u.DB.WithContext(ctx).Model(&entity.Block{}).
		Joins("JOIN plot ON plot.id = block.plot_id").
		Where("block.id IN ? AND plot.cooperative_id = ?", blockIDs, cooperativeID).
		Find(&blocks).Error
	if err != nil {
		return nil, fmt.Errorf("reading blocks of cooperative %s: %w", cooperativeID, err)
	}

	candidates := make([]agronomy.ShiftCandidate, len(blocks))
	for i, block := range blocks {
		candidates[i] = agronomy.ShiftCandidate{
			BlockID:      block.ID,
			PlantingDate: block.PlantingDate,
		}
	}
	return candidates, nil
}

func (u *StaggerUseCase) persistShifts(
	ctx context.Context, cooperativeID string, shifts []agronomy.PlannedShift,
) error {
	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	for _, shift := range shifts {
		if err := tx.Model(&entity.Block{}).Where("id = ?", shift.BlockID).
			Update("planting_date", shift.ShiftedDate).Error; err != nil {
			return fmt.Errorf("shifting block %s: %w", shift.BlockID, err)
		}
	}

	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(tx, cooperative, cooperativeID); err != nil {
		return fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	log := append(parseStaggerLogEntries(cooperative.StaggerApplied), entriesFor(shifts)...)
	encoded, err := json.Marshal(log)
	if err != nil {
		return fmt.Errorf("encoding the stagger log of cooperative %s: %w", cooperativeID, err)
	}

	if err := tx.Model(&entity.Cooperative{}).Where("id = ?", cooperativeID).
		Update("stagger_applied", json.RawMessage(encoded)).Error; err != nil {
		return fmt.Errorf("recording the stagger of cooperative %s: %w", cooperativeID, err)
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing the stagger of cooperative %s: %w", cooperativeID, err)
	}
	return nil
}

func entriesFor(shifts []agronomy.PlannedShift) []staggerLogEntry {
	entries := make([]staggerLogEntry, len(shifts))
	for i, shift := range shifts {
		entries[i] = staggerLogEntry{
			SeasonLabel: constants.StaggerSeasonPrefix +
				strconv.Itoa(shift.ShiftedDate.UTC().Year()),
			BlockID:      shift.BlockID,
			OriginalDate: agronomy.ToISODate(shift.OriginalDate),
			ShiftedDate:  agronomy.ToISODate(shift.ShiftedDate),
		}
	}
	return entries
}

func parseStaggerLogEntries(raw json.RawMessage) []staggerLogEntry {
	entries := []staggerLogEntry{}
	if len(raw) == 0 {
		return entries
	}
	if err := json.Unmarshal(raw, &entries); err != nil {
		return []staggerLogEntry{}
	}
	return entries
}

func refusalFrom(refused []agronomy.Refusal) *StaggerRefusal {
	refusal := &StaggerRefusal{Code: constants.StaggerNothingToShift}

	for _, entry := range refused {
		switch entry.Reason {
		case constants.RefusedAlreadyPlanted:
			refusal.AlreadyPlanted++
		case constants.RefusedWouldBeInPast:
			refusal.WouldBeInPast++
		}
	}
	return refusal
}
