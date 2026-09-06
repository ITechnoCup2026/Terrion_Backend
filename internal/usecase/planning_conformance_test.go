package usecase

import (
	"context"
	"testing"

	"gorm.io/gorm"

	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
)

// Tarif Permentan yang dipakai RDKK. Angkanya tidak penting bagi uji ini;
// yang diuji adalah bahwa rencana membawanya, bukan berapa besarnya.
func seedFertiliserRates(t *testing.T, db *gorm.DB) {
	t.Helper()

	for _, rate := range []entity.FertiliserRate{
		{CommodityID: riceCommodity, InputItem: "Urea", KgPerHa: 250, Source: "Permentan"},
		{CommodityID: riceCommodity, InputItem: "SP-36", KgPerHa: 100, Source: "Permentan"},
		{CommodityID: riceCommodity, InputItem: "KCl", KgPerHa: 75, Source: "Permentan"},
	} {
		if err := db.Create(&rate).Error; err != nil {
			t.Fatalf("seeding rate %s: %v", rate.InputItem, err)
		}
	}
}

// Spesifikasi fitur menuntut tiap rencana MENYATAKAN dasarnya: ambang mana yang
// dipakai (kapasitas koperasi vs 2,5 x median), pupuk yang dibutuhkan, dan
// anggota mana yang melewati batas subsidi 2 ha. Ketiganya sudah dihitung mesin
// lama; yang kurang adalah meneruskannya keluar.

func TestProposeStatesWhichCapacityThresholdItUsed(t *testing.T) {
	db := seedPlanningFixture(t)

	proposal, err := planningUseCase(t, db).Propose(
		context.Background(), homeCoop, planSeason, "", planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, plan := range proposal.Plans {
		if len(plan.Thresholds) == 0 {
			t.Fatalf("rencana %q tidak menyatakan satu pun ambang; pengurus tidak "+
				"bisa tahu angka puncaknya dibandingkan terhadap apa", plan.Objective)
		}
		for _, threshold := range plan.Thresholds {
			// Fikstur ini tidak menyemai kapasitas koperasi, jadi satu-satunya
			// dasar yang jujur adalah 2,5 x median.
			if threshold.Basis != constants.ThresholdMedian {
				t.Errorf("dasar ambang = %q, mau %q ketika kapasitas belum diisi",
					threshold.Basis, constants.ThresholdMedian)
			}
			if threshold.TonnesPerWeek <= 0 {
				t.Errorf("ambang %v ton/minggu tidak masuk akal", threshold.TonnesPerWeek)
			}
		}
	}
}

func TestProposeSaysCapacityWhenTheCooperativeStatedOne(t *testing.T) {
	db := seedPlanningFixture(t)
	if err := db.Create(&entity.CooperativeCapacity{
		CooperativeID: homeCoop, CommodityID: riceCommodity, TonnesPerWeek: 9,
	}).Error; err != nil {
		t.Fatalf("seeding capacity: %v", err)
	}

	proposal, err := planningUseCase(t, db).Propose(
		context.Background(), homeCoop, planSeason, "", planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, plan := range proposal.Plans {
		for _, threshold := range plan.Thresholds {
			if threshold.CommodityID != riceCommodity {
				continue
			}
			if threshold.Basis != constants.ThresholdCapacity {
				t.Errorf("dasar ambang = %q, mau %q ketika koperasi menyatakan kapasitas",
					threshold.Basis, constants.ThresholdCapacity)
			}
			if threshold.TonnesPerWeek != 9 {
				t.Errorf("ambang = %v, mau 9 ton/minggu seperti yang dinyatakan koperasi",
					threshold.TonnesPerWeek)
			}
		}
	}
}

func TestProposeCarriesTheFertiliserARdkkWouldNeed(t *testing.T) {
	db := seedPlanningFixture(t)
	seedFertiliserRates(t, db)

	proposal, err := planningUseCase(t, db).Propose(
		context.Background(), homeCoop, planSeason, "", planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, plan := range proposal.Plans {
		if len(plan.Fertiliser.Totals) == 0 {
			t.Fatalf("rencana %q tidak membawa kebutuhan pupuk; RDKK pra-musim "+
				"adalah alasan utama fitur ini ada", plan.Objective)
		}
		for _, line := range plan.Fertiliser.Totals {
			if line.QuantityKg <= 0 {
				t.Errorf("%s = %v kg", line.InputItem, line.QuantityKg)
			}
		}
		if len(plan.Fertiliser.Members) == 0 {
			t.Error("kebutuhan pupuk tidak terbaca per anggota")
		}
	}
}

func TestProposeNamesTheMembersOverTheSubsidyCap(t *testing.T) {
	db := seedPlanningFixture(t)
	seedFertiliserRates(t, db)
	// Satu lahan seluas 3 ha membuat pemiliknya melewati batas subsidi 2 ha.
	if err := db.Model(&entity.Plot{}).Where("id = ?", "plot-1").
		Update("area_ha", 3.0).Error; err != nil {
		t.Fatalf("widening plot-1: %v", err)
	}

	proposal, err := planningUseCase(t, db).Propose(
		context.Background(), homeCoop, planSeason, "", planningNow)
	if err != nil {
		t.Fatalf("Propose: %v", err)
	}

	for _, plan := range proposal.Plans {
		flagged := map[string]float64{}
		for _, member := range plan.Fertiliser.Members {
			if member.OverSubsidyCap {
				flagged[member.MemberName] = member.ExcessHa
			}
		}
		if len(flagged) == 0 {
			t.Fatalf("rencana %q tidak menandai satu pun anggota di atas 2 ha, "+
				"padahal ada lahan 3 ha", plan.Objective)
		}
		for name, excess := range flagged {
			if name == "" {
				t.Error("anggota ditandai tanpa nama; spesifikasi meminta menurut nama")
			}
			if excess <= 0 {
				t.Errorf("%s ditandai dengan kelebihan %v ha", name, excess)
			}
		}
	}
}
