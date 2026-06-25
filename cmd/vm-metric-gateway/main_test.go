package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crms-agent/internal/gnocchi"
	"crms-agent/internal/sample"
)

func TestGnocchiSinkSendsSample(t *testing.T) {
	var sawResource bool
	var sawMeasures bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/resource/generic":
			sawResource = true
			w.WriteHeader(http.StatusCreated)
		case "/v1/batch/resources/metrics/measures":
			if r.URL.Query().Get("create_metrics") != "true" {
				t.Fatalf("create_metrics = %q", r.URL.Query().Get("create_metrics"))
			}
			sawMeasures = true
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	sink := gnocchiSink{
		client:            gnocchi.NewClient(server.URL, "admin", "", server.Client()),
		archivePolicyName: "vm_guest_usage_default",
	}
	err := sink.SendSample(t.Context(), sample.Sample{
		Timestamp:  time.Date(2026, 6, 25, 1, 2, 3, 0, time.UTC),
		InstanceID: "vm-1",
		ProjectID:  "project-1",
		Metrics: map[string]float64{
			"guest.cpu.used_percent": 12.5,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !sawResource {
		t.Fatal("expected resource request")
	}
	if !sawMeasures {
		t.Fatal("expected measures request")
	}
}
