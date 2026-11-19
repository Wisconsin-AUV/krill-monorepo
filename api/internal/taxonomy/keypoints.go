package taxonomy

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

const (
	maxKeypoints       = 64
	maxKeypointNameLen = 40
)

// Visibility values of a box's keypoints, which match YOLO-pose's.
const (
	Unlabeled = 0
	Hidden    = 1
	Visible   = 2
)

// Keypoints may lie as far outside the frame as rotated boxes' corners.
const (
	MinKeypoint = -1
	MaxKeypoint = 2
)

type Keypoint struct {
	Name   string `json:"name"`
	Mirror *int32 `json:"mirror,omitempty"`
}

type Quad struct {
	Corners   []int32 `json:"corners"`
	Reference bool    `json:"reference,omitempty"`
	Center    *int32  `json:"center,omitempty"`
}

type Edge struct {
	From int32 `json:"from"`
	To   int32 `json:"to"`
}

// Pose is a label type's keypoints and the quads and edges between them.
type Pose struct {
	Keypoints []Keypoint
	Quads     []Quad
	Edges     []Edge
}

func ParsePose(lt db.LabelType) (Pose, error) {
	var p Pose
	for _, f := range []struct {
		raw []byte
		v   any
	}{{lt.Keypoints, &p.Keypoints}, {lt.Quads, &p.Quads}, {lt.KeypointEdges, &p.Edges}} {
		if len(f.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(f.raw, f.v); err != nil {
			return Pose{}, fmt.Errorf("decode keypoints of label type %d: %w", lt.ID, err)
		}
	}
	return p, nil
}

// Encode returns the keypoints, quads, and edges columns.
func (p Pose) Encode() (keypoints, quads, edges []byte, err error) {
	if keypoints, err = json.Marshal(nonNil(p.Keypoints)); err != nil {
		return nil, nil, nil, err
	}
	if quads, err = json.Marshal(nonNil(p.Quads)); err != nil {
		return nil, nil, nil, err
	}
	if edges, err = json.Marshal(nonNil(p.Edges)); err != nil {
		return nil, nil, nil, err
	}
	return keypoints, quads, edges, nil
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

type poseRequest interface {
	GetKeypoints() []*krillv1.KeypointDef
	GetQuads() []*krillv1.KeypointQuad
	GetEdges() []*krillv1.KeypointEdge
}

func poseFromProto(req poseRequest) Pose {
	var p Pose
	for _, k := range req.GetKeypoints() {
		p.Keypoints = append(p.Keypoints, Keypoint{Name: strings.TrimSpace(k.GetName()), Mirror: k.Mirror})
	}
	p.Quads = QuadsFromProto(req.GetQuads())
	for _, e := range req.GetEdges() {
		p.Edges = append(p.Edges, Edge{From: e.GetFrom(), To: e.GetTo()})
	}
	return p
}

func QuadsFromProto(quads []*krillv1.KeypointQuad) []Quad {
	var out []Quad
	for _, q := range quads {
		out = append(out, Quad{Corners: slices.Clone(q.GetCorners()), Reference: q.GetReference(), Center: q.Center})
	}
	return out
}

func (p Pose) Proto() (keypoints []*krillv1.KeypointDef, quads []*krillv1.KeypointQuad, edges []*krillv1.KeypointEdge) {
	for _, k := range p.Keypoints {
		keypoints = append(keypoints, &krillv1.KeypointDef{Name: k.Name, Mirror: k.Mirror})
	}
	for _, e := range p.Edges {
		edges = append(edges, &krillv1.KeypointEdge{From: e.From, To: e.To})
	}
	return keypoints, p.QuadsProto(), edges
}

func (p Pose) QuadsProto() []*krillv1.KeypointQuad {
	var out []*krillv1.KeypointQuad
	for _, q := range p.Quads {
		out = append(out, &krillv1.KeypointQuad{Corners: q.Corners, Reference: q.Reference, Center: q.Center})
	}
	return out
}

func (p Pose) Count() int { return len(p.Keypoints) }

// QuadSlots is the keypoints a quad prompt fills: the quad's corners and its
// center. Nil when there is no prompt or no such quad.
func (p Pose) QuadSlots(fill *krillv1.QuadPrompt) []int32 {
	if fill == nil || int(fill.GetQuad()) >= len(p.Quads) || fill.GetQuad() < 0 {
		return nil
	}
	q := p.Quads[fill.GetQuad()]
	slots := slices.Clone(q.Corners)
	if q.Center != nil {
		slots = append(slots, *q.Center)
	}
	return slots
}

func (p Pose) Names() []string {
	out := make([]string, len(p.Keypoints))
	for i, k := range p.Keypoints {
		out[i] = k.Name
	}
	return out
}

// HasReference reports whether one of the quads is covered by the type's
// reference design.
func (p Pose) HasReference() bool {
	return slices.ContainsFunc(p.Quads, func(q Quad) bool { return q.Reference })
}

// Mirror returns the keypoint i swaps with when an image is mirrored.
func (p Pose) Mirror(i int) int {
	if m := p.Keypoints[i].Mirror; m != nil {
		return int(*m)
	}
	return i
}

// validate normalizes p and checks it is a pose a label type can have.
func (p *Pose) validate(oriented bool) error {
	n := int32(len(p.Keypoints)) //nolint:gosec // capped below
	if n > maxKeypoints {
		return fmt.Errorf("a label type has at most %d keypoints", maxKeypoints)
	}
	if n > 0 && oriented {
		return errors.New("an oriented label type can't have keypoints")
	}
	valid := func(i int32) bool { return i >= 0 && i < n }
	names := map[string]bool{}
	for i, k := range p.Keypoints {
		switch {
		case k.Name == "":
			return errors.New("keypoints need a name")
		case len(k.Name) > maxKeypointNameLen:
			return fmt.Errorf("keypoint names must be at most %d characters", maxKeypointNameLen)
		case names[k.Name]:
			return fmt.Errorf("two keypoints are named %q", k.Name)
		}
		names[k.Name] = true
		if k.Mirror == nil {
			continue
		}
		m := *k.Mirror
		if !valid(m) {
			return fmt.Errorf("keypoint %q mirrors a keypoint that doesn't exist", k.Name)
		}
		if int(m) == i {
			p.Keypoints[i].Mirror = nil
		}
	}
	for i, k := range p.Keypoints {
		if k.Mirror != nil && p.Mirror(int(*k.Mirror)) != i {
			return fmt.Errorf("keypoint %q mirrors %q, but not the other way around", k.Name, p.Keypoints[*k.Mirror].Name)
		}
	}

	corners := map[int32]bool{}
	centers := map[int32]bool{}
	references := 0
	for _, q := range p.Quads {
		if len(q.Corners) != 4 {
			return errors.New("a quad has 4 corners")
		}
		for j, c := range q.Corners {
			if !valid(c) {
				return errors.New("a quad's corner is not a keypoint")
			}
			if slices.Contains(q.Corners[:j], c) {
				return errors.New("a quad's corners must be different keypoints")
			}
			corners[c] = true
		}
		if q.Reference {
			references++
		}
		if q.Center != nil {
			c := *q.Center
			if !valid(c) {
				return errors.New("a quad's center is not a keypoint")
			}
			if centers[c] {
				return errors.New("two quads share a center")
			}
			centers[c] = true
		}
	}
	for c := range centers {
		if corners[c] {
			return errors.New("a quad's center can't be a quad's corner")
		}
	}
	if references > 1 {
		return errors.New("only one quad can be covered by the reference design")
	}
	for _, e := range p.Edges {
		if !valid(e.From) || !valid(e.To) || e.From == e.To {
			return errors.New("an edge must join two different keypoints")
		}
	}
	return nil
}

// Origins checks given, the index each of p's keypoints had in old, with -1
// for a new one, and returns it filled in when empty, where every keypoint
// keeps its index. It returns nil when every keypoint of old keeps its index,
// since boxes' keypoints then stay as they are.
func (p Pose) Origins(old Pose, given []int32) ([]int32, error) {
	n, was := len(p.Keypoints), len(old.Keypoints)
	origins := given
	if len(given) == 0 {
		origins = make([]int32, n)
		for i := range origins {
			origins[i] = -1
			if i < was {
				origins[i] = int32(i) //nolint:gosec // keypoints per type are capped
			}
		}
	} else if len(given) != n {
		return nil, fmt.Errorf("%d keypoints have %d origins", n, len(given))
	}
	seen := map[int32]bool{}
	for _, o := range origins {
		switch {
		case o < -1 || int(o) >= was:
			return nil, fmt.Errorf("keypoint origin %d is not one of the %d keypoints", o, was)
		case o >= 0 && seen[o]:
			return nil, fmt.Errorf("two keypoints come from keypoint %d", o)
		}
		seen[o] = true
	}
	for j := range was {
		if j >= n || int(origins[j]) != j {
			return origins, nil
		}
	}
	return nil, nil
}

// SetCenters places each quad's center keypoint where its diagonals cross,
// in kps, which are x, y, visibility triples for every keypoint of p.
func (p Pose) SetCenters(kps []float64) {
	for _, q := range p.Quads {
		if q.Center == nil {
			continue
		}
		c := 3 * int(*q.Center)
		kps[c], kps[c+1], kps[c+2] = 0, 0, Unlabeled
		var pts [4][2]float64
		hidden := true
		labeled := true
		for j, k := range q.Corners {
			v := kps[3*k+2]
			labeled = labeled && v != Unlabeled
			hidden = hidden && v == Hidden
			pts[j] = [2]float64{kps[3*k], kps[3*k+1]}
		}
		if !labeled {
			continue
		}
		x, y, ok := intersect(pts[0], pts[2], pts[1], pts[3])
		if !ok || x < MinKeypoint || x > MaxKeypoint || y < MinKeypoint || y > MaxKeypoint {
			continue
		}
		kps[c], kps[c+1], kps[c+2] = x, y, Visible
		if hidden {
			kps[c+2] = Hidden
		}
	}
}

// intersect returns where the lines through a0, a1 and b0, b1 cross.
func intersect(a0, a1, b0, b1 [2]float64) (x, y float64, ok bool) {
	dax, day := a1[0]-a0[0], a1[1]-a0[1]
	dbx, dby := b1[0]-b0[0], b1[1]-b0[1]
	den := dax*dby - day*dbx
	if math.Abs(den) < 1e-12 {
		return 0, 0, false
	}
	t := ((b0[0]-a0[0])*dby - (b0[1]-a0[1])*dbx) / den
	return a0[0] + t*dax, a0[1] + t*day, true
}
