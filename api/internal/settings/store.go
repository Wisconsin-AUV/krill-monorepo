// Package settings resolves each KRILL_ setting from the environment, then
// the admin panel's saved value, then its default, and applies saved changes
// without a restart.
package settings

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wauv/krill/api/internal/audit"
	"github.com/wauv/krill/api/internal/config"
	"github.com/wauv/krill/api/internal/db"
)

type Source int

const (
	SourceDefault Source = iota
	SourceEnv
	SourceSaved
)

type saved struct {
	value     string
	updatedAt time.Time
	// updatedBy is empty when the account that saved it was deleted.
	updatedBy string
}

type state struct {
	cfg   config.Config
	saved map[string]saved
}

type Store struct {
	pool *pgxpool.Pool
	env  config.Lookup
	// mu serializes Update so two saves can't each validate against a state
	// the other is about to replace.
	mu        sync.Mutex
	cur       atomic.Pointer[state]
	listeners []func(config.Config)
}

// Open loads the saved settings and parses the config they make with env.
func Open(ctx context.Context, pool *pgxpool.Pool, env config.Lookup) (*Store, error) {
	s := &Store{pool: pool, env: env}
	rows, err := db.New(pool).ListSettings(ctx)
	if err != nil {
		return nil, fmt.Errorf("load settings: %w", err)
	}
	vals := make(map[string]saved, len(rows))
	for _, r := range rows {
		vals[r.Key] = saved{value: r.Value, updatedAt: r.UpdatedAt.Time, updatedBy: r.UpdatedByName.String}
	}
	cfg, err := config.Parse(s.lookup(vals))
	if err != nil {
		return nil, err
	}
	s.cur.Store(&state{cfg: cfg, saved: vals})
	return s, nil
}

func (s *Store) lookup(vals map[string]saved) config.Lookup {
	return func(key string) (string, bool) {
		if v, ok := s.env(key); ok {
			return v, true
		}
		if def, _ := config.Find(key); def.EnvOnly {
			return "", false
		}
		v, ok := vals[key]
		return v.value, ok
	}
}

func (s *Store) Config() config.Config {
	return s.cur.Load().cfg
}

// OnChange calls fn with the current config and again after every Update.
// Register listeners before serving requests.
func (s *Store) OnChange(fn func(config.Config)) {
	s.listeners = append(s.listeners, fn)
	fn(s.Config())
}

// Entry is a setting as the admin panel shows it.
type Entry struct {
	config.Setting
	Source Source
	// Value is empty for secrets.
	Value     string
	IsSet     bool
	UpdatedAt time.Time
	UpdatedBy string
}

func (s *Store) List() []Entry {
	st := s.cur.Load()
	out := make([]Entry, len(config.Settings))
	for i, def := range config.Settings {
		e := Entry{Setting: def, Value: def.Default}
		if v, ok := s.env(def.Key); ok {
			e.Source, e.Value = SourceEnv, v
		} else if v, ok := st.saved[def.Key]; ok && !def.EnvOnly {
			e.Source, e.Value, e.UpdatedAt, e.UpdatedBy = SourceSaved, v.value, v.updatedAt, v.updatedBy
		}
		e.IsSet = e.Value != ""
		if def.Kind == config.KindSecret {
			e.Value = ""
		}
		out[i] = e
	}
	return out
}

// Change saves Value for Key, or drops the saved value when Reset is set.
type Change struct {
	Key   string
	Value string
	Reset bool
}

// Update validates the config the changes would make and applies them all or
// none. Errors about one setting are *config.FieldError.
func (s *Store) Update(ctx context.Context, by uuid.UUID, changes []Change) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.cur.Load()
	next := make(map[string]saved, len(st.saved)+len(changes))
	for k, v := range st.saved {
		next[k] = v
	}
	seen := map[string]bool{}
	for i, c := range changes {
		def, ok := config.Find(c.Key)
		if !ok {
			return fmt.Errorf("unknown setting %s", c.Key)
		}
		if seen[c.Key] {
			return fmt.Errorf("%s is changed twice", c.Key)
		}
		seen[c.Key] = true
		if def.EnvOnly {
			return &config.FieldError{Key: c.Key, Message: "can only be set in the environment"}
		}
		if _, ok := s.env(c.Key); ok {
			return &config.FieldError{Key: c.Key, Message: "is set in the environment, which takes precedence"}
		}
		if c.Reset {
			delete(next, c.Key)
			continue
		}
		changes[i].Value = strings.TrimSpace(c.Value)
		next[c.Key] = saved{value: changes[i].Value}
	}
	cfg, err := config.Parse(s.lookup(next))
	if err != nil {
		return err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	q := db.New(tx)
	for _, c := range changes {
		if c.Reset {
			err = q.DeleteSetting(ctx, c.Key)
		} else {
			err = q.UpsertSetting(ctx, db.UpsertSettingParams{Key: c.Key, Value: c.Value, UpdatedBy: pgtype.UUID{Bytes: by, Valid: by != uuid.Nil}})
		}
		if err != nil {
			return fmt.Errorf("save %s: %w", c.Key, err)
		}
		if err := recordChange(ctx, q, st.saved, c); err != nil {
			return err
		}
	}
	rows, err := q.ListSettings(ctx)
	if err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	vals := make(map[string]saved, len(rows))
	for _, r := range rows {
		vals[r.Key] = saved{value: r.Value, updatedAt: r.UpdatedAt.Time, updatedBy: r.UpdatedByName.String}
	}
	s.cur.Store(&state{cfg: cfg, saved: vals})
	for _, fn := range s.listeners {
		fn(cfg)
	}
	return nil
}

// recordChange audits the value the admin panel showed before and after.
// Secrets are only noted as changed.
func recordChange(ctx context.Context, q *db.Queries, before map[string]saved, c Change) error {
	def, _ := config.Find(c.Key)
	from, to := def.Default, def.Default
	if v, ok := before[c.Key]; ok {
		from = v.value
	}
	if !c.Reset {
		to = c.Value
	}
	if from == to {
		return nil
	}
	details := map[string]any{}
	if def.Kind != config.KindSecret {
		details = audit.Change(from, to)
	}
	if c.Reset {
		details["reset"] = true
	}
	return audit.Record(ctx, q, audit.Entry{
		Action:     audit.SettingUpdate,
		TargetType: audit.TargetSetting,
		TargetID:   c.Key,
		TargetName: def.Label,
		Details:    details,
	})
}
