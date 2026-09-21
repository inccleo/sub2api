package typesafe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const (
	DefaultBaseURL  = "https://api.typesafe.ai"
	SystemOnePath   = "/v1/systemone"
	JevLatestModel  = "jev-latest"
	JevPreviewModel = "jev-preview"
	Jev1130Model    = "jev-1.13.0"
)

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type SystemOneResponse struct {
	Body  []byte
	Model string
	Usage Usage
}

func SupportedModels() []string {
	return []string{JevLatestModel, JevPreviewModel, Jev1130Model}
}

func IsSupportedModel(model string) bool {
	switch model {
	case JevLatestModel, JevPreviewModel, Jev1130Model:
		return true
	default:
		return false
	}
}

func RequestedModel(body []byte) string {
	var envelope struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(body, &envelope) != nil {
		return ""
	}
	return strings.TrimSpace(envelope.Model)
}

func NewSystemOneRequest(ctx context.Context, baseURL, key string, body []byte) (*http.Request, error) {
	endpoint, err := url.JoinPath(strings.TrimRight(baseURL, "/"), SystemOnePath)
	if err != nil {
		return nil, errors.New("typesafe invalid endpoint")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, errors.New("typesafe invalid request")
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func DecodeSystemOneResponse(r io.Reader) (*SystemOneResponse, error) {
	body, err := io.ReadAll(io.LimitReader(r, 4<<20))
	if err != nil || !json.Valid(body) {
		return nil, errors.New("typesafe invalid response")
	}
	var envelope struct {
		Model string `json:"model"`
		Usage Usage  `json:"usage"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, errors.New("typesafe invalid response")
	}
	return &SystemOneResponse{Body: body, Model: envelope.Model, Usage: envelope.Usage}, nil
}
