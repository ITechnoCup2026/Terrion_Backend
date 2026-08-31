package entity_test

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"testing"

	"gorm.io/gorm/schema"

	"terrion-backend/internal/entity"
)

const migrationsDir = "../../db/migrations"

// The entities exist to be mapped onto the columns db/migrations creates. A
// mistyped column tag or a column added to a migration and forgotten here fails
// at runtime on the first query, so the two are compared directly instead.
func TestEntitiesMatchMigrations(t *testing.T) {
	columns := columnsFromMigrations(t)

	models := []any{
		&entity.Cooperative{}, &entity.AppUser{}, &entity.Member{},
		&entity.Commodity{}, &entity.Variety{}, &entity.FertiliserRate{},
		&entity.ReferencePrice{}, &entity.RegionStat{},
		&entity.Plot{}, &entity.Block{},
		&entity.WeatherDaily{}, &entity.WeatherNormal{},
		&entity.CooperativeCapacity{}, &entity.Calibration{},
		&entity.SupplyContractRequest{}, &entity.InputOrder{}, &entity.InputOrderLine{},
	}

	for _, model := range models {
		parsed, err := schema.Parse(model, &sync.Map{}, schema.NamingStrategy{})
		if err != nil {
			t.Fatalf("parsing %T: %v", model, err)
		}

		want, ok := columns[parsed.Table]
		if !ok {
			t.Errorf("%T maps to table %q, which no migration creates", model, parsed.Table)
			continue
		}

		got := make([]string, 0, len(parsed.Fields))
		for _, field := range parsed.Fields {
			got = append(got, field.DBName)
		}
		sort.Strings(got)
		sort.Strings(want)

		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Errorf("%T columns\n  struct:    %v\n  migration: %v", model, got, want)
		}
	}
}

// public_plot is a view, so it has no create table to compare against. Its
// column list is the security boundary for every unauthenticated read — the
// absence of lat, lng and nik_hash is the point — so it is asserted literally.
func TestPublicPlotViewColumns(t *testing.T) {
	parsed, err := schema.Parse(&entity.PublicPlot{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parsing PublicPlot: %v", err)
	}

	got := make([]string, 0, len(parsed.Fields))
	for _, field := range parsed.Fields {
		got = append(got, field.DBName)
	}
	sort.Strings(got)

	want := []string{
		"area_ha", "district", "member_name", "name",
		"public_id", "terrain_seed", "tile_size_m2", "village",
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("public_plot columns\n  struct: %v\n  want:   %v", got, want)
	}
}

var (
	commentPattern    = regexp.MustCompile(`--[^\n]*`)
	createTablePatten = regexp.MustCompile(`(?i)create\s+table\s+(\w+)\s*\(`)
)

// Every column each migration's create table statements declare, by table name.
func columnsFromMigrations(t *testing.T) map[string][]string {
	t.Helper()

	files, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no up migrations found in %s: %v", migrationsDir, err)
	}

	tables := map[string][]string{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("reading %s: %v", file, err)
		}
		sql := commentPattern.ReplaceAllString(string(raw), "")

		for _, match := range createTablePatten.FindAllStringSubmatchIndex(sql, -1) {
			name := sql[match[2]:match[3]]
			tables[name] = columnNames(sql[match[1]:])
		}
	}
	return tables
}

// The column names in a create table body, given the text just after its
// opening parenthesis. Splits on top-level commas so that numeric(9,6) and the
// case expression in plot.tile_size_m2 stay in one piece.
func columnNames(body string) []string {
	depth := 0
	start := 0
	fragments := []string{}

closing:
	for i := 0; i < len(body); i++ {
		switch body[i] {
		case '(':
			depth++
		case ')':
			if depth == 0 {
				fragments = append(fragments, body[start:i])
				break closing
			}
			depth--
		case ',':
			if depth == 0 {
				fragments = append(fragments, body[start:i])
				start = i + 1
			}
		}
	}

	names := []string{}
	for _, fragment := range fragments {
		fields := strings.Fields(fragment)
		if len(fields) == 0 {
			continue
		}
		switch strings.ToLower(fields[0]) {
		case "constraint", "primary", "unique", "check", "foreign", "exclude":
			continue
		}
		names = append(names, fields[0])
	}
	return names
}
