package planning

import (
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type simulatedWindow struct {
	window       agronomy.DateRange
	plausibility constants.Plausibility
}

type Simulator struct {
	normals     []agronomy.ClimateNormal
	cache       map[string]simulatedWindow
	simulations int
}

func NewSimulator(normals []agronomy.ClimateNormal) *Simulator {
	return &Simulator{normals: normals, cache: map[string]simulatedWindow{}}
}

func (s *Simulator) Simulations() int {
	return s.simulations
}

func (s *Simulator) Window(
	varietyID string, variety agronomy.Variety,
	calibration *agronomy.Calibration, plantingDate time.Time,
) (agronomy.DateRange, constants.Plausibility, error) {
	key := varietyID + "|" + agronomy.ToISODate(plantingDate)
	if cached, known := s.cache[key]; known {
		return cached.window, cached.plausibility, nil
	}

	predicted, err := agronomy.PredictHarvest(agronomy.HarvestInput{
		PlantingDate: plantingDate,
		Climatology:  s.normals,
		Variety:      variety,
		Calibration:  calibration,
	})
	if err != nil {
		return agronomy.DateRange{}, "", err
	}

	simulated := simulatedWindow{
		window:       agronomy.DateRange{Start: predicted.Start, End: predicted.End},
		plausibility: predicted.Plausibility,
	}
	s.cache[key] = simulated
	s.simulations++

	return simulated.window, simulated.plausibility, nil
}

func (s *Simulator) YieldPerHaRange(
	model agronomy.YieldModel, variety agronomy.Variety,
	plantingDate, harvestEnd time.Time, areaHa float64,
) (float64, float64, float64) {
	features := agronomy.DeriveYieldFeatures(agronomy.YieldFeaturesInput{
		PlantingDate: plantingDate,
		ThroughDate:  harvestEnd,
		AreaHa:       areaHa,
		Variety:      variety,
		Weather:      SyntheticWeather(s.normals, plantingDate, harvestEnd),
	})
	return agronomy.PredictYieldRange(model, features, variety)
}
