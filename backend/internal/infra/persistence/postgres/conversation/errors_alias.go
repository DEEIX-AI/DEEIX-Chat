package conversation

import "errors"

var (
	ErrFileNotFound         = errors.New("file not found")
	ErrStorageQuotaExceeded = errors.New("storage quota exceeded")
)
