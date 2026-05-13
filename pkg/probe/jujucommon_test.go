package probe

import (
	"testing"
)

func TestIsJujuExperiment(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{"juju-add-app", true},
		{"juju-integrate", true},
		{"juju-remove-relation", true},
		{"pod-delete", false},
		{"container-kill", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isJujuExperiment(tc.name); got != tc.expected {
			t.Errorf("isJujuExperiment(%q) = %v, want %v", tc.name, got, tc.expected)
		}
	}
}

func TestGetJujuCredentialsFileDefault(t *testing.T) {
	got := getJujuCredentialsFile()
	if got != defaultJujuCredentialsFile {
		t.Errorf("expected %s, got %s", defaultJujuCredentialsFile, got)
	}
}
