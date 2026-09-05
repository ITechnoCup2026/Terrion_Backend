package aiclient

import (
	"fmt"
	"math"
	"sort"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/planning"
)

type RequestInput struct {
	RequestID  string
	Seed       int64
	Season     planning.Season
	Objectives []planning.Objective
	Candidates []planning.Candidate
	Demand     []planning.DemandRow
	Capacity   *float64
}

type Mapping struct {
	plotByRef      map[string]string
	commodityByRef map[string]string
	varietyByRef   map[string]string
	refByCommodity map[string]string
	candidateIDs   map[string]struct{}
}

func (m *Mapping) Knows(candidateID string) bool {
	_, ok := m.candidateIDs[candidateID]
	return ok
}

func (m *Mapping) PlotID(ref string) (string, bool) {
	id, ok := m.plotByRef[ref]
	return id, ok
}

func (m *Mapping) VarietyID(ref string) (string, bool) {
	id, ok := m.varietyByRef[ref]
	return id, ok
}

var plausibilityRef = map[constants.Plausibility]string{
	constants.PlausibilityOk:    "plausible",
	constants.PlausibilityEarly: "early",
	constants.PlausibilityLate:  "late",
}

func round2(value float64) float64 {
	return math.Round(value*100) / 100
}

func assignRefs(prefix string, ids []string) (map[string]string, map[string]string) {
	unique := make([]string, 0, len(ids))
	seen := map[string]bool{}
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	sort.Strings(unique)

	refByID := make(map[string]string, len(unique))
	idByRef := make(map[string]string, len(unique))
	for i, id := range unique {
		ref := fmt.Sprintf("%s%d", prefix, i+1)
		refByID[id] = ref
		idByRef[ref] = id
	}
	return refByID, idByRef
}

func BuildRequest(input RequestInput) (Request, *Mapping) {
	usable := make([]planning.Candidate, 0, len(input.Candidates))
	for _, candidate := range input.Candidates {
		if _, ok := plausibilityRef[candidate.Plausibility]; ok {
			usable = append(usable, candidate)
		}
	}
	sort.Slice(usable, func(i, j int) bool { return usable[i].ID < usable[j].ID })
	if len(usable) > MaxCandidates {
		usable = usable[:MaxCandidates]
	}

	plotIDs := make([]string, 0, len(usable))
	commodityIDs := make([]string, 0, len(usable))
	varietyIDs := make([]string, 0, len(usable))
	for _, candidate := range usable {
		plotIDs = append(plotIDs, candidate.PlotID)
		commodityIDs = append(commodityIDs, candidate.CommodityID)
		varietyIDs = append(varietyIDs, candidate.VarietyID)
	}
	for _, row := range input.Demand {
		commodityIDs = append(commodityIDs, row.CommodityID)
	}

	plotRefByID, plotByRef := assignRefs("p", plotIDs)
	commodityRefByID, commodityByRef := assignRefs("k", commodityIDs)
	varietyRefByID, varietyByRef := assignRefs("v", varietyIDs)

	candidates := make([]Candidate, 0, len(usable))
	knownIDs := make(map[string]struct{}, len(usable))
	for _, candidate := range usable {
		knownIDs[candidate.ID] = struct{}{}

		var price *float64
		if candidate.PricePerKg != nil {
			rounded := round2(*candidate.PricePerKg)
			price = &rounded
		}

		candidates = append(candidates, Candidate{
			ID:           candidate.ID,
			PlotRef:      plotRefByID[candidate.PlotID],
			AreaHa:       round2(candidate.AreaHa),
			CommodityRef: commodityRefByID[candidate.CommodityID],
			VarietyRef:   varietyRefByID[candidate.VarietyID],
			PlantingDate: agronomy.ToISODate(candidate.PlantingDate),
			HarvestStart: agronomy.ToISODate(candidate.HarvestStart),
			HarvestEnd:   agronomy.ToISODate(candidate.HarvestEnd),
			TonnesLow:    round2(candidate.TonnesLow),
			TonnesMid:    round2(candidate.TonnesMid),
			TonnesHigh:   round2(candidate.TonnesHigh),
			Plausibility: plausibilityRef[candidate.Plausibility],
			PricePerKg:   price,
		})
	}

	demand := make([]DemandRow, 0, len(input.Demand))
	for _, row := range input.Demand {
		ref, ok := commodityRefByID[row.CommodityID]
		if !ok {
			continue
		}
		demand = append(demand, DemandRow{
			CommodityRef: ref,
			ISOWeek:      planning.WeekKey(row.Week),
			Kg:           row.Kg,
		})
	}
	sort.Slice(demand, func(i, j int) bool {
		if demand[i].CommodityRef != demand[j].CommodityRef {
			return demand[i].CommodityRef < demand[j].CommodityRef
		}
		return demand[i].ISOWeek < demand[j].ISOWeek
	})
	if len(demand) > MaxDemandRows {
		demand = demand[:MaxDemandRows]
	}

	objectives := make([]string, 0, len(input.Objectives))
	for _, objective := range input.Objectives {
		objectives = append(objectives, string(objective))
	}

	var capacity *float64
	if input.Capacity != nil {
		rounded := round2(*input.Capacity)
		capacity = &rounded
	}

	request := Request{
		ContractVersion: ContractVersion,
		RequestID:       input.RequestID,
		Seed:            input.Seed,
		Season: Season{
			Label: input.Season.Label,
			Start: agronomy.ToISODate(input.Season.Start),
			End:   agronomy.ToISODate(input.Season.End),
		},
		Objectives:            objectives,
		CapacityTonnesPerWeek: capacity,
		Candidates:            candidates,
		Demand:                demand,
	}

	return request, &Mapping{
		plotByRef:      plotByRef,
		commodityByRef: commodityByRef,
		varietyByRef:   varietyByRef,
		refByCommodity: commodityRefByID,
		candidateIDs:   knownIDs,
	}
}
