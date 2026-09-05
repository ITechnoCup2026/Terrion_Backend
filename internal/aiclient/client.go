package aiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"terrion-backend/internal/constants"
)

type Client struct {
	HTTP    *http.Client
	BaseURL string
	Token   string
	Timeout time.Duration
	breaker *breaker
}

func NewClient(baseURL, token string, timeout time.Duration) *Client {
	trimmed := strings.TrimSpace(baseURL)
	if trimmed == "" {
		return nil
	}

	return &Client{
		// Timeout di klien adalah jaring pengaman per permintaan; yang
		// menegakkan anggaran sebenarnya adalah deadline milik Propose,
		// yang selalu sama ketat atau lebih ketat dari ini.
		HTTP:    &http.Client{Timeout: timeout},
		BaseURL: strings.TrimRight(trimmed, "/"),
		Token:   token,
		Timeout: timeout,
		breaker: newBreaker(),
	}
}

type ServiceError struct {
	Status    int
	Code      string
	Message   string
	Retryable bool
}

func (e *ServiceError) Error() string {
	return fmt.Sprintf("aiclient: %d %s: %s", e.Status, e.Code, e.Message)
}

// Propose memanggil layanan AI, dengan satu kali percobaan ulang.
//
// Deadline-nya dibuat SEKALI di sini dan dibagi oleh kedua percobaan beserta
// jeda di antaranya, karena c.Timeout adalah anggaran untuk seluruh panggilan
// (ARCHITECTURE.md §6.1: "termasuk 1 retry"), bukan jatah per percobaan.
//
// Dulu setiap percobaan membuat deadline-nya sendiri dan tidak ada deadline di
// atas keduanya — controller mengoper ctx.UserContext() yang polos. Terukur:
// layanan AI yang menggantung menahan pengguna 7,35 detik, dua kali lipat dari
// 3,5 detik yang dijanjikan, dan percobaan keduanya mengulang seluruh
// komputasi Python (CP-SAT + Monte Carlo + tiga panggilan LLM) justru ketika
// layanan itu sedang lambat. Dengan satu deadline bersama, percobaan kedua
// hanya berangkat kalau memang masih ada waktu untuknya.
func (c *Client) Propose(ctx context.Context, request Request) (*Response, error) {
	if !c.breaker.allow() {
		return nil, ErrBreakerOpen
	}

	call, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	response, err := c.post(call, request)
	if err == nil {
		c.breaker.succeed()
		return response, nil
	}

	var service *ServiceError
	if !asServiceError(err, &service) || !service.Retryable {
		c.breaker.fail()
		return nil, err
	}

	select {
	case <-time.After(backoff()):
	case <-call.Done():
		// Anggaran habis sebelum jeda selesai. Yang dikembalikan adalah galat
		// percobaan pertama, bukan call.Err(): ia menyebutkan apa yang
		// sebenarnya terjadi, dan tetap bertipe *ServiceError bagi pemanggil.
		c.breaker.fail()
		return nil, err
	}

	response, err = c.post(call, request)
	if err != nil {
		c.breaker.fail()
		return nil, err
	}

	c.breaker.succeed()
	return response, nil
}

func (c *Client) Health(ctx context.Context) error {
	call, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	request, err := http.NewRequestWithContext(
		call, http.MethodGet, c.BaseURL+constants.AIHealthPath, nil)
	if err != nil {
		return err
	}

	response, err := c.HTTP.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("aiclient: health %d", response.StatusCode)
	}
	return nil
}

func (c *Client) post(ctx context.Context, request Request) (*Response, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return nil, &ServiceError{Code: "encode_failed", Message: err.Error()}
	}

	// Tanpa WithTimeout sendiri: deadline-nya milik Propose, dan dibagi
	// dengan percobaan yang lain.
	httpRequest, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.BaseURL+constants.AIProposePath, bytes.NewReader(body))
	if err != nil {
		return nil, &ServiceError{Code: "build_failed", Message: err.Error()}
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+c.Token)
	httpRequest.Header.Set("X-Request-Id", request.RequestID)

	httpResponse, err := c.HTTP.Do(httpRequest)
	if err != nil {
		return nil, &ServiceError{Code: "transport", Message: err.Error(), Retryable: true}
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return nil, decodeServiceError(httpResponse)
	}

	var decoded Response
	if err := json.NewDecoder(httpResponse.Body).Decode(&decoded); err != nil {
		return nil, &ServiceError{
			Status: httpResponse.StatusCode, Code: "malformed_response", Message: err.Error()}
	}

	if !strings.HasPrefix(decoded.ContractVersion, constants.AIContractMajor+".") {
		return nil, &ServiceError{
			Status: httpResponse.StatusCode,
			Code:   "contract_version_unsupported",
			Message: fmt.Sprintf("layanan menjawab %q, backend berbicara %q",
				decoded.ContractVersion, constants.AIContractVersion),
		}
	}

	return &decoded, nil
}

func decodeServiceError(response *http.Response) *ServiceError {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	_ = json.NewDecoder(response.Body).Decode(&envelope)

	code := envelope.Error.Code
	if code == "" {
		code = "unknown"
	}

	return &ServiceError{
		Status:    response.StatusCode,
		Code:      code,
		Message:   envelope.Error.Message,
		Retryable: response.StatusCode >= http.StatusInternalServerError,
	}
}

func asServiceError(err error, target **ServiceError) bool {
	service, ok := err.(*ServiceError)
	if !ok {
		return false
	}
	*target = service
	return true
}

func backoff() time.Duration {
	return constants.AIRetryBackoff +
		time.Duration(rand.Int63n(int64(2*constants.AIRetryJitter))) - constants.AIRetryJitter
}
