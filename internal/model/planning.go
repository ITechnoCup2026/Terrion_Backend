package model

type SeasonPlanAssignmentRequest struct {
	PlotID       string `json:"plot_id" validate:"required"`
	VarietyID    string `json:"variety_id" validate:"required"`
	PlantingDate string `json:"planting_date" validate:"required,datetime=2006-01-02"`
}

type ApplySeasonPlanRequest struct {
	SeasonLabel string                        `json:"season_label" validate:"required,min=3"`
	Objective   string                        `json:"objective" validate:"required,oneof=aman pendapatan pasar"`
	Assignments []SeasonPlanAssignmentRequest `json:"assignments" validate:"required,min=1,dive"`
}
