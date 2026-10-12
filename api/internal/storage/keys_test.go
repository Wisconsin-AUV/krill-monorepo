package storage

import (
	"strings"
	"testing"
)

// Deleting a video removes VideoPrefix and re-ingesting removes
// EmbeddingsPrefix, so an embedding outside either would outlive its frames.
func TestEmbeddingKey(t *testing.T) {
	key := EmbeddingKey(12, 7)
	if want := "videos/12/embeddings/000007.pt"; key != want {
		t.Errorf("key = %q, want %q", key, want)
	}
	if !strings.HasPrefix(key, VideoPrefix(12)) || !strings.HasPrefix(key, EmbeddingsPrefix(12)) {
		t.Errorf("%q is outside the video's prefixes", key)
	}
	if strings.HasPrefix(key, FramesPrefix(12)) {
		t.Errorf("%q is under the frames prefix", key)
	}
	if strings.HasPrefix(EmbeddingKey(1, 0), VideoPrefix(12)) || strings.HasPrefix(EmbeddingKey(123, 0), VideoPrefix(12)) {
		t.Error("another video's embedding is under this video's prefix")
	}
}
