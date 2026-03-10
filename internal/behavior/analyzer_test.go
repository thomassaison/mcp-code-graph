package behavior

import (
	"context"
	"errors"
	"testing"
)

func TestMockAnalyzer(t *testing.T) {
	analyzer := NewMockAnalyzer()

	req := AnalysisRequest{
		PackageName:  "testpkg",
		FunctionName: "TestFunc",
		Signature:    "func TestFunc() error",
		Code:         "func TestFunc() error { log.Println(\"test\"); return nil }",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) == 0 {
		t.Error("Analyze() returned no behaviors")
	}
}

func TestMockAnalyzerWithPresetBehaviors(t *testing.T) {
	preset := []string{BehaviorLogging, BehaviorErrorHandle}
	analyzer := NewMockAnalyzer().WithBehaviors(preset)

	req := AnalysisRequest{
		PackageName:  "testpkg",
		FunctionName: "TestFunc",
		Signature:    "func TestFunc() error",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}

	if len(behaviors) != len(preset) {
		t.Errorf("Analyze() returned %d behaviors, want %d", len(behaviors), len(preset))
	}
	for i, b := range preset {
		if behaviors[i] != b {
			t.Errorf("Analyze() behaviors[%d] = %q, want %q", i, behaviors[i], b)
		}
	}
}

func TestMockAnalyzerWithError(t *testing.T) {
	expectedErr := errors.New("analysis failed")
	analyzer := NewMockAnalyzer().WithError(expectedErr)

	req := AnalysisRequest{
		PackageName:  "testpkg",
		FunctionName: "TestFunc",
	}

	behaviors, err := analyzer.Analyze(context.Background(), req)
	if err != expectedErr {
		t.Errorf("Analyze() error = %v, want %v", err, expectedErr)
	}
	if behaviors != nil {
		t.Error("Analyze() should return nil behaviors on error")
	}
}
