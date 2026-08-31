package rdkk

import (
	"math"
	"sort"

	"terrion-backend/internal/constants"
)

type PlantedBlock struct {
	BlockID     string
	MemberID    string
	MemberName  string
	CommodityID string
	AreaHa      float64
}

type FertiliserRate struct {
	CommodityID string
	InputItem   string
	KgPerHa     float64
	Source      string
}

type RequirementLine struct {
	InputItem  string
	QuantityKg float64
	Sources    []string
}

type MemberRequirement struct {
	MemberID       string
	MemberName     string
	PlantedHa      float64
	OverSubsidyCap bool
	ExcessHa       float64
	Lines          []RequirementLine
}

type Aggregate struct {
	Members                 []MemberRequirement
	Totals                  []RequirementLine
	CommoditiesWithoutRates []string
}

func AggregateInputs(blocks []PlantedBlock, rates []FertiliserRate) Aggregate {
	ratesByCommodity := map[string][]FertiliserRate{}
	for _, rate := range rates {
		ratesByCommodity[rate.CommodityID] = append(ratesByCommodity[rate.CommodityID], rate)
	}

	type bucket struct {
		memberName      string
		areaByCommodity map[string]float64
		order           []string
	}

	byMember := map[string]*bucket{}
	memberOrder := []string{}
	unrated := map[string]bool{}

	for _, block := range blocks {
		if _, rated := ratesByCommodity[block.CommodityID]; !rated {
			unrated[block.CommodityID] = true
		}

		held, seen := byMember[block.MemberID]
		if !seen {
			held = &bucket{memberName: block.MemberName, areaByCommodity: map[string]float64{}}
			byMember[block.MemberID] = held
			memberOrder = append(memberOrder, block.MemberID)
		}
		if _, counted := held.areaByCommodity[block.CommodityID]; !counted {
			held.order = append(held.order, block.CommodityID)
		}
		held.areaByCommodity[block.CommodityID] += block.AreaHa
	}

	totals := newLineSet()
	members := make([]MemberRequirement, 0, len(byMember))

	for _, memberID := range memberOrder {
		held := byMember[memberID]
		lines := newLineSet()
		plantedHa := 0.0

		for _, commodityID := range held.order {
			areaHa := held.areaByCommodity[commodityID]
			plantedHa += areaHa

			for _, rate := range ratesByCommodity[commodityID] {
				quantityKg := rate.KgPerHa * areaHa
				lines.add(rate.InputItem, quantityKg, rate.Source)
				totals.add(rate.InputItem, quantityKg, rate.Source)
			}
		}

		members = append(members, MemberRequirement{
			MemberID:       memberID,
			MemberName:     held.memberName,
			PlantedHa:      plantedHa,
			OverSubsidyCap: plantedHa > constants.SubsidyCapHa,
			ExcessHa:       math.Max(0, plantedHa-constants.SubsidyCapHa),
			Lines:          lines.ordered(),
		})
	}

	sort.SliceStable(members, func(i, j int) bool {
		if members[i].MemberName != members[j].MemberName {
			return members[i].MemberName < members[j].MemberName
		}
		return members[i].MemberID < members[j].MemberID
	})

	withoutRates := make([]string, 0, len(unrated))
	for commodityID := range unrated {
		withoutRates = append(withoutRates, commodityID)
	}
	sort.Strings(withoutRates)

	return Aggregate{
		Members:                 members,
		Totals:                  totals.ordered(),
		CommoditiesWithoutRates: withoutRates,
	}
}

type lineSet struct {
	byItem map[string]*RequirementLine
	order  []string
}

func newLineSet() *lineSet {
	return &lineSet{byItem: map[string]*RequirementLine{}}
}

func (s *lineSet) add(inputItem string, quantityKg float64, source string) {
	line, seen := s.byItem[inputItem]
	if !seen {
		s.byItem[inputItem] = &RequirementLine{
			InputItem:  inputItem,
			QuantityKg: quantityKg,
			Sources:    []string{source},
		}
		s.order = append(s.order, inputItem)
		return
	}

	line.QuantityKg += quantityKg
	for _, known := range line.Sources {
		if known == source {
			return
		}
	}
	line.Sources = append(line.Sources, source)
}

func (s *lineSet) ordered() []RequirementLine {
	items := append([]string{}, s.order...)
	sort.Strings(items)

	lines := make([]RequirementLine, len(items))
	for i, item := range items {
		lines[i] = *s.byItem[item]
	}
	return lines
}
