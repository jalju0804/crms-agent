package metadata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Info struct {
	InstanceID       string `json:"uuid"`
	ProjectID        string `json:"project_id"`
	Hostname         string `json:"hostname"`
	Name             string `json:"name"`
	AvailabilityZone string `json:"availability_zone"`
}

func Parse(body []byte) (Info, error) {
	var info Info
	if err := json.Unmarshal(body, &info); err != nil {
		return Info{}, err
	}
	if info.InstanceID == "" {
		return Info{}, errors.New("metadata uuid is missing")
	}
	return info, nil
}

func Fetch(ctx context.Context, client *http.Client, url string) (Info, error) {
	if client == nil {
		client = &http.Client{Timeout: 2 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Info{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return Info{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return Info{}, fmt.Errorf("metadata service returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Info{}, err
	}
	return Parse(body)
}
