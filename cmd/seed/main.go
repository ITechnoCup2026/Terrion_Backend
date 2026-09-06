package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"hash/fnv"
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
	// Short, unique, and safe in an email local part: the demo pengurus for
	// this cooperative signs in as pengurus.<key>@<domain>. Numbering the
	// accounts instead would mean every cooperative added in the middle of the
	// list silently renames somebody else's login.
	key      string
	name     string
	village  string
	district string
	province string
	lat      float64
	lng      float64
	// The pengurus who gets an account for this cooperative. Seeded members
	// are not accounts; this person is both.
	manager string
	// phone adalah kontak koperasi yang dipakai pembeli untuk membalas.
	phone    string
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

	specs := cooperativeSpecs()
	seeded := make([]*seededCooperative, len(specs))

	for i, spec := range specs {
		created, err := plant(db, spec, now)
		if err != nil {
			log.Fatalf("seeding %s: %v", spec.name, err)
		}
		seeded[i] = created
		fmt.Printf("Koperasi %-30s %-20s %2d lahan, %3d blok\n",
			spec.name, spec.province, len(created.plotIDs), created.blocks)
	}

	accounts := []accountSpec{}
	if *withAccounts {
		accounts = accountSpecs(*emailDomain, specs)
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

  - dua belas koperasi di sepuluh provinsi, supaya Atlas dan Katalog
    menyala di luar Jawa
  - anggota, lahan, dan blok pada enam komoditas dan seluruh varietas acuan
  - panen lampau yang lengkap dengan harga dan tanggal bayar, supaya panel
    Dampak dan kalibrasi model terisi
  - tumpukan panen yang sengaja dibuat pada satu minggu, supaya deteksi
    tabrakan menyala dan penggeseran tanam bisa dicoba
  - pesanan sarana produksi dan permintaan pasokan pembeli

Jalankan dari akar repo, karena .env dibaca dari direktori kerja.

Cuaca adalah bagian paling lambat: sepuluh tahun data harian diunduh dari
Open-Meteo untuk setiap sel grid 0,25 derajat, dan dua belas koperasi
menyentuh jauh lebih banyak sel daripada dua. Pakai -weather=false untuk
mengisi data tanpa menunggu; proyeksi panen akan kosong sampai backfill
dijalankan.

`)
	flag.PrintDefaults()
}

// One login per cooperative, plus the two short ones the first cooperative has
// always had.
//
// The first cooperative keeps `pengurus@` and `kader@` because that pair is
// what gets typed during a demo and it is the only one carrying both roles.
// Every other cooperative gets pengurus.<key>@, so a reviewer can sign into
// any province rather than only ever seeing Subang's data.
func accountSpecs(domain string, specs []cooperativeSpec) []accountSpec {
	accounts := []accountSpec{
		{constants.RolePengurus, "pengurus@" + domain, "Bu Sri Wahyuni", "", 0},
		{constants.RoleKader, "kader@" + domain, "Pak Asep Suryana", "", 0},
	}

	for i, spec := range specs {
		if i == 0 {
			continue
		}
		accounts = append(accounts, accountSpec{
			role:        constants.RolePengurus,
			email:       "pengurus." + spec.key + "@" + domain,
			fullName:    spec.manager,
			cooperative: i,
		})
	}

	return append(accounts, accountSpec{
		role:         constants.RoleBuyer,
		email:        "pembeli@" + domain,
		fullName:     buyerName,
		organisation: "PT Pangan Nusantara",
		cooperative:  -1,
	})
}

// Every demo cooperative, in the order the rest of the seed indexes them.
//
// Index 0 and 1 are load-bearing: the input orders and the buyer's supply
// requests are written against them by position, and Subang is the one tuned
// to pile several harvests into a single week so the collision detector has
// something to find. New cooperatives are appended, never inserted.
func cooperativeSpecs() []cooperativeSpec {
	return []cooperativeSpec{
		subang(),    // Jawa Barat
		brebes(),    // Jawa Tengah
		garut(),     // Jawa Barat
		wonosobo(),  // Jawa Tengah
		malang(),    // Jawa Timur
		karo(),      // Sumatera Utara
		banyuasin(), // Sumatera Selatan
		lampung(),   // Lampung
		sidrap(),    // Sulawesi Selatan
		tabanan(),   // Bali
		lombok(),    // Nusa Tenggara Barat
		barito(),    // Kalimantan Selatan
	}
}

func subang() cooperativeSpec {
	return cooperativeSpec{
		key:      "subang",
		name:     subangName,
		village:  "Jalancagak",
		district: "Subang",
		province: "Jawa Barat",
		lat:      -6.4200,
		lng:      107.6800,
		manager:  "Bu Sri Wahyuni",
		phone:    "081300000001",
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
		key:      "brebes",
		name:     brebesName,
		village:  "Bumiayu",
		district: "Brebes",
		province: "Jawa Tengah",
		lat:      -7.2000,
		lng:      108.9800,
		manager:  "Pak Joko Purnomo",
		phone:    "081300000002",
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

// The ten cooperatives added beyond the original two.
//
// Each one is a real growing district paired with what that district actually
// grows -- potatoes and carrots on the Karo and Dieng plateaus, maize on the
// Lampung plains, rice in Sidrap and the Barito swamps -- because the Atlas
// and the catalogue are read as a map of Indonesian agriculture, and a
// cooperative growing strawberries in Banyuasin would say the data is invented
// before a single figure is checked.
//
// Plots are kept within about 0.06 degrees of their cooperative on purpose.
// Weather is backfilled per 0.25-degree grid cell, ten years each, so plots
// scattered across a regency turn one download into four.
//
// Between them these plant every variety added in 20260907000013; a variety
// nothing stands on is a row nobody ever sees.

func garut() cooperativeSpec {
	return cooperativeSpec{
		key:      "garut",
		name:     "KUD Mekar Tani Garut",
		village:  "Cikajang",
		district: "Garut",
		province: "Jawa Barat",
		lat:      -7.3700,
		lng:      107.8000,
		manager:  "Pak Endang Suherman",
		phone:    "081300000003",
		capacity: map[string]float64{"kentang": 7},
		plots: []plotSpec{
			{
				member: "Pak Endang Suherman", name: "Kebun Cikajang 1",
				lat: -7.3665, lng: 107.8045,
				blocks: []blockSpec{
					{"kentang", "Repita", 0.75, 68, nil},
					{"kentang", "Granola Kembang", 0.55, 34, nil},
					{"kentang", "Repita", 0.75, 122,
						&harvestSpec{97, 20.0, 13000, 12}},
				},
			},
			{
				member: "Pak Endang Suherman", name: "Kebun Cikajang 2",
				lat: -7.3755, lng: 107.7930,
				blocks: []blockSpec{
					{"wortel", "Chantenay", 0.50, 58, nil},
					{"wortel", "Imperator", 0.35, 26, nil},
				},
			},
			{
				member: "Bu Neneng Hasanah", name: "Kebun Cisurupan",
				lat: -7.3510, lng: 107.8215,
				blocks: []blockSpec{
					{"cabai", "Cabai keriting", 0.42, 76, nil},
					{"cabai", "Cabai besar Lembang-1", 0.30, 40, nil},
					{"cabai", "Cabai keriting", 0.42, 124,
						&harvestSpec{102, 9.8, 47000, 9}},
				},
			},
			{
				member: "Pak Dadang Sopandi", name: "Ladang Bayongbong",
				lat: -7.3890, lng: 107.7760,
				blocks: []blockSpec{
					{"kentang", "Amudra", 0.60, 50, nil},
					{"beri", "Stroberi Sweet Charlie", 0.24, 22, nil},
				},
			},
			{
				member: "Bu Neneng Hasanah", name: "Kebun Pasirwangi",
				lat: -7.3395, lng: 107.8330,
				blocks: []blockSpec{{"wortel", "Kuroda", 0.55, 44, nil}},
			},
		},
	}
}

func wonosobo() cooperativeSpec {
	return cooperativeSpec{
		key:      "wonosobo",
		name:     "KUD Dieng Makmur Wonosobo",
		village:  "Kejajar",
		district: "Wonosobo",
		province: "Jawa Tengah",
		lat:      -7.2100,
		lng:      109.9100,
		manager:  "Bu Tri Astuti",
		phone:    "081300000004",
		capacity: map[string]float64{"kentang": 6},
		plots: []plotSpec{
			{
				member: "Bu Tri Astuti", name: "Kebun Kejajar 1",
				lat: -7.2065, lng: 109.9145,
				blocks: []blockSpec{
					{"kentang", "Median", 0.70, 64, nil},
					{"kentang", "Granola Kembang", 0.50, 30, nil},
					{"kentang", "Median", 0.70, 118,
						&harvestSpec{95, 19.5, 12600, 11}},
				},
			},
			{
				member: "Bu Tri Astuti", name: "Kebun Kejajar 2",
				lat: -7.2155, lng: 109.9035,
				blocks: []blockSpec{
					{"wortel", "Lokal Tawangmangu", 0.45, 54, nil},
					{"wortel", "Kuroda", 0.35, 24, nil},
				},
			},
			{
				member: "Pak Sarwono", name: "Kebun Garung",
				lat: -7.2290, lng: 109.9280,
				blocks: []blockSpec{
					{"kentang", "Amudra", 0.65, 72, nil},
					{"kentang", "Repita", 0.45, 36, nil},
				},
			},
			{
				member: "Pak Sarwono", name: "Ladang Mojotengah",
				lat: -7.2440, lng: 109.8920,
				blocks: []blockSpec{
					{"jagung", "Srikandi Kuning", 0.90, 48, nil},
					{"cabai", "Cabai keriting", 0.28, 30, nil},
				},
			},
			{
				member: "Bu Sumarni", name: "Kebun Sikunang",
				lat: -7.1950, lng: 109.9350,
				blocks: []blockSpec{
					{"beri", "Stroberi Earlibrite", 0.22, 40, nil},
					{"wortel", "Chantenay", 0.40, 20, nil},
				},
			},
		},
	}
}

func malang() cooperativeSpec {
	return cooperativeSpec{
		key:      "malang",
		name:     "KUD Rukun Tani Malang",
		village:  "Tumpang",
		district: "Malang",
		province: "Jawa Timur",
		lat:      -8.0079,
		lng:      112.7550,
		manager:  "Pak Hariyanto",
		phone:    "081300000005",
		capacity: map[string]float64{"jagung": 10},
		plots: []plotSpec{
			{
				member: "Bu Sri Rahayu", name: "Ladang Tumpang 1",
				lat: -8.0035, lng: 112.7590,
				blocks: []blockSpec{
					{"jagung", "Nasa-29", 1.40, 65, nil},
					{"jagung", "NK Perkasa", 1.10, 32, nil},
					{"jagung", "Nasa-29", 1.40, 128,
						&harvestSpec{102, 9.1, 5000, 12}},
				},
			},
			{
				member: "Bu Sri Rahayu", name: "Ladang Tumpang 2",
				lat: -8.0120, lng: 112.7480,
				blocks: []blockSpec{{"jagung", "Bima-20 URI", 0.95, 48, nil}},
			},
			{
				member: "Pak Hariyanto", name: "Kebun Poncokusumo",
				lat: -8.0345, lng: 112.7905,
				blocks: []blockSpec{
					{"kentang", "Median", 0.65, 70, nil},
					{"wortel", "Kuroda", 0.45, 55, nil},
					{"kentang", "Median", 0.65, 120,
						&harvestSpec{96, 20.5, 12800, 10}},
				},
			},
			{
				member: "Pak Suparno", name: "Sawah Pakis",
				lat: -7.9720, lng: 112.7100,
				blocks: []blockSpec{
					{"padi", "Inpari 32 HDB", 1.70, 95, nil},
					{"padi", "Inpari 32 HDB", 1.70, 140,
						&harvestSpec{116, 6.3, 6400, 9}},
				},
			},
			{
				member: "Bu Endah Prihatin", name: "Kebun Wajak",
				lat: -8.0510, lng: 112.7345,
				blocks: []blockSpec{
					{"cabai", "Cabai keriting", 0.40, 58, nil},
					{"beri", "Stroberi Earlibrite", 0.20, 25, nil},
				},
			},
			{
				member: "Pak Suparno", name: "Ladang Jabung",
				lat: -7.9880, lng: 112.7820,
				blocks: []blockSpec{{"jagung", "Pioneer P27", 1.20, 40, nil}},
			},
		},
	}
}

func karo() cooperativeSpec {
	return cooperativeSpec{
		key:      "karo",
		name:     "KUD Karo Bertani",
		village:  "Berastagi",
		district: "Karo",
		province: "Sumatera Utara",
		lat:      3.1900,
		lng:      98.5100,
		manager:  "Pak Jhon Sitepu",
		phone:    "081300000006",
		capacity: map[string]float64{"kentang": 8},
		plots: []plotSpec{
			{
				member: "Pak Jhon Sitepu", name: "Kebun Berastagi 1",
				lat: 3.1935, lng: 98.5060,
				blocks: []blockSpec{
					{"kentang", "Granola Kembang", 0.80, 72, nil},
					{"kentang", "Repita", 0.55, 35, nil},
					{"kentang", "Granola Kembang", 0.80, 124,
						&harvestSpec{99, 21.5, 13100, 14}},
				},
			},
			{
				member: "Pak Jhon Sitepu", name: "Kebun Berastagi 2",
				lat: 3.1860, lng: 98.5145,
				blocks: []blockSpec{
					{"wortel", "Imperator", 0.60, 66, nil},
					{"wortel", "Chantenay", 0.40, 28, nil},
				},
			},
			{
				member: "Bu Rosmita Br Ginting", name: "Ladang Simpang Empat",
				lat: 3.2085, lng: 98.4880,
				blocks: []blockSpec{
					{"cabai", "Cabai merah Tanjung-2", 0.50, 80, nil},
					{"cabai", "Cabai keriting", 0.35, 44, nil},
					{"cabai", "Cabai merah Tanjung-2", 0.50, 126,
						&harvestSpec{104, 9.4, 48000, 8}},
				},
			},
			{
				member: "Pak Rendi Tarigan", name: "Kebun Merdeka",
				lat: 3.1710, lng: 98.5290,
				blocks: []blockSpec{
					{"beri", "Stroberi Sweet Charlie", 0.30, 52, nil},
					{"beri", "Stroberi Rosalinda", 0.22, 24, nil},
				},
			},
			{
				member: "Bu Ester Br Sembiring", name: "Kebun Tigapanah",
				lat: 3.1590, lng: 98.5405,
				blocks: []blockSpec{
					{"kentang", "Amudra", 0.70, 60, nil},
					{"wortel", "Kuroda", 0.50, 42, nil},
				},
			},
			{
				member: "Pak Rendi Tarigan", name: "Ladang Barusjahe",
				lat: 3.1445, lng: 98.5620,
				blocks: []blockSpec{{"kentang", "Median", 0.90, 30, nil}},
			},
		},
	}
}

func banyuasin() cooperativeSpec {
	return cooperativeSpec{
		key:      "banyuasin",
		name:     "KUD Sriwijaya Tani",
		village:  "Tanjung Lago",
		district: "Banyuasin",
		province: "Sumatera Selatan",
		lat:      -2.6300,
		lng:      104.6200,
		manager:  "Pak Ahmad Fauzi",
		phone:    "081300000007",
		capacity: map[string]float64{"padi": 20},
		plots: []plotSpec{
			{
				member: "Pak Ahmad Fauzi", name: "Sawah Tanjung Lago 1",
				lat: -2.6265, lng: 104.6240,
				blocks: []blockSpec{
					{"padi", "Inpari 42 Agritan GSR", 2.40, 92, nil},
					{"padi", "Inpari 42 Agritan GSR", 2.40, 145,
						&harvestSpec{119, 6.9, 6300, 11}},
				},
			},
			{
				member: "Pak Ahmad Fauzi", name: "Sawah Tanjung Lago 2",
				lat: -2.6340, lng: 104.6155,
				blocks: []blockSpec{{"padi", "Cisadane", 2.10, 104, nil}},
			},
			{
				member: "Bu Marlina", name: "Sawah Muara Telang",
				lat: -2.6520, lng: 104.6480,
				blocks: []blockSpec{
					{"padi", "IR42", 1.80, 118, nil},
					{"padi", "IR42", 1.80, 160,
						&harvestSpec{132, 5.4, 6100, 20}},
				},
			},
			{
				member: "Pak Zainal Abidin", name: "Sawah Air Saleh",
				lat: -2.6055, lng: 104.5915,
				blocks: []blockSpec{{"padi", "Ciliwung", 2.60, 88, nil}},
			},
			{
				member: "Bu Siti Khodijah", name: "Ladang Betung",
				lat: -2.5930, lng: 104.6605,
				blocks: []blockSpec{
					{"jagung", "Bisi-2", 1.30, 50, nil},
					{"jagung", "Srikandi Kuning", 0.90, 26, nil},
				},
			},
		},
	}
}

func lampung() cooperativeSpec {
	return cooperativeSpec{
		key:      "lamteng",
		name:     "KUD Sinar Tani Terbanggi",
		village:  "Terbanggi Besar",
		district: "Lampung Tengah",
		province: "Lampung",
		lat:      -4.8200,
		lng:      105.2200,
		manager:  "Pak Sugiyono",
		phone:    "081300000008",
		capacity: map[string]float64{"jagung": 14},
		plots: []plotSpec{
			{
				member: "Pak Sugiyono", name: "Ladang Terbanggi 1",
				lat: -4.8165, lng: 105.2245,
				blocks: []blockSpec{
					{"jagung", "NK Perkasa", 2.20, 62, nil},
					{"jagung", "Nasa-29", 1.60, 30, nil},
					{"jagung", "NK Perkasa", 2.20, 126,
						&harvestSpec{100, 9.8, 4900, 13}},
				},
			},
			{
				member: "Pak Sugiyono", name: "Ladang Terbanggi 2",
				lat: -4.8255, lng: 105.2130,
				blocks: []blockSpec{{"jagung", "Bima-20 URI", 1.80, 44, nil}},
			},
			{
				member: "Bu Ngatinem", name: "Ladang Seputih Mataram",
				lat: -4.8480, lng: 105.2510,
				blocks: []blockSpec{
					{"jagung", "Bisi-2", 1.50, 56, nil},
					{"jagung", "Pioneer P27", 1.20, 22, nil},
				},
			},
			{
				member: "Pak Wagimin", name: "Sawah Punggur",
				lat: -4.7960, lng: 105.1885,
				blocks: []blockSpec{
					{"padi", "Mekongga", 1.90, 98, nil},
					{"padi", "Mekongga", 1.90, 142,
						&harvestSpec{118, 6.2, 6200, 10}},
				},
			},
			{
				member: "Bu Rukmini", name: "Ladang Gunung Sugih",
				lat: -4.8620, lng: 105.1960,
				blocks: []blockSpec{
					{"cabai", "Cabai rawit Bhaskara", 0.45, 68, nil},
					{"jagung", "Srikandi Kuning", 1.00, 34, nil},
				},
			},
		},
	}
}

func sidrap() cooperativeSpec {
	return cooperativeSpec{
		key:      "sidrap",
		name:     "KUD Bina Tani Sidrap",
		village:  "Maritengngae",
		district: "Sidenreng Rappang",
		province: "Sulawesi Selatan",
		lat:      -3.8500,
		lng:      119.8000,
		manager:  "Pak Andi Baso",
		phone:    "081300000009",
		capacity: map[string]float64{"padi": 22},
		plots: []plotSpec{
			{
				member: "Pak Andi Baso", name: "Sawah Pangkajene 1",
				lat: -3.8465, lng: 119.8045,
				blocks: []blockSpec{
					{"padi", "Inpari 32 HDB", 2.80, 96, nil},
					{"padi", "Inpari 32 HDB", 2.80, 140,
						&harvestSpec{114, 7.1, 6000, 8}},
				},
			},
			{
				member: "Pak Andi Baso", name: "Sawah Pangkajene 2",
				lat: -3.8555, lng: 119.7930,
				blocks: []blockSpec{{"padi", "Inpari 30 Ciherang Sub1", 2.30, 106, nil}},
			},
			{
				member: "Bu Hj. Rosmini", name: "Sawah Maritengngae",
				lat: -3.8290, lng: 119.8215,
				blocks: []blockSpec{
					{"padi", "Mekongga", 2.00, 90, nil},
					{"padi", "Mekongga", 2.00, 136,
						&harvestSpec{112, 6.6, 6150, 12}},
				},
			},
			{
				member: "Pak Abd. Rahman", name: "Sawah Watang Pulu",
				lat: -3.8720, lng: 119.7745,
				blocks: []blockSpec{{"padi", "Ciliwung", 1.70, 112, nil}},
			},
			{
				member: "Bu Nurul Hidayah", name: "Ladang Baranti",
				lat: -3.8180, lng: 119.8390,
				blocks: []blockSpec{
					{"jagung", "Nasa-29", 1.40, 52, nil},
					{"padi", "Situ Bagendit", 0.80, 38, nil},
				},
			},
		},
	}
}

func tabanan() cooperativeSpec {
	return cooperativeSpec{
		key:      "tabanan",
		name:     "Subak Sari Tabanan",
		village:  "Penebel",
		district: "Tabanan",
		province: "Bali",
		lat:      -8.4700,
		lng:      115.1000,
		manager:  "Pak I Wayan Sudira",
		phone:    "081300000010",
		capacity: map[string]float64{"padi": 9},
		plots: []plotSpec{
			{
				member: "Pak I Wayan Sudira", name: "Subak Jatiluwih 1",
				lat: -8.4665, lng: 115.1045,
				blocks: []blockSpec{
					{"padi", "Inpari 42 Agritan GSR", 1.10, 94, nil},
					{"padi", "Inpari 42 Agritan GSR", 1.10, 138,
						&harvestSpec{117, 6.4, 7400, 7}},
				},
			},
			{
				member: "Pak I Wayan Sudira", name: "Subak Jatiluwih 2",
				lat: -8.4745, lng: 115.0935,
				blocks: []blockSpec{{"padi", "Cisadane", 0.85, 108, nil}},
			},
			{
				member: "Bu Ni Made Ayu", name: "Subak Penebel",
				lat: -8.4870, lng: 115.1180,
				blocks: []blockSpec{
					{"padi", "Situ Bagendit", 0.95, 72, nil},
					{"wortel", "Lokal Tawangmangu", 0.30, 46, nil},
				},
			},
			{
				member: "Pak I Ketut Gede", name: "Kebun Pupuan",
				lat: -8.4510, lng: 115.0790,
				blocks: []blockSpec{
					{"cabai", "Cabai besar Lembang-1", 0.35, 64, nil},
					{"beri", "Stroberi Rosalinda", 0.18, 28, nil},
				},
			},
			{
				member: "Bu Ni Nyoman Warti", name: "Subak Marga",
				lat: -8.4955, lng: 115.1265,
				blocks: []blockSpec{{"padi", "Ciliwung", 1.25, 86, nil}},
			},
		},
	}
}

func lombok() cooperativeSpec {
	return cooperativeSpec{
		key:      "lomteng",
		name:     "KUD Mandiri Lombok Tengah",
		village:  "Praya",
		district: "Lombok Tengah",
		province: "Nusa Tenggara Barat",
		lat:      -8.7000,
		lng:      116.2700,
		manager:  "Pak Lalu Ahmad",
		phone:    "081300000011",
		capacity: map[string]float64{"padi": 11, "jagung": 8},
		plots: []plotSpec{
			{
				member: "Pak Lalu Ahmad", name: "Sawah Praya 1",
				lat: -8.6965, lng: 116.2745,
				blocks: []blockSpec{
					{"padi", "Inpari 32 HDB", 1.60, 90, nil},
					{"padi", "Inpari 32 HDB", 1.60, 134,
						&harvestSpec{115, 6.0, 6800, 9}},
				},
			},
			{
				member: "Pak Lalu Ahmad", name: "Sawah Praya 2",
				lat: -8.7055, lng: 116.2635,
				blocks: []blockSpec{{"padi", "Mekongga", 1.35, 100, nil}},
			},
			{
				member: "Bu Baiq Hartini", name: "Ladang Batukliang",
				lat: -8.6790, lng: 116.2915,
				blocks: []blockSpec{
					{"jagung", "Bima-20 URI", 1.50, 58, nil},
					{"jagung", "Bisi-2", 1.10, 30, nil},
				},
			},
			{
				member: "Pak Lalu Sahid", name: "Ladang Jonggat",
				lat: -8.7210, lng: 116.2480,
				blocks: []blockSpec{
					{"jagung", "Srikandi Kuning", 1.20, 46, nil},
					{"cabai", "Cabai rawit Bhaskara", 0.35, 36, nil},
				},
			},
			{
				member: "Bu Baiq Nuraini", name: "Sawah Pujut",
				lat: -8.7340, lng: 116.2830,
				blocks: []blockSpec{{"padi", "Situ Bagendit", 1.05, 76, nil}},
			},
		},
	}
}

func barito() cooperativeSpec {
	return cooperativeSpec{
		key:      "batola",
		name:     "KUD Barito Tani",
		village:  "Alalak",
		district: "Barito Kuala",
		province: "Kalimantan Selatan",
		lat:      -3.2600,
		lng:      114.5700,
		manager:  "Pak Rusdiansyah",
		phone:    "081300000012",
		capacity: map[string]float64{"padi": 12},
		plots: []plotSpec{
			{
				member: "Pak Rusdiansyah", name: "Sawah Alalak 1",
				lat: -3.2565, lng: 114.5745,
				blocks: []blockSpec{
					{"padi", "IR42", 1.90, 110, nil},
					{"padi", "IR42", 1.90, 156,
						&harvestSpec{130, 5.2, 6500, 16}},
				},
			},
			{
				member: "Pak Rusdiansyah", name: "Sawah Alalak 2",
				lat: -3.2650, lng: 114.5630,
				blocks: []blockSpec{{"padi", "Cisadane", 1.55, 98, nil}},
			},
			{
				member: "Bu Norhalisah", name: "Sawah Anjir Muara",
				lat: -3.2385, lng: 114.5895,
				blocks: []blockSpec{{"padi", "Ciliwung", 2.05, 84, nil}},
			},
			{
				member: "Pak Ahmad Rifani", name: "Sawah Mandastana",
				lat: -3.2810, lng: 114.5480,
				blocks: []blockSpec{{"padi", "Inpari 30 Ciherang Sub1", 1.70, 92, nil}},
			},
			{
				member: "Bu Mariatul Kiptiah", name: "Ladang Belawang",
				lat: -3.2940, lng: 114.5960,
				blocks: []blockSpec{
					{"jagung", "Bisi-2", 1.15, 54, nil},
					{"cabai", "Cabai keriting", 0.30, 32, nil},
				},
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
	if spec.phone != "" {
		phone := spec.phone
		cooperative.Phone = &phone
	}
	if err := tx.Create(cooperative).Error; err != nil {
		return nil, fmt.Errorf("creating the cooperative: %w", err)
	}
	seeded.id = cooperative.ID

	// Seeded from the name's hash, not its length. Length collided the moment
	// there was more than a handful of cooperatives -- "KUD Barito Tani" and
	// "KUD Karo Bertani" would have drawn the identical terrain on every plot.
	naming := fnv.New64a()
	naming.Write([]byte(spec.name))
	random := rand.New(rand.NewSource(int64(naming.Sum64() & math.MaxInt64)))

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
	for index, account := range accounts {
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
		// Nomor contoh, supaya tautan WhatsApp di layar Permintaan punya
		// sesuatu untuk dibuka saat diuji. Nomor uji Indonesia, bukan nomor
		// siapa pun: 0812-0000-00NN.
		phone := fmt.Sprintf("0812000000%02d", index+1)
		profile.Phone = &phone

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
	// Taken from the specs rather than a hardcoded pair, so a cooperative added
	// to the list is also a cooperative -reset knows how to remove. The old
	// two-name literal meant every province added here would have survived a
	// reset and been seeded again beside itself.
	specs := cooperativeSpecs()
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.name
	}

	cooperatives := []entity.Cooperative{}
	if err := db.Where("name IN ?", names).
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

	// One line each. Twelve four-line blocks pushed the account table and the
	// public plot links off the top of the terminal, which are the two things
	// somebody actually runs this to read.
	fmt.Println("\nKoperasi")
	for _, cooperative := range seeded {
		fmt.Printf("  %-30s %-20s %s, %s\n",
			cooperative.spec.name, cooperative.spec.province,
			cooperative.spec.village, cooperative.spec.district)
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
