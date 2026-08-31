package rdkk_test

import (
	"math"
	"sort"
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/rdkk"
)

var rates = []rdkk.FertiliserRate{
	{CommodityID: "padi", InputItem: "urea", KgPerHa: 250, Source: "Permentan 40/2007"},
	{CommodityID: "padi", InputItem: "sp36", KgPerHa: 100, Source: "Permentan 40/2007"},
	{CommodityID: "jagung", InputItem: "urea", KgPerHa: 300, Source: "Kementan jagung"},
}

func planted(blockID, memberID, memberName, commodityID string, areaHa float64) rdkk.PlantedBlock {
	return rdkk.PlantedBlock{
		BlockID:     blockID,
		MemberID:    memberID,
		MemberName:  memberName,
		CommodityID: commodityID,
		AreaHa:      areaHa,
	}
}

func ujang(blockID, commodityID string, areaHa float64) rdkk.PlantedBlock {
	return planted(blockID, "m1", "Pak Ujang", commodityID, areaHa)
}

func lineFor(t *testing.T, lines []rdkk.RequirementLine, item string) rdkk.RequirementLine {
	t.Helper()
	for _, line := range lines {
		if line.InputItem == item {
			return line
		}
	}
	t.Fatalf("no line for %q in %+v", item, lines)
	return rdkk.RequirementLine{}
}

func closeTo(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 5e-6 {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}

func TestAggregateInputsOfNothingIsEmpty(t *testing.T) {
	result := rdkk.AggregateInputs(nil, rates)

	if len(result.Members) != 0 || len(result.Totals) != 0 ||
		len(result.CommoditiesWithoutRates) != 0 {
		t.Errorf("result = %+v, want everything empty", result)
	}
}

func TestAggregateInputsMultipliesTheRateByThePlantedArea(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{ujang("b1", "padi", 1.2)}, rates)

	if len(result.Members) != 1 {
		t.Fatalf("len(Members) = %d, want 1", len(result.Members))
	}
	closeTo(t, "urea", lineFor(t, result.Members[0].Lines, "urea").QuantityKg, 300)
	closeTo(t, "sp36", lineFor(t, result.Members[0].Lines, "sp36").QuantityKg, 120)
}

func TestAggregateInputsSumsAMemberAcrossBlocks(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		ujang("a", "padi", 0.6),
		ujang("b", "padi", 0.4),
	}, rates)

	closeTo(t, "PlantedHa", result.Members[0].PlantedHa, 1)
	closeTo(t, "urea", lineFor(t, result.Members[0].Lines, "urea").QuantityKg, 250)
}

func TestAggregateInputsAddsOneInputAcrossCommodities(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		ujang("a", "padi", 1),
		ujang("b", "jagung", 0.5),
	}, rates)

	closeTo(t, "urea", lineFor(t, result.Members[0].Lines, "urea").QuantityKg, 400)
}

func TestAggregateInputsRollsMembersUpIntoTotals(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		planted("a", "m1", "Pak Ujang", "padi", 1),
		planted("b", "m2", "Bu Imas", "padi", 1.5),
	}, rates)

	closeTo(t, "total urea", lineFor(t, result.Totals, "urea").QuantityKg, 625)
	closeTo(t, "total sp36", lineFor(t, result.Totals, "sp36").QuantityKg, 250)
}

func TestAggregateInputsKeepsOneRowPerMember(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		planted("a", "m1", "Pak Ujang", "padi", 1),
		planted("b", "m1", "Pak Ujang", "padi", 1),
		planted("c", "m2", "Bu Imas", "padi", 1),
	}, rates)

	ids := []string{}
	for _, member := range result.Members {
		ids = append(ids, member.MemberID)
	}
	sort.Strings(ids)

	if len(ids) != 2 || ids[0] != "m1" || ids[1] != "m2" {
		t.Errorf("member ids = %v, want [m1 m2]", ids)
	}
}

func TestSubsidyCapIsTwoHectares(t *testing.T) {
	if constants.SubsidyCapHa != 2 {
		t.Errorf("SubsidyCapHa = %v, want 2", constants.SubsidyCapHa)
	}
}

func TestAggregateInputsFlagsAMemberPastTheCapWithoutCuttingQuantities(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{ujang("b1", "padi", 3)}, rates)

	member := result.Members[0]
	if !member.OverSubsidyCap {
		t.Error("OverSubsidyCap = false, want true")
	}
	closeTo(t, "ExcessHa", member.ExcessHa, 1)
	closeTo(t, "urea", lineFor(t, member.Lines, "urea").QuantityKg, 750)
}

func TestAggregateInputsDoesNotFlagAMemberExactlyOnTheCap(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{ujang("b1", "padi", 2)}, rates)

	if result.Members[0].OverSubsidyCap {
		t.Error("OverSubsidyCap = true, want false at exactly the cap")
	}
	if result.Members[0].ExcessHa != 0 {
		t.Errorf("ExcessHa = %v, want 0", result.Members[0].ExcessHa)
	}
}

func TestAggregateInputsAppliesTheCapPerFarmer(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		planted("a", "m1", "Pak Ujang", "padi", 1.5),
		planted("b", "m2", "Bu Imas", "padi", 1.5),
	}, rates)

	for _, member := range result.Members {
		if member.OverSubsidyCap {
			t.Errorf("%s flagged, want nobody over the cap: it is per farmer, not per cooperative",
				member.MemberName)
		}
	}
}

func TestAggregateInputsCountsAFarmerPastTheCapAcrossSmallBlocks(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		ujang("a", "padi", 1.2),
		ujang("b", "padi", 1.1),
	}, rates)

	if !result.Members[0].OverSubsidyCap {
		t.Error("OverSubsidyCap = false, want true")
	}
	closeTo(t, "ExcessHa", result.Members[0].ExcessHa, 0.3)
}

func TestAggregateInputsCarriesEveryDistinctSource(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		ujang("a", "padi", 1),
		ujang("b", "jagung", 1),
	}, rates)

	sources := lineFor(t, result.Totals, "urea").Sources
	sort.Strings(sources)

	if len(sources) != 2 || sources[0] != "Kementan jagung" || sources[1] != "Permentan 40/2007" {
		t.Errorf("sources = %v, want both rate documents", sources)
	}
}

func TestAggregateInputsDoesNotRepeatASharedSource(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		planted("a", "m1", "Pak Ujang", "padi", 1),
		planted("b", "m2", "Bu Imas", "padi", 1),
	}, rates)

	sources := lineFor(t, result.Totals, "urea").Sources
	if len(sources) != 1 || sources[0] != "Permentan 40/2007" {
		t.Errorf("sources = %v, want one entry", sources)
	}
}

func TestAggregateInputsNamesCommoditiesWithNoPublishedRate(t *testing.T) {
	result := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		ujang("a", "padi", 1),
		ujang("b", "beri", 1),
	}, rates)

	if len(result.CommoditiesWithoutRates) != 1 || result.CommoditiesWithoutRates[0] != "beri" {
		t.Errorf("CommoditiesWithoutRates = %v, want [beri]", result.CommoditiesWithoutRates)
	}
	closeTo(t, "PlantedHa", result.Members[0].PlantedHa, 2)
	closeTo(t, "urea", lineFor(t, result.Members[0].Lines, "urea").QuantityKg, 250)
}
