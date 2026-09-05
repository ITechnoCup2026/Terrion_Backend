package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/config"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/plots"
	"terrion-backend/internal/repository"
	"terrion-backend/internal/supabase"
	"terrion-backend/internal/usecase"
	"terrion-backend/internal/weather"
)

type harvestSpec struct {
	daysAfterPlanting int
	yieldPerHa        float64
	pricePerKg        float64
	paymentAfterDays  int
}

type blockSpec struct {
	commodity      string
	variety        string
	areaHa         float64
	plantedDaysAgo int
	harvest        *harvestSpec
}

type plotSpec struct {
	member string
	name   string
	lat    float64
	lng    float64
	blocks []blockSpec
}

type cooperativeSpec struct {
	name     string
	village  string
	district string
	province string
	lat      float64
	lng      float64
	capacity map[string]float64
	plots    []plotSpec
}

type accountSpec struct {
	role         constants.UserRole
	email        string
	fullName     string
	organisation string
	cooperative  int
}

type seededCooperative struct {
	id      string
	spec    cooperativeSpec
	plotIDs map[string]string
	members map[string]string
	blocks  int
}

const (
	subangName = "KUD Tani Makmur Subang"
	brebesName = "KUD Sumber Rejeki Brebes"
	buyerName  = "Pak Budi Santoso"
)

func main() {
	reset := flag.Bool("reset", false, "delete demo data and demo accounts before seeding")
	resetOnly := flag.Bool("reset-only", false, "delete demo data and stop")
	withAccounts := flag.Bool("accounts", true, "create the Supabase demo accounts")
	withWeather := flag.Bool("weather", true, "backfill Open-Meteo weather for each grid cell")
	password := flag.String("password", "terrion-demo-2026", "password for every demo account")
	emailDomain := flag.String("email-domain", "terrion.test", "domain of the demo account emails")

	flag.Usage = usage
	flag.Parse()

	cfg := config.NewConfig()
	log := config.NewLogger(cfg)
	db := config.NewDatabase(cfg, log)
	goTrue := supabase.NewClient(
		cfg.Supabase.URL, cfg.Supabase.AnonKey, cfg.Supabase.ServiceRoleKey)

	ctx := context.Background()
	now := time.Now().UTC()

	if *reset || *resetOnly {
		if err := clear(ctx, db, goTrue, log); err != nil {
			log.Fatalf("clearing the demo data: %v", err)
		}
		if *resetOnly {
			fmt.Println("Data demo dihapus.")
			return
		}
	}

	specs := []cooperativeSpec{subang(), brebes()}
	seeded := make([]*seededCooperative, len(specs))

	for i, spec := range specs {
		created, err := plant(db, spec, now)
		if err != nil {
			log.Fatalf("seeding %s: %v", spec.name, err)
		}
		seeded[i] = created
		fmt.Printf("Koperasi %-26s %d lahan, %d blok\n",
			spec.name, len(created.plotIDs), created.blocks)
	}

	accounts := []accountSpec{}
	if *withAccounts {
		accounts = accountSpecs(*emailDomain)
		if err := register(ctx, db, goTrue, accounts, seeded, *password); err != nil {
			log.Fatalf("creating the demo accounts: %v", err)
		}
		fmt.Printf("Akun         %d dibuat\n", len(accounts))
	}

	if *withWeather {
		if err := fetchWeather(ctx, db, log, seeded, now); err != nil {
			log.Fatalf("backfilling weather: %v", err)
		}
	}

	projection := projectionUseCase(db, log)

	if *withWeather {
		fitted, err := calibrate(ctx, db, projection, seeded, now)
		if err != nil {
			log.Fatalf("fitting calibrations: %v", err)
		}
		fmt.Printf("Kalibrasi    %d varietas\n", fitted)
	}

	if err := orderInputs(db, seeded[0], now); err != nil {
		log.Fatalf("creating input orders: %v", err)
	}
	fmt.Printf("Pesanan      3 pesanan sarana produksi\n")

	if *withAccounts && *withWeather {
		requests, err := requestSupply(ctx, db, log, projection, seeded, now)
		if err != nil {
			log.Fatalf("creating supply requests: %v", err)
		}
		fmt.Printf("Permintaan   %d permintaan pasokan\n", requests)
	}

	report(db, seeded, accounts, *password)
}

func usage() {
	fmt.Fprint(os.Stderr, `usage: go run ./cmd/seed [flags]

Mengisi basis data dengan koperasi contoh yang lengkap, supaya setiap layar
Terrion punya sesuatu untuk ditampilkan saat diuji:

  - dua koperasi (Jawa Barat dan Jawa Tengah) untuk Atlas dan Katalog
  - anggota, lahan, dan blok pada enam komoditas dan dua belas varietas
  - panen lampau yang lengkap dengan harga dan tanggal bayar, supaya panel
    Dampak dan kalibrasi model terisi
  - tumpukan panen yang sengaja dibuat pada satu minggu, supaya deteksi
    tabrakan menyala dan penggeseran tanam bisa dicoba
  - pesanan sarana produksi dan permintaan pasokan pembeli

Jalankan dari akar repo, karena .env dibaca dari direktori kerja.

`)
	flag.PrintDefaults()
}

func accountSpecs(domain string) []accountSpec {
	return []accountSpec{
		{constants.RolePengurus, "pengurus@" + domain, "Bu Sri Wahyuni", "", 0},
		{constants.RoleKader, "kader@" + domain, "Pak Asep Suryana", "", 0},
		{constants.RolePengurus, "pengurus2@" + domain, "Pak Joko Purnomo", "", 1},
		{constants.RoleBuyer, "pembeli@" + domain, buyerName, "PT Pangan Nusantara", -1},
	}
}

func subang() cooperativeSpec {
	return cooperativeSpec{
		name:     subangName,
		village:  "Jalancagak",
		district: "Subang",
		province: "Jawa Barat",
		lat:      -6.4200,
		lng:      107.6800,
		capacity: map[string]float64{"padi": 18, "jagung": 12},
		plots: []plotSpec{
			{
				member: "Bu Siti Aminah", name: "Sawah Ciasem 1",
				lat: -6.4180, lng: 107.6820,
				blocks: []blockSpec{
					{"padi", "Ciherang", 1.20, 99, nil},
					{"padi", "IR64", 0.60, 60, nil},
					{"padi", "Ciherang", 1.20, 140,
						&harvestSpec{128, 6.4, 6800, 8}},
				},
			},
			{
				member: "Bu Siti Aminah", name: "Sawah Ciasem 2",
				lat: -6.4215, lng: 107.6795,
				blocks: []blockSpec{
					{"padi", "Ciherang", 0.90, 108, nil},
					{"padi", "Ciherang", 0.90, 150,
						&harvestSpec{131, 6.1, 6950, 14}},
				},
			},
			{
				member: "Pak Asep Suryana", name: "Kebun Jalancagak",
				lat: -6.4102, lng: 107.7010,
				blocks: []blockSpec{
					{"cabai", "Cabai rawit", 0.45, 72, nil},
					{"cabai", "Cabai merah", 0.35, 40, nil},
					{"cabai", "Cabai rawit", 0.45, 118,
						&harvestSpec{101, 8.0, 52000, 7}},
				},
			},
			{
				member: "Pak Dedi Mulyana", name: "Ladang Cipeundeuy",
				lat: -6.4390, lng: 107.7220,
				blocks: []blockSpec{
					{"jagung", "Bisi-18", 1.60, 58, nil},
					{"jagung", "Bisi-18", 1.60, 120,
						&harvestSpec{103, 8.2, 5200, 15}},
				},
			},
			{
				member: "Pak Dedi Mulyana", name: "Ladang Kalijati",
				lat: -6.4440, lng: 107.7405,
				blocks: []blockSpec{
					{"jagung", "Pioneer P35", 0.80, 27, nil},
					{"wortel", "Nantes", 0.40, 53, nil},
					{"jagung", "Pioneer P35", 0.80, 112,
						&harvestSpec{97, 8.6, 5300, 14}},
				},
			},
			{
				member: "Bu Euis Rohaeti", name: "Kebun Sagalaherang",
				lat: -6.3985, lng: 107.6640,
				blocks: []blockSpec{
					{"wortel", "Lokal Cipanas", 0.55, 74, nil},
					{"beri", "Stroberi lokal", 0.25, 48, nil},
					{"wortel", "Lokal Cipanas", 0.55, 110,
						&harvestSpec{95, 21.0, 9100, 11}},
				},
			},
			{
				member: "Pak Yayan Supriatna", name: "Sawah Pagaden",
				lat: -6.4610, lng: 107.7690,
				blocks: []blockSpec{
					{"padi", "IR64", 2.10, 102, nil},
					{"padi", "IR64", 2.10, 148,
						&harvestSpec{124, 5.8, 6700, 9}},
				},
			},
			{
				member: "Bu Nining Kurniasih", name: "Kebun Tanjungsiang",
				lat: -6.3920, lng: 107.7480,
				blocks: []blockSpec{
					{"kentang", "Granola", 0.70, 78, nil},
					{"kentang", "Atlantic", 0.50, 38, nil},
					{"kentang", "Granola", 0.70, 125,
						&harvestSpec{98, 19.0, 13400, 18}},
				},
			},
			{
				member: "Pak Ujang Hidayat", name: "Ladang Cisalak",
				lat: -6.4055, lng: 107.7810,
				blocks: []blockSpec{
					{"jagung", "Bisi-18", 1.10, 80, nil},
					{"beri", "Stroberi Kalifornia", 0.30, 20, nil},
					{"jagung", "Bisi-18", 1.10, 130,
						&harvestSpec{106, 7.9, 5100, 0}},
				},
			},
			{
				member: "Bu Wiwin Sartika", name: "Sawah Binong",
				lat: -6.4700, lng: 107.6910,
				blocks: []blockSpec{{"padi", "Ciherang", 1.40, 70, nil}},
			},
			{
				member: "Pak Cecep Firmansyah", name: "Sawah Compreng",
				lat: -6.4520, lng: 107.7115,
				blocks: []blockSpec{{"padi", "Ciherang", 1.80, 70, nil}},
			},
			{
				member: "Bu Rina Marlina", name: "Sawah Pusakanagara",
				lat: -6.4285, lng: 107.7955,
				blocks: []blockSpec{
					{"padi", "Ciherang", 2.20, 70, nil},
					{"padi", "Ciherang", 2.20, 133,
						&harvestSpec{126, 6.6, 7100, 5}},
				},
			},
			{
				member: "Pak Yayan Supriatna", name: "Sawah Patokbeusi",
				lat: -6.4655, lng: 107.7350,
				blocks: []blockSpec{{"padi", "Ciherang", 1.60, 70, nil}},
			},
			{
				member: "Pak Ujang Hidayat", name: "Sawah Blanakan",
				lat: -6.4790, lng: 107.6725,
				blocks: []blockSpec{{"padi", "Ciherang", 1.90, 70, nil}},
			},
		},
	}
}

func brebes() cooperativeSpec {
	return cooperativeSpec{
		name:     brebesName,
		village:  "Bumiayu",
		district: "Brebes",
		province: "Jawa Tengah",
		lat:      -7.2000,
		lng:      108.9800,
		capacity: map[string]float64{},
		plots: []plotSpec{
			{
				member: "Pak Slamet Riyadi", name: "Sawah Bumiayu",
				lat: -7.2035, lng: 108.9760,
				blocks: []blockSpec{{"padi", "Ciherang", 1.30, 88, nil}},
			},
			{
				member: "Bu Wiwik Handayani", name: "Ladang Sirampog",
				lat: -7.1880, lng: 109.0120,
				blocks: []blockSpec{
					{"jagung", "Bisi-18", 0.95, 66, nil},
					{"cabai", "Cabai merah", 0.40, 44, nil},
				},
			},
			{
				member: "Pak Bambang Sutrisno", name: "Kebun Paguyangan",
				lat: -7.2240, lng: 108.9405,
				blocks: []blockSpec{{"kentang", "Granola", 0.85, 75, nil}},
			},
			{
				member: "Bu Endang Lestari", name: "Kebun Salem",
				lat: -7.1655, lng: 108.9550,
				blocks: []blockSpec{
					{"wortel", "Nantes", 0.50, 60, nil},
					{"beri", "Stroberi Kalifornia", 0.20, 30, nil},
				},
			},
			{
				member: "Pak Slamet Riyadi", name: "Sawah Tonjong",
				lat: -7.2410, lng: 109.0035,
				blocks: []blockSpec{{"padi", "IR64", 1.10, 105, nil}},
			},
		},
	}
}

type reference struct {
	commodityID map[string]string
	varietyID   map[string]string
}

func loadReference(db *gorm.DB) (reference, error) {
	loaded := reference{
		commodityID: map[string]string{},
		varietyID:   map[string]string{},
	}

	commodities := []entity.Commodity{}
	if err := db.Find(&commodities).Error; err != nil {
		return loaded, fmt.Errorf("reading commodities: %w", err)
	}
	slugOf := map[string]string{}
	for _, commodity := range commodities {
		loaded.commodityID[commodity.Slug] = commodity.ID
		slugOf[commodity.ID] = commodity.Slug
	}
	if len(commodities) == 0 {
		return loaded, errors.New(
			"tabel commodity kosong: jalankan `go run cmd/migrate/main.go up` dulu")
	}

	varieties := []entity.Variety{}
	if err := db.Find(&varieties).Error; err != nil {
		return loaded, fmt.Errorf("reading varieties: %w", err)
	}
	for _, variety := range varieties {
		loaded.varietyID[slugOf[variety.CommodityID]+"|"+variety.Name] = variety.ID
	}

	return loaded, nil
}

func plant(db *gorm.DB, spec cooperativeSpec, now time.Time) (*seededCooperative, error) {
	catalogue, err := loadReference(db)
	if err != nil {
		return nil, err
	}

	seeded := &seededCooperative{
		spec:    spec,
		plotIDs: map[string]string{},
		members: map[string]string{},
	}

	tx := db.Begin()
	defer tx.Rollback()

	cooperative := &entity.Cooperative{
		ID:             uuid.NewString(),
		Name:           spec.name,
		Village:        spec.village,
		District:       spec.district,
		Province:       spec.province,
		Lat:            spec.lat,
		Lng:            spec.lng,
		StaggerApplied: json.RawMessage("[]"),
		CreatedAt:      agronomy.AddDays(now, -420),
	}
	if err := tx.Create(cooperative).Error; err != nil {
		return nil, fmt.Errorf("creating the cooperative: %w", err)
	}
	seeded.id = cooperative.ID

	random := rand.New(rand.NewSource(int64(len(spec.name))))

	for _, wanted := range spec.plots {
		memberID, known := seeded.members[wanted.member]
		if !known {
			member := &entity.Member{
				ID:            uuid.NewString(),
				CooperativeID: cooperative.ID,
				Name:          wanted.member,
				CreatedAt:     agronomy.AddDays(now, -410),
			}
			if err := tx.Create(member).Error; err != nil {
				return nil, fmt.Errorf("creating member %q: %w", wanted.member, err)
			}
			seeded.members[wanted.member] = member.ID
			memberID = member.ID
		}

		standing := 0.0
		for _, block := range wanted.blocks {
			if block.harvest == nil {
				standing += block.areaHa
			}
		}

		plot := &entity.Plot{
			ID:            uuid.NewString(),
			CooperativeID: cooperative.ID,
			MemberID:      memberID,
			PublicID:      uuid.NewString()[:constants.PublicIDLength],
			Name:          wanted.name,
			AreaHa:        plots.RoundArea(standing),
			Lat:           wanted.lat,
			Lng:           wanted.lng,
			TerrainSeed:   random.Intn(2147483647),
			Decorations:   json.RawMessage("[]"),
			CreatedAt:     agronomy.AddDays(now, -400),
		}
		if err := tx.Create(plot).Error; err != nil {
			return nil, fmt.Errorf("creating plot %q: %w", wanted.name, err)
		}
		seeded.plotIDs[wanted.name] = plot.ID

		for index, wantedBlock := range wanted.blocks {
			commodityID, named := catalogue.commodityID[wantedBlock.commodity]
			if !named {
				return nil, fmt.Errorf("komoditas %q tidak ada di tabel acuan",
					wantedBlock.commodity)
			}
			varietyID, planted := catalogue.varietyID[wantedBlock.commodity+"|"+wantedBlock.variety]
			if !planted {
				return nil, fmt.Errorf("varietas %q tidak ada di tabel acuan",
					wantedBlock.variety)
			}

			plantingDate := agronomy.AddDays(agronomy.StartOfDay(now), -wantedBlock.plantedDaysAgo)

			block := &entity.Block{
				ID:           uuid.NewString(),
				PlotID:       plot.ID,
				Label:        plots.BlockLabel(index),
				AreaHa:       wantedBlock.areaHa,
				OrderIndex:   index,
				CommodityID:  commodityID,
				VarietyID:    varietyID,
				PlantingDate: plantingDate,
			}

			if recorded := wantedBlock.harvest; recorded != nil {
				harvestDate := agronomy.AddDays(plantingDate, recorded.daysAfterPlanting)
				yieldKg := math.Round(recorded.yieldPerHa * wantedBlock.areaHa * 1000)
				price := recorded.pricePerKg

				block.ActualHarvestDate = &harvestDate
				block.ActualYieldKg = &yieldKg
				block.ActualPricePerKg = &price

				if recorded.paymentAfterDays > 0 {
					paymentDate := agronomy.AddDays(harvestDate, recorded.paymentAfterDays)
					block.PaymentReceivedDate = &paymentDate
				}
			}

			if err := tx.Create(block).Error; err != nil {
				return nil, fmt.Errorf("creating a block of plot %q: %w", wanted.name, err)
			}
			seeded.blocks++
		}
	}

	for slug, tonnesPerWeek := range spec.capacity {
		commodityID, named := catalogue.commodityID[slug]
		if !named {
			continue
		}
		capacity := &entity.CooperativeCapacity{
			CooperativeID: cooperative.ID,
			CommodityID:   commodityID,
			TonnesPerWeek: tonnesPerWeek,
		}
		if err := tx.Create(capacity).Error; err != nil {
			return nil, fmt.Errorf("creating capacity for %q: %w", slug, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("committing the cooperative: %w", err)
	}
	return seeded, nil
}

func register(
	ctx context.Context, db *gorm.DB, goTrue *supabase.Client,
	accounts []accountSpec, seeded []*seededCooperative, password string,
) error {
	for _, account := range accounts {
		userID, err := goTrue.CreateUser(ctx, account.email, password)
		if err != nil {
			return fmt.Errorf(
				"membuat akun %s (pakai -reset bila akun demo sudah ada): %w",
				account.email, err)
		}

		profile := &entity.AppUser{
			ID:        userID,
			Role:      account.role,
			FullName:  account.fullName,
			CreatedAt: time.Now().UTC(),
		}
		if account.cooperative >= 0 {
			profile.CooperativeID = &seeded[account.cooperative].id
		}
		if account.organisation != "" {
			organisation := account.organisation
			profile.Organisation = &organisation
		}

		if err := db.Create(profile).Error; err != nil {
			if deleteErr := goTrue.DeleteUser(ctx, userID); deleteErr != nil {
				return fmt.Errorf("akun %s menggantung di Supabase: %w", account.email, deleteErr)
			}
			return fmt.Errorf("menyimpan profil %s: %w", account.email, err)
		}
	}
	return nil
}

func weatherUseCase(db *gorm.DB, log *logrus.Logger) *usecase.WeatherUseCase {
	return usecase.NewWeatherUseCase(db, log, &repository.WeatherRepository{}, weather.NewClient())
}

func projectionUseCase(db *gorm.DB, log *logrus.Logger) *usecase.ProjectionUseCase {
	return usecase.NewProjectionUseCase(db, log,
		&repository.PlotRepository{}, &repository.BlockRepository{},
		&repository.VarietyRepository{}, &repository.CalibrationRepository{},
		weatherUseCase(db, log))
}

func fetchWeather(
	ctx context.Context, db *gorm.DB, log *logrus.Logger,
	seeded []*seededCooperative, now time.Time,
) error {
	cells := map[weather.GridCell]bool{}
	for _, cooperative := range seeded {
		for _, wanted := range cooperative.spec.plots {
			cells[weather.SnapToGrid(wanted.lat, wanted.lng)] = true
		}
	}

	weatherUC := weatherUseCase(db, log)

	for cell := range cells {
		fmt.Printf("Cuaca        %.2f, %.2f — mengunduh 10 tahun dari Open-Meteo…\n",
			cell.GridLat, cell.GridLng)

		fetched, cancel := context.WithTimeout(ctx, constants.WeatherBackfillTimeout)
		result, err := weatherUC.BackfillGrid(fetched, cell, now)
		cancel()
		if err != nil {
			return fmt.Errorf("sel %v: %w", cell, err)
		}

		if result.Skipped {
			fmt.Printf("Cuaca        %.2f, %.2f — sudah lengkap (%d hari)\n",
				cell.GridLat, cell.GridLng, result.Rows)
			continue
		}
		fmt.Printf("Cuaca        %.2f, %.2f — %d hari tersimpan\n",
			cell.GridLat, cell.GridLng, result.Rows)
	}
	return nil
}

func calibrate(
	ctx context.Context, db *gorm.DB, projection *usecase.ProjectionUseCase,
	seeded []*seededCooperative, now time.Time,
) (int, error) {
	fitted := 0

	for _, cooperative := range seeded {
		varieties := map[string]bool{}

		blocks := []entity.Block{}
		err := db.Model(&entity.Block{}).
			Joins("JOIN plot ON plot.id = block.plot_id").
			Where("plot.cooperative_id = ? AND block.actual_harvest_date IS NOT NULL",
				cooperative.id).
			Find(&blocks).Error
		if err != nil {
			return fitted, fmt.Errorf("reading harvested blocks: %w", err)
		}
		for _, block := range blocks {
			varieties[block.VarietyID] = true
		}

		for varietyID := range varieties {
			row, err := projection.RefitCalibration(ctx, cooperative.id, varietyID, now)
			if err != nil {
				return fitted, fmt.Errorf("fitting variety %s: %w", varietyID, err)
			}
			if row != nil {
				fitted++
			}
		}
	}
	return fitted, nil
}

func orderInputs(db *gorm.DB, cooperative *seededCooperative, now time.Time) error {
	type line struct {
		item     string
		quantity float64
		retail   float64
		bulk     float64
	}
	type order struct {
		season string
		status constants.OrderStatus
		age    int
		lines  []line
	}

	wanted := []order{
		{
			season: "MT II 2026", status: constants.OrderCompleted, age: 150,
			lines: []line{
				{"urea", 120, 135000, 118000},
				{"sp36", 60, 165000, 149000},
				{"kcl", 45, 195000, 178000},
			},
		},
		{
			season: "MT I 2026/2027", status: constants.OrderSubmitted, age: 21,
			lines: []line{
				{"urea", 96, 0, 0},
				{"npk", 40, 0, 0},
			},
		},
		{
			season: constants.RdkkDefaultLabel, status: constants.OrderDraft, age: 3,
			lines: []line{
				{"urea", 88, 0, 0},
				{"sp36", 42, 0, 0},
				{"kcl", 30, 0, 0},
			},
		},
	}

	tx := db.Begin()
	defer tx.Rollback()

	for _, wantedOrder := range wanted {
		created := &entity.InputOrder{
			ID:            uuid.NewString(),
			CooperativeID: cooperative.id,
			SeasonLabel:   wantedOrder.season,
			Status:        wantedOrder.status,
			CreatedAt:     agronomy.AddDays(now, -wantedOrder.age),
		}
		if err := tx.Create(created).Error; err != nil {
			return fmt.Errorf("creating an input order: %w", err)
		}

		for _, wantedLine := range wantedOrder.lines {
			row := &entity.InputOrderLine{
				ID:           uuid.NewString(),
				InputOrderID: created.ID,
				Item:         wantedLine.item,
				Quantity:     wantedLine.quantity,
				Unit:         fmt.Sprintf("karung %d kg", constants.KgPerSack),
			}
			if wantedLine.retail > 0 {
				retail, bulk := wantedLine.retail, wantedLine.bulk
				row.RetailPricePerUnit = &retail
				row.BulkPricePerUnit = &bulk
			}
			if err := tx.Create(row).Error; err != nil {
				return fmt.Errorf("creating an input order line: %w", err)
			}
		}
	}

	return tx.Commit().Error
}

func requestSupply(
	ctx context.Context, db *gorm.DB, log *logrus.Logger,
	projection *usecase.ProjectionUseCase, seeded []*seededCooperative, now time.Time,
) (int, error) {
	buyer := new(entity.AppUser)
	if err := db.Where("full_name = ? AND role = ?",
		buyerName, constants.RoleBuyer).Take(buyer).Error; err != nil {
		return 0, fmt.Errorf("reading the demo buyer: %w", err)
	}

	type wantedRequest struct {
		cooperative int
		share       float64
		status      constants.RequestStatus
		age         int
		latest      bool
		notes       string
	}

	wanted := []wantedRequest{
		{0, 0.55, constants.RequestAccepted, 12, false,
			"Preferensi pengiriman: Antar ke gudang pembeli.\n" +
				"Butuh gabah kering giling, kadar air maksimal 14%."},
		{0, 0.30, constants.RequestPending, 3, false,
			"Preferensi pengiriman: Ambil sendiri di koperasi.\n" +
				"Mohon konfirmasi sebelum akhir minggu."},
		{0, 0.25, constants.RequestPending, 1, false,
			"Preferensi pengiriman: Belum ditentukan."},
		{0, 1.40, constants.RequestDeclined, 20, false,
			"Preferensi pengiriman: Antar ke gudang pembeli.\n" +
				"Permintaan besar untuk kontrak tahunan."},
		{0, 0.60, constants.RequestAccepted, 30, true,
			"Preferensi pengiriman: Antar ke gudang pembeli.\n" +
				"Kontrak berulang tiap musim, dipakai perencanaan musim depan."},
		{1, 0.45, constants.RequestPending, 5, false,
			"Preferensi pengiriman: Ambil sendiri di koperasi."},
	}

	heaviest := make([][]agronomy.WeekBucket, len(seeded))
	furthest := make([][]agronomy.WeekBucket, len(seeded))

	for i, cooperative := range seeded {
		projected, err := projection.ProjectCooperative(ctx, cooperative.id, now)
		if err != nil {
			return 0, fmt.Errorf("projecting %s: %w", cooperative.spec.name, err)
		}

		weeks := agronomy.BucketByWeek(projected.Projections)
		horizon := agronomy.AddDays(agronomy.ISOWeekStart(now), constants.DefaultHorizonWeeks*7)

		usable := []agronomy.WeekBucket{}
		for _, week := range weeks {
			if week.WeekStart.Before(now) || week.WeekStart.After(horizon) || week.Tonnes <= 0 {
				continue
			}
			usable = append(usable, week)
		}

		byTonnes := append([]agronomy.WeekBucket{}, usable...)
		sort.SliceStable(byTonnes, func(left, right int) bool {
			return byTonnes[left].Tonnes > byTonnes[right].Tonnes
		})

		byWeek := append([]agronomy.WeekBucket{}, usable...)
		sort.SliceStable(byWeek, func(left, right int) bool {
			return byWeek[right].WeekStart.Before(byWeek[left].WeekStart)
		})

		heaviest[i] = byTonnes
		furthest[i] = byWeek
	}

	created := 0
	taken := map[string]bool{}

	tx := db.Begin()
	defer tx.Rollback()

	for _, request := range wanted {
		available := heaviest[request.cooperative]
		if request.latest {
			available = furthest[request.cooperative]
		}

		week := agronomy.WeekBucket{}
		found := false
		for _, candidate := range available {
			key := candidate.CommodityID + "|" + candidate.ISOWeek
			if taken[key] {
				continue
			}
			taken[key] = true
			week, found = candidate, true
			break
		}
		if !found {
			log.Warnf("tidak ada minggu panen tersisa untuk permintaan di %s",
				seeded[request.cooperative].spec.name)
			continue
		}

		volumeKg := math.Round(week.Tonnes * constants.KgPerTonne * request.share)
		if volumeKg <= 0 {
			continue
		}

		notes := request.notes
		row := &entity.SupplyContractRequest{
			ID:                uuid.NewString(),
			CooperativeID:     seeded[request.cooperative].id,
			BuyerID:           buyer.ID,
			BuyerName:         buyer.FullName,
			BuyerOrganisation: buyer.Organisation,
			CommodityID:       week.CommodityID,
			VolumeKg:          volumeKg,
			WindowStart:       week.WeekStart,
			WindowEnd:         agronomy.AddDays(week.WeekStart, 6),
			Status:            request.status,
			Notes:             &notes,
			CreatedAt:         agronomy.AddDays(now, -request.age),
		}
		if request.status != constants.RequestPending {
			respondedAt := agronomy.AddDays(now, -request.age+1)
			row.RespondedAt = &respondedAt
		}

		if err := tx.Create(row).Error; err != nil {
			return created, fmt.Errorf("creating a supply request: %w", err)
		}
		created++
	}

	catalogue, err := loadReference(db)
	if err != nil {
		return created, err
	}

	type pastRequest struct {
		slug     string
		daysAgo  int
		volumeKg float64
		notes    string
	}

	past := []pastRequest{
		{"padi", 210, 25000, "Kontrak tahunan gabah, diulang tiap MT I."},
		{"jagung", 180, 18000, "Kontrak tahunan jagung pipilan, diulang tiap MT I."},
		{"padi", 30, 21000, "Kontrak tahunan gabah, diulang tiap MT II."},
	}

	for _, wantedPast := range past {
		commodityID, named := catalogue.commodityID[wantedPast.slug]
		if !named {
			continue
		}

		windowStart := agronomy.ISOWeekStart(agronomy.AddDays(now, -wantedPast.daysAgo))
		respondedAt := agronomy.AddDays(windowStart, -20)
		notes := "Preferensi pengiriman: Antar ke gudang pembeli.\n" + wantedPast.notes

		row := &entity.SupplyContractRequest{
			ID:                uuid.NewString(),
			CooperativeID:     seeded[0].id,
			BuyerID:           buyer.ID,
			BuyerName:         buyer.FullName,
			BuyerOrganisation: buyer.Organisation,
			CommodityID:       commodityID,
			VolumeKg:          wantedPast.volumeKg,
			WindowStart:       windowStart,
			WindowEnd:         agronomy.AddDays(windowStart, 6),
			Status:            constants.RequestAccepted,
			Notes:             &notes,
			CreatedAt:         agronomy.AddDays(windowStart, -30),
			RespondedAt:       &respondedAt,
		}
		if err := tx.Create(row).Error; err != nil {
			return created, fmt.Errorf("creating a past supply request: %w", err)
		}
		created++
	}

	if err := tx.Commit().Error; err != nil {
		return created, fmt.Errorf("committing the supply requests: %w", err)
	}
	return created, nil
}

func clear(
	ctx context.Context, db *gorm.DB, goTrue *supabase.Client, log *logrus.Logger,
) error {
	cooperatives := []entity.Cooperative{}
	if err := db.Where("name IN ?", []string{subangName, brebesName}).
		Find(&cooperatives).Error; err != nil {
		return fmt.Errorf("reading the demo cooperatives: %w", err)
	}

	ids := make([]string, len(cooperatives))
	for i, cooperative := range cooperatives {
		ids[i] = cooperative.ID
	}

	profiles := []entity.AppUser{}
	if len(ids) > 0 {
		if err := db.Where("cooperative_id IN ?", ids).Find(&profiles).Error; err != nil {
			return fmt.Errorf("reading the demo profiles: %w", err)
		}
	}

	buyers := []entity.AppUser{}
	if err := db.Where("full_name = ? AND role = ?", buyerName, constants.RoleBuyer).
		Find(&buyers).Error; err != nil {
		return fmt.Errorf("reading the demo buyer: %w", err)
	}

	for _, profile := range append(profiles, buyers...) {
		if err := goTrue.DeleteUser(ctx, profile.ID); err != nil {
			log.Warnf("akun %s tidak bisa dihapus di Supabase: %v", profile.ID, err)
			if err := db.Delete(&entity.AppUser{}, "id = ?", profile.ID).Error; err != nil {
				return fmt.Errorf("deleting profile %s: %w", profile.ID, err)
			}
		}
	}

	if len(ids) == 0 {
		return nil
	}
	if err := db.Where("id IN ?", ids).Delete(&entity.Cooperative{}).Error; err != nil {
		return fmt.Errorf("deleting the demo cooperatives: %w", err)
	}
	return nil
}

func report(
	db *gorm.DB, seeded []*seededCooperative, accounts []accountSpec, password string,
) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 72))
	fmt.Println("DATA DEMO SIAP")
	fmt.Println(strings.Repeat("-", 72))

	if len(accounts) > 0 {
		fmt.Println("\nAkun (kata sandi sama untuk semua):", password)
		for _, account := range accounts {
			home := "—"
			if account.cooperative >= 0 {
				home = seeded[account.cooperative].spec.name
			}
			fmt.Printf("  %-9s %-28s %-22s %s\n",
				account.role, account.email, account.fullName, home)
		}
	}

	fmt.Println("\nKoperasi")
	for _, cooperative := range seeded {
		fmt.Printf("  %s\n    id       %s\n    wilayah  %s, %s, %s\n",
			cooperative.spec.name, cooperative.id,
			cooperative.spec.village, cooperative.spec.district, cooperative.spec.province)
	}

	first := seeded[0]
	plotRows := []entity.Plot{}
	if err := db.Where("cooperative_id = ?", first.id).
		Order("name").Limit(3).Find(&plotRows).Error; err == nil && len(plotRows) > 0 {
		fmt.Println("\nHalaman lahan publik untuk dicoba tanpa login")
		for _, plot := range plotRows {
			fmt.Printf("  /garden/%s   (%s)\n", plot.PublicID, plot.Name)
		}
	}

	fmt.Println("\nLangkah berikutnya")
	fmt.Println("  1. go run ./cmd/web")
	fmt.Println("  2. cd ../Terrion_Frontend && pnpm dev")
	fmt.Println("  3. buka http://localhost:3000/login")
	fmt.Println()
}
