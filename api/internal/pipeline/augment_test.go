package pipeline

import (
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

func TestCheckAugmentation(t *testing.T) {
	edges := &krillv1.Augmentation{Degrees: 180, Translate: 0.9, Scale: 0.9, Shear: 45, Perspective: 0.001, HsvH: 1, Fliplr: 1, Aspect: 2}
	if err := checkAugmentation(edges); err != nil {
		t.Fatalf("limits themselves: %v", err)
	}
	unset := &krillv1.Augmentation{}
	if err := checkAugmentation(unset); err != nil || unset.GetAspect() != 1 {
		t.Fatalf("zero aspect: err %v, aspect %v", err, unset.GetAspect())
	}
	for _, a := range []*krillv1.Augmentation{
		{Degrees: -1, Aspect: 1},
		{Perspective: 0.01, Aspect: 1},
		{Mosaic: 1.5, Aspect: 1},
		{Aspect: 0.5},
		{Aspect: 3},
	} {
		if checkAugmentation(a) == nil {
			t.Errorf("accepted %v", a)
		}
	}
}
