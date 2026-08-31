package rdkk_test

import (
	"testing"
	"time"

	"terrion-backend/internal/rdkk"
)

var documentMeta = rdkk.DocumentMeta{
	CooperativeName: "Koperasi Tani Subang Jaya",
	Village:         "Pamanukan",
	District:        "Kabupaten Subang",
	Province:        "Jawa Barat",
	SeasonLabel:     "musim ini",
	PrintedAt:       time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
}

func document(blocks []rdkk.PlantedBlock, using []rdkk.FertiliserRate) rdkk.Document {
	return rdkk.BuildDocument(rdkk.AggregateInputs(blocks, using), documentMeta)
}

func rowFor(t *testing.T, doc rdkk.Document, memberName string) rdkk.DocumentRow {
	t.Helper()
	for _, row := range doc.Rows {
		if row.MemberName == memberName {
			return row
		}
	}
	t.Fatalf("no row for %q", memberName)
	return rdkk.DocumentRow{}
}

func columnOf(t *testing.T, doc rdkk.Document, item string) int {
	t.Helper()
	for i, column := range doc.Columns {
		if column == item {
			return i
		}
	}
	t.Fatalf("no column for %q in %v", item, doc.Columns)
	return -1
}

func TestBuildDocumentOfNothingHasNoGrid(t *testing.T) {
	doc := document(nil, rates)

	if len(doc.Columns) != 0 || len(doc.Rows) != 0 ||
		len(doc.Totals) != 0 || len(doc.Sources) != 0 {
		t.Errorf("doc = %+v, want an empty form", doc)
	}
}

func TestBuildDocumentTakesColumnsFromTheUnionOfInputItems(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 1),
		planted("b2", "m2", "Bu Iis", "jagung", 1),
	}, rates)

	if len(doc.Columns) != 2 || doc.Columns[0] != "sp36" || doc.Columns[1] != "urea" {
		t.Errorf("Columns = %v, want [sp36 urea]", doc.Columns)
	}
}

func TestBuildDocumentLeavesAnUnneededInputNullNeverZero(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 1),
		planted("b2", "m2", "Bu Iis", "jagung", 1),
	}, rates)

	iis := rowFor(t, doc, "Bu Iis")
	if iis.QuantitiesKg[columnOf(t, doc, "sp36")] != nil {
		t.Error("sp36 cell is set, want nil: a printed 0 is an order for zero sacks")
	}
	urea := iis.QuantitiesKg[columnOf(t, doc, "urea")]
	if urea == nil {
		t.Fatal("urea cell is nil, want 300")
	}
	closeTo(t, "urea", *urea, 300)
}

func TestBuildDocumentOrdersRowsByMemberName(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m2", "Pak Ujang", "padi", 1),
		planted("b2", "m1", "Bu Iis", "padi", 1),
	}, rates)

	if doc.Rows[0].MemberName != "Bu Iis" || doc.Rows[1].MemberName != "Pak Ujang" {
		t.Errorf("rows = %q, %q, want Bu Iis then Pak Ujang",
			doc.Rows[0].MemberName, doc.Rows[1].MemberName)
	}
}

func TestBuildDocumentCarriesTheAggregatesOwnTotals(t *testing.T) {
	aggregate := rdkk.AggregateInputs([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 2),
		planted("b2", "m2", "Bu Iis", "padi", 1),
	}, rates)
	doc := rdkk.BuildDocument(aggregate, documentMeta)

	closeTo(t, "total urea", doc.Totals[columnOf(t, doc, "urea")], 750)
}

func TestBuildDocumentListsEveryRateDocumentOnce(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 1),
		planted("b2", "m2", "Bu Iis", "jagung", 1),
	}, rates)

	if len(doc.Sources) != 2 ||
		doc.Sources[0] != "Kementan jagung" || doc.Sources[1] != "Permentan 40/2007" {
		t.Errorf("Sources = %v, want both documents once each", doc.Sources)
	}
}

func TestBuildDocumentCarriesAnUnverifiedSourceToTheForm(t *testing.T) {
	doc := document(
		[]rdkk.PlantedBlock{planted("b1", "m1", "Pak Ujang", "cabai", 1)},
		[]rdkk.FertiliserRate{{
			CommodityID: "cabai", InputItem: "urea", KgPerHa: 200,
			Source: "BELUM DIVERIFIKASI",
		}})

	if len(doc.Sources) != 1 || doc.Sources[0] != "BELUM DIVERIFIKASI" {
		t.Errorf("Sources = %v, want the unverified marker to survive to the paper",
			doc.Sources)
	}
}

func TestBuildDocumentCountsMembersOverTheCap(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 3.5),
		planted("b2", "m2", "Bu Iis", "padi", 1),
	}, rates)

	if doc.MembersOverCap != 1 {
		t.Errorf("MembersOverCap = %d, want 1", doc.MembersOverCap)
	}
	ujangRow := rowFor(t, doc, "Pak Ujang")
	if !ujangRow.OverSubsidyCap {
		t.Error("OverSubsidyCap = false, want true")
	}
	closeTo(t, "ExcessHa", ujangRow.ExcessHa, 1.5)
}

func TestBuildDocumentNamesCommoditiesWithNoPublishedRate(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "beri", 1),
	}, rates)

	if len(doc.CommoditiesWithoutRates) != 1 || doc.CommoditiesWithoutRates[0] != "beri" {
		t.Errorf("CommoditiesWithoutRates = %v, want [beri]", doc.CommoditiesWithoutRates)
	}
}

func TestBuildDocumentCarriesTheLetterheadAndTheAreaItCovers(t *testing.T) {
	doc := document([]rdkk.PlantedBlock{
		planted("b1", "m1", "Pak Ujang", "padi", 2),
		planted("b2", "m2", "Bu Iis", "padi", 1.5),
	}, rates)

	if doc.Meta.CooperativeName != "Koperasi Tani Subang Jaya" ||
		doc.Meta.District != "Kabupaten Subang" ||
		doc.Meta.Province != "Jawa Barat" ||
		doc.Meta.SeasonLabel != "musim ini" {
		t.Errorf("Meta = %+v, want the cooperative identity the form is filed under", doc.Meta)
	}
	closeTo(t, "TotalPlantedHa", doc.TotalPlantedHa, 3.5)
	if doc.MemberCount != 2 {
		t.Errorf("MemberCount = %d, want 2", doc.MemberCount)
	}
}
