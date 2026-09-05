package pagination

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		name         string
		page         int
		pageSize     int
		wantPage     int
		wantPageSize int
	}{
		{name: "defaults", wantPage: DefaultPage, wantPageSize: DefaultPageSize},
		{name: "valid", page: 3, pageSize: 50, wantPage: 3, wantPageSize: 50},
		{name: "maximum", page: 2, pageSize: MaxPageSize + 1, wantPage: 2, wantPageSize: MaxPageSize},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			page, pageSize := Normalize(test.page, test.pageSize)
			if page != test.wantPage || pageSize != test.wantPageSize {
				t.Fatalf("Normalize(%d, %d) = (%d, %d), want (%d, %d)", test.page, test.pageSize, page, pageSize, test.wantPage, test.wantPageSize)
			}
		})
	}
}

func TestParseUsesPaginationDefaultsForInvalidValues(t *testing.T) {
	page, pageSize := Parse("invalid", "-1")
	if page != DefaultPage || pageSize != DefaultPageSize {
		t.Fatalf("Parse returned (%d, %d), want (%d, %d)", page, pageSize, DefaultPage, DefaultPageSize)
	}
}

func TestOffsetSaturatesOverflow(t *testing.T) {
	maxInt := int(^uint(0) >> 1)
	offset, limit := Offset(maxInt, MaxPageSize)
	if offset != maxInt || limit != MaxPageSize {
		t.Fatalf("Offset returned (%d, %d), want (%d, %d)", offset, limit, maxInt, MaxPageSize)
	}
}
