package db

import "testing"

func TestInstalledVectorExtension(t *testing.T) {
	if err := installedVectorExtension("0.8.2"); err != nil {
		t.Fatalf("installed pgvector was rejected: %v", err)
	}
	if err := installedVectorExtension("  "); err == nil {
		t.Fatal("missing pgvector must stay an error")
	}
}
