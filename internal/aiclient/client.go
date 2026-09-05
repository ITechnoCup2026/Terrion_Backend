package aiclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

const (
	DefaultTimeout = 3500 * time.Millisecond
	retryBackoff   = 250 * time.Millisecond
	retryJitter    = 100 * time.Millisecond
	cacheKeyPrefix = "terrion:ai:plan:v1:"
)

var (
	ErrDisabled     = errors.New("ai_service_disabled")
	ErrBreakerOpen  = errors.New("ai_breaker_open")
	ErrUnavailable  = errors.New("ai_service_unavailable")
	ErrContractDrif = errors.New("ai_contract_version_unsupported")
)

type Client struct {
	baseURL string
	token   string
	timeout time.Duration
	http    *http.Client
	breaker *Breaker
	log     *logrus.Logger
	random  *rand.Rand
}

func New(baseURL, token string, timeout time.Duration, log *logrus.Logger) *Client {
	if baseURL == "" {
		return nil
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		baseURL: baseURL,
		token:   token,
		timeout: timeout,
		http:    &http.Client{Timeout: timeout},
		breaker: NewBreaker(DefaultBreakerThreshold, DefaultBreakerCooldown),
		log:     log,
		random:  rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func CacheKey(request Request) string {
	anonymous := request
	anonymous.RequestID = ""

	payload, err := json.Marshal(anonymous)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(payload)
	return cacheKeyPrefix + hex.EncodeToString(sum[:])
}

func (c *Client) Enabled() bool { return c != nil }

func (c *Client) Propose(ctx context.Context, request Request) (Response, error) {
	if c == nil {
		return Response{}, ErrDisabled
	}
	if !c.breaker.Allow() {
		return Response{}, ErrBreakerOpen
	}

	call, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	response, retryable, err := c.attempt(call, request)
	if err == nil {
		c.breaker.Succeed()
		return response, nil
	}

	if retryable && call.Err() == nil {
		select {
		case <-call.Done():
		case <-time.After(c.backoff()):
			response, _, err = c.attempt(call, request)
			if err == nil {
				c.breaker.Succeed()
				return response, nil
			}
		}
	}

	c.breaker.Fail()
	return Response{}, err
}

func (c *Client) backoff() time.Duration {
	jitter := time.Duration(c.random.Int63n(int64(2*retryJitter))) - retryJitter
	return retryBackoff + jitter
}

func (c *Client) attempt(ctx context.Context, request Request) (Response, bool, error) {
	payload, err := json.Marshal(request)
	if err != nil {
		return Response{}, false, err
	}

	httpRequest, err := http.NewRequestWithContext(
		ctx, http.MethodPost, c.baseURL+"/v1/plan/propose", bytes.NewReader(payload))
	if err != nil {
		return Response{}, false, err
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	httpRequest.Header.Set("Authorization", "Bearer "+c.token)
	httpRequest.Header.Set("X-Request-Id", request.RequestID)

	httpResponse, err := c.http.Do(httpRequest)
	if err != nil {
		c.warn("ai_transport_failed", err.Error(), request.RequestID)
		return Response{}, true, ErrUnavailable
	}
	defer httpResponse.Body.Close()

	if httpResponse.StatusCode != http.StatusOK {
		return Response{}, retryableStatus(httpResponse.StatusCode), c.decodeFailure(httpResponse, request)
	}

	var decoded Response
	if err := json.NewDecoder(httpResponse.Body).Decode(&decoded); err != nil {
		c.warn("ai_response_unreadable", err.Error(), request.RequestID)
		return Response{}, false, ErrUnavailable
	}
	return decoded, false, nil
}

func retryableStatus(status int) bool {
	return status == http.StatusInternalServerError ||
		status == http.StatusServiceUnavailable ||
		status == http.StatusGatewayTimeout
}

func (c *Client) decodeFailure(response *http.Response, request Request) error {
	var envelope ErrorEnvelope
	_ = json.NewDecoder(response.Body).Decode(&envelope)
	code := envelope.Error.Code
	if code == "" {
		code = fmt.Sprintf("http_%d", response.StatusCode)
	}

	switch response.StatusCode {
	case http.StatusBadRequest, http.StatusUnauthorized:
		c.error("ai_request_rejected", code, request.RequestID)
	case http.StatusConflict:
		c.error("ai_contract_mismatch", code, request.RequestID)
		return ErrContractDrif
	default:
		c.warn("ai_call_failed", code, request.RequestID)
	}
	return ErrUnavailable
}

func (c *Client) warn(event, detail, requestID string) {
	if c.log != nil {
		c.log.WithFields(logrus.Fields{"detail": detail, "request_id": requestID}).Warn(event)
	}
}

func (c *Client) error(event, detail, requestID string) {
	if c.log != nil {
		c.log.WithFields(logrus.Fields{"detail": detail, "request_id": requestID}).Error(event)
	}
}

func ValidatePlans(response Response, mapping *Mapping, objectives []string) error {
	if len(response.Plans) != len(objectives) {
		return fmt.Errorf("plan_count_mismatch: %d objektif, %d rencana",
			len(objectives), len(response.Plans))
	}

	for i, plan := range response.Plans {
		if plan.Objective != objectives[i] {
			return fmt.Errorf("plan_objective_mismatch pada posisi %d: %q", i, plan.Objective)
		}
		seen := make(map[string]bool, len(plan.CandidateIDs))
		for _, id := range plan.CandidateIDs {
			if !mapping.Knows(id) {
				return fmt.Errorf("plan_assignment_rejected: %q tidak pernah diterbitkan", id)
			}
			if seen[id] {
				return fmt.Errorf("plan_assignment_rejected: %q muncul dua kali", id)
			}
			seen[id] = true
		}
	}
	return nil
}
