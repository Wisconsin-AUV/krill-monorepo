package logs

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// Handler copies every record inner handles into a Buffer, redacted, and
// leaves what inner writes unchanged.
type Handler struct {
	inner slog.Handler
	buf   *Buffer
	attrs []*krillv1.LogAttr
	// prefix is the open groups, dotted.
	prefix string
}

func NewHandler(inner slog.Handler, buf *Buffer) *Handler {
	return &Handler{inner: inner, buf: buf}
}

func (h *Handler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	attrs := make([]*krillv1.LogAttr, len(h.attrs), len(h.attrs)+r.NumAttrs())
	copy(attrs, h.attrs)
	r.Attrs(func(a slog.Attr) bool {
		attrs = flatten(h.prefix, a, attrs)
		return true
	})
	t := r.Time
	if t.IsZero() {
		t = time.Now()
	}
	h.buf.Append(&krillv1.LogRecord{
		Time:    timestamppb.New(t),
		Level:   level(r.Level),
		Message: redactText(r.Message),
		Attrs:   attrs,
	})
	return h.inner.Handle(ctx, r)
}

func (h *Handler) WithAttrs(as []slog.Attr) slog.Handler {
	attrs := append([]*krillv1.LogAttr(nil), h.attrs...)
	for _, a := range as {
		attrs = flatten(h.prefix, a, attrs)
	}
	return &Handler{inner: h.inner.WithAttrs(as), buf: h.buf, attrs: attrs, prefix: h.prefix}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	return &Handler{inner: h.inner.WithGroup(name), buf: h.buf, attrs: h.attrs, prefix: h.prefix + name + "."}
}

func level(l slog.Level) krillv1.LogLevel {
	switch {
	case l < slog.LevelInfo:
		return krillv1.LogLevel_LOG_LEVEL_DEBUG
	case l < slog.LevelWarn:
		return krillv1.LogLevel_LOG_LEVEL_INFO
	case l < slog.LevelError:
		return krillv1.LogLevel_LOG_LEVEL_WARN
	}
	return krillv1.LogLevel_LOG_LEVEL_ERROR
}

// flatten appends a with groups spelled out as dotted keys, as slog's JSON
// handler nests them.
func flatten(prefix string, a slog.Attr, out []*krillv1.LogAttr) []*krillv1.LogAttr {
	v := a.Value.Resolve()
	if v.Kind() == slog.KindGroup {
		p := prefix
		if a.Key != "" {
			p += a.Key + "."
		}
		for _, g := range v.Group() {
			out = flatten(p, g, out)
		}
		return out
	}
	if a.Key == "" {
		return out
	}
	key := prefix + a.Key
	return append(out, &krillv1.LogAttr{Key: key, Value: redactAttr(key, text(v))})
}

func text(v slog.Value) string {
	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindAny:
		if err, ok := v.Any().(error); ok {
			return err.Error()
		}
		return fmt.Sprint(v.Any())
	}
	return v.String()
}
