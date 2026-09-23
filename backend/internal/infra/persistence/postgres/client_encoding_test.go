package db

import "testing"

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
