package agronomy

import (
	"math"

	"terrion-backend/internal/constants"
)

const featureCount = 3

type YieldFeatures struct {
	VarietyBaselineYieldPerHa float64
	GddRatio                  float64
	AreaHa                    float64
	MeanTempC                 float64
}

type YieldObservation struct {
	ActualYieldPerHa float64
	Features         YieldFeatures
}

type YieldModel struct {
	MeanIndex     float64
	Coefficients  []float64
	FeatureMeans  []float64
	FeatureSds    []float64
	NObservations int
	ResidualSd    float64
}

func featureVector(features YieldFeatures) []float64 {
	return []float64{features.GddRatio, features.AreaHa, features.MeanTempC}
}

func FitYieldModel(observations []YieldObservation) YieldModel {
	usable := make([]YieldObservation, 0, len(observations))
	for _, observation := range observations {
		if observation.Features.VarietyBaselineYieldPerHa > 0 {
			usable = append(usable, observation)
		}
	}

	if len(usable) == 0 {
		return YieldModel{
			MeanIndex:    1,
			Coefficients: make([]float64, featureCount),
			FeatureMeans: make([]float64, featureCount),
			FeatureSds:   ones(featureCount),
		}
	}

	count := len(usable)
	raw := make([][]float64, count)
	index := make([]float64, count)
	for i, observation := range usable {
		raw[i] = featureVector(observation.Features)
		index[i] = observation.ActualYieldPerHa / observation.Features.VarietyBaselineYieldPerHa
	}
	meanIndex := mean(index)

	featureMeans := make([]float64, featureCount)
	featureSds := make([]float64, featureCount)
	for j := range featureCount {
		column := make([]float64, count)
		for i, row := range raw {
			column[i] = row[j]
		}
		featureMeans[j] = mean(column)

		squares := make([]float64, count)
		for i, value := range column {
			squares[i] = (value - featureMeans[j]) * (value - featureMeans[j])
		}
		sd := math.Sqrt(mean(squares))
		if sd > constants.ZeroVarianceEpsilon {
			featureSds[j] = sd
		} else {
			featureSds[j] = 1
		}
	}

	standardised := make([][]float64, count)
	centred := make([]float64, count)
	for i, row := range raw {
		standardised[i] = make([]float64, featureCount)
		for j, value := range row {
			standardised[i][j] = (value - featureMeans[j]) / featureSds[j]
		}
		centred[i] = index[i] - meanIndex
	}

	normal := make([][]float64, featureCount)
	moment := make([]float64, featureCount)
	for i := range featureCount {
		normal[i] = make([]float64, featureCount)
		for j := range featureCount {
			for _, row := range standardised {
				normal[i][j] += row[i] * row[j]
			}
		}
		normal[i][i] += constants.YieldRidgeLambda
		for t, row := range standardised {
			moment[i] += row[i] * centred[t]
		}
	}

	coefficients := solve(normal, moment)

	sumSquaredResiduals := 0.0
	for i := range count {
		fitted := meanIndex
		for j, coefficient := range coefficients {
			fitted += standardised[i][j] * coefficient
		}
		residual := index[i] - fitted
		sumSquaredResiduals += residual * residual
	}
	residualSd := 0.0
	if count > 1 {
		residualSd = math.Sqrt(sumSquaredResiduals / float64(count-1))
	}

	return YieldModel{
		MeanIndex:     meanIndex,
		Coefficients:  coefficients,
		FeatureMeans:  featureMeans,
		FeatureSds:    featureSds,
		NObservations: count,
		ResidualSd:    residualSd,
	}
}

func PredictYieldPerHa(model YieldModel, features YieldFeatures) float64 {
	fitted := model.MeanIndex
	for j, value := range featureVector(features) {
		fitted += model.Coefficients[j] * ((value - model.FeatureMeans[j]) / model.FeatureSds[j])
	}

	observations := float64(model.NObservations)
	trust := observations / (observations + constants.YieldShrinkageK)
	index := 1 + (fitted-1)*trust

	return math.Max(0, index*features.VarietyBaselineYieldPerHa)
}

func solve(a [][]float64, b []float64) []float64 {
	size := len(b)
	augmented := make([][]float64, size)
	for i := range augmented {
		augmented[i] = append(append([]float64{}, a[i]...), b[i])
	}

	for col := range size {
		pivot := col
		for row := col + 1; row < size; row++ {
			if math.Abs(augmented[row][col]) > math.Abs(augmented[pivot][col]) {
				pivot = row
			}
		}
		augmented[col], augmented[pivot] = augmented[pivot], augmented[col]

		diagonal := augmented[col][col]
		if math.Abs(diagonal) < constants.SingularPivotEpsilon {
			continue
		}
		for row := range size {
			if row == col {
				continue
			}
			factor := augmented[row][col] / diagonal
			for c := col; c <= size; c++ {
				augmented[row][c] -= factor * augmented[col][c]
			}
		}
	}

	solution := make([]float64, size)
	for i := range size {
		if math.Abs(augmented[i][i]) < constants.SingularPivotEpsilon {
			continue
		}
		solution[i] = augmented[i][size] / augmented[i][i]
	}
	return solution
}

func mean(values []float64) float64 {
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func ones(size int) []float64 {
	values := make([]float64, size)
	for i := range values {
		values[i] = 1
	}
	return values
}
