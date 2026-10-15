package taxonomy

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"connectrpc.com/connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/internal/db"
)

func TestClassChanges(t *testing.T) {
	attrs := func(a ...Attribute) []byte {
		b, err := json.Marshal(a)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	types := []db.LabelType{
		{ID: 1, Name: "gate", Attributes: attrs()},
		{ID: 2, Name: "buoy", Attributes: attrs(Attribute{"size", []string{"big", "small"}})},
		{ID: 3, Name: "bin", Attributes: attrs()},
		{ID: 4, Name: "marker", Attributes: attrs(Attribute{"size", []string{"large", "small"}})},
	}
	plan := MergePlan{To: map[value]value{
		{"size", "big"}:   {"size", "large"},
		{"size", "small"}: {"size", "small"},
	}}
	got, err := ClassChanges(types, 2, 4, plan)
	if err != nil {
		t.Fatal(err)
	}
	want := []*krillv1.ExportClassChange{
		{Name: "buoy-big", OldId: 1, NewId: -1, Becomes: "marker-large"},
		{Name: "buoy-small", OldId: 2, NewId: -1, Becomes: "marker-small"},
		{Name: "bin", OldId: 3, NewId: 1},
		{Name: "marker-large", OldId: 4, NewId: 2},
		{Name: "marker-small", OldId: 5, NewId: 3},
	}
	if len(got) != len(want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.GetName() != w.GetName() || g.GetOldId() != w.GetOldId() || g.GetNewId() != w.GetNewId() || g.GetBecomes() != w.GetBecomes() {
			t.Errorf("change %d = %v, want %v", i, g, w)
		}
	}

	delete(plan.To, value{"size", "small"})
	got, err = ClassChanges(types, 2, 4, plan)
	if err != nil {
		t.Fatal(err)
	}
	if got[1].GetBecomes() != "" {
		t.Errorf("buoy-small with its size dropped becomes %q, want nothing until the size is set", got[1].GetBecomes())
	}
}

func TestMergeLabelTypes(t *testing.T) {
	url := os.Getenv("KRILL_TEST_DATABASE_URL")
	if url == "" {
		t.Skip("KRILL_TEST_DATABASE_URL is not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	s := NewService(pool, nil)

	labelType := func(attrs string) int64 {
		var id int64
		err := pool.QueryRow(ctx, `
			INSERT INTO label_types (name, color, position, attributes) VALUES ($1, '#ffffff', 1000, $2) RETURNING id`,
			"merge_test_"+uuid.NewString()[:8], attrs,
		).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			_, _ = pool.Exec(context.Background(), `DELETE FROM tracks WHERE label_type_id = $1`, id)
			_, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, id)
		})
		return id
	}
	source := labelType(`[{"name": "size", "options": ["big", "small"]}, {"name": "color", "options": ["red", "blue"]}]`)
	target := labelType(`[{"name": "size", "options": ["large", "small"]}, {"name": "shade", "options": ["dark", "light"]}]`)

	var videoID, clipID int64
	err = pool.QueryRow(ctx, `INSERT INTO videos (name, filename, status) VALUES ('v', 'v.mp4', 'ready') RETURNING id`).Scan(&videoID)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM videos WHERE id = $1`, videoID) })
	err = pool.QueryRow(ctx, `INSERT INTO clips (video_id, idx, start_frame, frame_count) VALUES ($1, 0, 0, 3) RETURNING id`, videoID).Scan(&clipID)
	if err != nil {
		t.Fatal(err)
	}
	var frames []int64
	for i := range 3 {
		var id int64
		err := pool.QueryRow(ctx, `INSERT INTO frames (video_id, clip_id, idx, phash) VALUES ($1, $2, $3, 0) RETURNING id`, videoID, clipID, i).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		frames = append(frames, id)
	}
	track := func(typeID int64, attrs string) int64 {
		var id int64
		err := pool.QueryRow(ctx, `INSERT INTO tracks (clip_id, label_type_id, attributes) VALUES ($1, $2, $3) RETURNING id`, clipID, typeID, attrs).Scan(&id)
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range frames {
			_, err := pool.Exec(ctx, `INSERT INTO annotations (track_id, frame_id, x, y, width, height) VALUES ($1, $2, 0.1, 0.1, 0.2, 0.2)`, id, f)
			if err != nil {
				t.Fatal(err)
			}
		}
		return id
	}
	bigRed := track(source, `{"size": "big", "color": "red"}`)
	blue := track(source, `{"color": "blue"}`)
	kept := track(target, `{"size": "small", "shade": "light"}`)
	_, err = pool.Exec(ctx, `INSERT INTO gold_frames (frame_id, boxes) VALUES ($1, $2)`, frames[0], `[
		{"label_type_id": `+strconv.FormatInt(source, 10)+`, "attributes": {"size": "big", "color": "blue"}, "x": 0.1, "y": 0.1, "width": 0.2, "height": 0.2},
		{"label_type_id": `+strconv.FormatInt(target, 10)+`, "attributes": {"size": "small"}, "x": 0.5, "y": 0.5, "width": 0.2, "height": 0.2}
	]`)
	if err != nil {
		t.Fatal(err)
	}

	boxesBefore := 0
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = $1`, clipID).Scan(&boxesBefore); err != nil {
		t.Fatal(err)
	}

	m := func(sa, so, ta, to string) *krillv1.AttributeValueMapping {
		return &krillv1.AttributeValueMapping{SourceAttribute: sa, SourceOption: so, TargetAttribute: ta, TargetOption: to}
	}
	invalid := []struct {
		name     string
		mappings []*krillv1.AttributeValueMapping
	}{
		{"unknown target option", []*krillv1.AttributeValueMapping{m("size", "big", "size", "huge")}},
		{"two attributes into one", []*krillv1.AttributeValueMapping{m("size", "big", "shade", "dark"), m("color", "red", "shade", "light")}},
		{"value mapped twice", []*krillv1.AttributeValueMapping{m("size", "big", "size", "large"), m("size", "big", "size", "small")}},
		{"value the source lacks", []*krillv1.AttributeValueMapping{m("weight", "heavy", "size", "large")}},
	}
	for _, tt := range invalid {
		t.Run(tt.name, func(t *testing.T) {
			_, err := s.MergeLabelTypes(ctx, &krillv1.MergeLabelTypesRequest{SourceId: source, TargetId: target, Mappings: tt.mappings})
			if connect.CodeOf(err) != connect.CodeInvalidArgument {
				t.Fatalf("err = %v, want invalid argument", err)
			}
		})
	}

	mappings := []*krillv1.AttributeValueMapping{
		m("size", "big", "size", "large"),
		m("size", "small", "size", "small"),
		m("color", "red", "shade", "dark"),
		m("color", "blue", "", ""),
	}
	preview, err := s.PreviewMergeLabelTypes(ctx, &krillv1.PreviewMergeLabelTypesRequest{SourceId: source, TargetId: target, Mappings: mappings})
	if err != nil {
		t.Fatal(err)
	}
	if preview.GetTrackCount() != 2 || preview.GetBoxCount() != 6 || preview.GetGoldFrameCount() != 1 {
		t.Errorf("preview = %d tracks, %d boxes, %d gold frames, want 2, 6, 1",
			preview.GetTrackCount(), preview.GetBoxCount(), preview.GetGoldFrameCount())
	}
	res, err := s.MergeLabelTypes(ctx, &krillv1.MergeLabelTypesRequest{SourceId: source, TargetId: target, Mappings: mappings})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetTrackCount() != 2 || res.GetDroppedValueCount() != 1 {
		t.Errorf("merged %d tracks dropping %d values, want 2 and 1", res.GetTrackCount(), res.GetDroppedValueCount())
	}

	attrsOf := func(id int64) (int64, map[string]string) {
		var typeID int64
		var raw []byte
		if err := pool.QueryRow(ctx, `SELECT label_type_id, attributes FROM tracks WHERE id = $1`, id).Scan(&typeID, &raw); err != nil {
			t.Fatal(err)
		}
		values, err := ParseValues(raw)
		if err != nil {
			t.Fatal(err)
		}
		return typeID, values
	}
	for _, tt := range []struct {
		track int64
		want  map[string]string
	}{
		{bigRed, map[string]string{"size": "large", "shade": "dark"}},
		{blue, map[string]string{}},
		{kept, map[string]string{"size": "small", "shade": "light"}},
	} {
		typeID, got := attrsOf(tt.track)
		if typeID != target {
			t.Errorf("track %d has type %d, want %d", tt.track, typeID, target)
		}
		if len(got) != len(tt.want) {
			t.Errorf("track %d attributes = %v, want %v", tt.track, got, tt.want)
		}
		for k, v := range tt.want {
			if got[k] != v {
				t.Errorf("track %d attributes = %v, want %v", tt.track, got, tt.want)
			}
		}
	}

	boxesAfter := 0
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM annotations a JOIN tracks t ON t.id = a.track_id WHERE t.clip_id = $1`, clipID).Scan(&boxesAfter); err != nil {
		t.Fatal(err)
	}
	if boxesAfter != boxesBefore {
		t.Errorf("boxes = %d after merging, want %d", boxesAfter, boxesBefore)
	}
	var exists bool
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM label_types WHERE id = $1)`, source).Scan(&exists); err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("source type still exists")
	}

	var gold []struct {
		LabelTypeID int64             `json:"label_type_id"`
		Attributes  map[string]string `json:"attributes"`
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT boxes FROM gold_frames WHERE frame_id = $1`, frames[0]).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &gold); err != nil {
		t.Fatal(err)
	}
	if len(gold) != 2 || gold[0].LabelTypeID != target || gold[0].Attributes["size"] != "large" || len(gold[0].Attributes) != 1 {
		t.Errorf("gold boxes = %+v, want the source box moved to the target with size large", gold)
	}
	if gold[1].Attributes["size"] != "small" {
		t.Errorf("gold box of the target changed: %+v", gold[1])
	}
}
