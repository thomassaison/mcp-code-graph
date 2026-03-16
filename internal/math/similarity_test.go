package math

import (
	"math"
	"testing"
)

func TestCosineSimilarity_identicalVectors(t *testing.T) {
	t.Parallel()
	a := []float32{1, 2, 3}
	got := CosineSimilarity(a, a)
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity(a, a) = %v, want 1.0", got)
	}
}

func TestCosineSimilarity_parallelVectors(t *testing.T) {
	t.Parallel()
	a := []float32{1, 2, 3}
	b := []float32{2, 4, 6}
	got := CosineSimilarity(a, b)
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity = %v, want 1.0 (parallel vectors)", got)
	}
}

func TestCosineSimilarity_orthogonalVectors(t *testing.T) {
	t.Parallel()
	a := []float32{1, 0}
	b := []float32{0, 1}
	got := CosineSimilarity(a, b)
	if math.Abs(float64(got)) > 1e-6 {
		t.Errorf("CosineSimilarity = %v, want 0.0 (orthogonal)", got)
	}
}

func TestCosineSimilarity_oppositeVectors(t *testing.T) {
	t.Parallel()
	a := []float32{1, 2, 3}
	b := []float32{-1, -2, -3}
	got := CosineSimilarity(a, b)
	if math.Abs(float64(got+1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity = %v, want -1.0 (opposite)", got)
	}
}

func TestCosineSimilarity_emptyVectors(t *testing.T) {
	t.Parallel()
	got := CosineSimilarity([]float32{}, []float32{})
	if got != 0 {
		t.Errorf("CosineSimilarity(empty, empty) = %v, want 0", got)
	}
}

func TestCosineSimilarity_nilVectors(t *testing.T) {
	t.Parallel()
	got := CosineSimilarity(nil, nil)
	if got != 0 {
		t.Errorf("CosineSimilarity(nil, nil) = %v, want 0", got)
	}
}

func TestCosineSimilarity_differentLengths(t *testing.T) {
	t.Parallel()
	a := []float32{1, 2}
	b := []float32{1, 2, 3}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("CosineSimilarity(different lengths) = %v, want 0", got)
	}
}

func TestCosineSimilarity_zeroNormA(t *testing.T) {
	t.Parallel()
	a := []float32{0, 0, 0}
	b := []float32{1, 2, 3}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("CosineSimilarity(zero norm A) = %v, want 0", got)
	}
}

func TestCosineSimilarity_zeroNormB(t *testing.T) {
	t.Parallel()
	a := []float32{1, 2, 3}
	b := []float32{0, 0, 0}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("CosineSimilarity(zero norm B) = %v, want 0", got)
	}
}

func TestCosineSimilarity_bothZeroNorm(t *testing.T) {
	t.Parallel()
	a := []float32{0, 0}
	b := []float32{0, 0}
	got := CosineSimilarity(a, b)
	if got != 0 {
		t.Errorf("CosineSimilarity(both zero) = %v, want 0", got)
	}
}

func TestCosineSimilarity_negativeValues(t *testing.T) {
	t.Parallel()
	a := []float32{1, -1, 0}
	b := []float32{0, 1, -1}
	// dot = 0*1 + (-1)*1 + 0*(-1) = -1
	// normA = sqrt(1+1+0) = sqrt(2)
	// normB = sqrt(0+1+1) = sqrt(2)
	// cos = -1 / 2 = -0.5
	got := CosineSimilarity(a, b)
	want := float32(-0.5)
	if math.Abs(float64(got-want)) > 1e-6 {
		t.Errorf("CosineSimilarity(negative) = %v, want %v", got, want)
	}
}

func TestCosineSimilarity_singleElement(t *testing.T) {
	t.Parallel()
	got := CosineSimilarity([]float32{3}, []float32{3})
	if math.Abs(float64(got-1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity single element = %v, want 1.0", got)
	}
}

func TestCosineSimilarity_oneEmptyOneNonEmpty(t *testing.T) {
	t.Parallel()
	got := CosineSimilarity([]float32{}, []float32{1, 2})
	if got != 0 {
		t.Errorf("CosineSimilarity(empty, nonempty) = %v, want 0", got)
	}
}
