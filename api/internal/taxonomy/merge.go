package taxonomy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"connectrpc.com/connect"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/db"
	"github.com/wauv/krill/api/internal/rpc"
	"github.com/wauv/krill/api/internal/storage"
)

type value struct{ attribute, option string }

// MergePlan says what each attribute value of a source type's tracks
// becomes on the target type.
type MergePlan struct {
	// Values lists the source's options in order, then values its tracks
	// have that it no longer defines.
	Values []value
	// To holds the mapped values. Values missing from it are dropped.
	To map[value]value
}

// PlanMerge checks the requested mappings, or maps each value to the
// target's attribute and option of the same name when there are none.
// observed are values tracks of the source have.
func PlanMerge(source, target []Attribute, observed []value, requested []*krillv1.AttributeValueMapping) (MergePlan, error) {
	p := MergePlan{To: map[value]value{}}
	seen := map[value]bool{}
	add := func(v value) {
		if !seen[v] {
			seen[v] = true
			p.Values = append(p.Values, v)
		}
	}
	for _, a := range source {
		for _, o := range a.Options {
			add(value{a.Name, o})
		}
	}
	for _, v := range observed {
		add(v)
	}

	if len(requested) == 0 {
		for _, v := range p.Values {
			if i := slices.IndexFunc(target, func(a Attribute) bool { return a.Name == v.attribute }); i >= 0 && slices.Contains(target[i].Options, v.option) {
				p.To[v] = v
			}
		}
		return p, nil
	}

	given := map[value]bool{}
	// Two source attributes feeding one target attribute could give a track
	// two values for it.
	fedBy := map[string]string{}
	for _, m := range requested {
		from := value{m.GetSourceAttribute(), m.GetSourceOption()}
		if !seen[from] {
			return MergePlan{}, fmt.Errorf("%s=%s is not a value of the source type", from.attribute, from.option)
		}
		if given[from] {
			return MergePlan{}, fmt.Errorf("%s=%s is mapped twice", from.attribute, from.option)
		}
		given[from] = true
		to := value{m.GetTargetAttribute(), m.GetTargetOption()}
		if to == (value{}) {
			continue
		}
		i := slices.IndexFunc(target, func(a Attribute) bool { return a.Name == to.attribute })
		if i < 0 || !slices.Contains(target[i].Options, to.option) {
			return MergePlan{}, fmt.Errorf("%s=%s is not a value of the target type", to.attribute, to.option)
		}
		if prev, ok := fedBy[to.attribute]; ok && prev != from.attribute {
			return MergePlan{}, fmt.Errorf("%s and %s both map to %s", prev, from.attribute, to.attribute)
		}
		fedBy[to.attribute] = from.attribute
		p.To[from] = to
	}
	return p, nil
}

// mappingJSON is the shape the merge queries read:
// {"source attribute": {"source option": ["target attribute", "target option"]}}.
func (p MergePlan) mappingJSON() ([]byte, error) {
	m := map[string]map[string][2]string{}
	for from, to := range p.To {
		if m[from.attribute] == nil {
			m[from.attribute] = map[string][2]string{}
		}
		m[from.attribute][from.option] = [2]string{to.attribute, to.option}
	}
	return json.Marshal(m)
}

func (p MergePlan) proto(counts map[value]int32) []*krillv1.AttributeValueMapping {
	out := make([]*krillv1.AttributeValueMapping, len(p.Values))
	for i, v := range p.Values {
		to := p.To[v]
		out[i] = &krillv1.AttributeValueMapping{
			SourceAttribute: v.attribute,
			SourceOption:    v.option,
			TargetAttribute: to.attribute,
			TargetOption:    to.option,
			TrackCount:      counts[v],
		}
	}
	return out
}

// Unfilled lists the target's attributes no mapping sets.
func (p MergePlan) Unfilled(target []Attribute) []string {
	var out []string
	for _, a := range target {
		filled := false
		for _, to := range p.To {
			if to.attribute == a.Name {
				filled = true
				break
			}
		}
		if !filled {
			out = append(out, a.Name)
		}
	}
	return out
}

// classValues lists the attribute values of each class Classes returns, in
// the same order.
func classValues(defs []Attribute) []map[string]string {
	out := []map[string]string{{}}
	for _, a := range defs {
		next := make([]map[string]string, 0, len(out)*len(a.Options))
		for _, c := range out {
			for _, o := range a.Options {
				n := make(map[string]string, len(c)+1)
				for k, v := range c {
					n[k] = v
				}
				n[a.Name] = o
				next = append(next, n)
			}
		}
		out = next
	}
	return out
}

// ClassChanges lists the exported classes whose ID changes when source
// merges into target, and what each of the source's classes becomes. types
// are in export order.
func ClassChanges(types []db.LabelType, sourceID, targetID int64, p MergePlan) ([]*krillv1.ExportClassChange, error) {
	defs := make(map[int64][]Attribute, len(types))
	after := map[string]int32{}
	var targetName string
	for _, t := range types {
		attrs, err := ParseAttributes(t.Attributes)
		if err != nil {
			return nil, fmt.Errorf("label type %d: %w", t.ID, err)
		}
		defs[t.ID] = attrs
		if t.ID == targetID {
			targetName = t.Name
		}
		if t.ID != sourceID {
			for _, c := range Classes(t.Name, attrs) {
				after[c] = int32(len(after)) //nolint:gosec // classes per type are capped
			}
		}
	}

	var out []*krillv1.ExportClassChange
	var old int32
	for _, t := range types {
		values := classValues(defs[t.ID])
		for i, name := range Classes(t.Name, defs[t.ID]) {
			change := &krillv1.ExportClassChange{Name: name, OldId: old, NewId: -1}
			old++
			if t.ID == sourceID {
				mapped := map[string]string{}
				for k, v := range values[i] {
					if to, ok := p.To[value{k, v}]; ok {
						mapped[to.attribute] = to.option
					}
				}
				if Complete(defs[targetID], mapped) {
					change.Becomes = ClassName(targetName, defs[targetID], mapped)
				}
				out = append(out, change)
				continue
			}
			change.NewId = after[name]
			if change.NewId != change.OldId {
				out = append(out, change)
			}
		}
	}
	return out, nil
}

type mergeInput struct {
	source, target db.LabelType
	targetAttrs    []Attribute
	plan           MergePlan
	counts         map[value]int32
}

// planMerge loads both types and resolves the mappings against the values
// the source's tracks have now.
func planMerge(ctx context.Context, q *db.Queries, sourceID, targetID int64, requested []*krillv1.AttributeValueMapping, lock bool) (mergeInput, error) {
	if sourceID == targetID {
		return mergeInput{}, rpc.Invalid("pick two different label types")
	}
	var types []db.LabelType
	var err error
	if lock {
		types, err = q.LockLabelTypes(ctx, []int64{sourceID, targetID})
	} else {
		types, err = q.ListAllLabelTypes(ctx)
	}
	if err != nil {
		return mergeInput{}, rpc.Internal(err, "load label types")
	}
	var in mergeInput
	found := 0
	for _, t := range types {
		switch t.ID {
		case sourceID:
			in.source = t
			found++
		case targetID:
			in.target = t
			found++
		}
	}
	if found != 2 {
		return mergeInput{}, connect.NewError(connect.CodeNotFound, errors.New("label type not found"))
	}
	sourceAttrs, err := ParseAttributes(in.source.Attributes)
	if err != nil {
		return mergeInput{}, rpc.Internal(err, "decode label type")
	}
	if in.targetAttrs, err = ParseAttributes(in.target.Attributes); err != nil {
		return mergeInput{}, rpc.Internal(err, "decode label type")
	}
	rows, err := q.CountTrackValues(ctx, sourceID)
	if err != nil {
		return mergeInput{}, rpc.Internal(err, "count attribute values")
	}
	in.counts = make(map[value]int32, len(rows))
	observed := make([]value, len(rows))
	for i, r := range rows {
		observed[i] = value{r.Attribute, r.Option}
		in.counts[observed[i]] = r.Tracks
	}
	if in.plan, err = PlanMerge(sourceAttrs, in.targetAttrs, observed, requested); err != nil {
		return mergeInput{}, rpc.Invalid("%s", err)
	}
	return in, nil
}

func (s *Service) PreviewMergeLabelTypes(ctx context.Context, req *krillv1.PreviewMergeLabelTypesRequest) (*krillv1.PreviewMergeLabelTypesResponse, error) {
	in, err := planMerge(ctx, s.q, req.GetSourceId(), req.GetTargetId(), req.GetMappings(), false)
	if err != nil {
		return nil, err
	}
	impact, err := s.q.GetMergeImpact(ctx, in.source.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	types, err := s.q.ListAllLabelTypes(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "list label types")
	}
	changes, err := ClassChanges(types, in.source.ID, in.target.ID, in.plan)
	if err != nil {
		return nil, rpc.Internal(err, "list classes")
	}
	return &krillv1.PreviewMergeLabelTypesResponse{
		TrackCount:         impact.TrackCount,
		BoxCount:           impact.BoxCount,
		ClipCount:          impact.ClipCount,
		VideoCount:         impact.VideoCount,
		TrashedTrackCount:  impact.TrashedTrackCount,
		GoldFrameCount:     impact.GoldFrameCount,
		GoldAttemptCount:   impact.GoldAttemptCount,
		OpenReviewCount:    impact.OpenReviewCount,
		ExampleCount:       impact.ExampleCount,
		Mappings:           in.plan.proto(in.counts),
		ClassChanges:       changes,
		UnfilledAttributes: in.plan.Unfilled(in.targetAttrs),
	}, nil
}

func (s *Service) MergeLabelTypes(ctx context.Context, req *krillv1.MergeLabelTypesRequest) (*krillv1.MergeLabelTypesResponse, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, rpc.Internal(err, "begin")
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := s.q.WithTx(tx)
	in, err := planMerge(ctx, q, req.GetSourceId(), req.GetTargetId(), req.GetMappings(), true)
	if err != nil {
		return nil, err
	}
	impact, err := q.GetMergeImpact(ctx, in.source.ID)
	if err != nil {
		return nil, rpc.Internal(err, "count labels")
	}
	mapping, err := in.plan.mappingJSON()
	if err != nil {
		return nil, rpc.Internal(err, "encode mapping")
	}
	tracks, err := q.MergeTracks(ctx, db.MergeTracksParams{SourceID: in.source.ID, TargetID: in.target.ID, Mapping: mapping})
	if err != nil {
		return nil, rpc.Internal(err, "move tracks")
	}
	if err := q.MergeRejectedPrelabels(ctx, db.MergeRejectedPrelabelsParams{SourceID: in.source.ID, TargetID: in.target.ID}); err != nil {
		return nil, rpc.Internal(err, "move rejected proposals")
	}
	if err := q.MergeProposedLabelTypes(ctx, db.MergeProposedLabelTypesParams{SourceID: in.source.ID, TargetID: in.target.ID}); err != nil {
		return nil, rpc.Internal(err, "move proposed classes")
	}
	gold, err := q.MergeGoldFrames(ctx, db.MergeGoldFramesParams{SourceID: in.source.ID, TargetID: in.target.ID, Mapping: mapping})
	if err != nil {
		return nil, rpc.Internal(err, "rewrite gold frames")
	}
	attempts, err := q.MergeGoldAttempts(ctx, db.MergeGoldAttemptsParams{SourceID: in.source.ID, TargetID: in.target.ID, Mapping: mapping})
	if err != nil {
		return nil, rpc.Internal(err, "rewrite gold attempts")
	}
	if _, err := q.DeleteLabelType(ctx, in.source.ID); err != nil {
		return nil, rpc.Internal(err, "delete label type")
	}
	var dropped int32
	for _, v := range in.plan.Values {
		if _, ok := in.plan.To[v]; !ok {
			dropped += in.counts[v]
		}
	}
	err = record(ctx, q, audit.LabelTypeMerge, in.source, map[string]any{
		"into":          in.target.Name,
		"tracks":        tracks,
		"boxes":         impact.BoxCount,
		"gold_frames":   gold,
		"gold_attempts": attempts,
		"dropped":       dropped,
	})
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, rpc.Internal(err, "commit")
	}
	if s.store != nil {
		if err := s.store.RemovePrefix(ctx, storage.LabelExamplesPrefix(in.source.ID)); err != nil {
			slog.WarnContext(ctx, "remove label examples", "label_type_id", in.source.ID, "err", err)
		}
	}
	return &krillv1.MergeLabelTypesResponse{
		TrackCount:        int32(tracks), //nolint:gosec // track counts fit in int32
		BoxCount:          impact.BoxCount,
		GoldFrameCount:    int32(gold), //nolint:gosec // gold frame counts fit in int32
		DroppedValueCount: dropped,
	}, nil
}
