package ingest

import (
	"reflect"
	"testing"
)

func TestChunkClips(t *testing.T) {
	tests := []struct {
		name  string
		total int
		fps   float64
		want  []clipRange
	}{
		{"empty", 0, 30, nil},
		{"shorter than one clip", 100, 30, []clipRange{{0, 100}}},
		{"exact multiple", 900, 30, []clipRange{{0, 450}, {450, 450}}},
		{"long tail kept", 1100, 30, []clipRange{{0, 450}, {450, 450}, {900, 200}}},
		{"short tail merged", 1000, 30, []clipRange{{0, 450}, {450, 550}}},
		{"low fps", 40, 2, []clipRange{{0, 30}, {30, 10}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := chunkClips(tt.total, tt.fps); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("chunkClips(%d, %v) = %v, want %v", tt.total, tt.fps, got, tt.want)
			}
		})
	}
}
