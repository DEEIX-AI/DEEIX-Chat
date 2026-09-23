package db

import "testing"

func TestInformationSchemaCatalogStaysOnDatabaseName(t *testing.T) {
	tests := []struct {
		name    string
		dbName  string
		catalog string
		uses    bool
	}{
		{name: "ordinary postgres", dbName: "deeix_chat", catalog: "deeix_chat", uses: false},
		{name: "equal even if flag is set", dbName: "deeix_chat", catalog: "deeix_chat", uses: true},
		{name: "different catalog unused by information_schema", dbName: "deeix_chat", catalog: "other", uses: false},
		{name: "missing catalog", dbName: "deeix_chat", catalog: "", uses: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, diverged := informationSchemaCatalog(tt.dbName, tt.catalog, tt.uses)
			if diverged || got != "" {
				t.Fatalf("informationSchemaCatalog(%q, %q, %v) = %q, %v; want ordinary database name", tt.dbName, tt.catalog, tt.uses, got, diverged)
			}
		})
	}
}

func TestInformationSchemaCatalogUsesNileCatalog(t *testing.T) {
	got, diverged := informationSchemaCatalog("deeix_pgdb", "nile", true)
	if !diverged || got != "nile" {
		t.Fatalf("got %q, %v; want nile, true", got, diverged)
	}
}
