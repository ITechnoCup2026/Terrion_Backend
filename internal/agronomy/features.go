package agronomy

import "time"

type YieldObservationInput struct {
	PlantingDate  time.Time
	HarvestDate   time.Time
	AreaHa        float64
	ActualYieldKg float64
	Variety       Variety
	Weather       []TempDay
}

type YieldFeaturesInput struct {
	PlantingDate time.Time
	ThroughDate  time.Time
	AreaHa       float64
	Variety      Variety
	Weather      []TempDay
}

type seasonSummary struct {
	gdd       float64
	meanTempC float64
	days      int
}

func summariseSeason(weather []TempDay, variety Variety, from, to time.Time) seasonSummary {
	fromISO := ToISODate(from)
	toISO := ToISODate(to)

	summary := seasonSummary{meanTempC: variety.BaseTempC}
	temperatureTotal := 0.0

	for _, day := range weather {
		if day.Date < fromISO || day.Date > toISO {
			continue
		}
		summary.gdd += GddForDay(day, variety.BaseTempC)
		temperatureTotal += (day.TMin + day.TMax) / 2
		summary.days++
	}

	if summary.days > 0 {
		summary.meanTempC = temperatureTotal / float64(summary.days)
	}
	return summary
}

func baselineYieldPerHa(variety Variety) float64 {
	return (variety.YieldPerHaMin + variety.YieldPerHaMax) / 2
}

func DeriveYieldObservation(input YieldObservationInput) (YieldObservation, bool) {
	if input.AreaHa <= 0 || input.Variety.GddRequirement <= 0 {
		return YieldObservation{}, false
	}

	season := summariseSeason(input.Weather, input.Variety, input.PlantingDate, input.HarvestDate)
	if season.days == 0 {
		return YieldObservation{}, false
	}

	return YieldObservation{
		ActualYieldPerHa: input.ActualYieldKg / 1000 / input.AreaHa,
		Features: YieldFeatures{
			VarietyBaselineYieldPerHa: baselineYieldPerHa(input.Variety),
			GddRatio:                  season.gdd / input.Variety.GddRequirement,
			AreaHa:                    input.AreaHa,
			MeanTempC:                 season.meanTempC,
		},
	}, true
}

func DeriveYieldFeatures(input YieldFeaturesInput) YieldFeatures {
	season := summariseSeason(input.Weather, input.Variety, input.PlantingDate, input.ThroughDate)

	gddRatio := 0.0
	if input.Variety.GddRequirement > 0 {
		gddRatio = season.gdd / input.Variety.GddRequirement
	}

	return YieldFeatures{
		VarietyBaselineYieldPerHa: baselineYieldPerHa(input.Variety),
		GddRatio:                  gddRatio,
		AreaHa:                    input.AreaHa,
		MeanTempC:                 season.meanTempC,
	}
}
