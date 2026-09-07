package diagnostics

import (
	"context"
	"fmt"
)

type BuildEvent struct {
	BuildID      string `json:"build_id"`
	ReleaseID    string `json:"release_id"`
	Service      string `json:"service"`
	Stage        string `json:"stage"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Exception    string `json:"exception"`
}

type Decision struct {
	Action       string `json:"action"`
	EventID      string `json:"event_id,omitempty"`
	ErrorGroupID string `json:"error_group_id,omitempty"`
}

type ErrorTracker interface {
	Capture(context.Context, CaptureInput, string) (CapturedEvent, error)
	Get(context.Context, string) (map[string]any, error)
}

type BuildMonitor struct {
	tracker ErrorTracker
}

func NewBuildMonitor(tracker ErrorTracker) *BuildMonitor {
	return &BuildMonitor{tracker: tracker}
}

func (m *BuildMonitor) Record(ctx context.Context, event BuildEvent) (Decision, error) {
	if event.Status != "failed" {
		return Decision{Action: "accepted"}, nil
	}

	captured, err := m.tracker.Capture(ctx, CaptureInput{
		Message:     event.ErrorMessage,
		Level:       "error",
		Fingerprint: []string{"build", event.Service, event.Stage},
		Exception:   event.Exception,
		Context: map[string]any{
			"build_id":   event.BuildID,
			"release_id": event.ReleaseID,
			"service":    event.Service,
			"stage":      event.Stage,
		},
	}, event.BuildID)
	if err != nil {
		return Decision{}, err
	}
	if captured.EventID == "" {
		return Decision{}, fmt.Errorf("capture response has no event_id")
	}

	recorded, err := m.tracker.Get(ctx, captured.EventID)
	if err != nil {
		return Decision{}, err
	}
	groupID, _ := recorded["error_group_id"].(string)
	if groupID == "" {
		groupID = captured.ErrorGroupID
	}
	return Decision{Action: "captured", EventID: captured.EventID, ErrorGroupID: groupID}, nil
}
