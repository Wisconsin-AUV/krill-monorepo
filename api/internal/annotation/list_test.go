package annotation

import "testing"

func TestParsePage(t *testing.T) {
	tests := []struct {
		name      string
		size      int32
		token     string
		wantSize  int32
		wantAfter int64
		wantErr   bool
	}{
		{"defaults", 0, "", defaultPageSize, 0, false},
		{"clamped", 10_000, "", maxPageSize, 0, false},
		{"negative size", -1, "", 0, 0, true},
		{"next page", 20, "42", 20, 42, false},
		{"garbage token", 20, "abc", 0, 0, true},
		{"zero token", 20, "0", 0, 0, true},
		{"negative token", 20, "-5", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			size, after, err := parsePage(tt.size, tt.token)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if size != tt.wantSize || after != tt.wantAfter {
				t.Errorf("parsePage = %d, %d, want %d, %d", size, after, tt.wantSize, tt.wantAfter)
			}
		})
	}
}
