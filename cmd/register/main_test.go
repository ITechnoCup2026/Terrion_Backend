package main

import (
	"strings"
	"testing"

	"terrion-backend/internal/constants"
)

func pengurusAccount() account {
	return account{
		role:     constants.RolePengurus,
		email:    "sri@kudsubang.id",
		fullName: "Bu Sri",
	}
}

func newCooperative() cooperative {
	return cooperative{
		create:   true,
		name:     "KUD Subang",
		village:  "Jalancagak",
		district: "Subang",
		province: "Jawa Barat",
		lat:      -6.25,
		lng:      107.75,
	}
}

func TestCheckAcceptsAPengurusBringingANewCooperative(t *testing.T) {
	if err := check(pengurusAccount(), newCooperative()); err != nil {
		t.Errorf("check: %v", err)
	}
}

func TestCheckAcceptsAnAccountJoiningAnExistingCooperative(t *testing.T) {
	joining := cooperative{id: "11111111-1111-4111-8111-111111111111"}

	for _, role := range []constants.UserRole{constants.RolePengurus, constants.RoleKader} {
		requested := pengurusAccount()
		requested.role = role

		if err := check(requested, joining); err != nil {
			t.Errorf("role %q: %v", role, err)
		}
	}
}

func TestCheckRefusesAKaderBringingACooperative(t *testing.T) {
	requested := pengurusAccount()
	requested.role = constants.RoleKader

	err := check(requested, newCooperative())

	if err == nil {
		t.Fatal("check accepted a kader creating a cooperative, want a refusal")
	}
	if !strings.Contains(err.Error(), "pengurus") {
		t.Errorf("err = %q, want it to point at registering the pengurus first", err)
	}
}

func TestCheckRefusesAnAccountWithNoCooperativeAtAll(t *testing.T) {
	if err := check(pengurusAccount(), cooperative{}); err == nil {
		t.Error("check accepted a pengurus with no cooperative, want a refusal")
	}
}

func TestCheckRefusesBothCooperativeFlagsAtOnce(t *testing.T) {
	both := newCooperative()
	both.id = "11111111-1111-4111-8111-111111111111"

	if err := check(pengurusAccount(), both); err == nil {
		t.Error("check accepted -cooperative and -create-cooperative together, want a refusal")
	}
}

func TestCheckRefusesABuyerWithACooperative(t *testing.T) {
	buyer := account{
		role: constants.RoleBuyer, email: "diana@example.com", fullName: "Diana",
	}

	if err := check(buyer, cooperative{id: "11111111-1111-4111-8111-111111111111"}); err == nil {
		t.Error("check accepted a buyer with a cooperative, want a refusal")
	}
}

func TestCheckAcceptsABuyerWithNoCooperative(t *testing.T) {
	buyer := account{
		role: constants.RoleBuyer, email: "diana@example.com", fullName: "Diana",
	}

	if err := check(buyer, cooperative{}); err != nil {
		t.Errorf("check: %v", err)
	}
}

func TestCheckRejectsAnUnusableAccount(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*account)
	}{
		{"unknown role", func(a *account) { a.role = "penjual" }},
		{"empty role", func(a *account) { a.role = "" }},
		{"malformed email", func(a *account) { a.email = "sri-at-kudsubang" }},
		{"no email", func(a *account) { a.email = "" }},
		{"name too short", func(a *account) { a.fullName = "S" }},
	}

	for _, test := range tests {
		requested := pengurusAccount()
		test.mutate(&requested)

		if err := check(requested, newCooperative()); err == nil {
			t.Errorf("%s: check returned nil error, want a refusal", test.name)
		}
	}
}

func TestCheckRejectsAnIncompleteOrMisplacedCooperative(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*cooperative)
	}{
		{"no name", func(c *cooperative) { c.name = "" }},
		{"no village", func(c *cooperative) { c.village = "" }},
		{"no district", func(c *cooperative) { c.district = "" }},
		{"no province", func(c *cooperative) { c.province = "" }},
		{"latitude outside Indonesia", func(c *cooperative) { c.lat = 48.85 }},
		{"longitude outside Indonesia", func(c *cooperative) { c.lng = 2.35 }},
	}

	for _, test := range tests {
		home := newCooperative()
		test.mutate(&home)

		if err := check(pengurusAccount(), home); err == nil {
			t.Errorf("%s: check returned nil error, want a refusal", test.name)
		}
	}
}

func TestGeneratedPasswordIsUnpredictableAndReadable(t *testing.T) {
	first, err := generatePassword()
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}
	second, err := generatePassword()
	if err != nil {
		t.Fatalf("generatePassword: %v", err)
	}

	if first == second {
		t.Error("two generated passwords are identical")
	}
	if !strings.HasPrefix(first, constants.GeneratedPasswordPrefix) {
		t.Errorf("password %q does not carry the prefix", first)
	}
	if len(first) < 12 {
		t.Errorf("password %q is shorter than the signup form's own minimum", first)
	}
}
