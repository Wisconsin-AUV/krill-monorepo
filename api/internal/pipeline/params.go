package pipeline

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"

	"google.golang.org/protobuf/encoding/protojson"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

var pluginName = regexp.MustCompile(`^[a-z][a-z0-9_]{0,39}$`)

// checkSpec rejects a plugin a worker offers when the panel couldn't show or
// save it.
func checkSpec(s *krillv1.PrelabelerSpec) error {
	if !pluginName.MatchString(s.GetName()) {
		return fmt.Errorf("pre-labeler name %q must be lowercase letters, digits, or underscores", s.GetName())
	}
	if len(s.GetOutputs()) == 0 {
		return fmt.Errorf("pre-labeler %s has no outputs", s.GetName())
	}
	seen := map[string]bool{}
	for _, o := range s.GetOutputs() {
		if !pluginName.MatchString(o) || seen[o] {
			return fmt.Errorf("pre-labeler %s has a bad or repeated output %q", s.GetName(), o)
		}
		seen[o] = true
	}
	seen = map[string]bool{}
	for _, ps := range s.GetParams() {
		if !pluginName.MatchString(ps.GetName()) || seen[ps.GetName()] {
			return fmt.Errorf("pre-labeler %s has a bad or repeated param %q", s.GetName(), ps.GetName())
		}
		seen[ps.GetName()] = true
		if err := checkParam(ps, ps.GetDefaultValue()); err != nil {
			return fmt.Errorf("pre-labeler %s default: %w", s.GetName(), err)
		}
	}
	return nil
}

// checkParam rejects a value of the wrong kind or outside the spec's bounds.
func checkParam(s *krillv1.ParamSpec, v *krillv1.ParamValue) error {
	name := s.GetLabel()
	if name == "" {
		name = s.GetName()
	}
	switch s.GetKind() {
	case krillv1.ParamKind_PARAM_KIND_INT, krillv1.ParamKind_PARAM_KIND_FLOAT:
		n, ok := v.GetValue().(*krillv1.ParamValue_Number)
		if !ok {
			return fmt.Errorf("%s must be a number", name)
		}
		if math.IsNaN(n.Number) || math.IsInf(n.Number, 0) {
			return fmt.Errorf("%s must be a number", name)
		}
		if s.GetKind() == krillv1.ParamKind_PARAM_KIND_INT && n.Number != math.Trunc(n.Number) {
			return fmt.Errorf("%s must be a whole number", name)
		}
		if s.GetMax() > s.GetMin() && (n.Number < s.GetMin() || n.Number > s.GetMax()) {
			return fmt.Errorf("%s must be between %g and %g", name, s.GetMin(), s.GetMax())
		}
	case krillv1.ParamKind_PARAM_KIND_BOOL:
		if _, ok := v.GetValue().(*krillv1.ParamValue_Flag); !ok {
			return fmt.Errorf("%s must be on or off", name)
		}
	case krillv1.ParamKind_PARAM_KIND_HSV_RANGE:
		r := v.GetHsv()
		if r == nil {
			return fmt.Errorf("%s must be an HSV range", name)
		}
		for _, h := range []int32{r.GetHMin(), r.GetHMax()} {
			if h < 0 || h > 179 {
				return fmt.Errorf("%s hue must be between 0 and 179", name)
			}
		}
		for _, c := range []int32{r.GetSMin(), r.GetSMax(), r.GetVMin(), r.GetVMax()} {
			if c < 0 || c > 255 {
				return fmt.Errorf("%s saturation and value must be between 0 and 255", name)
			}
		}
		if r.GetSMin() > r.GetSMax() || r.GetVMin() > r.GetVMax() {
			return fmt.Errorf("%s minimum saturation and value must not be above the maximum", name)
		}
	default:
		return fmt.Errorf("%s has an unknown kind", name)
	}
	return nil
}

// resolveParams checks values against the spec and returns every param in
// spec order, with the default for any not given.
func resolveParams(spec *krillv1.PrelabelerSpec, values []*krillv1.ParamValue) ([]*krillv1.ParamValue, error) {
	given := map[string]*krillv1.ParamValue{}
	for _, v := range values {
		if _, dup := given[v.GetName()]; dup {
			return nil, fmt.Errorf("%s is given twice", v.GetName())
		}
		given[v.GetName()] = v
	}
	out := make([]*krillv1.ParamValue, 0, len(spec.GetParams()))
	for _, ps := range spec.GetParams() {
		v, ok := given[ps.GetName()]
		delete(given, ps.GetName())
		if !ok {
			v = ps.GetDefaultValue()
		} else if err := checkParam(ps, v); err != nil {
			return nil, err
		}
		out = append(out, &krillv1.ParamValue{Name: ps.GetName(), Value: v.GetValue()})
	}
	for name := range given {
		return nil, fmt.Errorf("%s is not a param of %s", name, spec.GetName())
	}
	return out, nil
}

// savedParams decodes the params column. Saved values that no longer fit the
// spec fall back to its default, so a worker update never breaks a plugin.
func savedParams(spec *krillv1.PrelabelerSpec, raw []byte) ([]*krillv1.ParamValue, error) {
	var saved map[string]json.RawMessage
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, fmt.Errorf("decode params: %w", err)
	}
	out := make([]*krillv1.ParamValue, 0, len(spec.GetParams()))
	for _, ps := range spec.GetParams() {
		v := &krillv1.ParamValue{}
		if r, ok := saved[ps.GetName()]; !ok || protojson.Unmarshal(r, v) != nil || checkParam(ps, v) != nil {
			v = ps.GetDefaultValue()
		}
		out = append(out, &krillv1.ParamValue{Name: ps.GetName(), Value: v.GetValue()})
	}
	return out, nil
}

func encodeParams(values []*krillv1.ParamValue) ([]byte, error) {
	m := make(map[string]json.RawMessage, len(values))
	for _, v := range values {
		b, err := protojson.Marshal(v)
		if err != nil {
			return nil, err
		}
		m[v.GetName()] = b
	}
	return json.Marshal(m)
}

type savedOutput struct {
	LabelTypeID int64             `json:"label_type_id"`
	Attributes  map[string]string `json:"attributes"`
}

func savedOutputs(spec *krillv1.PrelabelerSpec, raw []byte) ([]*krillv1.PrelabelerOutput, error) {
	var saved map[string]savedOutput
	if err := json.Unmarshal(raw, &saved); err != nil {
		return nil, fmt.Errorf("decode outputs: %w", err)
	}
	out := make([]*krillv1.PrelabelerOutput, len(spec.GetOutputs()))
	for i, name := range spec.GetOutputs() {
		o := saved[name]
		out[i] = &krillv1.PrelabelerOutput{Name: name, LabelTypeId: o.LabelTypeID, Attributes: o.Attributes}
	}
	return out, nil
}

func prelabeler(r db.Prelabeler, updatedBy string) (*krillv1.Prelabeler, error) {
	spec := &krillv1.PrelabelerSpec{}
	if err := protojson.Unmarshal(r.Spec, spec); err != nil {
		return nil, fmt.Errorf("decode %s spec: %w", r.Name, err)
	}
	params, err := savedParams(spec, r.Params)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.Name, err)
	}
	outputs, err := savedOutputs(spec, r.Outputs)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", r.Name, err)
	}
	return &krillv1.Prelabeler{
		Spec:      spec,
		Enabled:   r.Enabled,
		Params:    params,
		Outputs:   outputs,
		UpdatedAt: ts(r.UpdatedAt),
		UpdatedBy: updatedBy,
	}, nil
}

var errNoOutput = errors.New("enable needs at least one output mapped to a label type")
