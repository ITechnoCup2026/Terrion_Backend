package rdkk

import (
	"sort"
	"time"
)

type DocumentMeta struct {
	CooperativeName string
	Village         string
	District        string
	Province        string
	SeasonLabel     string
	PrintedAt       time.Time
}

type DocumentRow struct {
	MemberID       string
	MemberName     string
	PlantedHa      float64
	QuantitiesKg   []*float64
	OverSubsidyCap bool
	ExcessHa       float64
}

type Document struct {
	Meta                    DocumentMeta
	Columns                 []string
	Rows                    []DocumentRow
	Totals                  []float64
	Sources                 []string
	MemberCount             int
	TotalPlantedHa          float64
	MembersOverCap          int
	CommoditiesWithoutRates []string
}

func BuildDocument(aggregate Aggregate, meta DocumentMeta) Document {
	columns := make([]string, len(aggregate.Totals))
	totals := make([]float64, len(aggregate.Totals))
	for i, line := range aggregate.Totals {
		columns[i] = line.InputItem
		totals[i] = line.QuantityKg
	}

	rows := make([]DocumentRow, len(aggregate.Members))
	totalPlantedHa := 0.0
	membersOverCap := 0

	for i, member := range aggregate.Members {
		byItem := make(map[string]float64, len(member.Lines))
		for _, line := range member.Lines {
			byItem[line.InputItem] = line.QuantityKg
		}

		quantities := make([]*float64, len(columns))
		for column, item := range columns {
			if quantityKg, needed := byItem[item]; needed {
				held := quantityKg
				quantities[column] = &held
			}
		}

		rows[i] = DocumentRow{
			MemberID:       member.MemberID,
			MemberName:     member.MemberName,
			PlantedHa:      member.PlantedHa,
			QuantitiesKg:   quantities,
			OverSubsidyCap: member.OverSubsidyCap,
			ExcessHa:       member.ExcessHa,
		}

		totalPlantedHa += member.PlantedHa
		if member.OverSubsidyCap {
			membersOverCap++
		}
	}

	return Document{
		Meta:                    meta,
		Columns:                 columns,
		Rows:                    rows,
		Totals:                  totals,
		Sources:                 distinctSources(aggregate.Totals),
		MemberCount:             len(rows),
		TotalPlantedHa:          totalPlantedHa,
		MembersOverCap:          membersOverCap,
		CommoditiesWithoutRates: aggregate.CommoditiesWithoutRates,
	}
}

func distinctSources(totals []RequirementLine) []string {
	seen := map[string]bool{}
	sources := []string{}

	for _, line := range totals {
		for _, source := range line.Sources {
			if seen[source] {
				continue
			}
			seen[source] = true
			sources = append(sources, source)
		}
	}

	sort.Strings(sources)
	return sources
}
