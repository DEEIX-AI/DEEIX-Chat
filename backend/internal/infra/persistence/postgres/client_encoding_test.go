package db

import (
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestCanonicalClientEncoding(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "UTF8", want: "UTF8"},
		{in: "UTF-8", want: "UTF8"},
		{in: " utf-8 ", want: "UTF8"},
		{in: "SQL_ASCII", want: ""},
		{in: "", want: ""},
	}
	for _, tt := range tests {
		if got := canonicalClientEncoding(tt.in); got != tt.want {
			t.Fatalf("canonicalClientEncoding(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConfigureNileSessionUsesSimpleProtocol(t *testing.T) {
	cfg, err := pgx.ParseConfig("postgres://user:secret@127.0.0.1:5432/deeix")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultQueryExecMode == pgx.QueryExecModeSimpleProtocol {
		t.Fatal("ordinary postgres must not start in the simple protocol")
	}
	configureNileSession(cfg)
	if cfg.DefaultQueryExecMode != pgx.QueryExecModeSimpleProtocol {
		t.Fatalf("Nile query mode = %v", cfg.DefaultQueryExecMode)
	}
}
