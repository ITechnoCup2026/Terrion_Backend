package model

import "terrion-backend/internal/constants"

type SignupRequest struct {
	FullName        string `json:"full_name" validate:"required,min=2"`
	Organisation    string `json:"organisation" validate:"required,min=2"`
	Email           string `json:"email" validate:"required,email"`
	Password        string `json:"password" validate:"required,min=8"`
	ConfirmPassword string `json:"confirm_password" validate:"required,eqfield=Password"`
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type SignupResponse struct {
	Outcome string `json:"outcome"`
	Email   string `json:"email,omitempty"`
}

type UserResponse struct {
	ID            string             `json:"id"`
	Role          constants.UserRole `json:"role"`
	CooperativeID *string            `json:"cooperative_id"`
	FullName      string             `json:"full_name"`
	Organisation  *string            `json:"organisation"`
}
