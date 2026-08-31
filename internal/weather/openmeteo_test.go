package weather_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"terrion-backend/internal/agronomy"
	"terrion-backend/internal/weather"
)

type recordedRequest struct {
	url string
}

func fakeOpenMeteo(t *testing.T, status int, body string) (*weather.Client, *recordedRequest) {
	t.Helper()

	recorded := &recordedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded.url = r.URL.String()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("writing fake response: %v", err)
		}
	}))
	t.Cleanup(server.Close)

	client := weather.NewClient()
	client.ArchiveURL = server.URL + "/v1/archive"
	client.ForecastURL = server.URL + "/v1/forecast"
	return client, recorded
}

func januaryRange(t *testing.T, from, to string) (time.Time, time.Time) {
	t.Helper()
	start, err := agronomy.UTCDate(from)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", from, err)
	}
	end, err := agronomy.UTCDate(to)
	if err != nil {
		t.Fatalf("UTCDate(%q): %v", to, err)
	}
	return start, end
}

func TestFetchHistoryZipsParallelArraysIntoOneEntryPerDay(t *testing.T) {
	client, _ := fakeOpenMeteo(t, http.StatusOK, `{"daily":{
		"time":["2024-01-01","2024-01-02"],
		"temperature_2m_min":[21,22],
		"temperature_2m_max":[30,31]}}`)

	start, end := januaryRange(t, "2024-01-01", "2024-01-02")
	days, err := client.FetchHistory(context.Background(), -7.25, 107.75, start, end)
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}

	want := []agronomy.TempDay{
		{Date: "2024-01-01", TMin: 21, TMax: 30},
		{Date: "2024-01-02", TMin: 22, TMax: 31},
	}
	if len(days) != len(want) {
		t.Fatalf("len(days) = %d, want %d", len(days), len(want))
	}
	for i, got := range days {
		if got != want[i] {
			t.Errorf("days[%d] = %+v, want %+v", i, got, want[i])
		}
	}
}

func TestFetchHistoryDropsADayWithAMissingReading(t *testing.T) {
	client, _ := fakeOpenMeteo(t, http.StatusOK, `{"daily":{
		"time":["2024-01-01","2024-01-02","2024-01-03"],
		"temperature_2m_min":[21,null,23],
		"temperature_2m_max":[30,31,null]}}`)

	start, end := januaryRange(t, "2024-01-01", "2024-01-03")
	days, err := client.FetchHistory(context.Background(), -7.25, 107.75, start, end)
	if err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}

	if len(days) != 1 || days[0] != (agronomy.TempDay{Date: "2024-01-01", TMin: 21, TMax: 30}) {
		t.Errorf("days = %+v, want only the complete day", days)
	}
}

func TestFetchHistoryAsksTheArchiveForTheRequestedRangeInUTC(t *testing.T) {
	client, recorded := fakeOpenMeteo(t, http.StatusOK, `{"daily":{"time":[]}}`)

	start, end := januaryRange(t, "2015-01-01", "2024-12-31")
	if _, err := client.FetchHistory(context.Background(), -7.25, 107.75, start, end); err != nil {
		t.Fatalf("FetchHistory: %v", err)
	}

	for _, want := range []string{
		"/v1/archive", "start_date=2015-01-01", "end_date=2024-12-31", "timezone=UTC",
	} {
		if !strings.Contains(recorded.url, want) {
			t.Errorf("request %q does not contain %q", recorded.url, want)
		}
	}
}

func TestFetchForecastAsksForSixteenDays(t *testing.T) {
	client, recorded := fakeOpenMeteo(t, http.StatusOK, `{"daily":{"time":[]}}`)

	if _, err := client.FetchForecast(context.Background(), -7.25, 107.75); err != nil {
		t.Fatalf("FetchForecast: %v", err)
	}

	for _, want := range []string{"/v1/forecast", "forecast_days=16"} {
		if !strings.Contains(recorded.url, want) {
			t.Errorf("request %q does not contain %q", recorded.url, want)
		}
	}
}

func TestFetchForecastFailsOnAnErrorStatus(t *testing.T) {
	client, _ := fakeOpenMeteo(t, http.StatusTooManyRequests, `{}`)

	_, err := client.FetchForecast(context.Background(), -7.25, 107.75)
	if err == nil {
		t.Fatal("FetchForecast returned nil error, want a failure")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("error = %q, want it to name status 429", err)
	}
}
