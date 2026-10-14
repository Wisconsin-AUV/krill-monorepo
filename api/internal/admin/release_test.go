package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

const changelog = `# Changelog

## [Unreleased]

- Not out yet.

## [0.5.0] - 2026-11-01

### Added

- Admin panel.
- A long entry that wraps
  onto a second line.

## [0.4.0] - 2026-10-14

### Fixed

- Something old.
`

func fakeGitHub(t *testing.T, tags string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/tags"):
			_, _ = w.Write([]byte(tags))
		case strings.HasSuffix(r.URL.Path, "/v0.5.0/CHANGELOG.md"):
			_, _ = w.Write([]byte(changelog))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func testReleases(srv *httptest.Server, current string) *Releases {
	r := NewReleases(current)
	r.apiURL, r.rawURL = srv.URL, srv.URL
	return r
}

func TestUpdateAvailableListsOnlyNewerReleases(t *testing.T) {
	// Tags come back in no useful order, and prereleases must not count.
	srv := fakeGitHub(t, `[{"name":"v0.4.0"},{"name":"v0.6.0-rc.1"},{"name":"v0.5.0"},{"name":"v0.10.0-beta"},{"name":"latest"}]`)
	st := testReleases(srv, "v0.4.0").Status(t.Context(), "v0.4.0", true, false)

	if st.GetState() != krillv1.UpdateState_UPDATE_STATE_AVAILABLE || st.GetLatest() != "v0.5.0" {
		t.Fatalf("state = %v, latest = %q, err = %q", st.GetState(), st.GetLatest(), st.GetError())
	}
	if len(st.GetReleases()) != 1 || st.GetReleases()[0].GetVersion() != "v0.5.0" {
		t.Fatalf("releases = %v", st.GetReleases())
	}
	items := st.GetReleases()[0].GetSections()[0].GetItems()
	if len(items) != 2 || items[1] != "A long entry that wraps onto a second line." {
		t.Errorf("items = %q", items)
	}
}

func TestUpToDateAndDevBuilds(t *testing.T) {
	srv := fakeGitHub(t, `[{"name":"v0.5.0"}]`)
	// Builds without the v prefix still compare.
	if st := testReleases(srv, "0.5.0").Status(t.Context(), "0.5.0", true, false); st.GetState() != krillv1.UpdateState_UPDATE_STATE_UP_TO_DATE {
		t.Errorf("0.5.0 state = %v, err = %q", st.GetState(), st.GetError())
	}
	st := testReleases(srv, "dev").Status(t.Context(), "dev", true, false)
	if st.GetState() != krillv1.UpdateState_UPDATE_STATE_UNSPECIFIED || st.GetLatest() != "v0.5.0" || st.GetError() == "" {
		t.Errorf("dev state = %v, latest = %q, err = %q", st.GetState(), st.GetLatest(), st.GetError())
	}
}

func TestCheckOffMakesNoRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("GitHub was called with the check off")
	}))
	defer srv.Close()
	st := testReleases(srv, "v0.4.0").Status(t.Context(), "v0.4.0", false, true)
	if st.GetCheckEnabled() || st.GetLatest() != "" {
		t.Errorf("status = %v", st)
	}
}

func TestBuildsAfterARelease(t *testing.T) {
	srv := fakeGitHub(t, `[{"name":"v0.4.0"},{"name":"v0.5.0"}]`)
	tests := []struct {
		version string
		state   krillv1.UpdateState
		ahead   int32
		commit  string
	}{
		// Commits after the latest release are ahead, not behind it as semver
		// would read v0.5.0-6-g2d343e3.
		{"v0.5.0-6-g2d343e3", krillv1.UpdateState_UPDATE_STATE_AHEAD, 6, "2d343e3"},
		{"v0.5.0-dirty", krillv1.UpdateState_UPDATE_STATE_AHEAD, 0, ""},
		// Commits after an older release still need the update.
		{"v0.4.0-6-g2d343e3", krillv1.UpdateState_UPDATE_STATE_AVAILABLE, 6, "2d343e3"},
		{"dev-2d343e3", krillv1.UpdateState_UPDATE_STATE_UNSPECIFIED, 0, "2d343e3"},
	}
	for _, tt := range tests {
		st := testReleases(srv, tt.version).Status(t.Context(), tt.version, true, false)
		if st.GetState() != tt.state || st.GetCommitsAhead() != tt.ahead || st.GetCommit() != tt.commit {
			t.Errorf("%s: state = %v, ahead = %d, commit = %q", tt.version, st.GetState(), st.GetCommitsAhead(), st.GetCommit())
		}
	}
}
