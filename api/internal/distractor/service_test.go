package distractor

import "testing"

func TestValidKeyRejectsOtherObjects(t *testing.T) {
	for _, key := range []string{
		"backups/krill.dump",
		"datasets/1/dataset.zip",
		"distractors/",
		"distractors/../backups/krill.dump",
		"distractors/0123456789abcdef0123456789abcdef/x.jpg",
		"distractors/0123456789ABCDEF0123456789ABCDEF",
		"model-uploads/0123456789abcdef0123456789abcdef",
		"x/distractors/0123456789abcdef0123456789abcdef",
	} {
		if ValidKey(key) {
			t.Errorf("ValidKey(%q) = true", key)
		}
	}
	if !ValidKey("distractors/0123456789abcdef0123456789abcdef") {
		t.Error("rejected a key CreateDistractorUpload issues")
	}
}
