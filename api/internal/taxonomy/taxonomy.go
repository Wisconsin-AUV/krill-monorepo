// Package taxonomy validates label types and the per-track attribute values
// assigned against them.
package taxonomy

import (
	"encoding/json"
	"fmt"
	"regexp"
	"slices"
	"strings"
)

const (
	maxNameLen    = 40
	maxAttributes = 8
	maxOptions    = 64
	// Each type exports one class per combination of options.
	maxClasses = 256
)

var (
	namePattern   = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)
	optionPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_]*$`)
	colorPattern  = regexp.MustCompile(`^#[0-9a-f]{6}$`)
)

type Attribute struct {
	Name    string   `json:"name"`
	Options []string `json:"options"`
}

func ParseAttributes(raw []byte) ([]Attribute, error) {
	var attrs []Attribute
	if len(raw) == 0 {
		return attrs, nil
	}
	if err := json.Unmarshal(raw, &attrs); err != nil {
		return nil, fmt.Errorf("parse attributes: %w", err)
	}
	return attrs, nil
}

func ParseValues(raw []byte) (map[string]string, error) {
	values := map[string]string{}
	if len(raw) == 0 {
		return values, nil
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, fmt.Errorf("parse attribute values: %w", err)
	}
	return values, nil
}

// NormalizeType trims and lowercases the input, then checks the rules that
// keep exported class names unambiguous.
func NormalizeType(name, color string, attrs []Attribute) (string, string, []Attribute, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if err := checkName("name", name); err != nil {
		return "", "", nil, err
	}
	color = strings.ToLower(strings.TrimSpace(color))
	if !colorPattern.MatchString(color) {
		return "", "", nil, fmt.Errorf("color must be a hex color like #0ea5e9")
	}
	if len(attrs) > maxAttributes {
		return "", "", nil, fmt.Errorf("at most %d attributes", maxAttributes)
	}

	out := make([]Attribute, 0, len(attrs))
	seen := map[string]bool{}
	for _, a := range attrs {
		an := strings.ToLower(strings.TrimSpace(a.Name))
		if err := checkName("attribute name", an); err != nil {
			return "", "", nil, err
		}
		if seen[an] {
			return "", "", nil, fmt.Errorf("attribute %q is listed twice", an)
		}
		seen[an] = true
		if len(a.Options) == 0 {
			return "", "", nil, fmt.Errorf("attribute %q needs at least one option", an)
		}
		if len(a.Options) > maxOptions {
			return "", "", nil, fmt.Errorf("attribute %q has more than %d options", an, maxOptions)
		}
		opts := make([]string, 0, len(a.Options))
		for _, o := range a.Options {
			o = strings.ToLower(strings.TrimSpace(o))
			if len(o) > maxNameLen || !optionPattern.MatchString(o) {
				return "", "", nil, fmt.Errorf("option %q of %q must be lowercase letters, digits, or underscores", o, an)
			}
			if slices.Contains(opts, o) {
				return "", "", nil, fmt.Errorf("option %q of %q is listed twice", o, an)
			}
			opts = append(opts, o)
		}
		out = append(out, Attribute{Name: an, Options: opts})
	}
	classes := 1
	for _, a := range out {
		classes *= len(a.Options)
		if classes > maxClasses {
			return "", "", nil, fmt.Errorf("attributes would export more than %d classes for %q", maxClasses, name)
		}
	}
	return name, color, out, nil
}

func checkName(field, s string) error {
	if s == "" || len(s) > maxNameLen || !namePattern.MatchString(s) {
		return fmt.Errorf("%s %q must start with a letter and use lowercase letters, digits, or underscores (max %d)", field, s, maxNameLen)
	}
	return nil
}

// CheckValues rejects attributes the type does not define and options that
// are not allowed. Missing attributes are fine: they are filled in later.
func CheckValues(defs []Attribute, values map[string]string) error {
	for k, v := range values {
		i := slices.IndexFunc(defs, func(a Attribute) bool { return a.Name == k })
		if i < 0 {
			return fmt.Errorf("attribute %q is not defined for this type", k)
		}
		if !slices.Contains(defs[i].Options, v) {
			return fmt.Errorf("%q is not an option for %q", v, k)
		}
	}
	return nil
}

// KeepValid drops values that are no longer valid for defs, for example
// after a track changes type.
func KeepValid(defs []Attribute, values map[string]string) map[string]string {
	out := map[string]string{}
	for _, a := range defs {
		if v, ok := values[a.Name]; ok && slices.Contains(a.Options, v) {
			out[a.Name] = v
		}
	}
	return out
}

// Complete reports whether every attribute has a valid value.
func Complete(defs []Attribute, values map[string]string) bool {
	for _, a := range defs {
		if !slices.Contains(a.Options, values[a.Name]) {
			return false
		}
	}
	return true
}

// ClassName flattens a type and its attribute values into a YOLO class name,
// for example torpedo_hole-big-red. Values follow the order of defs.
func ClassName(name string, defs []Attribute, values map[string]string) string {
	parts := []string{name}
	for _, a := range defs {
		parts = append(parts, values[a.Name])
	}
	return strings.Join(parts, "-")
}

// Classes lists every class name a type can produce, in a stable order.
func Classes(name string, defs []Attribute) []string {
	classes := []string{name}
	for _, a := range defs {
		next := make([]string, 0, len(classes)*len(a.Options))
		for _, c := range classes {
			for _, o := range a.Options {
				next = append(next, c+"-"+o)
			}
		}
		classes = next
	}
	return classes
}
