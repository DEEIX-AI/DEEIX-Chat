// Package embedding defines the application-facing contract for text embeddings.
package embedding

// Request describes one batch sent to an embedding provider.
type Request struct {
	APIBase        string
	APIKey         string
	Model          string
	Texts          []string
	Dimensions     int
	TimeoutSeconds int
}
