package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"

	diagnostics "example.com/release-build-diagnostics"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	monitor := diagnostics.NewBuildMonitor(diagnostics.NewClient(key))

	mux := http.NewServeMux()
	mux.HandleFunc("POST /build-events", func(w http.ResponseWriter, r *http.Request) {
		var event diagnostics.BuildEvent
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&event); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid build event"})
			return
		}
		decision, err := monitor.Record(r.Context(), event)
		if err != nil {
			var apiErr *diagnostics.APIError
			if errors.As(err, &apiErr) && apiErr.Status >= 400 && apiErr.Status < 500 {
				writeJSON(w, apiErr.Status, map[string]string{"error": apiErr.Code})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "error tracking request failed"})
			return
		}
		writeJSON(w, http.StatusOK, decision)
	})

	log.Println("release diagnostics listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
