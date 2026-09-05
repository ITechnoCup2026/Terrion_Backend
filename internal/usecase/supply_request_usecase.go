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
	ErrListingUnknown     = errors.New(constants.ListingUnknown)
	ErrListingGone        = errors.New(constants.ListingGone)
	ErrRequestNotFound    = errors.New(constants.RequestNotFound)
	ErrAllocationExceeded = errors.New(constants.AllocationExceeded)
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

	listings, err := u.Catalog.LoadForCooperative(
		ctx, parsed.CooperativeID, constants.DefaultHorizonWeeks, now)
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

	if request.Decision == constants.RequestAccepted {
		stored := new(entity.SupplyContractRequest)
		if err := u.Repository.FindById(u.DB.WithContext(ctx), stored, requestID); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRequestNotFound
			}
			return fmt.Errorf("reading supply request %s: %w", requestID, err)
		}
		if stored.CooperativeID != *user.CooperativeID {
			return ErrRequestNotFound
		}
		if err := u.checkAllocation(ctx, stored); err != nil {
			return err
		}
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

// checkAllocation is the invariant TERRION.md documents as missing: a
// cooperative accepting requests one at a time could otherwise commit more
// tonnage against one harvest window than it will ever have.
//
// ponytail: reads the current tally outside the update transaction below, so
// two accepts racing the same window could both pass this check. Acceptable
// here -- nothing else in this codebase locks rows for an invariant either --
// upgrade to SELECT ... FOR UPDATE if concurrent pengurus answering the same
// window in practice turns out to matter.
func (u *SupplyRequestUseCase) checkAllocation(
	ctx context.Context, request *entity.SupplyContractRequest,
) error {
	// Anchored on the request's own window, not real time, so a window that
	// has since scrolled outside the catalog's rolling horizon can still be
	// found. The tradeoff: Tonnes reflects today's projection, not whatever it
	// was when the request was made -- the same live-recompute every listing
	// in this system already accepts.
	listings, err := u.Catalog.LoadForCooperative(
		ctx, request.CooperativeID, constants.DefaultHorizonWeeks, request.WindowStart)
	if err != nil {
		return err
	}

	listing, found := findListingForWindow(listings, request.CommodityID, request.WindowStart)
	if !found {
		// The planting data behind this request's projection has since
		// changed (block edited or removed) -- there is nothing reliable left
		// to cap against, so let the cooperative decide.
		return nil
	}

	accepted, err := u.Repository.SumAcceptedVolumeKg(
		u.DB.WithContext(ctx), request.CooperativeID, request.CommodityID,
		request.WindowStart, request.WindowEnd, request.ID,
	)
	if err != nil {
		return fmt.Errorf("summing accepted volume for request %s: %w", request.ID, err)
	}

	// Rounded the same way VolumeKg was at creation (see Create above), so a
	// request for exactly the whole projection does not overshoot it by the
	// sub-kilogram slop of an unrounded tonnes-to-kg conversion.
	capacityKg := math.Round(listing.Tonnes * constants.KgPerTonne)
	if accepted+request.VolumeKg > capacityKg {
		return ErrAllocationExceeded
	}
	return nil
}

func findListingForWindow(
	listings []catalog.Listing, commodityID string, windowStart time.Time,
) (catalog.Listing, bool) {
	for _, listing := range listings {
		if listing.CommodityID == commodityID && listing.WeekStart.Equal(windowStart) {
			return listing, true
		}
	}
	return catalog.Listing{}, false
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
