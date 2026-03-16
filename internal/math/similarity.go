// anthropic/claude-sonnet-4-6
package math

import "math"

// CosineSimilarity computes the cosine similarity between two vectors.
// Returns 0 if vectors have different lengths or are empty.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}

	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dot / float32(math.Sqrt(float64(normA*normB)))
}

// DotProduct computes the dot product of two equal-length vectors.
// Returns 0 if vectors have different lengths or are empty.
func DotProduct(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot float32
	for i := range a {
		dot += a[i] * b[i]
	}
	return dot
}

// L2Norm computes the L2 (Euclidean) norm of a vector.
func L2Norm(a []float32) float32 {
	var sum float32
	for _, v := range a {
		sum += v * v
	}
	return float32(math.Sqrt(float64(sum)))
}
