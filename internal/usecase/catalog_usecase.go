package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/catalog"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/repository"
)

type CatalogUseCase struct {
	DB                    *gorm.DB
	Log                   *logrus.Logger
	Redis                 *redis.Client
	CooperativeRepository *repository.CooperativeRepository
	CommodityRepository   *repository.CommodityRepository
	BlockRepository       *repository.BlockRepository
	VarietyRepository     *repository.VarietyRepository
	Projection            *ProjectionUseCase
}

func NewCatalogUseCase(
	db *gorm.DB, log *logrus.Logger, cache *redis.Client,
	cooperativeRepository *repository.CooperativeRepository,
	commodityRepository *repository.CommodityRepository,
	blockRepository *repository.BlockRepository,
	varietyRepository *repository.VarietyRepository,
	projection *ProjectionUseCase,
) *CatalogUseCase {
	return &CatalogUseCase{
		DB:                    db,
		Log:                   log,
		Redis:                 cache,
		CooperativeRepository: cooperativeRepository,
		CommodityRepository:   commodityRepository,
		BlockRepository:       blockRepository,
		VarietyRepository:     varietyRepository,
		Projection:            projection,
	}
}

type CatalogCommodity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Catalog struct {
	Listings    []catalog.Listing  `json:"listings"`
	Commodities []CatalogCommodity `json:"commodities"`
	Provinces   []string           `json:"provinces"`
}

func (u *CatalogUseCase) Load(
	ctx context.Context, weeks int, now time.Time,
) (Catalog, error) {
	key := constants.CatalogCacheKey + agronomy.ToISODate(now) +
		":" + strconv.Itoa(weeks)

	if cached, found := u.readCache(ctx, key); found {
		return cached, nil
	}

	cooperatives, err := u.CooperativeRepository.FindAll(u.DB.WithContext(ctx))
	if err != nil {
		return Catalog{}, fmt.Errorf("reading cooperatives: %w", err)
	}

	sources, err := u.sourcesFor(ctx, cooperatives, now)
	if err != nil {
		return Catalog{}, err
	}

	built := Catalog{
		Listings:    catalog.BuildListings(sources, now, weeks),
		Commodities: []CatalogCommodity{},
		Provinces:   []string{},
	}
	built.Commodities, built.Provinces = filterOptionsOf(built.Listings)

	u.writeCache(ctx, key, built)
	return built, nil
}

func (u *CatalogUseCase) LoadForCooperative(
	ctx context.Context, cooperativeID string, weeks int, now time.Time,
) ([]catalog.Listing, error) {
	cooperative := new(entity.Cooperative)
	if err := u.CooperativeRepository.FindById(
		u.DB.WithContext(ctx), cooperative, cooperativeID); err != nil {
		return nil, fmt.Errorf("reading cooperative %s: %w", cooperativeID, err)
	}

	sources, err := u.sourcesFor(ctx, []entity.Cooperative{*cooperative}, now)
	if err != nil {
		return nil, err
	}

	return catalog.BuildListings(sources, now, weeks), nil
}

func (u *CatalogUseCase) sourcesFor(
	ctx context.Context, cooperatives []entity.Cooperative, now time.Time,
) ([]catalog.Source, error) {
	db := u.DB.WithContext(ctx)

	commodityRows, err := u.CommodityRepository.FindAll(db)
	if err != nil {
		return nil, fmt.Errorf("reading commodities: %w", err)
	}
	commodityNames := make(map[string]string, len(commodityRows))
	for _, row := range commodityRows {
		commodityNames[row.ID] = row.Name
	}

	sources := []catalog.Source{}
	for _, cooperative := range cooperatives {
		projection, err := u.Projection.ProjectCooperative(ctx, cooperative.ID, now)
		if err != nil {
			return nil, err
		}
		if len(projection.Projections) == 0 {
			continue
		}

		varietyByBlock, err := u.varietyNamesOf(db, projection.Blocks)
		if err != nil {
			return nil, err
		}

		climatology := map[string]bool{}
		for blockID, window := range projection.Windows {
			if window.Basis == constants.BasisClimatology {
				climatology[blockID] = true
			}
		}

		sources = append(sources, catalog.Source{
			CooperativeID:     cooperative.ID,
			CooperativeName:   cooperative.Name,
			Province:          cooperative.Province,
			District:          cooperative.District,
			Village:           cooperative.Village,
			Projections:       projection.Projections,
			VarietyByBlock:    varietyByBlock,
			ClimatologyBlocks: climatology,
			CommodityNames:    commodityNames,
		})
	}
	return sources, nil
}

func (u *CatalogUseCase) varietyNamesOf(
	db *gorm.DB, blocks []entity.Block,
) (map[string]string, error) {
	varietyIDs := map[string]bool{}
	for _, block := range blocks {
		varietyIDs[block.VarietyID] = true
	}

	varieties, err := u.VarietyRepository.FindByIDs(db, keysOf(varietyIDs))
	if err != nil {
		return nil, fmt.Errorf("reading varieties: %w", err)
	}
	nameOf := make(map[string]string, len(varieties))
	for _, variety := range varieties {
		nameOf[variety.ID] = variety.Name
	}

	byBlock := make(map[string]string, len(blocks))
	for _, block := range blocks {
		if name, known := nameOf[block.VarietyID]; known {
			byBlock[block.ID] = name
		}
	}
	return byBlock, nil
}

func filterOptionsOf(listings []catalog.Listing) ([]CatalogCommodity, []string) {
	commodities := map[string]string{}
	provinces := map[string]bool{}

	for _, listing := range listings {
		commodities[listing.CommodityID] = listing.CommodityName
		provinces[listing.Province] = true
	}

	options := make([]CatalogCommodity, 0, len(commodities))
	for id, name := range commodities {
		options = append(options, CatalogCommodity{ID: id, Name: name})
	}
	sort.Slice(options, func(i, j int) bool { return options[i].Name < options[j].Name })

	names := make([]string, 0, len(provinces))
	for province := range provinces {
		names = append(names, province)
	}
	sort.Strings(names)

	return options, names
}

func (u *CatalogUseCase) readCache(ctx context.Context, key string) (Catalog, bool) {
	if u.Redis == nil {
		return Catalog{}, false
	}

	raw, err := u.Redis.Get(ctx, key).Bytes()
	if err != nil {
		return Catalog{}, false
	}

	cached := Catalog{}
	if err := json.Unmarshal(raw, &cached); err != nil {
		u.Log.Warnf("discarding unreadable cached catalogue at %s: %v", key, err)
		return Catalog{}, false
	}
	return cached, true
}

func (u *CatalogUseCase) writeCache(ctx context.Context, key string, built Catalog) {
	if u.Redis == nil {
		return
	}

	raw, err := json.Marshal(built)
	if err != nil {
		u.Log.Warnf("not caching the catalogue: %v", err)
		return
	}
	if err := u.Redis.Set(ctx, key, raw, constants.CatalogCacheTTL).Err(); err != nil {
		u.Log.Warnf("not caching the catalogue at %s: %v", key, err)
	}
}

func (u *CatalogUseCase) Invalidate(ctx context.Context, now time.Time) {
	if u.Redis == nil {
		return
	}

	pattern := constants.CatalogCacheKey + agronomy.ToISODate(now) + "*"
	keys, err := u.Redis.Keys(ctx, pattern).Result()
	if err != nil {
		u.Log.Errorf("scanning catalogue cache keys: %v", err)
		return
	}
	if len(keys) == 0 {
		return
	}
	if err := u.Redis.Del(ctx, keys...).Err(); err != nil {
		u.Log.Errorf("clearing catalogue cache: %v", err)
	}
}
