package rdkk_test

import (
	"testing"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/rdkk"
)

func requirement(inputItem string, quantityKg float64) rdkk.RequirementLine {
	return rdkk.RequirementLine{
		InputItem:  inputItem,
		QuantityKg: quantityKg,
		Sources:    []string{"Permentan 01/2025"},
	}
}

func TestToOrderLinesConvertsKilogramsIntoWholeSacks(t *testing.T) {
	drafts := rdkk.ToOrderLines([]rdkk.RequirementLine{requirement("urea", 500)})

	if len(drafts) != 1 {
		t.Fatalf("len(drafts) = %d, want 1", len(drafts))
	}
	if drafts[0].Item != "urea" || drafts[0].Quantity != 10 || drafts[0].QuantityKg != 500 {
		t.Errorf("draft = %+v, want 10 sacks of urea from 500 kg", drafts[0])
	}
	if drafts[0].Unit != "karung 50 kg" {
		t.Errorf("Unit = %q, want karung 50 kg", drafts[0].Unit)
	}
}

func TestToOrderLinesRoundsAPartSackUp(t *testing.T) {
	tests := []struct {
		item       string
		quantityKg float64
		want       float64
	}{
		{"npk", 7340, 147},
		{"kcl", 50.1, 2},
		{"sp36", 49.9, 1},
	}

	for _, test := range tests {
		drafts := rdkk.ToOrderLines([]rdkk.RequirementLine{
			requirement(test.item, test.quantityKg),
		})
		if drafts[0].Quantity != test.want {
			t.Errorf("%v kg = %v sacks, want %v: rounding down orders less than the season needs",
				test.quantityKg, drafts[0].Quantity, test.want)
		}
	}
}

func TestToOrderLinesKeepsTheExactKilograms(t *testing.T) {
	drafts := rdkk.ToOrderLines([]rdkk.RequirementLine{requirement("urea", 7340)})

	if drafts[0].QuantityKg != 7340 {
		t.Errorf("QuantityKg = %v, want the exact 7340", drafts[0].QuantityKg)
	}
}

func TestToOrderLinesDropsLinesWithNothingToOrder(t *testing.T) {
	drafts := rdkk.ToOrderLines([]rdkk.RequirementLine{
		requirement("urea", 0),
		requirement("npk", 120),
	})

	if len(drafts) != 1 || drafts[0].Item != "npk" {
		t.Errorf("drafts = %+v, want only npk", drafts)
	}
}

func TestToOrderLinesOfNothingIsEmpty(t *testing.T) {
	if drafts := rdkk.ToOrderLines(nil); len(drafts) != 0 {
		t.Errorf("len(drafts) = %d, want 0", len(drafts))
	}
}

func TestToOrderLinesPreservesTheOrderItWasGiven(t *testing.T) {
	drafts := rdkk.ToOrderLines([]rdkk.RequirementLine{
		requirement("kcl", 100),
		requirement("npk", 100),
		requirement("urea", 100),
	})

	want := []string{"kcl", "npk", "urea"}
	for i, draft := range drafts {
		if draft.Item != want[i] {
			t.Errorf("drafts[%d].Item = %q, want %q", i, draft.Item, want[i])
		}
	}
}

func TestKgPerSackIsFifty(t *testing.T) {
	if constants.KgPerSack != 50 {
		t.Errorf("KgPerSack = %v, want 50", constants.KgPerSack)
	}
}
