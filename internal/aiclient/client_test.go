package aiclient_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"terrion-backend/internal/aiclient"
)

func goldenResponseBody(t *testing.T) []byte {
	t.Helper()
	return readGolden(t, "propose_response.golden.json")
}

func serverReturning(t *testing.T, status int, body []byte, calls *atomic.Int32) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if got := r.Header.Get("Authorization"); got != "Bearer rahasia" {
			t.Errorf("token tidak terkirim, dapat %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return server
}

func TestADisabledClientNeverTouchesTheNetwork(t *testing.T) {
	var client *aiclient.Client

	_, err := client.Propose(context.Background(), aiclient.Request{})

	if !errors.Is(err, aiclient.ErrDisabled) {
		t.Fatalf("AI_SERVICE_URL kosong harus berarti nonaktif, dapat %v", err)
	}
	if client.Enabled() {
		t.Fatal("klien nil melaporkan dirinya aktif")
	}
}

func TestASuccessfulCallReturnsThePlans(t *testing.T) {
	var calls atomic.Int32
	server := serverReturning(t, http.StatusOK, goldenResponseBody(t), &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	response, err := client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if err != nil {
		t.Fatalf("panggilan gagal: %v", err)
	}
	if len(response.Plans) != 3 || calls.Load() != 1 {
		t.Fatalf("dapat %d rencana dalam %d panggilan", len(response.Plans), calls.Load())
	}
}

func TestAVersionMismatchIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	body, _ := json.Marshal(map[string]any{
		"error": map[string]string{"code": "contract_version_unsupported", "message": ""},
	})
	server := serverReturning(t, http.StatusConflict, body, &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	_, err := client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if !errors.Is(err, aiclient.ErrContractDrif) {
		t.Fatalf("409 harus dikenali sebagai deploy tidak seiring, dapat %v", err)
	}
	if calls.Load() != 1 {
		t.Fatalf("409 tidak boleh diulang, dapat %d panggilan", calls.Load())
	}
}

func TestAnUnauthenticatedCallIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	server := serverReturning(t, http.StatusUnauthorized, []byte(`{}`), &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	_, _ = client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if calls.Load() != 1 {
		t.Fatalf("401 tidak boleh diulang, dapat %d panggilan", calls.Load())
	}
}

func TestAServerErrorIsRetriedExactlyOnce(t *testing.T) {
	var calls atomic.Int32
	server := serverReturning(t, http.StatusInternalServerError, []byte(`{}`), &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	_, err := client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if err == nil {
		t.Fatal("500 dua kali harus berakhir sebagai galat")
	}
	if calls.Load() != 2 {
		t.Fatalf("500 harus diulang tepat sekali, dapat %d panggilan", calls.Load())
	}
}

func TestTheBreakerStopsCallingADeadService(t *testing.T) {
	var calls atomic.Int32
	server := serverReturning(t, http.StatusBadRequest, []byte(`{}`), &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	for range 3 {
		_, _ = client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})
	}
	before := calls.Load()

	_, err := client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if !errors.Is(err, aiclient.ErrBreakerOpen) {
		t.Fatalf("breaker harus terbuka setelah tiga kegagalan, dapat %v", err)
	}
	if calls.Load() != before {
		t.Fatal("breaker terbuka masih menyentuh jaringan")
	}
}

func TestAnUnreadableBodyIsTreatedAsUnavailable(t *testing.T) {
	var calls atomic.Int32
	server := serverReturning(t, http.StatusOK, []byte(`bukan json`), &calls)
	client := aiclient.New(server.URL, "rahasia", time.Second, nil)

	_, err := client.Propose(context.Background(), aiclient.Request{RequestID: "r1"})

	if !errors.Is(err, aiclient.ErrUnavailable) {
		t.Fatalf("respons yang tidak terbaca harus jatuh ke fallback, dapat %v", err)
	}
}

func TestCacheKeyIgnoresTheRequestId(t *testing.T) {
	first := aiclient.Request{RequestID: "a", Seed: 1, Objectives: []string{"aman"}}
	second := aiclient.Request{RequestID: "b", Seed: 1, Objectives: []string{"aman"}}

	if aiclient.CacheKey(first) != aiclient.CacheKey(second) {
		t.Fatal("request_id ikut di-hash; setiap permintaan jadi unik dan cache tidak pernah kena")
	}
}

func TestCacheKeyChangesWhenTheProblemChanges(t *testing.T) {
	first := aiclient.Request{RequestID: "a", Seed: 1}
	second := aiclient.Request{RequestID: "a", Seed: 2}

	if aiclient.CacheKey(first) == aiclient.CacheKey(second) {
		t.Fatal("seed yang berbeda harus menghasilkan kunci cache yang berbeda")
	}
}

func TestValidatePlansRejectsACandidateGoNeverIssued(t *testing.T) {
	_, mapping := aiclient.BuildRequest(poisonedInput(t))
	response := aiclient.Response{Plans: []aiclient.PlanResult{
		{Objective: "aman", CandidateIDs: []string{"c001", "c999"}},
	}}

	if err := aiclient.ValidatePlans(response, mapping, []string{"aman"}); err == nil {
		t.Fatal("kandidat yang tidak pernah diterbitkan Go harus ditolak")
	}
}

func TestValidatePlansRejectsADuplicateAssignment(t *testing.T) {
	_, mapping := aiclient.BuildRequest(poisonedInput(t))
	response := aiclient.Response{Plans: []aiclient.PlanResult{
		{Objective: "aman", CandidateIDs: []string{"c001", "c001"}},
	}}

	if err := aiclient.ValidatePlans(response, mapping, []string{"aman"}); err == nil {
		t.Fatal("kandidat yang muncul dua kali harus ditolak")
	}
}

func TestValidatePlansRejectsAReorderedObjectiveList(t *testing.T) {
	_, mapping := aiclient.BuildRequest(poisonedInput(t))
	response := aiclient.Response{Plans: []aiclient.PlanResult{
		{Objective: "pendapatan", CandidateIDs: []string{"c001"}},
	}}

	if err := aiclient.ValidatePlans(response, mapping, []string{"aman"}); err == nil {
		t.Fatal("urutan objektif yang berubah harus ditolak")
	}
}

func TestValidatePlansAcceptsAWellFormedResponse(t *testing.T) {
	_, mapping := aiclient.BuildRequest(poisonedInput(t))
	response := aiclient.Response{Plans: []aiclient.PlanResult{
		{Objective: "aman", CandidateIDs: []string{"c001", "c002"}},
	}}

	if err := aiclient.ValidatePlans(response, mapping, []string{"aman"}); err != nil {
		t.Fatalf("respons yang benar ditolak: %v", err)
	}
}
