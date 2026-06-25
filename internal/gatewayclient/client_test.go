package gatewayclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"crms-agent/internal/sample"
)

func TestClientPostsSampleWithBearerToken(t *testing.T) {
	var got sample.Sample
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %q", r.Method)
		}
		if r.URL.Path != "/v1/samples" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := New(server.URL+"/", "secret", server.Client())
	s := validClientSample()
	if err := client.SendSample(t.Context(), s); err != nil {
		t.Fatal(err)
	}
	if got.InstanceID != "vm-1" {
		t.Fatalf("InstanceID = %q", got.InstanceID)
	}
	if got.Metrics["guest.cpu.used_percent"] != 12.5 {
		t.Fatalf("metric = %v", got.Metrics["guest.cpu.used_percent"])
	}
}

func TestClientOmitsAuthorizationWhenTokenIsEmpty(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Fatalf("Authorization = %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	client := New(server.URL, "", server.Client())
	if err := client.SendSample(t.Context(), validClientSample()); err != nil {
		t.Fatal(err)
	}
}

func TestClientReturnsErrorForNonAcceptedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gateway failed", http.StatusBadGateway)
	}))
	defer server.Close()

	client := New(server.URL, "", server.Client())
	err := client.SendSample(t.Context(), validClientSample())
	if err == nil {
		t.Fatal("SendSample returned nil error")
	}
	if !strings.Contains(err.Error(), "502") {
		t.Fatalf("error = %v", err)
	}
	if !strings.Contains(err.Error(), "gateway failed") {
		t.Fatalf("error = %v", err)
	}
}

func validClientSample() sample.Sample {
	return sample.Sample{
		Timestamp:  time.Date(2026, 6, 25, 1, 2, 3, 0, time.UTC),
		InstanceID: "vm-1",
		ProjectID:  "project-1",
		Metrics: map[string]float64{
			"guest.cpu.used_percent": 12.5,
		},
	}
}
