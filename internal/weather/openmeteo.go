package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/constants"
)

type Client struct {
	HTTP        *http.Client
	ArchiveURL  string
	ForecastURL string
}

func NewClient() *Client {
	return &Client{
		HTTP:        &http.Client{Timeout: constants.OpenMeteoTimeout},
		ArchiveURL:  constants.OpenMeteoArchiveURL,
		ForecastURL: constants.OpenMeteoForecastURL,
	}
}

type dailyResponse struct {
	Daily struct {
		Time    []string   `json:"time"`
		TempMin []*float64 `json:"temperature_2m_min"`
		TempMax []*float64 `json:"temperature_2m_max"`
	} `json:"daily"`
}

func (c *Client) FetchHistory(
	ctx context.Context, lat, lng float64, start, end time.Time,
) ([]agronomy.TempDay, error) {
	query := baseQuery(lat, lng)
	query.Set("start_date", agronomy.ToISODate(start))
	query.Set("end_date", agronomy.ToISODate(end))

	return c.fetchDaily(ctx, c.ArchiveURL, query)
}

func (c *Client) FetchForecast(ctx context.Context, lat, lng float64) ([]agronomy.TempDay, error) {
	query := baseQuery(lat, lng)
	query.Set("forecast_days", strconv.Itoa(constants.OpenMeteoForecastDays))

	return c.fetchDaily(ctx, c.ForecastURL, query)
}

func baseQuery(lat, lng float64) url.Values {
	query := url.Values{}
	query.Set("latitude", strconv.FormatFloat(lat, 'f', -1, 64))
	query.Set("longitude", strconv.FormatFloat(lng, 'f', -1, 64))
	query.Set("daily", constants.OpenMeteoDailyFields)
	query.Set("timezone", "UTC")
	return query
}

func (c *Client) fetchDaily(
	ctx context.Context, endpoint string, query url.Values,
) ([]agronomy.TempDay, error) {
	address := endpoint + "?" + query.Encode()

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, fmt.Errorf("building open-meteo request: %w", err)
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return nil, fmt.Errorf("calling open-meteo: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo request failed with status %d: %s",
			response.StatusCode, endpoint)
	}

	var body dailyResponse
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		return nil, fmt.Errorf("decoding open-meteo response: %w", err)
	}

	return zipDaily(body), nil
}

func zipDaily(body dailyResponse) []agronomy.TempDay {
	daily := body.Daily
	days := make([]agronomy.TempDay, 0, len(daily.Time))

	for i, date := range daily.Time {
		if i >= len(daily.TempMin) || i >= len(daily.TempMax) {
			break
		}
		tempMin, tempMax := daily.TempMin[i], daily.TempMax[i]
		if tempMin == nil || tempMax == nil {
			continue
		}
		days = append(days, agronomy.TempDay{Date: date, TMin: *tempMin, TMax: *tempMax})
	}
	return days
}
