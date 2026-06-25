package sample

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriterAppendsSingleJSONLine(t *testing.T) {
	path := filepath.Join(t.TempDir(), "samples.jsonl")
	writer := NewWriter(path)
	s := Sample{
		Timestamp:  time.Date(2026, 5, 28, 0, 0, 0, 0, time.UTC),
		InstanceID: "instance-1",
		ProjectID:  "project-1",
		Metrics: map[string]float64{
			"guest.cpu.used_percent": 12.5,
		},
	}

	size, err := writer.Append(s)
	if err != nil {
		t.Fatalf("Append returned error: %v", err)
	}
	if size == 0 {
		t.Fatal("Append returned zero size")
	}

	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("expected one JSONL row")
	}
	line := scanner.Text()
	if scanner.Scan() {
		t.Fatal("expected only one JSONL row")
	}

	var got Sample
	if err := json.Unmarshal([]byte(line), &got); err != nil {
		t.Fatalf("row is not valid sample JSON: %v", err)
	}
	if got.Metrics["guest.cpu.used_percent"] != 12.5 {
		t.Fatalf("metric = %v", got.Metrics["guest.cpu.used_percent"])
	}
}
