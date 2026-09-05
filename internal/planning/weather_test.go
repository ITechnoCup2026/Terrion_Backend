package planning_test

import (
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/planning"
)

func normalsForYear(meanC float64) []agronomy.ClimateNormal {
	normals := make([]agronomy.ClimateNormal, 0, 366)
	for day := 1; day <= 366; day++ {
		normals = append(normals, agronomy.ClimateNormal{DayOfYear: day, MeanC: meanC, SdC: 1.2})
	}
	return normals
}

func mustDate(t *testing.T, iso string) time.Time {
	t.Helper()
	parsed, err := agronomy.UTCDate(iso)
	if err != nil {
		t.Fatalf("tanggal %q: %v", iso, err)
	}
	return parsed
}

func TestSeriesForWindowCoversEveryDayFromClimatology(t *testing.T) {
	from := mustDate(t, "2026-10-05")
	to := mustDate(t, "2026-10-14")

	series := planning.SeriesForWindow(nil, normalsForYear(27.8), from, to)

	if len(series) != 10 {
		t.Fatalf("jendela 10 hari harus menghasilkan 10 hari, dapat %d", len(series))
	}
	if series[0].Date != "2026-10-05" || series[9].Date != "2026-10-14" {
		t.Fatalf("tepi jendela salah: %s..%s", series[0].Date, series[9].Date)
	}
}

func TestSeriesForWindowPrefersRealReadingsOverNormals(t *testing.T) {
	observed := []agronomy.TempDay{{Date: "2026-10-06", TMin: 20, TMax: 34}}

	series := planning.SeriesForWindow(
		observed, normalsForYear(27.8), mustDate(t, "2026-10-05"), mustDate(t, "2026-10-07"))

	if series[1].TMin != 20 || series[1].TMax != 34 {
		t.Fatalf("bacaan nyata harus menang atas normals, dapat %+v", series[1])
	}
	if series[0].TMin != 27.8 {
		t.Fatalf("hari tanpa bacaan harus memakai normals, dapat %+v", series[0])
	}
}

func TestPlanningFeaturesLandInsideTheTrainingDistribution(t *testing.T) {
	variety := agronomy.Variety{
		GddRequirement: 1500, BaseTempC: 10,
		DaysToHarvestMin: 90, DaysToHarvestMax: 110,
		YieldPerHaMin: 4, YieldPerHaMax: 7,
	}
	normals := normalsForYear(27.8)
	planting := mustDate(t, "2026-10-05")
	harvest := agronomy.AddDays(planting, 100)

	bare := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: planting, ThroughDate: harvest, AreaHa: 0.8, Variety: variety,
	})
	stitched := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: planting, ThroughDate: harvest, AreaHa: 0.8, Variety: variety,
		Weather: planning.SeriesForWindow(nil, normals, planting, harvest),
	})

	if bare.GddRatio != 0 || bare.MeanTempC != variety.BaseTempC {
		t.Fatalf("prasyarat uji berubah: blok tanpa cuaca seharusnya nol, dapat %+v", bare)
	}
	if stitched.GddRatio < 0.5 || stitched.GddRatio > 1.5 {
		t.Fatalf("GddRatio rencana harus mendekati 1, dapat %v", stitched.GddRatio)
	}
	if stitched.MeanTempC < 27 || stitched.MeanTempC > 28.6 {
		t.Fatalf("MeanTempC rencana harus mendekati normal, dapat %v", stitched.MeanTempC)
	}
}
