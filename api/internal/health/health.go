package health

import (
	"context"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

type Service struct {
	krillv1connect.UnimplementedHealthServiceHandler
	version string
}

func NewService(version string) *Service {
	return &Service{version: version}
}

func (s *Service) Check(_ context.Context, _ *krillv1.CheckRequest) (*krillv1.CheckResponse, error) {
	return &krillv1.CheckResponse{Version: s.version}, nil
}
