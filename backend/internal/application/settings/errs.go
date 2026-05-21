package settings

import "errors"

// ErrInvalidSetting marks validation failures for system setting updates.
var ErrInvalidSetting = errors.New("invalid setting")
