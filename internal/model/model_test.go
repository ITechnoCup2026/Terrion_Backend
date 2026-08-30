package model

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebResponse_OmitsEmptyErrors(t *testing.T) {
	resp := WebResponse[string]{Data: "ok"}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	if strings.Contains(string(b), "errors") {
		t.Errorf("json = %s, expected no \"errors\" key when Errors is empty", b)
	}
	if !strings.Contains(string(b), `"data":"ok"`) {
		t.Errorf("json = %s, expected data field", b)
	}
}

func TestWebResponse_IncludesErrorsWhenSet(t *testing.T) {
	resp := WebResponse[string]{Errors: "bad request"}

	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}

	if !strings.Contains(string(b), `"errors":"bad request"`) {
		t.Errorf("json = %s, expected errors field", b)
	}
}
