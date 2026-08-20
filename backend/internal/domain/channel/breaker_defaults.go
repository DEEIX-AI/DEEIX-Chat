package channel

import (
	"strings"
)

// BreakerDefaults 熔断器全局默认参数（来自 circuit_breaker.defaults 全局设置）。
type BreakerDefaults struct {
	Enabled                  bool
	ModelFailureThreshold    int
	ModelDurationMin         int
	ModelWindowMin           int
	UpstreamFailureThreshold int
	UpstreamModelThreshold   int
	UpstreamThresholdLogic   string
	UpstreamDurationMin      int
	UpstreamWindowMin        int
}

// DefaultBreakerDefaults 返回熔断器内置默认参数；全局开关默认关闭。
func DefaultBreakerDefaults() BreakerDefaults {
	return BreakerDefaults{
		ModelFailureThreshold:    5,
		ModelDurationMin:         15,
		ModelWindowMin:           3,
		UpstreamFailureThreshold: 20,
		UpstreamModelThreshold:   3,
		UpstreamThresholdLogic:   "or",
		UpstreamDurationMin:      30,
		UpstreamWindowMin:        5,
	}
}

// Valid 校验熔断默认参数是否能安全用于计数和分钟到秒的换算；0 表示沿用内置默认值。
func (d BreakerDefaults) Valid() bool {
	for _, value := range []int{
		d.ModelFailureThreshold,
		d.ModelDurationMin,
		d.ModelWindowMin,
		d.UpstreamFailureThreshold,
		d.UpstreamModelThreshold,
		d.UpstreamDurationMin,
		d.UpstreamWindowMin,
	} {
		if value < 0 {
			return false
		}
	}
	maxMinutes := int(^uint(0)>>1) / 60
	for _, value := range []int{
		d.ModelDurationMin,
		d.ModelWindowMin,
		d.UpstreamDurationMin,
		d.UpstreamWindowMin,
	} {
		if value > maxMinutes {
			return false
		}
	}
	logic := strings.TrimSpace(d.UpstreamThresholdLogic)
	return logic == "" || logic == "and" || logic == "or"
}
