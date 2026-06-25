package metadata

import "testing"

func TestParseOpenStackMetadata(t *testing.T) {
	body := []byte(`{
		"uuid": "6d85f487-76e8-45fb-b0df-2324cde2ab5b",
		"project_id": "ff12b31365a145c6a56368854df1b8b4",
		"hostname": "k8s-master-2.novalocal",
		"name": "k8s-master-2",
		"availability_zone": "nova"
	}`)

	info, err := Parse(body)
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}

	if info.InstanceID != "6d85f487-76e8-45fb-b0df-2324cde2ab5b" {
		t.Fatalf("InstanceID = %q", info.InstanceID)
	}
	if info.ProjectID != "ff12b31365a145c6a56368854df1b8b4" {
		t.Fatalf("ProjectID = %q", info.ProjectID)
	}
	if info.Hostname != "k8s-master-2.novalocal" {
		t.Fatalf("Hostname = %q", info.Hostname)
	}
	if info.Name != "k8s-master-2" {
		t.Fatalf("Name = %q", info.Name)
	}
	if info.AvailabilityZone != "nova" {
		t.Fatalf("AvailabilityZone = %q", info.AvailabilityZone)
	}
}

func TestParseRejectsMissingUUID(t *testing.T) {
	if _, err := Parse([]byte(`{"project_id":"tenant"}`)); err == nil {
		t.Fatal("Parse returned nil error for missing uuid")
	}
}
