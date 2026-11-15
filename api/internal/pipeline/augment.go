package pipeline

import (
	"fmt"
	"math"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// defaultAugmentation keeps Ultralytics' defaults except for mirroring, and
// rotates a little since the sub rolls and signs lie at any angle.
func defaultAugmentation() *krillv1.Augmentation {
	return &krillv1.Augmentation{
		Degrees:    10,
		Translate:  0.1,
		Scale:      0.5,
		HsvH:       0.015,
		HsvS:       0.7,
		HsvV:       0.4,
		Mosaic:     1,
		Aspect:     1.2,
		Underwater: 0.3,
		Shadows:    0.3,
		Noise:      0.2,
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
}

// checkAugmentation rejects out-of-range values. An aspect of 0 means no
// stretch, since the field can't be told apart from unset.
func checkAugmentation(a *krillv1.Augmentation) error {
	if a.GetAspect() == 0 {
		a.Aspect = 1
	}
	for _, l := range augmentLimits {
		v := l.get(a)
		if math.IsNaN(float64(v)) || v < l.min || v > l.max {
			return fmt.Errorf("%s must be between %g and %g", l.name, l.min, l.max)
		}
	}
	return nil
}
