package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"crms-agent/internal/registry"
	"crms-agent/internal/sample"
)

type SampleSink interface {
	SendSample(ctx context.Context, s sample.Sample) error
}

type Options struct {
	AuthToken string
	Ready     bool
	Sink      SampleSink
	Registry  *registry.Registry
}

type Server struct {
	authToken string
	ready     bool
	sink      SampleSink
	registry  *registry.Registry
	mux       *http.ServeMux
}

func NewServer(opts Options) *Server {
	reg := opts.Registry
	if reg == nil {
		reg = registry.New()
	}
	s := &Server{
		authToken: opts.AuthToken,
		ready:     opts.Ready,
		sink:      opts.Sink,
		registry:  reg,
		mux:       http.NewServeMux(),
	}
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/readyz", s.handleReady)
	s.mux.HandleFunc("/v1/samples", s.handleSamples)
	s.mux.HandleFunc("/v1/agents", s.handleAgents)
	s.mux.HandleFunc("/v1/agents/", s.handleAgent)
	return s
}

func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.ready {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status":  "not_ready",
			"gnocchi": "unavailable",
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ready",
		"gnocchi": "ok",
	})
}

func (s *Server) handleSamples(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorize(w, r) {
		return
	}

	var body sample.Sample
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := decoder.Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid sample JSON")
		return
	}
	if err := validateSample(body); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.sink == nil {
		err := errors.New("sample sink is not configured")
		s.registry.RecordError(body, err.Error())
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := s.sink.SendSample(r.Context(), body); err != nil {
		s.registry.RecordError(body, err.Error())
		writeError(w, http.StatusBadGateway, fmt.Sprintf("write sample: %v", err))
		return
	}
	s.registry.RecordSuccess(body)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "accepted",
		"instance_id":  body.InstanceID,
		"metric_count": len(body.Metrics),
	})
}

func (s *Server) handleAgents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorize(w, r) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"agents": s.registry.List()})
}

func (s *Server) handleAgent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !s.authorize(w, r) {
		return
	}
	instanceID := strings.TrimPrefix(r.URL.Path, "/v1/agents/")
	if instanceID == "" {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	status, ok := s.registry.Get(instanceID)
	if !ok {
		writeError(w, http.StatusNotFound, "agent not found")
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func (s *Server) authorize(w http.ResponseWriter, r *http.Request) bool {
	if s.authToken == "" {
		return true
	}
	want := "Bearer " + s.authToken
	if r.Header.Get("Authorization") != want {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return false
	}
	return true
}

func validateSample(s sample.Sample) error {
	if s.Timestamp.IsZero() {
		return errors.New("timestamp must be present")
	}
	if s.InstanceID == "" {
		return errors.New("instance_id must be present")
	}
	if len(s.Metrics) == 0 {
		return errors.New("metrics must contain at least one metric")
	}
	for name := range s.Metrics {
		if name == "" {
			return errors.New("metric name must not be empty")
		}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
