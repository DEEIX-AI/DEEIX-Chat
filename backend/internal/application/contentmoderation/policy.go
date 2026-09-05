package contentmoderation

import (
	"sort"
	"strings"
)

// Policy holds the four category arrays (empty array = skip that surface).
type Policy struct {
	InputTextCategories   []string
	OutputTextCategories  []string
	InputImageCategories  []string
	OutputImageCategories []string
	Version               int64
}

type policyJSON struct {
	InputTextCategories   []string `json:"inputTextCategories"`
	OutputTextCategories  []string `json:"outputTextCategories"`
	InputImageCategories  []string `json:"inputImageCategories"`
	OutputImageCategories []string `json:"outputImageCategories"`
	Version               int64    `json:"version"`
}

func newPolicyJSON(policy Policy) policyJSON {
	return policyJSON(policy)
}

func (document policyJSON) toPolicy() Policy {
	return Policy(document)
}

// Enabled reports whether any surface has categories selected.
func (p Policy) Enabled() bool {
	return len(p.InputTextCategories) > 0 ||
		len(p.OutputTextCategories) > 0 ||
		len(p.InputImageCategories) > 0 ||
		len(p.OutputImageCategories) > 0
}

// CategoriesFor returns selected categories for a direction+modality surface.
func (p Policy) CategoriesFor(direction, modality string) []string {
	switch {
	case direction == DirectionInput && modality == ModalityText:
		return append([]string(nil), p.InputTextCategories...)
	case direction == DirectionOutput && modality == ModalityText:
		return append([]string(nil), p.OutputTextCategories...)
	case direction == DirectionInput && modality == ModalityImage:
		return append([]string(nil), p.InputImageCategories...)
	case direction == DirectionOutput && modality == ModalityImage:
		return append([]string(nil), p.OutputImageCategories...)
	default:
		return nil
	}
}

// NormalizePolicy validates and normalizes category selections.
func NormalizePolicy(p Policy) (Policy, error) {
	out := Policy{Version: p.Version}
	var err error
	if out.InputTextCategories, err = normalizeTextCategories(p.InputTextCategories); err != nil {
		return Policy{}, err
	}
	if out.OutputTextCategories, err = normalizeTextCategories(p.OutputTextCategories); err != nil {
		return Policy{}, err
	}
	if out.InputImageCategories, err = normalizeImageCategories(p.InputImageCategories); err != nil {
		return Policy{}, err
	}
	if out.OutputImageCategories, err = normalizeImageCategories(p.OutputImageCategories); err != nil {
		return Policy{}, err
	}
	return out, nil
}

func normalizeTextCategories(raw []string) ([]string, error) {
	return normalizeCategories(raw, false)
}

func normalizeImageCategories(raw []string) ([]string, error) {
	return normalizeCategories(raw, true)
}

func normalizeCategories(raw []string, imageOnly bool) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		cat := strings.TrimSpace(item)
		if cat == "" {
			continue
		}
		if !IsKnownCategory(cat) {
			return nil, ErrInvalidCategories
		}
		if imageOnly {
			if IsTextOnlyCategory(cat) || !IsImageCategory(cat) {
				return nil, ErrImageTextOnlyCategory
			}
		}
		if _, ok := seen[cat]; ok {
			continue
		}
		seen[cat] = struct{}{}
		out = append(out, cat)
	}
	sort.Strings(out)
	return out, nil
}
