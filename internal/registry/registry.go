package registry

import (
	"sort"
	"sync"
	"time"

	"crms-agent/internal/sample"
)

type Status struct {
	InstanceID        string    `json:"instance_id"`
	ProjectID         string    `json:"project_id,omitempty"`
	Hostname          string    `json:"hostname,omitempty"`
	Name              string    `json:"name,omitempty"`
	AvailabilityZone  string    `json:"availability_zone,omitempty"`
	LastSeen          time.Time `json:"last_seen"`
	LastSuccessAt     time.Time `json:"last_success_at,omitempty"`
	LastMetricCount   int       `json:"last_metric_count"`
	LastError         string    `json:"last_error,omitempty"`
}

type Registry struct {
	mu       sync.RWMutex
	statuses map[string]Status
	now      func() time.Time
}

func New() *Registry {
	return &Registry{
		statuses: map[string]Status{},
		now: func() time.Time {
			return time.Now().UTC()
		},
	}
}

func (r *Registry) RecordSuccess(s sample.Sample) {
	if s.InstanceID == "" {
		return
	}
	seen := r.sampleTime(s)

	r.mu.Lock()
	defer r.mu.Unlock()

	status := r.statuses[s.InstanceID]
	status = applySample(status, s, seen)
	status.LastSuccessAt = seen
	status.LastMetricCount = len(s.Metrics)
	status.LastError = ""
	r.statuses[s.InstanceID] = status
}

func (r *Registry) RecordError(s sample.Sample, message string) {
	if s.InstanceID == "" {
		return
	}
	seen := r.sampleTime(s)

	r.mu.Lock()
	defer r.mu.Unlock()

	status := r.statuses[s.InstanceID]
	status = applySample(status, s, seen)
	status.LastMetricCount = len(s.Metrics)
	status.LastError = message
	r.statuses[s.InstanceID] = status
}

func (r *Registry) Get(instanceID string) (Status, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	status, ok := r.statuses[instanceID]
	return status, ok
}

func (r *Registry) List() []Status {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make([]Status, 0, len(r.statuses))
	for _, status := range r.statuses {
		statuses = append(statuses, status)
	}
	sort.Slice(statuses, func(i, j int) bool {
		return statuses[i].LastSeen.After(statuses[j].LastSeen)
	})
	return statuses
}

func (r *Registry) sampleTime(s sample.Sample) time.Time {
	if !s.Timestamp.IsZero() {
		return s.Timestamp.UTC()
	}
	return r.now()
}

func applySample(status Status, s sample.Sample, seen time.Time) Status {
	status.InstanceID = s.InstanceID
	status.ProjectID = s.ProjectID
	status.Hostname = s.Hostname
	status.Name = s.Name
	status.AvailabilityZone = s.AvailabilityZone
	status.LastSeen = seen
	return status
}
