package aiclient

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

func Fingerprint(request Request) string {
	request.RequestID = ""

	body, err := json.Marshal(request)
	if err != nil {
		return ""
	}

	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}
