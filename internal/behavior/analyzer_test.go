package behavior

import (
	"context"
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
}
