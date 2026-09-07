package diagnostics

import (
	"context"
	"reflect"
	"testing"
)

type trackerStub struct {
	captures []CaptureInput
	keys     []string
	gets     []string
}

func (s *trackerStub) Capture(_ context.Context, in CaptureInput, key string) (CapturedEvent, error) {
	s.captures = append(s.captures, in)
	s.keys = append(s.keys, key)
	return CapturedEvent{EventID: "evt_42"}, nil
}

func (s *trackerStub) Get(_ context.Context, eventID string) (map[string]any, error) {
	s.gets = append(s.gets, eventID)
	return map[string]any{"error_group_id": "grp_compile"}, nil
}

func TestBuildMonitorDecision(t *testing.T) {
	tests := []struct {
		name         string
		status       string
		want         Decision
		wantCaptures int
	}{
		{name: "successful build passes without capture", status: "passed", want: Decision{Action: "accepted"}},
		{name: "failed build is captured and grouped", status: "failed", want: Decision{Action: "captured", EventID: "evt_42", ErrorGroupID: "grp_compile"}, wantCaptures: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stub := &trackerStub{}
			monitor := NewBuildMonitor(stub)
			got, err := monitor.Record(context.Background(), BuildEvent{
				BuildID: "build-1842", ReleaseID: "release-91", Service: "compiler", Stage: "compile",
				Status: tt.status, ErrorMessage: "module graph failed", Exception: "compile: missing module",
			})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("decision = %#v, want %#v", got, tt.want)
			}
			if len(stub.captures) != tt.wantCaptures {
				t.Fatalf("captures = %d, want %d", len(stub.captures), tt.wantCaptures)
			}
			if tt.status == "failed" {
				if stub.keys[0] != "build-1842" {
					t.Fatalf("idempotency key = %q", stub.keys[0])
				}
				wantFingerprint := []string{"build", "compiler", "compile"}
				if !reflect.DeepEqual(stub.captures[0].Fingerprint, wantFingerprint) {
					t.Fatalf("fingerprint = %#v", stub.captures[0].Fingerprint)
				}
				if !reflect.DeepEqual(stub.gets, []string{"evt_42"}) {
					t.Fatalf("get calls = %#v", stub.gets)
				}
			}
		})
	}
}
