package behavior

import "testing"

func TestBehaviorConstants(t *testing.T) {
	tests := []struct {
		name     string
		behavior string
	}{
		{"logging", BehaviorLogging},
		{"error-handle", BehaviorErrorHandle},
		{"database", BehaviorDatabase},
		{"http-client", BehaviorHTTPClient},
		{"file-io", BehaviorFileIO},
		{"concurrency", BehaviorConcurrency},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.behavior == "" {
				t.Errorf("Behavior constant %s is empty", tt.name)
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
