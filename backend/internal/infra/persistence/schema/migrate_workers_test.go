package schema

import "testing"

func TestAutoMigrateWorkers(t *testing.T) {
	if got := autoMigrateWorkers(1, 60); got != 1 {
		t.Fatalf("serial request = %d", got)
	}
	if got := autoMigrateWorkers(8, 60); got != 8 {
		t.Fatalf("capped workers = %d", got)
	}
	if got := autoMigrateWorkers(8, 3); got != 3 {
		t.Fatalf("workers above model count = %d", got)
	}
	if got := autoMigrateWorkers(0, 10); got != 1 {
		t.Fatalf("non-positive workers = %d", got)
	}
}
