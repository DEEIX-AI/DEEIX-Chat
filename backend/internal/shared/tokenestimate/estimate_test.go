package tokenestimate

import "testing"

func TestEstimateContract(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		want    int64
	}{
		{name: "empty", content: "", want: 0},
		{name: "ascii rounds by four", content: "abcde", want: 2},
		{name: "cjk rounds by one and a half", content: "中文日", want: 2},
		{name: "mixed buckets round independently", content: "中文abcde", want: 4},
		{name: "cjk punctuation uses cjk weight", content: "。、", want: 2},
		{name: "bopomofo uses cjk weight", content: "ㄅㄆㄇ", want: 2},
		{name: "extension b uses cjk weight", content: "𠀀𠀁𠀂", want: 2},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := Estimate(test.content); got != test.want {
				t.Fatalf("Estimate(%q) = %d, want %d", test.content, got, test.want)
			}
		})
	}
}
