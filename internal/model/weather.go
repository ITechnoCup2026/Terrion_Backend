package model

type WeatherRefreshResponse struct {
	Cells       int      `json:"cells"`
	RowsWritten int      `json:"rows_written"`
	Backfilled  int      `json:"backfilled"`
	Failed      []string `json:"failed"`
}
