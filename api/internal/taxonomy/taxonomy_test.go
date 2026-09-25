package taxonomy

import (
	"reflect"
	"testing"
)

func TestNormalizeType(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		color   string
		attrs   []Attribute
		want    string
		wantErr bool
	}{
		{"simple", " Gate ", "#0EA5E9", nil, "gate", false},
		{"snake case", "torpedo_hole", "#0ea5e9", nil, "torpedo_hole", false},
		{"leading digit", "1gate", "#0ea5e9", nil, "", true},
		{"dash", "torpedo-hole", "#0ea5e9", nil, "", true},
		{"bad color", "gate", "blue", nil, "", true},
		{"attribute", "bin", "#0ea5e9", []Attribute{{Name: "role", Options: []string{"a", "B "}}}, "bin", false},
		{"empty options", "bin", "#0ea5e9", []Attribute{{Name: "role"}}, "", true},
		{"duplicate attribute", "bin", "#0ea5e9", []Attribute{{"role", []string{"a"}}, {"role", []string{"b"}}}, "", true},
		{"duplicate option", "bin", "#0ea5e9", []Attribute{{"role", []string{"a", "a"}}}, "", true},
		{"dash in option", "bin", "#0ea5e9", []Attribute{{"role", []string{"a-b"}}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _, attrs, err := NormalizeType(tt.in, tt.color, tt.attrs)
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("name = %q, want %q", got, tt.want)
			}
			if tt.name == "attribute" && !reflect.DeepEqual(attrs[0].Options, []string{"a", "b"}) {
				t.Errorf("options = %v, want [a b]", attrs[0].Options)
			}
		})
	}
}

func TestValues(t *testing.T) {
	defs := []Attribute{{"size", []string{"big", "small"}}, {"role", []string{"red", "blue"}}}

	if err := CheckValues(defs, map[string]string{"size": "big"}); err != nil {
		t.Errorf("partial values rejected: %v", err)
	}
	if err := CheckValues(defs, map[string]string{"size": "huge"}); err == nil {
		t.Error("unknown option accepted")
	}
	if err := CheckValues(defs, map[string]string{"color": "red"}); err == nil {
		t.Error("unknown attribute accepted")
	}
	if Complete(defs, map[string]string{"size": "big"}) {
		t.Error("partial values reported complete")
	}
	if !Complete(defs, map[string]string{"size": "big", "role": "red"}) {
		t.Error("full values reported incomplete")
	}
	got := KeepValid(defs[:1], map[string]string{"size": "small", "role": "red"})
	if !reflect.DeepEqual(got, map[string]string{"size": "small"}) {
		t.Errorf("KeepValid = %v", got)
	}
}

func TestClasses(t *testing.T) {
	defs := []Attribute{{"size", []string{"big", "small"}}, {"role", []string{"red", "blue"}}}
	want := []string{
		"torpedo_hole-big-red", "torpedo_hole-big-blue",
		"torpedo_hole-small-red", "torpedo_hole-small-blue",
	}
	if got := Classes("torpedo_hole", defs); !reflect.DeepEqual(got, want) {
		t.Errorf("Classes = %v, want %v", got, want)
	}
	if got := Classes("gate", nil); !reflect.DeepEqual(got, []string{"gate"}) {
		t.Errorf("Classes without attributes = %v", got)
	}
	if got := ClassName("torpedo_hole", defs, map[string]string{"size": "small", "role": "blue"}); got != "torpedo_hole-small-blue" {
		t.Errorf("ClassName = %q", got)
	}
}

func TestValidExampleKey(t *testing.T) {
	const name = "0123456789abcdef0123456789abcdef"
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{"issued", "label-types/3/examples/" + name, true},
		{"other type", "label-types/4/examples/" + name, false},
		{"type id prefix", "label-types/31/examples/" + name, false},
		{"video source", "videos/1/source", false},
		{"traversal", "label-types/3/examples/../../../videos/1/source", false},
		{"dataset", "label-types/3/examples/" + name + "/../../../../datasets/1/dataset.zip", false},
		{"empty name", "label-types/3/examples/", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidExampleKey(3, tt.key); got != tt.want {
				t.Errorf("ValidExampleKey(3, %q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}
