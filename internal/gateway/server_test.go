package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crms-agent/internal/registry"
	"crms-agent/internal/sample"
)

type fakeSink struct {
	err     error
	samples []sample.Sample
}

func (f *fakeSink) SendSample(ctx context.Context, s sample.Sample) error {
	f.samples = append(f.samples, s)
	return f.err
}

func TestSamplesRejectsMissingBearerToken(t *testing.T) {
	sink := &fakeSink{}
	server := NewServer(Options{
		AuthToken: "secret",
		Ready:     true,
		Sink:      sink,
		Registry:  registry.New(),
	})

	req := newSampleRequest(t, validSample("vm-1"))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(sink.samples) != 0 {
		t.Fatalf("sink samples = %d", len(sink.samples))
	}
}

func TestSamplesAcceptsValidSampleAndRecordsAgent(t *testing.T) {
	sink := &fakeSink{}
	reg := registry.New()
	server := NewServer(Options{
		AuthToken: "secret",
		Ready:     true,
		Sink:      sink,
		Registry:  reg,
	})
	s := validSample("vm-1")

	req := newSampleRequest(t, s)
	req.Header.Set("Authorization", "Bearer secret")
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(sink.samples) != 1 {
		t.Fatalf("sink samples = %d", len(sink.samples))
	}
	status, ok := reg.Get("vm-1")
	if !ok {
		t.Fatal("expected vm-1 in registry")
	}
	if status.LastMetricCount != 2 {
		t.Fatalf("LastMetricCount = %d", status.LastMetricCount)
	}
	if status.LastError != "" {
		t.Fatalf("LastError = %q", status.LastError)
	}

	var got map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got["status"] != "accepted" {
		t.Fatalf("response status = %#v", got["status"])
	}
	if got["instance_id"] != "vm-1" {
		t.Fatalf("response instance_id = %#v", got["instance_id"])
	}
}

func TestSamplesAllowsEmptyAuthTokenForLocalDevelopment(t *testing.T) {
	sink := &fakeSink{}
	server := NewServer(Options{
		Ready:    true,
		Sink:     sink,
		Registry: registry.New(),
	})

	req := newSampleRequest(t, validSample("vm-1"))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSamplesRejectsMissingInstanceID(t *testing.T) {
	sink := &fakeSink{}
	reg := registry.New()
	server := NewServer(Options{
		Ready:    true,
		Sink:     sink,
		Registry: reg,
	})
	s := validSample("")

	req := newSampleRequest(t, s)
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if len(sink.samples) != 0 {
		t.Fatalf("sink samples = %d", len(sink.samples))
	}
	if len(reg.List()) != 0 {
		t.Fatalf("registry list = %#v", reg.List())
	}
}

func TestSamplesReturnsBadGatewayAndRecordsErrorWhenSinkFails(t *testing.T) {
	sink := &fakeSink{err: errors.New("gnocchi unavailable")}
	reg := registry.New()
	server := NewServer(Options{
		Ready:    true,
		Sink:     sink,
		Registry: reg,
	})

	req := newSampleRequest(t, validSample("vm-1"))
	rec := httptest.NewRecorder()
	server.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	status, ok := reg.Get("vm-1")
	if !ok {
		t.Fatal("expected vm-1 in registry")
	}
	if !strings.Contains(status.LastError, "gnocchi unavailable") {
		t.Fatalf("LastError = %q", status.LastError)
	}
	if !status.LastSuccessAt.IsZero() {
		t.Fatalf("LastSuccessAt = %v", status.LastSuccessAt)
	}
}

func TestAgentsListAndGet(t *testing.T) {
	reg := registry.New()
	oldSample := validSample("old")
	oldSample.Timestamp = time.Date(2026, 6, 25, 1, 0, 0, 0, time.UTC)
	newSample := validSample("new")
	newSample.Timestamp = oldSample.Timestamp.Add(time.Minute)
	reg.RecordSuccess(oldSample)
	reg.RecordSuccess(newSample)
	server := NewServer(Options{
		Ready:    true,
		Sink:     &fakeSink{},
		Registry: reg,
	})

	listReq := httptest.NewRequest(http.MethodGet, "/v1/agents", nil)
	listRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(listRec, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d, body = %s", listRec.Code, listRec.Body.String())
	}
	var list struct {
		Agents []registry.Status `json:"agents"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Agents) != 2 || list.Agents[0].InstanceID != "new" {
		t.Fatalf("agents = %#v", list.Agents)
	}

	getReq := httptest.NewRequest(http.MethodGet, "/v1/agents/new", nil)
	getRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	var got registry.Status
	if err := json.Unmarshal(getRec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.InstanceID != "new" {
		t.Fatalf("InstanceID = %q", got.InstanceID)
	}

	missingReq := httptest.NewRequest(http.MethodGet, "/v1/agents/missing", nil)
	missingRec := httptest.NewRecorder()
	server.Handler().ServeHTTP(missingRec, missingReq)
	if missingRec.Code != http.StatusNotFound {
		t.Fatalf("missing status = %d", missingRec.Code)
	}
}

func TestHealthAndReady(t *testing.T) {
	readyServer := NewServer(Options{
		Ready:    true,
		Sink:     &fakeSink{},
		Registry: registry.New(),
	})

	healthReq := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	healthRec := httptest.NewRecorder()
	readyServer.Handler().ServeHTTP(healthRec, healthReq)
	if healthRec.Code != http.StatusOK {
		t.Fatalf("health status = %d", healthRec.Code)
	}

	readyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	readyRec := httptest.NewRecorder()
	readyServer.Handler().ServeHTTP(readyRec, readyReq)
	if readyRec.Code != http.StatusOK {
		t.Fatalf("ready status = %d", readyRec.Code)
	}

	notReadyServer := NewServer(Options{
		Ready:    false,
		Sink:     &fakeSink{},
		Registry: registry.New(),
	})
	notReadyReq := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	notReadyRec := httptest.NewRecorder()
	notReadyServer.Handler().ServeHTTP(notReadyRec, notReadyReq)
	if notReadyRec.Code != http.StatusServiceUnavailable {
		t.Fatalf("not ready status = %d", notReadyRec.Code)
	}
}

func newSampleRequest(t *testing.T, s sample.Sample) *http.Request {
	t.Helper()
	body, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/samples", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func validSample(instanceID string) sample.Sample {
	return sample.Sample{
		Timestamp:        time.Date(2026, 6, 25, 1, 2, 3, 0, time.UTC),
		InstanceID:       instanceID,
		ProjectID:        "project-1",
		Hostname:         "host-1",
		Name:             "name-1",
		AvailabilityZone: "nova",
		Metrics: map[string]float64{
			"guest.cpu.used_percent":    12.5,
			"guest.memory.used_percent": 34.5,
		},
	}
}
