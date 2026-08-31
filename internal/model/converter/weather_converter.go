package converter

import (
	"terrion-backend/internal/model"
	"terrion-backend/internal/usecase"
)

func WeatherRefreshToResponse(result usecase.RefreshResult) *model.WeatherRefreshResponse {
	failed := result.Failed
	if failed == nil {
		failed = []string{}
	}

	return &model.WeatherRefreshResponse{
		Cells:       result.Cells,
		RowsWritten: result.RowsWritten,
		Backfilled:  result.Backfilled,
		Failed:      failed,
	}
}
