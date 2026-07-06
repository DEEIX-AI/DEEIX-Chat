package billing

import (
	"context"
	"strings"
)

// OfficialPricingSuggestion is a matched pricing entry for a platform model.
type OfficialPricingSuggestion struct {
	PlatformModelName       string
	InputUSDPerMTokens      float64
	OutputUSDPerMTokens     float64
	CacheReadUSDPerMTokens  float64
	CacheWriteUSDPerMTokens float64
}

// FetchOfficialPricingSuggestions fetches live pricing from models.dev and
// returns suggestions for active platform models that have no pricing yet.
func (s *Service) FetchOfficialPricingSuggestions(ctx context.Context) ([]OfficialPricingSuggestion, error) {
	validNames, err := s.activePlatformModelNames(ctx)
	if err != nil {
		return nil, err
	}
	if len(validNames) == 0 {
		return nil, nil
	}

	existingPricing, _, err := s.repo.ListModelPricing(ctx, "", 0, 5000)
	if err != nil {
		return nil, err
	}
	alreadyPriced := make(map[string]struct{}, len(existingPricing))
	for _, item := range existingPricing {
		alreadyPriced[strings.TrimSpace(item.PlatformModelName)] = struct{}{}
	}

	vendorByModel := s.buildVendorByModelMap(ctx)

	pricingByVendor, err := FetchModelsDevPricing(ctx)
	if err != nil {
		return nil, err
	}

	var suggestions []OfficialPricingSuggestion
	for name := range validNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		if _, ok := alreadyPriced[name]; ok {
			continue
		}
		vendor := vendorByModel[name]
		entry := lookupPricing(pricingByVendor, name, vendor)
		if entry == nil {
			continue
		}
		suggestions = append(suggestions, OfficialPricingSuggestion{
			PlatformModelName:       name,
			InputUSDPerMTokens:      entry.InputUSDPerMTokens,
			OutputUSDPerMTokens:     entry.OutputUSDPerMTokens,
			CacheReadUSDPerMTokens:  entry.CacheReadUSDPerMTokens,
			CacheWriteUSDPerMTokens: entry.CacheWriteUSDPerMTokens,
		})
	}
	return suggestions, nil
}

func lookupPricing(byVendor map[string]map[string]OfficialPricingEntry, modelName string, vendor string) *OfficialPricingEntry {
	if vendor != "" {
		if vendorModels, ok := byVendor[vendor]; ok {
			if entry, ok := vendorModels[modelName]; ok {
				return &entry
			}
		}
	}
	for _, vendorModels := range byVendor {
		if entry, ok := vendorModels[modelName]; ok {
			return &entry
		}
	}
	return nil
}

// buildVendorByModelMap returns a map from platform model name to vendor.
func (s *Service) buildVendorByModelMap(ctx context.Context) map[string]string {
	if s.platformModelIdentityResolver == nil {
		return nil
	}
	names, err := s.activePlatformModelNames(ctx)
	if err != nil || names == nil {
		return nil
	}
	result := make(map[string]string, len(names))
	for name := range names {
		identity, err := s.resolvePlatformModelIdentity(ctx, name)
		if err != nil {
			continue
		}
		result[name] = strings.ToLower(strings.TrimSpace(identity.ModelVendor))
	}
	return result
}
