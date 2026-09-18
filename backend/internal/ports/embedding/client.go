// Package embedding defines the application-facing contract for text embeddings.
package embedding

// Request describes one batch sent to an embedding provider.
type Request struct {
	APIBase    string
	APIKey     string
	Model      string
	Texts      []string
	Dimensions int
	// OmitDimensions controls only request serialization. Dimensions remains the
	// expected response width and is always used for validation.
	OmitDimensions bool
	TimeoutSeconds int
}
