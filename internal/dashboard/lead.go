package dashboard

import (
	"sort"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type CollisionWeek struct {
	agronomy.FlaggedWeek
	PlotCount int
}

func SelectLeadCollision(weeks []CollisionWeek) *CollisionWeek {
	if len(weeks) == 0 {
		return nil
	}

	pool := []CollisionWeek{}
	for _, week := range weeks {
		if week.PlotCount >= constants.PileUpMinPlots {
			pool = append(pool, week)
		}
	}
	if len(pool) == 0 {
		pool = append(pool, weeks...)
	}

	sort.SliceStable(pool, func(i, j int) bool {
		if pool[i].Tonnes != pool[j].Tonnes {
			return pool[i].Tonnes > pool[j].Tonnes
		}
		return pool[i].PlotCount > pool[j].PlotCount
	})

	lead := pool[0]
	return &lead
}
