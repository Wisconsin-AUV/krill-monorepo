package export

import (
	"fmt"
	"math"

	"google.golang.org/protobuf/proto"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

const maxAugmentedCopies = 10

// DefaultAugmentation keeps Ultralytics' defaults except for mirroring, and
// varies saturation less so classes told apart by color stay apart.
func DefaultAugmentation() *krillv1.Augmentation {
	return &krillv1.Augmentation{
		Translate:   0.1,
		Scale:       0.5,
		HsvH:        0.015,
		HsvS:        0.4,
		HsvV:        0.4,
		Mosaic:      1,
		Aspect:      1.2,
		Underwater:  0.3,
		Shadows:     0.3,
		Noise:       0.2,
		Distractors: 0.2,
	}
}

type augmentLimit struct {
	name     string
	get      func(*krillv1.Augmentation) float32
	min, max float32
}

var augmentLimits = []augmentLimit{
	{"degrees", (*krillv1.Augmentation).GetDegrees, 0, 180},
	{"translate", (*krillv1.Augmentation).GetTranslate, 0, 0.9},
	{"scale", (*krillv1.Augmentation).GetScale, 0, 0.9},
	{"shear", (*krillv1.Augmentation).GetShear, 0, 45},
	// Past 0.001 Ultralytics' perspective warps most objects out of the frame.
	{"perspective", (*krillv1.Augmentation).GetPerspective, 0, 0.001},
	{"hue", (*krillv1.Augmentation).GetHsvH, 0, 1},
	{"saturation", (*krillv1.Augmentation).GetHsvS, 0, 1},
	{"brightness", (*krillv1.Augmentation).GetHsvV, 0, 1},
	{"horizontal flip", (*krillv1.Augmentation).GetFliplr, 0, 1},
	{"vertical flip", (*krillv1.Augmentation).GetFlipud, 0, 1},
	{"mosaic", (*krillv1.Augmentation).GetMosaic, 0, 1},
	{"aspect", (*krillv1.Augmentation).GetAspect, 1, 2},
	{"underwater", (*krillv1.Augmentation).GetUnderwater, 0, 1},
	{"shadows", (*krillv1.Augmentation).GetShadows, 0, 1},
	{"noise", (*krillv1.Augmentation).GetNoise, 0, 1},
	{"distractors", (*krillv1.Augmentation).GetDistractors, 0, 1},
}

// NormalizeAugmentation returns the defaults for an unset augmentation, and
// otherwise a copy, rejecting out-of-range values. An aspect of 0 means no
// stretch, since the field can't be told apart from unset.
func NormalizeAugmentation(in *krillv1.Augmentation) (*krillv1.Augmentation, error) {
	if in == nil {
		return DefaultAugmentation(), nil
	}
	a := proto.Clone(in).(*krillv1.Augmentation)
	if a.GetAspect() == 0 {
		a.Aspect = 1
	}
	for _, l := range augmentLimits {
		v := l.get(a)
		if math.IsNaN(float64(v)) || v < l.min || v > l.max {
			return nil, fmt.Errorf("%s must be between %g and %g", l.name, l.min, l.max)
		}
	}
	return a, nil
}
