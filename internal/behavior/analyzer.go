package behavior

import (
	"context"
)

type AnalysisRequest struct {
	PackageName  string
	FunctionName string
	Signature    string
	Docstring    string
	Code         string
}

type Analyzer interface {
	Analyze(ctx context.Context, req AnalysisRequest) ([]string, error)
}

type MockAnalyzer struct {
	behaviors []string
	err       error
}

func NewMockAnalyzer() *MockAnalyzer {
	return &MockAnalyzer{
		behaviors: []string{BehaviorLogging},
	}
}

func (m *MockAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) ([]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.behaviors) > 0 {
		return m.behaviors, nil
	}
	return []string{BehaviorLogging}, nil
}

func (m *MockAnalyzer) WithBehaviors(behaviors []string) *MockAnalyzer {
	m.behaviors = behaviors
	return m
}

func (m *MockAnalyzer) WithError(err error) *MockAnalyzer {
	m.err = err
	return m
}
