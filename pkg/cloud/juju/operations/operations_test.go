package operations

import (
	"encoding/json"
	"testing"
)

func TestJujuStatusParsesApplicationStatus(t *testing.T) {
	raw := `{
		"applications": {
			"mysql": {
				"application-status": {
					"current": "active",
					"message": "ready",
					"since": "2024-01-01T00:00:00Z"
				}
			}
		}
	}`

	var status JujuStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	app, ok := status.Applications["mysql"]
	if !ok {
		t.Fatal("mysql not found in applications")
	}
	if app.ApplicationStatus.Current != "active" {
		t.Errorf("expected active, got %s", app.ApplicationStatus.Current)
	}
	if app.ApplicationStatus.Message != "ready" {
		t.Errorf("expected ready, got %s", app.ApplicationStatus.Message)
	}
	if app.ApplicationStatus.Since != "2024-01-01T00:00:00Z" {
		t.Errorf("expected 2024-01-01T00:00:00Z, got %s", app.ApplicationStatus.Since)
	}
}

func TestJujuStatusParsesUnitStatus(t *testing.T) {
	raw := `{
		"applications": {
			"mysql": {
				"application-status": {
					"current": "active",
					"message": "ready"
				},
				"units": {
					"mysql/0": {
						"workload-status": {
							"current": "active",
							"message": "Primary",
							"since": "2024-01-01T00:00:00Z"
						},
						"juju-status": {
							"current": "idle",
							"message": ""
						}
					},
					"mysql/1": {
						"workload-status": {
							"current": "waiting",
							"message": "configuring"
						},
						"juju-status": {
							"current": "executing",
							"message": ""
						}
					}
				}
			}
		}
	}`

	var status JujuStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	app := status.Applications["mysql"]
	if len(app.Units) != 2 {
		t.Fatalf("expected 2 units, got %d", len(app.Units))
	}

	unit0 := app.Units["mysql/0"]
	if unit0.WorkloadStatus.Current != "active" {
		t.Errorf("expected active, got %s", unit0.WorkloadStatus.Current)
	}
	if unit0.WorkloadStatus.Message != "Primary" {
		t.Errorf("expected Primary, got %s", unit0.WorkloadStatus.Message)
	}

	unit1 := app.Units["mysql/1"]
	if unit1.WorkloadStatus.Current != "waiting" {
		t.Errorf("expected waiting, got %s", unit1.WorkloadStatus.Current)
	}
}

func TestJujuStatusNoUnits(t *testing.T) {
	raw := `{
		"applications": {
			"nginx": {
				"application-status": {
					"current": "blocked",
					"message": "no relations"
				}
			}
		}
	}`

	var status JujuStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	app := status.Applications["nginx"]
	if app.Units != nil && len(app.Units) != 0 {
		t.Errorf("expected nil/empty units, got %d", len(app.Units))
	}
	if app.ApplicationStatus.Current != "blocked" {
		t.Errorf("expected blocked, got %s", app.ApplicationStatus.Current)
	}
}

func TestPebbleServiceInfoParsing(t *testing.T) {
	raw := `[
		{"name": "myservice", "current": "active"},
		{"name": "other", "current": "inactive"}
	]`

	var services []PebbleServiceInfo
	if err := json.Unmarshal([]byte(raw), &services); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(services))
	}
	if services[0].Name != "myservice" || services[0].Current != "active" {
		t.Errorf("unexpected first service: %+v", services[0])
	}
	if services[1].Name != "other" || services[1].Current != "inactive" {
		t.Errorf("unexpected second service: %+v", services[1])
	}
}

func TestAllSupportedStatusValues(t *testing.T) {
	statuses := []string{"active", "waiting", "error", "blocked", "maintenance", "unknown", "terminated"}
	for _, s := range statuses {
		raw := `{"applications": {"app": {"application-status": {"current": "` + s + `"}}}}`
		var status JujuStatus
		if err := json.Unmarshal([]byte(raw), &status); err != nil {
			t.Fatalf("failed to unmarshal status %s: %v", s, err)
		}
		if status.Applications["app"].ApplicationStatus.Current != s {
			t.Errorf("expected %s, got %s", s, status.Applications["app"].ApplicationStatus.Current)
		}
	}
}
