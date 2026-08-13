package contentmoderation

import domaincm "github.com/DEEIX-AI/DEEIX-Chat/backend/internal/domain/contentmoderation"

// Direction / modality aliases for application code.
const (
	DirectionInput  = domaincm.DirectionInput
	DirectionOutput = domaincm.DirectionOutput
	ModalityText    = domaincm.ModalityText
	ModalityImage   = domaincm.ModalityImage
)

// Official Omni Moderation categories (13 total).
var allTextCategories = []string{
	"hate",
	"hate/threatening",
	"harassment",
	"harassment/threatening",
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"sexual",
	"sexual/minors",
	"violence",
	"violence/graphic",
	"illicit",
	"illicit/violent",
}

// Image-applicable categories (excludes official text-only categories).
var imageCategories = []string{
	"self-harm",
	"self-harm/intent",
	"self-harm/instructions",
	"sexual",
	"violence",
	"violence/graphic",
}

// textOnlyCategories are not shown/configurable on image policy pages.
var textOnlyCategories = map[string]struct{}{
	"hate":                   {},
	"hate/threatening":       {},
	"harassment":             {},
	"harassment/threatening": {},
	"sexual/minors":          {},
	"illicit":                {},
	"illicit/violent":        {},
}

// AllTextCategories returns the full official set for admin UI defaults.
func AllTextCategories() []string {
	return append([]string(nil), allTextCategories...)
}

// ImageCategories returns categories valid for image moderation.
func ImageCategories() []string {
	return append([]string(nil), imageCategories...)
}

// IsTextOnlyCategory reports whether a category is text-only.
func IsTextOnlyCategory(category string) bool {
	_, ok := textOnlyCategories[category]
	return ok
}

// IsKnownCategory reports whether category is in the official list.
func IsKnownCategory(category string) bool {
	for _, item := range allTextCategories {
		if item == category {
			return true
		}
	}
	return false
}

// IsImageCategory reports whether category applies to images.
func IsImageCategory(category string) bool {
	for _, item := range imageCategories {
		if item == category {
			return true
		}
	}
	return false
}
