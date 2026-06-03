package buildinfo

import (
	"os"
	"strings"
	"sync"
)

// Version is injected from the repository-level VERSION file at build time.
var Version = "dev"

// Commit is injected from the current git commit at build time.
var Commit = "unknown"

// BuildTime is injected as an RFC3339 UTC timestamp at build time.
var BuildTime = "unknown"

var (
	versionOnce  sync.Once
	versionValue string
)

type Info struct {
	Product   string `json:"product"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildTime string `json:"buildTime"`
	BuildID   string `json:"buildID"`
}

func Snapshot() Info {
	return Info{
		Product:   "DEEIX Chat",
		Version:   ResolveVersion(),
		Commit:    Commit,
		BuildTime: BuildTime,
		BuildID:   ResolveBuildID(),
	}
}

func ResolveBuildID() string {
	parts := []string{ResolveVersion()}
	for _, value := range []string{Commit, BuildTime} {
		normalized := strings.TrimSpace(value)
		if normalized == "" || normalized == "unknown" || normalized == "dev" {
			continue
		}
		parts = append(parts, normalized)
	}
	return strings.Join(parts, "-")
}

func ResolveVersion() string {
	versionOnce.Do(func() {
		if normalized := strings.TrimSpace(Version); normalized != "" && normalized != "dev" {
			versionValue = normalized
			return
		}
		for _, path := range []string{"VERSION", "../VERSION", "../../VERSION"} {
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			if normalized := strings.TrimSpace(string(content)); normalized != "" {
				versionValue = normalized
				return
			}
		}
		versionValue = strings.TrimSpace(Version)
		if versionValue == "" {
			versionValue = "dev"
		}
	})
	return versionValue
}
