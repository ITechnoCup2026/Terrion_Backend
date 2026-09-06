package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
	"terrion-backend/internal/entity"
	"terrion-backend/internal/planning"
)

func seedAppliedPlan(
	t *testing.T, useCase *PlanningUseCase, user *entity.AppUser,
	assignments []planning.Assignment,
) *entity.SeasonPlan {
	t.Helper()

	plan := &entity.SeasonPlan{
		ID:            "plan-" + time.Now().Format("150405.000000000"),
		CooperativeID: homeCoop,
		SeasonLabel:   planSeason,
		SeasonStart:   planningNow,
		SeasonEnd:     agronomy.AddDays(planningNow, 180),
		Objective:     constants.ObjectiveSafe,
		Status:        constants.PlanApplied,
		CreatedBy:     user.ID,
		CreatedAt:     planningNow,
	}
	if err := useCase.persistPlan(context.Background(), plan, assignments); err != nil {
		t.Fatalf("persistPlan: %v", err)
	}
	return plan
}

func TestGetIncludesAnUnviewedMemberShare(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	user := planningManager(t, db)
	plan := seedAppliedPlan(t, useCase, user, planAssignments(1))

	stored, err := useCase.Get(context.Background(), user, plan.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if len(stored.ShareTokens) != 1 {
		t.Fatalf("len(ShareTokens) = %d, want 1", len(stored.ShareTokens))
	}

	token := stored.ShareTokens[0]
	if token.MemberID != "member-plot-1" {
		t.Errorf("MemberID = %q, want %q", token.MemberID, "member-plot-1")
	}
	if token.ID == "" {
		t.Error("token ID is empty, want the generated token")
	}
	if token.FirstViewedAt != nil {
		t.Errorf("FirstViewedAt = %v, want nil before anyone opens the link", *token.FirstViewedAt)
	}
}

func TestViewShareRejectsUnknownToken(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)

	_, err := useCase.ViewShare(context.Background(), "no-such-token", planningNow)
	if !errors.Is(err, ErrPlanShareNotFound) {
		t.Fatalf("err = %v, want ErrPlanShareNotFound", err)
	}
}

func TestViewShareSetsFirstViewedOnceAndLastViewedEveryTime(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	user := planningManager(t, db)
	plan := seedAppliedPlan(t, useCase, user, planAssignments(1))

	tokens, err := useCase.PlanShareTokenRepository.FindByPlanID(db, plan.ID)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("FindByPlanID: err=%v len=%d, want 1 token", err, len(tokens))
	}
	token := tokens[0].ID

	first := planningNow.Add(time.Hour)
	if _, err := useCase.ViewShare(context.Background(), token, first); err != nil {
		t.Fatalf("ViewShare (first): %v", err)
	}

	second := first.Add(time.Hour)
	if _, err := useCase.ViewShare(context.Background(), token, second); err != nil {
		t.Fatalf("ViewShare (second): %v", err)
	}

	loaded := new(entity.PlanShareToken)
	if err := useCase.PlanShareTokenRepository.FindById(db, loaded, token); err != nil {
		t.Fatalf("FindById: %v", err)
	}
	if loaded.FirstViewedAt == nil || !loaded.FirstViewedAt.Equal(first) {
		t.Errorf("FirstViewedAt = %v, want %v", loaded.FirstViewedAt, first)
	}
	if loaded.LastViewedAt == nil || !loaded.LastViewedAt.Equal(second) {
		t.Errorf("LastViewedAt = %v, want %v", loaded.LastViewedAt, second)
	}
}

func TestViewShareReflectsCancelledPlanStatusButKeepsItems(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	user := planningManager(t, db)
	plan := seedAppliedPlan(t, useCase, user, planAssignments(1))

	tokens, err := useCase.PlanShareTokenRepository.FindByPlanID(db, plan.ID)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("FindByPlanID: err=%v len=%d, want 1 token", err, len(tokens))
	}
	token := tokens[0].ID

	cancelAt := planningNow.Add(48 * time.Hour)
	if _, err := useCase.Cancel(context.Background(), user, plan.ID, cancelAt); err != nil {
		t.Fatalf("Cancel: %v", err)
	}

	share, err := useCase.ViewShare(context.Background(), token, cancelAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("ViewShare after cancel: %v", err)
	}
	if share.PlanStatus != string(constants.PlanCancelled) {
		t.Errorf("PlanStatus = %q, want %q", share.PlanStatus, constants.PlanCancelled)
	}
	if len(share.Items) != 1 {
		t.Errorf("len(Items) = %d, want 1 (the historical item stays visible after cancel)",
			len(share.Items))
	}
}

func TestViewShareOnlyIncludesTheTokensOwnMember(t *testing.T) {
	db := seedPlanningFixture(t)
	useCase := planningUseCase(t, db)
	user := planningManager(t, db)

	assignments := []planning.Assignment{
		{
			PlotID: "plot-1", MemberID: "member-plot-1", AreaHa: 2,
			CommodityID: riceCommodity, VarietyID: "variety-rice",
			PlantingDate: agronomy.AddDays(planningNow, 10),
			Window: agronomy.DateRange{
				Start: agronomy.AddDays(planningNow, 120), End: agronomy.AddDays(planningNow, 135),
			},
			Plausibility: constants.PlausibilityOk,
			TonnesLow:    5, TonnesMid: 6, TonnesHigh: 7,
		},
		{
			PlotID: "plot-2", MemberID: "member-plot-2", AreaHa: 5,
			CommodityID: riceCommodity, VarietyID: "variety-rice",
			PlantingDate: agronomy.AddDays(planningNow, 11),
			Window: agronomy.DateRange{
				Start: agronomy.AddDays(planningNow, 121), End: agronomy.AddDays(planningNow, 136),
			},
			Plausibility: constants.PlausibilityOk,
			TonnesLow:    8, TonnesMid: 9, TonnesHigh: 10,
		},
	}
	plan := seedAppliedPlan(t, useCase, user, assignments)

	if err := db.Create(&entity.FertiliserRate{
		CommodityID: riceCommodity, InputItem: "Urea", KgPerHa: 200, Source: "uji",
	}).Error; err != nil {
		t.Fatalf("seeding fertiliser rate: %v", err)
	}

	tokens, err := useCase.PlanShareTokenRepository.FindByPlanID(db, plan.ID)
	if err != nil || len(tokens) != 2 {
		t.Fatalf("FindByPlanID: err=%v len=%d, want 2 tokens", err, len(tokens))
	}

	var bigMemberToken string
	for _, token := range tokens {
		if token.MemberID == "member-plot-2" {
			bigMemberToken = token.ID
		}
	}
	if bigMemberToken == "" {
		t.Fatal("no token found for member-plot-2")
	}

	share, err := useCase.ViewShare(context.Background(), bigMemberToken, planningNow)
	if err != nil {
		t.Fatalf("ViewShare: %v", err)
	}
	if len(share.Items) != 1 {
		t.Fatalf("len(Items) = %d, want 1 (only this member's own assignment)", len(share.Items))
	}
	if share.Items[0].AreaHa != 5 {
		t.Errorf("AreaHa = %v, want 5 (member-plot-2's own plot, not member-plot-1's 2ha)",
			share.Items[0].AreaHa)
	}

	wantUreaKg := 200.0 * 5
	if len(share.Fertiliser) != 1 || share.Fertiliser[0].QuantityKg != wantUreaKg {
		t.Errorf("Fertiliser = %+v, want one line of %v kg Urea (5ha, not the other member's 2ha)",
			share.Fertiliser, wantUreaKg)
	}
}
