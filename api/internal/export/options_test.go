package export

import (
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestNormalizeAugmentation(t *testing.T) {
	edges := &krillv1.Augmentation{Degrees: 180, Translate: 0.9, Scale: 0.9, Shear: 45, Perspective: 0.001, HsvH: 1, Fliplr: 1, Aspect: 2, Distractors: 1}
	if _, err := NormalizeAugmentation(edges); err != nil {
		t.Fatalf("limits themselves: %v", err)
	}
	unset, err := NormalizeAugmentation(&krillv1.Augmentation{})
	if err != nil || unset.GetAspect() != 1 {
		t.Fatalf("zero aspect: err %v, aspect %v", err, unset.GetAspect())
	}
	for _, a := range []*krillv1.Augmentation{
		{Degrees: -1, Aspect: 1},
		{Perspective: 0.01, Aspect: 1},
		{Mosaic: 1.5, Aspect: 1},
		{Distractors: 1.5, Aspect: 1},
		{Aspect: 0.5},
		{Aspect: 3},
	} {
		if _, err := NormalizeAugmentation(a); err == nil {
			t.Errorf("accepted %v", a)
		}
	}
}

func TestNormalizeAugmentedCopies(t *testing.T) {
	for copies, ok := range map[int32]bool{0: true, 10: true, -1: false, 11: false} {
		opts, err := Normalize(&krillv1.ExportOptions{AugmentedCopies: copies})
		if (err == nil) != ok {
			t.Errorf("copies %d: err = %v", copies, err)
		}
		if ok && opts.GetAugmentation().GetHsvS() == 0 {
			t.Errorf("copies %d: augmentation not defaulted", copies)
		}
	}
	if _, err := Normalize(&krillv1.ExportOptions{AugmentedCopies: 1, Augmentation: &krillv1.Augmentation{Fliplr: 2}}); err == nil {
		t.Error("accepted an out-of-range augmentation")
	}
}
