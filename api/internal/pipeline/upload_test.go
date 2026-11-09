package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/wauv/krill/api/internal/db"
)

func TestValidUploadKeyRejectsOtherObjects(t *testing.T) {
	for _, key := range []string{
		"backups/krill.dump",
		"training-runs/1/best.pt",
		"model-uploads/",
		"model-uploads/../backups/krill.dump",
		"model-uploads/0123456789abcdef0123456789abcdef/best.pt",
		"model-uploads/0123456789ABCDEF0123456789ABCDEF",
		"x/model-uploads/0123456789abcdef0123456789abcdef",
	} {
		if validUploadKey(key) {
			t.Errorf("validUploadKey(%q) = true", key)
		}
	}
	if !validUploadKey("model-uploads/0123456789abcdef0123456789abcdef") {
		t.Error("rejected a key CreateModelUpload issues")
	}
}

func TestUploadedModelKeepsItsClassIDsAndDropsUnknownClasses(t *testing.T) {
	pool := testPool(t)
	ctx := t.Context()
	p := New(Deps{Pool: pool})
	var buoy, gate int64
	for _, id := range []*int64{&buoy, &gate} {
		*id = scalar[int64](t, pool, `INSERT INTO label_types (name, color, position) VALUES ('upload-test-' || gen_random_uuid(), '#ff0000', 99) RETURNING id`)
		t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM label_types WHERE id = $1`, *id) })
	}
	targets := []classTarget{
		{Name: "buoy", LabelTypeID: buoy, Attributes: map[string]string{}},
		{Name: "gate", LabelTypeID: gate, Attributes: map[string]string{}},
	}

	classes, matched := uploadClasses([]string{"buoy", "person", "gate"}, targets)
	if matched != 2 {
		t.Fatalf("matched = %d, want 2", matched)
	}
	raw, err := json.Marshal(classes)
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.Marshal(targets)
	if err != nil {
		t.Fatal(err)
	}
	// The model calls gate class 2 where a dataset calls it 1, so scoring it
	// as a baseline on one would count every gate as wrong.
	if sameClasses(raw, want) {
		t.Error("an upload with an extra class has the same classes as the dataset")
	}

	run, err := db.New(pool).CreateUploadedTrainingRun(ctx, db.CreateUploadedTrainingRunParams{
		Config: []byte(`{"imageSize": 320}`), Classes: raw, Filename: "best.pt", WeightsBytes: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM training_runs WHERE id = $1`, run.ID) })
	l, err := p.yoloLabeler(ctx, run.ID, 0.25)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := l.targets["person"]; ok {
		t.Error("proposes boxes of a class no label type exports")
	}
	if g := l.targets["gate"]; g.labelTypeID != gate || g.index != 2 {
		t.Errorf("gate target = %+v, want label type %d at index 2", g, gate)
	}
}
