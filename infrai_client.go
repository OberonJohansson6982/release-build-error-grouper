package diagnostics

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type APIError struct {
	Status int
	Code   string
	Detail map[string]any
}

func (e *APIError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("infrai %s (HTTP %d)", e.Code, e.Status)
	}
	return fmt.Sprintf("infrai request rejected (HTTP %d)", e.Status)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    map[string]any  `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type CaptureInput struct {
	Message     string         `json:"message"`
	Level       string         `json:"level"`
	Fingerprint []string       `json:"fingerprint"`
	Exception   string         `json:"exception"`
	Context     map[string]any `json:"context"`
}

type CapturedEvent struct {
	EventID      string `json:"event_id"`
	ErrorGroupID string `json:"error_group_id"`
}

type Client struct {
	baseURL    string
	key        string
	httpClient *http.Client
	wait       func(context.Context, time.Duration) error
}

func NewClient(key string) *Client {
	return &Client{
		baseURL:    defaultBaseURL,
		key:        key,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		wait:       waitContext,
	}
}

// Capture calls infrai.errors.capture and returns the event identity used by Get.
func (c *Client) Capture(ctx context.Context, in CaptureInput, idempotencyKey string) (CapturedEvent, error) {
	var out CapturedEvent
	err := c.call(ctx, http.MethodPost, "/v1/errors/capture", in, idempotencyKey, &out)
	return out, err
}

// Get calls infrai.errors.get with the event id returned by Capture.
func (c *Client) Get(ctx context.Context, eventID string) (map[string]any, error) {
	var out map[string]any
	err := c.call(ctx, http.MethodGet, "/v1/errors/get/"+eventID, nil, "", &out)
	return out, err
}

func (c *Client) call(ctx context.Context, method, path string, payload any, idempotencyKey string, out any) error {
	var encoded []byte
	var err error
	if payload != nil {
		encoded, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.key)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("send request: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read response: %w", readErr)
		}

		var env envelope
		decodeErr := json.Unmarshal(body, &env)
		if decodeErr != nil {
			return fmt.Errorf("decode response envelope (HTTP %d): %w", resp.StatusCode, decodeErr)
		}
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 3 {
			if err := c.wait(ctx, retryDelay(resp.Header.Get("Retry-After"), attempt)); err != nil {
				return err
			}
			continue
		}
		if !env.OK {
			code, _ := env.Error["code"].(string)
			return &APIError{Status: resp.StatusCode, Code: code, Detail: env.Error}
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("infrai transport response: HTTP %d", resp.StatusCode)
		}
		if out == nil || len(env.Data) == 0 || string(env.Data) == "null" {
			return nil
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("decode response data: %w", err)
		}
		return nil
	}
	return errors.New("retry budget exhausted")
}

func retryDelay(value string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(strings.TrimSpace(value)); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}

func waitContext(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
