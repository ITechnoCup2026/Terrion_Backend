package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/catalog"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/model"
	"terrion-backend/internal/repository"
)

var (
	ErrListingUnknown  = errors.New(constants.ListingUnknown)
	ErrListingGone     = errors.New(constants.ListingGone)
	ErrRequestNotFound = errors.New(constants.RequestNotFound)
)

type SupplyRequestUseCase struct {
	DB         *gorm.DB
	Log        *logrus.Logger
	Validate   *validator.Validate
	Repository *repository.SupplyRequestRepository
	Catalog    *CatalogUseCase
}

func NewSupplyRequestUseCase(
	db *gorm.DB, log *logrus.Logger, validate *validator.Validate,
	supplyRequestRepository *repository.SupplyRequestRepository,
	catalogUseCase *CatalogUseCase,
) *SupplyRequestUseCase {
	return &SupplyRequestUseCase{
		DB:         db,
		Log:        log,
		Validate:   validate,
		Repository: supplyRequestRepository,
		Catalog:    catalogUseCase,
	}
}

func (u *SupplyRequestUseCase) List(
	ctx context.Context, user *entity.AppUser,
) ([]entity.SupplyContractRequest, error) {
	db := u.DB.WithContext(ctx)

	if user.CooperativeID != nil {
		requests, err := u.Repository.FindForCooperative(db, *user.CooperativeID)
		if err != nil {
			return nil, fmt.Errorf("reading requests of cooperative %s: %w",
				*user.CooperativeID, err)
		}
		return requests, nil
	}

	requests, err := u.Repository.FindForBuyer(db, user.ID)
	if err != nil {
		return nil, fmt.Errorf("reading requests of buyer %s: %w", user.ID, err)
	}
	return requests, nil
}

func (u *SupplyRequestUseCase) Create(
	ctx context.Context, user *entity.AppUser,
	request *model.CreateSupplyRequestRequest, now time.Time,
) (*entity.SupplyContractRequest, error) {
	if err := u.Validate.Struct(request); err != nil {
		return nil, err
	}

	parsed, recognised := catalog.ParseListingID(request.ListingID)
	if !recognised {
		return nil, ErrListingUnknown
	}

	listings, err := u.Catalog.LoadForCooperative(ctx, parsed.CooperativeID, now)
	if err != nil {
		return nil, err
	}

	listing, offered := findListing(listings, request.ListingID)
	if !offered {
		return nil, ErrListingGone
	}

	stored := &entity.SupplyContractRequest{
		ID:                uuid.NewString(),
		CooperativeID:     listing.CooperativeID,
		BuyerID:           user.ID,
		BuyerName:         user.FullName,
		BuyerOrganisation: user.Organisation,
		CommodityID:       listing.CommodityID,
		VolumeKg:          math.Round(request.VolumeTonnes * constants.KgPerTonne),
		WindowStart:       listing.WeekStart,
		WindowEnd:         listing.WeekEnd,
		Status:            constants.RequestPending,
		Notes:             notesWithPreference(request.DeliveryPreference, request.Notes),
	}

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	if err := u.Repository.Create(tx, stored); err != nil {
		return nil, fmt.Errorf("creating supply request for buyer %s: %w", user.ID, err)
	}
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("committing supply request %s: %w", stored.ID, err)
	}

	return stored, nil
}

func (u *SupplyRequestUseCase) Respond(
	ctx context.Context, user *entity.AppUser,
	requestID string, request *model.RespondToRequestRequest, now time.Time,
) error {
	if err := u.Validate.Struct(request); err != nil {
		return err
	}
	if user.CooperativeID == nil {
		return ErrNoCooperative
	}

	respondedAt := now.UTC()

	tx := u.DB.WithContext(ctx).Begin()
	defer tx.Rollback()

	result := tx.Model(&entity.SupplyContractRequest{}).
		Where("id = ? AND cooperative_id = ?", requestID, *user.CooperativeID).
		Updates(map[string]any{
			"status":       request.Decision,
			"responded_at": respondedAt,
		})
	if result.Error != nil {
		return fmt.Errorf("answering supply request %s: %w", requestID, result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrRequestNotFound
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("committing answer to supply request %s: %w", requestID, err)
	}
	return nil
}

func findListing(listings []catalog.Listing, listingID string) (catalog.Listing, bool) {
	for _, listing := range listings {
		if listing.ID == listingID {
			return listing, true
		}
	}
	return catalog.Listing{}, false
}

func notesWithPreference(
	preference constants.DeliveryPreference, notes string,
) *string {
	label, known := constants.DeliveryPreferenceLabels[preference]
	if !known {
		label = constants.DeliveryPreferenceLabels[constants.DeliveryUndecided]
	}

	lines := []string{fmt.Sprintf("Preferensi pengiriman: %s.", label)}
	if trimmed := strings.TrimSpace(notes); trimmed != "" {
		lines = append(lines, trimmed)
	}

	composed := strings.Join(lines, "\n")
	return &composed
}
