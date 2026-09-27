package ingest

import "testing"

func TestParseRate(t *testing.T) {
	tests := []struct {
		in   string
		want float64
	}{
		{"30/1", 30},
		{"30000/1001", 30000.0 / 1001},
		{"25", 25},
		{"0/0", 0},
		{"", 0},
	}
	for _, tt := range tests {
		if got := parseRate(tt.in); got != tt.want {
			t.Errorf("parseRate(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
