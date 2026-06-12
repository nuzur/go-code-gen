package templatefuncs

import "testing"

func TestInc(t *testing.T) {
	tests := []struct {
		input    int
		expected int
	}{
		{0, 1},
		{5, 6},
		{-1, 0},
	}

	for _, tt := range tests {
		actual := Inc(tt.input)
		if actual != tt.expected {
			t.Errorf("Inc(%d) = %d; expected %d", tt.input, actual, tt.expected)
		}
	}
}
