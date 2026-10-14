package logs

import (
	"context"
	"errors"
	"os"
	"strings"

	"connectrpc.com/connect"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

const (
	defaultLimit = 500
	maxLimit     = 2000
)

type Service struct {
	krillv1connect.UnimplementedLogServiceHandler
	hub      *Hub
	hostname string
}

func NewService(hub *Hub) *Service {
	hostname, _ := os.Hostname()
	return &Service{hub: hub, hostname: hostname}
}

func (s *Service) ListLogSources(context.Context, *krillv1.ListLogSourcesRequest) (*krillv1.ListLogSourcesResponse, error) {
	sources := s.hub.sources()
	sources[0].Hostname = s.hostname
	return &krillv1.ListLogSourcesResponse{Sources: sources}, nil
}

func (s *Service) ListLogs(_ context.Context, req *krillv1.ListLogsRequest) (*krillv1.ListLogsResponse, error) {
	buf, ok := s.hub.buffer(req.GetSourceId())
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no logs from that source"))
	}
	limit := int(req.GetLimit())
	switch {
	case limit < 0:
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("limit must not be negative"))
	case limit == 0:
		limit = defaultLimit
	case limit > maxLimit:
		limit = maxLimit
	}
	records, last := buf.List(req.GetAfterSeq(), matcher(req.GetMinLevel(), req.GetQuery()), limit)
	return &krillv1.ListLogsResponse{Records: records, LastSeq: last}, nil
}

func matcher(minLevel krillv1.LogLevel, query string) func(*krillv1.LogRecord) bool {
	query = strings.ToLower(strings.TrimSpace(query))
	return func(r *krillv1.LogRecord) bool {
		if r.GetLevel() < minLevel {
			return false
		}
		if query == "" || strings.Contains(strings.ToLower(r.GetMessage()), query) {
			return true
		}
		for _, a := range r.GetAttrs() {
			if strings.Contains(strings.ToLower(a.GetKey()), query) || strings.Contains(strings.ToLower(a.GetValue()), query) {
				return true
			}
		}
		return false
	}
}
