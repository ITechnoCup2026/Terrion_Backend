package config

import "testing"

type sampleValidatorRequest struct {
	Name string `validate:"required"`
}

func TestNewValidator_ValidatesRequiredField(t *testing.T) {
	validate := NewValidator()

	if err := validate.Struct(&sampleValidatorRequest{Name: ""}); err == nil {
		t.Fatal("expected validation error for empty required field, got nil")
	}

	if err := validate.Struct(&sampleValidatorRequest{Name: "ok"}); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}
