// Package runtime defines contracts shared by application services and
// infrastructure adapters for managed runtime dependencies.
package runtime

import "errors"

// ErrContainerNotFound indicates that a requested managed container does not
// exist. The Docker adapter is responsible for translating engine-specific
// exit output into this stable infrastructure error.
var ErrContainerNotFound = errors.New("runtime container not found")
