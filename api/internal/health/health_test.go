package health

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
	"github.com/wauv/krill/api/gen/krill/v1/krillv1connect"
)

func TestCheck(t *testing.T) {
	mux := http.NewServeMux()
	mux.Handle(krillv1connect.NewHealthServiceHandler(NewService("v1.2.3")))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	client := krillv1connect.NewHealthServiceClient(srv.Client(), srv.URL)
	res, err := client.Check(context.Background(), &krillv1.CheckRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if res.GetVersion() != "v1.2.3" {
		t.Errorf("Version = %q, want v1.2.3", res.GetVersion())
	}
}
