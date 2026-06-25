package gnocchi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"crms-agent/internal/sample"
)

type ArchivePolicy struct {
	Name               string                    `json:"name"`
	AggregationMethods []string                  `json:"aggregation_methods"`
	Definition         []ArchivePolicyDefinition `json:"definition"`
}

type ArchivePolicyDefinition struct {
	Granularity string `json:"granularity"`
	Timespan    string `json:"timespan"`
}

type Measure struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

type MetricMeasures struct {
	ArchivePolicyName string    `json:"archive_policy_name"`
	Measures          []Measure `json:"measures"`
}

type BatchMeasures map[string]map[string]MetricMeasures

type GenericResource struct {
	ID        string `json:"id"`
	ProjectID string `json:"project_id,omitempty"`
	UserID    string `json:"user_id,omitempty"`
}

type Client struct {
	endpoint string
	username string
	password string
	client   *http.Client
}

func NewClient(endpoint, username, password string, client *http.Client) Client {
	if client == nil {
		client = http.DefaultClient
	}
	return Client{
		endpoint: strings.TrimRight(endpoint, "/"),
		username: username,
		password: password,
		client:   client,
	}
}

func DefaultArchivePolicy(name string) ArchivePolicy {
	return ArchivePolicy{
		Name:               name,
		AggregationMethods: []string{"mean", "min", "max"},
		Definition: []ArchivePolicyDefinition{
			{Granularity: "60s", Timespan: "7d"},
			{Granularity: "5m", Timespan: "90d"},
			{Granularity: "1h", Timespan: "365d"},
		},
	}
}

func BuildBatchMeasures(s sample.Sample, archivePolicyName string) (BatchMeasures, error) {
	if s.InstanceID == "" {
		return nil, errors.New("sample instance_id is required for gnocchi resource id")
	}
	resourceMetrics := map[string]MetricMeasures{}
	timestamp := s.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
	for name, value := range s.Metrics {
		resourceMetrics[name] = MetricMeasures{
			ArchivePolicyName: archivePolicyName,
			Measures: []Measure{
				{Timestamp: timestamp, Value: value},
			},
		}
	}
	return BatchMeasures{s.InstanceID: resourceMetrics}, nil
}

func (c Client) EnsureArchivePolicy(ctx context.Context, policy ArchivePolicy) error {
	body, err := json.Marshal(policy)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/archive_policy", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		return nil
	}
	return responseError("create archive policy", resp)
}

func (c Client) EnsureGenericResource(ctx context.Context, s sample.Sample) error {
	if s.InstanceID == "" {
		return errors.New("sample instance_id is required for gnocchi resource id")
	}
	resource := GenericResource{
		ID:        s.InstanceID,
		ProjectID: s.ProjectID,
	}
	body, err := json.Marshal(resource)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/resource/generic", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusCreated || resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusConflict {
		return nil
	}
	return responseError("create generic resource", resp)
}

func (c Client) SendSample(ctx context.Context, s sample.Sample, archivePolicyName string) error {
	if err := c.EnsureGenericResource(ctx, s); err != nil {
		return err
	}
	batch, err := BuildBatchMeasures(s, archivePolicyName)
	if err != nil {
		return err
	}
	body, err := json.Marshal(batch)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/v1/batch/resources/metrics/measures?create_metrics=true", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	c.setAuth(req)
	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusAccepted {
		return nil
	}
	return responseError("send measures", resp)
}

func (c Client) setAuth(req *http.Request) {
	if c.username != "" {
		req.SetBasicAuth(c.username, c.password)
	}
}

func responseError(action string, resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	return fmt.Errorf("%s: gnocchi returned %d: %s", action, resp.StatusCode, strings.TrimSpace(string(body)))
}
