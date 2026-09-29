package config

import (
	"reflect"
	"testing"
)

func TestCapabilitiesServerModeEnablesEverything(t *testing.T) {
	cfg := Config{}
	for name, enabled := range cfg.Capabilities().Flags() {
		if !enabled {
			t.Errorf("server mode: %s should be enabled", name)
		}
	}
}

func TestCapabilitiesLocalModeKeepsOnlyMetering(t *testing.T) {
	cfg := Config{LocalMode: true}
	want := map[string]bool{
		"multiUser":         false,
		"registration":      false,
		"identityProviders": false,
		"accountSecurity":   false,
		"announcements":     false,
		"billingGating":     false,
		"usageMetering":     true,
		"contentModeration": false,
		"sharing":           false,
	}
	if got := cfg.Capabilities().Flags(); !reflect.DeepEqual(got, want) {
		t.Fatalf("local mode capabilities = %v, want %v", got, want)
	}
}

// Enabled 的 switch 与 JSON 键名是两份清单；任一方增删而另一方没有跟上都在这里暴露。
func TestCapabilitiesEnabledMatchesEveryFlag(t *testing.T) {
	for _, cfg := range []Config{{}, {LocalMode: true}} {
		caps := cfg.Capabilities()
		flags := caps.Flags()
		if len(flags) == 0 {
			t.Fatal("no flags")
		}
		for name, enabled := range flags {
			if caps.Enabled(name) != enabled {
				t.Errorf("Enabled(%q) = %v, flag says %v", name, caps.Enabled(name), enabled)
			}
		}
	}
	// 每个键都必须能被 Enabled 区分：把它单独置真，其余为假。
	typ := reflect.TypeOf(Capabilities{})
	for i := 0; i < typ.NumField(); i++ {
		name := typ.Field(i).Tag.Get("json")
		var caps Capabilities
		reflect.ValueOf(&caps).Elem().Field(i).SetBool(true)
		if !caps.Enabled(name) {
			t.Errorf("Enabled(%q) does not read field %s", name, typ.Field(i).Name)
		}
	}
}

func TestCapabilitiesEnabledRejectsUnknownName(t *testing.T) {
	cfg := Config{}
	if cfg.Capabilities().Enabled("multiuser") {
		t.Fatal("unknown (miscased) name must not be enabled")
	}
}
