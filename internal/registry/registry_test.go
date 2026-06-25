package registry

import (
	"testing"
	"time"

	"crms-agent/internal/sample"
)

func TestRegistryRecordsSuccessAndListsMostRecentFirst(t *testing.T) {
	r := New()
	oldTime := time.Date(2026, 6, 25, 1, 0, 0, 0, time.UTC)
	newTime := oldTime.Add(time.Minute)

	r.RecordSuccess(sample.Sample{
		Timestamp:  oldTime,
		InstanceID: "old",
		Hostname:   "old-host",
		Metrics:    map[string]float64{"a": 1},
	})
	r.RecordSuccess(sample.Sample{
		Timestamp:        newTime,
		InstanceID:       "new",
		ProjectID:        "project",
		Hostname:         "new-host",
		Name:             "new-name",
		AvailabilityZone: "nova",
		Metrics:          map[string]float64{"a": 1, "b": 2},
	})

	got := r.List()
	if len(got) != 2 {
		t.Fatalf("List length = %d", len(got))
	}
	if got[0].InstanceID != "new" {
		t.Fatalf("first InstanceID = %q", got[0].InstanceID)
	}
	if got[0].ProjectID != "project" {
		t.Fatalf("ProjectID = %q", got[0].ProjectID)
	}
	if got[0].Hostname != "new-host" {
		t.Fatalf("Hostname = %q", got[0].Hostname)
	}
	if got[0].Name != "new-name" {
		t.Fatalf("Name = %q", got[0].Name)
	}
	if got[0].AvailabilityZone != "nova" {
		t.Fatalf("AvailabilityZone = %q", got[0].AvailabilityZone)
	}
	if got[0].LastMetricCount != 2 {
		t.Fatalf("LastMetricCount = %d", got[0].LastMetricCount)
	}
	if !got[0].LastSeen.Equal(newTime) {
		t.Fatalf("LastSeen = %v", got[0].LastSeen)
	}
	if !got[0].LastSuccessAt.Equal(newTime) {
		t.Fatalf("LastSuccessAt = %v", got[0].LastSuccessAt)
	}
	if got[0].LastError != "" {
		t.Fatalf("LastError = %q", got[0].LastError)
	}
}

func TestRegistryRecordsErrorWithoutSuccess(t *testing.T) {
	r := New()
	seen := time.Date(2026, 6, 25, 1, 0, 0, 0, time.UTC)

	r.RecordError(sample.Sample{
		Timestamp:  seen,
		InstanceID: "vm-1",
		Metrics:    map[string]float64{"a": 1},
	}, "gnocchi down")

	status, ok := r.Get("vm-1")
	if !ok {
		t.Fatal("expected vm-1")
	}
	if status.LastError != "gnocchi down" {
		t.Fatalf("LastError = %q", status.LastError)
	}
	if !status.LastSeen.Equal(seen) {
		t.Fatalf("LastSeen = %v", status.LastSeen)
	}
	if !status.LastSuccessAt.IsZero() {
		t.Fatalf("LastSuccessAt = %v", status.LastSuccessAt)
	}
}

func TestRegistryErrorPreservesPreviousSuccess(t *testing.T) {
	r := New()
	successAt := time.Date(2026, 6, 25, 1, 0, 0, 0, time.UTC)
	errorAt := successAt.Add(time.Minute)

	r.RecordSuccess(sample.Sample{
		Timestamp:  successAt,
		InstanceID: "vm-1",
		Metrics:    map[string]float64{"a": 1},
	})
	r.RecordError(sample.Sample{
		Timestamp:  errorAt,
		InstanceID: "vm-1",
		Metrics:    map[string]float64{"a": 1},
	}, "storage unavailable")

	status, ok := r.Get("vm-1")
	if !ok {
		t.Fatal("expected vm-1")
	}
	if !status.LastSuccessAt.Equal(successAt) {
		t.Fatalf("LastSuccessAt = %v", status.LastSuccessAt)
	}
	if !status.LastSeen.Equal(errorAt) {
		t.Fatalf("LastSeen = %v", status.LastSeen)
	}
	if status.LastError != "storage unavailable" {
		t.Fatalf("LastError = %q", status.LastError)
	}
}

func TestRegistryMissingInstanceIDIsIgnored(t *testing.T) {
	r := New()

	r.RecordSuccess(sample.Sample{Metrics: map[string]float64{"a": 1}})
	r.RecordError(sample.Sample{Metrics: map[string]float64{"a": 1}}, "missing instance")

	if got := r.List(); len(got) != 0 {
		t.Fatalf("List length = %d", len(got))
	}
}
