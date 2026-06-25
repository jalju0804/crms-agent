package sample

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type Sample struct {
	Timestamp        time.Time          `json:"timestamp"`
	InstanceID       string             `json:"instance_id,omitempty"`
	ProjectID        string             `json:"project_id,omitempty"`
	Hostname         string             `json:"hostname,omitempty"`
	Name             string             `json:"name,omitempty"`
	AvailabilityZone string             `json:"availability_zone,omitempty"`
	Metrics          map[string]float64 `json:"metrics"`
	Errors           []string           `json:"errors,omitempty"`
}

type Writer struct {
	path string
}

func NewWriter(path string) Writer {
	return Writer{path: path}
}

func (w Writer) Append(s Sample) (int, error) {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o750); err != nil {
		return 0, err
	}
	body, err := json.Marshal(s)
	if err != nil {
		return 0, err
	}
	body = append(body, '\n')
	file, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o640)
	if err != nil {
		return 0, err
	}
	defer file.Close()
	n, err := file.Write(body)
	if err != nil {
		return n, err
	}
	return n, nil
}
