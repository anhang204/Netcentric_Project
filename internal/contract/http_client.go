package contract

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// HTTPClient is shared by screens that call the Gin API.
type HTTPClient struct {
	baseURL string
	client  *http.Client
}

func NewHTTPClient(baseURL string) *HTTPClient {
	return &HTTPClient{baseURL: strings.TrimRight(baseURL, "/"), client: &http.Client{Timeout: 5 * time.Second}}
}

func (api *HTTPClient) Do(ctx context.Context, method, path, token string, input, output any) error {
	var body io.Reader
	if input != nil {
		encoded, err := json.Marshal(input)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, api.baseURL+path, body)
	if err != nil {
		return err
	}
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := api.client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot reach the backend: %w", err)
	}
	defer res.Body.Close()
	decoder := json.NewDecoder(io.LimitReader(res.Body, 1024*1024))
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var failure ErrorResponse
		if decoder.Decode(&failure) == nil && failure.Error.Message != "" {
			return fmt.Errorf("%s (HTTP %d)", failure.Error.Message, res.StatusCode)
		}
		return fmt.Errorf("request failed (HTTP %d)", res.StatusCode)
	}
	if output != nil {
		return decoder.Decode(output)
	}
	io.Copy(io.Discard, res.Body)
	return nil
}
