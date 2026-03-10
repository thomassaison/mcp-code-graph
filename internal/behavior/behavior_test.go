package behavior

import "testing"

func TestBehaviorConstants(t *testing.T) {
	tests := []struct {
		name     string
		behavior string
		expected string
	}{
		{"logging", BehaviorLogging, "logging"},
		{"error-handle", BehaviorErrorHandle, "error-handle"},
		{"database", BehaviorDatabase, "database"},
		{"http-client", BehaviorHTTPClient, "http-client"},
		{"file-io", BehaviorFileIO, "file-io"},
		{"concurrency", BehaviorConcurrency, "concurrency"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.behavior != tt.expected {
				t.Errorf("Behavior constant %s = %q, want %q", tt.name, tt.behavior, tt.expected)
			}
		})
	}
}

func TestAllBehaviors(t *testing.T) {
	expected := []string{
		BehaviorLogging,
		BehaviorErrorHandle,
		BehaviorDatabase,
		BehaviorHTTPClient,
		BehaviorFileIO,
		BehaviorConcurrency,
	}

	all := AllBehaviors()
	if len(all) != len(expected) {
		t.Errorf("AllBehaviors() returned %d behaviors, want %d", len(all), len(expected))
	}

	for _, b := range expected {
		found := false
		for _, a := range all {
			if a == b {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("AllBehaviors() missing behavior %s", b)
		}
	}
}

func TestIsValidBehavior(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"valid behavior", BehaviorLogging, true},
		{"invalid behavior", "invalid-behavior", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsValidBehavior(tt.input); got != tt.expected {
				t.Errorf("IsValidBehavior(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}
