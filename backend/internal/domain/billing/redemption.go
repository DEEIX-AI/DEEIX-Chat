package billing

import "strings"

// RedemptionCodeModeAvailableInBillingMode 判断兑换码模式是否可在当前计费模式下使用。
func RedemptionCodeModeAvailableInBillingMode(codeMode string, billingMode string) bool {
	switch strings.TrimSpace(billingMode) {
	case RedemptionCodeModeUsage:
		return strings.TrimSpace(codeMode) == RedemptionCodeModeUsage
	case RedemptionCodeModePeriod:
		switch strings.TrimSpace(codeMode) {
		case RedemptionCodeModeUsage, RedemptionCodeModePeriod:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// RedemptionCodeModesAvailableInBillingMode 返回当前计费模式下可用的兑换码模式集合。
func RedemptionCodeModesAvailableInBillingMode(billingMode string) []string {
	switch strings.TrimSpace(billingMode) {
	case RedemptionCodeModeUsage:
		return []string{RedemptionCodeModeUsage}
	case RedemptionCodeModePeriod:
		return []string{RedemptionCodeModeUsage, RedemptionCodeModePeriod}
	default:
		return nil
	}
}
