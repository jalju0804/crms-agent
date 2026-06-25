package gnocchi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"crms-agent/internal/sample"
)

func TestDefaultArchivePolicyMatchesGuestUsagePolicy(t *testing.T) {
	body, err := json.Marshal(DefaultArchivePolicy("vm_guest_usage_default"))
	if err != nil {
		t.Fatal(err)
	}

	var got ArchivePolicy
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatal(err)
	}

	if got.Name != "vm_guest_usage_default" {
		t.Fatalf("Name = %q", got.Name)
	}
	if len(got.AggregationMethods) != 3 {
		t.Fatalf("AggregationMethods = %#v", got.AggregationMethods)
	}
	wantMethods := map[string]bool{"mean": true, "min": true, "max": true}
	for _, method := range got.AggregationMethods {
		if !wantMethods[method] {
			t.Fatalf("unexpected aggregation method %q", method)
		}
	}
	wantDefs := []ArchivePolicyDefinition{
		{Granularity: "60s", Timespan: "7d"},
		{Granularity: "5m", Timespan: "90d"},
		{Granularity: "1h", Timespan: "365d"},
	}
	if len(got.Definition) != len(wantDefs) {
		t.Fatalf("Definition = %#v", got.Definition)
	}
	for i := range wantDefs {
		if got.Definition[i] != wantDefs[i] {
			t.Fatalf("Definition[%d] = %#v, want %#v", i, got.Definition[i], wantDefs[i])
		}
	}
}

func TestBuildBatchMeasuresUsesInstanceResourceAndNamedMetrics(t *testing.T) {
	s := sample.Sample{
		Timestamp:  time.Date(2026, 5, 28, 1, 2, 3, 0, time.UTC),
		InstanceID: "instance-1",
		Metrics: map[string]float64{
			"guest.cpu.used_percent":    12.5,
			"guest.memory.used_percent": 34.5,
		},
	}

	batch, err := BuildBatchMeasures(s, "vm_guest_usage_default")
	if err != nil {
		t.Fatal(err)
	}

	resource := batch["instance-1"]
	if len(resource) != 2 {
		t.Fatalf("resource metrics = %#v", resource)
	}
	cpu := resource["guest.cpu.used_percent"]
	if cpu.ArchivePolicyName != "vm_guest_usage_default" {
		t.Fatalf("ArchivePolicyName = %q", cpu.ArchivePolicyName)
	}
	if len(cpu.Measures) != 1 {
		t.Fatalf("Measures = %#v", cpu.Measures)
	}
	if cpu.Measures[0].Timestamp != "2026-05-28T01:02:03Z" {
		t.Fatalf("Timestamp = %q", cpu.Measures[0].Timestamp)
	}
	if cpu.Measures[0].Value != 12.5 {
		t.Fatalf("Value = %v", cpu.Measures[0].Value)
	}
}

func TestBuildBatchMeasuresRejectsMissingInstanceID(t *testing.T) {
	_, err := BuildBatchMeasures(sample.Sample{Metrics: map[string]float64{"guest.cpu.used_percent": 1}}, "policy")
	if err == nil {
		t.Fatal("BuildBatchMeasures returned nil error for missing instance id")
	}
}

func TestEnsureGenericResourcePostsInstanceResource(t *testing.T) {
	var got map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/resource/generic" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if user, _, ok := r.BasicAuth(); !ok || user != "admin" {
			t.Fatalf("missing basic auth")
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "", server.Client())
	err := client.EnsureGenericResource(t.Context(), sample.Sample{
		InstanceID: "instance-1",
		ProjectID:  "project-1",
		Hostname:   "host-1",
		Name:       "name-1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["id"] != "instance-1" {
		t.Fatalf("id = %q", got["id"])
	}
	if got["project_id"] != "project-1" {
		t.Fatalf("project_id = %q", got["project_id"])
	}
	if _, ok := got["host"]; ok {
		t.Fatal("generic resource payload should not include host")
	}
}
