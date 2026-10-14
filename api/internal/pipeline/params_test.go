package pipeline

import (
	"slices"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/export"
	"github.com/wauv/krill/api/internal/taxonomy"
)

func number(name string, v float64) *krillv1.ParamValue {
	return &krillv1.ParamValue{Name: name, Value: &krillv1.ParamValue_Number{Number: v}}
}

func hsv(name string, r *krillv1.HsvRange) *krillv1.ParamValue {
	return &krillv1.ParamValue{Name: name, Value: &krillv1.ParamValue_Hsv{Hsv: r}}
}

var testSpec = &krillv1.PrelabelerSpec{
	Name:    "slalom",
	Outputs: []string{"red", "white"},
	Params: []*krillv1.ParamSpec{
		{Name: "red", Kind: krillv1.ParamKind_PARAM_KIND_HSV_RANGE, DefaultValue: hsv("red", &krillv1.HsvRange{HMin: 170, HMax: 10, SMax: 255, VMax: 255})},
		{Name: "kernel", Kind: krillv1.ParamKind_PARAM_KIND_INT, Min: 1, Max: 31, DefaultValue: number("kernel", 5)},
		{Name: "min_area", Kind: krillv1.ParamKind_PARAM_KIND_FLOAT, Min: 0, Max: 1, DefaultValue: number("min_area", 0.001)},
	},
}

func TestResolveParams(t *testing.T) {
	tests := []struct {
		name    string
		values  []*krillv1.ParamValue
		wantErr bool
	}{
		{name: "defaults"},
		{name: "wrapping hue", values: []*krillv1.ParamValue{hsv("red", &krillv1.HsvRange{HMin: 175, HMax: 5, SMax: 255, VMax: 255})}},
		{name: "hue past 179", values: []*krillv1.ParamValue{hsv("red", &krillv1.HsvRange{HMax: 180, SMax: 255, VMax: 255})}, wantErr: true},
		{name: "saturation backwards", values: []*krillv1.ParamValue{hsv("red", &krillv1.HsvRange{SMin: 200, SMax: 100, VMax: 255})}, wantErr: true},
		{name: "fractional int", values: []*krillv1.ParamValue{number("kernel", 2.5)}, wantErr: true},
		{name: "out of bounds", values: []*krillv1.ParamValue{number("kernel", 99)}, wantErr: true},
		{name: "wrong kind", values: []*krillv1.ParamValue{number("red", 3)}, wantErr: true},
		{name: "unknown", values: []*krillv1.ParamValue{number("blur", 3)}, wantErr: true},
		{name: "twice", values: []*krillv1.ParamValue{number("kernel", 3), number("kernel", 5)}, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveParams(testSpec, tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && len(got) != len(testSpec.GetParams()) {
				t.Errorf("got %d params, want every one in the spec", len(got))
			}
		})
	}
}

func TestSavedParamsFallBackWhenTheSpecChanges(t *testing.T) {
	// Saved before the worker narrowed kernel's bounds.
	raw := []byte(`{"kernel": {"name": "kernel", "number": 51}, "min_area": {"name": "min_area", "number": 0.01}}`)
	got, err := savedParams(testSpec, raw)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].GetNumber() != 5 || got[2].GetNumber() != 0.01 {
		t.Errorf("params = %v, want kernel back at its default and min_area kept", got)
	}
}

func TestClassTargetsMatchExportClassIDs(t *testing.T) {
	types := []db.LabelType{
		{ID: 1, Name: "gate", Attributes: []byte(`[]`)},
		{ID: 2, Name: "bin", Attributes: []byte(`[{"name": "role", "options": ["a", "b"]}, {"name": "size", "options": ["big", "small"]}]`)},
	}
	got, err := classTargets(types)
	if err != nil {
		t.Fatal(err)
	}
	var exportTypes []export.LabelType
	for _, lt := range types {
		attrs, _ := taxonomy.ParseAttributes(lt.Attributes)
		exportTypes = append(exportTypes, export.LabelType{ID: lt.ID, Name: lt.Name, Attributes: attrs})
	}
	want := export.Build(export.Options{}, exportTypes, nil, nil, nil).Classes
	names := make([]string, len(got))
	for i, c := range got {
		names[i] = c.Name
		if c.LabelTypeID == 2 {
			attrs, _ := taxonomy.ParseAttributes(types[1].Attributes)
			if n := taxonomy.ClassName("bin", attrs, c.Attributes); n != c.Name {
				t.Errorf("class %s maps back to %s", c.Name, n)
			}
		}
	}
	if !slices.Equal(names, want) {
		t.Errorf("classes = %v, want export's %v", names, want)
	}
}
