package diagnostics

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientCaptureBoundary(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		responses []string
		wantCode  string
		wantCalls int
	}{
		{name: "decodes business rejection before status", status: 400, responses: []string{`{"ok":false,"data":null,"error":{"code":"BAD_BUILD_EVENT"},"metadata":{}}`}, wantCode: "BAD_BUILD_EVENT", wantCalls: 1},
		{name: "retries rate limit then returns event", status: 429, responses: []string{`{"ok":false,"data":null,"error":{"code":"RATE_LIMITED"},"metadata":{}}`, `{"ok":true,"data":{"event_id":"evt_7"},"error":null,"metadata":{}}`}, wantCalls: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Method != http.MethodPost {
					t.Errorf("method = %s", r.Method)
				}
				if r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "build-7" {
					t.Errorf("request headers were not propagated")
				}
				index := calls - 1
				if index >= len(tt.responses) {
					index = len(tt.responses) - 1
				}
				status := tt.status
				if calls > 1 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				_, _ = w.Write([]byte(tt.responses[index]))
			}))
			defer server.Close()

			client := NewClient("test-key")
			client.baseURL = server.URL
			client.httpClient = server.Client()
			client.wait = func(context.Context, time.Duration) error { return nil }
			got, err := client.Capture(context.Background(), CaptureInput{Message: "compile failed"}, "build-7")
			if tt.wantCode != "" {
				var apiErr *APIError
				if !errors.As(err, &apiErr) || apiErr.Code != tt.wantCode {
					t.Fatalf("error = %#v, want API code %q", err, tt.wantCode)
				}
			} else if err != nil || got.EventID != "evt_7" {
				t.Fatalf("capture = %#v, error = %v", got, err)
			}
			if calls != tt.wantCalls {
				t.Fatalf("calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}
