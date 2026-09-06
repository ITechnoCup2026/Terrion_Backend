package agronomy

import (
	"slices"
	"sort"
	"time"

	"terrion-backend/internal/constants"
)

type WeekBucket struct {
	ISOWeek     string
	WeekStart   time.Time
	CommodityID string
	Tonnes      float64
	BlockIDs    []string
}

type FlaggedWeek struct {
	WeekBucket
	Basis                constants.ThresholdBasis
	Threshold            float64
	ContributingBlockIDs []string
}

type StaggerSuggestion struct {
	ISOWeek         string
	CommodityID     string
	BlockIDs        []string
	ShiftDays       int
	TonnesMoved     float64
	ResultingTonnes float64
}

type CollisionReport struct {
	Weeks       []WeekBucket
	Flagged     []FlaggedWeek
	Suggestions []StaggerSuggestion
}

func BucketByWeek(projections []BlockProjection) []WeekBucket {
	buckets := map[string]*WeekBucket{}

	for _, projection := range projections {
		dayCount := max(1, DaysBetween(projection.Window.Start, projection.Window.End)+1)
		perDay := projection.ExpectedTonnes / float64(dayCount)

		for offset := range dayCount {
			day := AddDays(projection.Window.Start, offset)
			isoWeek := ISOWeekKey(day)
			key := projection.CommodityID + "|" + isoWeek

			bucket, exists := buckets[key]
			if !exists {
				buckets[key] = &WeekBucket{
					ISOWeek:     isoWeek,
					WeekStart:   ISOWeekStart(day),
					CommodityID: projection.CommodityID,
					Tonnes:      perDay,
					BlockIDs:    []string{projection.BlockID},
				}
				continue
			}

			bucket.Tonnes += perDay
			if !slices.Contains(bucket.BlockIDs, projection.BlockID) {
				bucket.BlockIDs = append(bucket.BlockIDs, projection.BlockID)
			}
		}
	}

	weeks := make([]WeekBucket, 0, len(buckets))
	for _, bucket := range buckets {
		weeks = append(weeks, *bucket)
	}
	sortWeeks(weeks)
	return weeks
}

// CommodityThreshold adalah ambang puncak mingguan satu komoditas beserta
// asal angkanya. Ia sudah dihitung di dalam DetectCollisions sejak awal, tetapi
// hanya menempel pada minggu yang tertandai — padahal rencana yang TIDAK
// menandai apa pun juga perlu menyatakan ia diukur terhadap apa.
type CommodityThreshold struct {
	CommodityID   string
	TonnesPerWeek float64
	Basis         constants.ThresholdBasis
}

// ThresholdsFor melaporkan ambang tiap komoditas yang muncul di proyeksi,
// tertandai atau tidak, terurut menurut komoditas supaya keluarannya stabil.
func ThresholdsFor(
	projections []BlockProjection, capacity map[string]float64,
) []CommodityThreshold {
	weeks := BucketByWeek(projections)

	seen := map[string]bool{}
	commodities := []string{}
	for _, week := range weeks {
		if !seen[week.CommodityID] {
			seen[week.CommodityID] = true
			commodities = append(commodities, week.CommodityID)
		}
	}
	sort.Strings(commodities)

	thresholds := make([]CommodityThreshold, 0, len(commodities))
	for _, commodityID := range commodities {
		tonnes, basis := thresholdFor(weeks, capacity, commodityID)
		thresholds = append(thresholds, CommodityThreshold{
			CommodityID:   commodityID,
			TonnesPerWeek: tonnes,
			Basis:         basis,
		})
	}
	return thresholds
}

func DetectCollisions(projections []BlockProjection, capacity map[string]float64) CollisionReport {
	if len(projections) == 0 {
		return CollisionReport{}
	}

	weeks := BucketByWeek(projections)

	flagged := []FlaggedWeek{}
	for _, week := range weeks {
		threshold, basis := thresholdFor(weeks, capacity, week.CommodityID)
		if threshold > 0 && week.Tonnes > threshold {
			flagged = append(flagged, FlaggedWeek{
				WeekBucket:           week,
				Basis:                basis,
				Threshold:            threshold,
				ContributingBlockIDs: append([]string{}, week.BlockIDs...),
			})
		}
	}

	suggestions := []StaggerSuggestion{}
	for _, week := range flagged {
		if best, found := relieve(week, projections); found {
			suggestions = append(suggestions, best)
		}
	}

	return CollisionReport{Weeks: weeks, Flagged: flagged, Suggestions: suggestions}
}

func relieve(week FlaggedWeek, projections []BlockProjection) (StaggerSuggestion, bool) {
	// Hanya blok yang belum ditanam. Menerbitkan saran yang menyentuh tanam
	// lampau berarti menawarkan tindakan yang pasti ditolak saat diterapkan,
	// dan dasbor tidak punya cara mengetahui itu sebelum tombolnya ditekan.
	shiftable := []BlockProjection{}
	for _, projection := range projections {
		if projection.Shiftable {
			shiftable = append(shiftable, projection)
		}
	}

	contributors := heaviestFirst(shiftable, week.ContributingBlockIDs)
	if len(contributors) == 0 {
		return StaggerSuggestion{}, false
	}

	var best StaggerSuggestion
	found := false

	for _, shiftDays := range constants.StaggerShiftCandidateDays {
		moved := []string{}
		remaining := append([]BlockProjection{}, projections...)

		for _, contributor := range contributors {
			moved = append(moved, contributor.BlockID)
			remaining = shiftWindow(remaining, contributor.BlockID, shiftDays)

			resulting := tonnesIn(BucketByWeek(remaining), week.ISOWeek, week.CommodityID)
			if resulting > week.Threshold {
				continue
			}

			candidate := StaggerSuggestion{
				ISOWeek:         week.ISOWeek,
				CommodityID:     week.CommodityID,
				BlockIDs:        append([]string{}, moved...),
				ShiftDays:       shiftDays,
				TonnesMoved:     week.Tonnes - resulting,
				ResultingTonnes: resulting,
			}
			if !found || len(candidate.BlockIDs) < len(best.BlockIDs) {
				best = candidate
				found = true
			}
			break
		}
	}

	return best, found
}

func heaviestFirst(projections []BlockProjection, blockIDs []string) []BlockProjection {
	contributors := []BlockProjection{}
	for _, projection := range projections {
		if slices.Contains(blockIDs, projection.BlockID) {
			contributors = append(contributors, projection)
		}
	}

	sort.SliceStable(contributors, func(i, j int) bool {
		if contributors[i].ExpectedTonnes != contributors[j].ExpectedTonnes {
			return contributors[i].ExpectedTonnes > contributors[j].ExpectedTonnes
		}
		return contributors[i].BlockID < contributors[j].BlockID
	})
	return contributors
}

func shiftWindow(projections []BlockProjection, blockID string, shiftDays int) []BlockProjection {
	shifted := make([]BlockProjection, len(projections))
	for i, projection := range projections {
		if projection.BlockID == blockID {
			projection.Window = DateRange{
				Start: AddDays(projection.Window.Start, shiftDays),
				End:   AddDays(projection.Window.End, shiftDays),
			}
		}
		shifted[i] = projection
	}
	return shifted
}

func tonnesIn(weeks []WeekBucket, isoWeek, commodityID string) float64 {
	for _, week := range weeks {
		if week.ISOWeek == isoWeek && week.CommodityID == commodityID {
			return week.Tonnes
		}
	}
	return 0
}

func thresholdFor(
	weeks []WeekBucket, capacity map[string]float64, commodityID string,
) (float64, constants.ThresholdBasis) {
	if stated, ok := capacity[commodityID]; ok {
		return stated, constants.ThresholdCapacity
	}

	tonnages := []float64{}
	for _, week := range weeks {
		if week.CommodityID == commodityID {
			tonnages = append(tonnages, week.Tonnes)
		}
	}
	return median(tonnages) * constants.MedianCapacityMultiplier, constants.ThresholdMedian
}

func median(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sorted := append([]float64{}, values...)
	sort.Float64s(sorted)

	middle := len(sorted) / 2
	if len(sorted)%2 == 1 {
		return sorted[middle]
	}
	return (sorted[middle-1] + sorted[middle]) / 2
}

func sortWeeks(weeks []WeekBucket) {
	sort.Slice(weeks, func(i, j int) bool {
		if !weeks[i].WeekStart.Equal(weeks[j].WeekStart) {
			return weeks[i].WeekStart.Before(weeks[j].WeekStart)
		}
		return weeks[i].CommodityID < weeks[j].CommodityID
	})
}
