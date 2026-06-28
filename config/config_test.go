package config

import (
	"testing"
)

func TestParseSize(testingT *testing.T) {
	tests := []struct {
		input    string
		expected int64
		hasError bool
	}{
		{"100G", 100 * 1024 * 1024 * 1024, false},
		{"500M", 500 * 1024 * 1024, false},
		{"10K", 10 * 1024, false},
		{"100B", 100, false},
		{"50", 50, false},
		{" 2 G ", 2 * 1024 * 1024 * 1024, false},
		{"1.5M", 1572864, false},
		{"", 0, true},
		{"invalid", 0, true},
		{"10X", 0, true},
	}

	for _, tc := range tests {
		testingT.Run(tc.input, func(t *testing.T) {
			res, err := ParseSize(tc.input)
			if tc.hasError {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tc.input)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error for input %q, got: %v", tc.input, err)
				}
				if res != tc.expected {
					t.Errorf("expected %d for input %q, got %d", tc.expected, tc.input, res)
				}
			}
		})
	}
}
