package admin

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/mod/semver"

	krillv1 "github.com/wauv/krill/api/gen/krill/v1"
)

// Repo is where releases are tagged.
const Repo = "Wisconsin-AUV/krill-monorepo"

const (
	cacheFor     = 6 * time.Hour
	retryFailure = 15 * time.Minute
	maxBody      = 2 << 20
)

// Latest is the newest release and what changed since the running version.
type Latest struct {
	Version   string
	URL       string
	Releases  []*krillv1.Release
	CheckedAt time.Time
	Err       error
}

// Build is what a version string says about the commit it was built from.
type Build struct {
	// Release is the vX.Y.Z tag the build is from or after, or "".
	Release string
	Ahead   int
	Commit  string
	Dirty   bool
}

var (
	describeForm = regexp.MustCompile(`^(v?\d+\.\d+\.\d+)(?:-(\d+)-g([0-9a-f]+))?(-dirty)?$`)
	commitForm   = regexp.MustCompile(`^(?:dev-)?([0-9a-f]{7,40})(-dirty)?$`)
)

// ParseBuild reads a version as git describe --tags --always --dirty prints
// it, like v0.4.0, v0.4.0-6-g2d343e3, or a bare hash before any tag.
func ParseBuild(v string) Build {
	if m := describeForm.FindStringSubmatch(v); m != nil {
		b := Build{Release: canonical(m[1]), Commit: m[3], Dirty: m[4] != ""}
		b.Ahead, _ = strconv.Atoi(m[2])
		return b
	}
	if m := commitForm.FindStringSubmatch(v); m != nil {
		return Build{Commit: m[1], Dirty: m[2] != ""}
	}
	return Build{}
}

// Releases looks the latest release up on GitHub and caches the answer.
type Releases struct {
	build   Build
	current string
	client  *http.Client
	apiURL  string
	rawURL  string

	mu     sync.Mutex
	cached *Latest
}

func NewReleases(version string) *Releases {
	build := ParseBuild(version)
	return &Releases{
		build:   build,
		current: build.Release,
		client:  &http.Client{Timeout: 10 * time.Second},
		apiURL:  "https://api.github.com",
		rawURL:  "https://raw.githubusercontent.com",
	}
}

// canonical returns v as a vX.Y.Z semver, or "" for builds like "dev".
func canonical(v string) string {
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	if !semver.IsValid(v) {
		return ""
	}
	return v
}

// Status reports how the running version compares with the latest release.
func (r *Releases) Status(ctx context.Context, version string, enabled, refresh bool) *krillv1.VersionStatus {
	b := r.build
	out := &krillv1.VersionStatus{
		Current:      version,
		CheckEnabled: enabled,
		Release:      b.Release,
		CommitsAhead: int32(b.Ahead), //nolint:gosec // commit counts fit in int32
		Commit:       b.Commit,
		Dirty:        b.Dirty,
	}
	if !enabled {
		return out
	}
	l := r.latest(ctx, refresh)
	out.CheckedAt = ts(l.CheckedAt)
	if l.Err != nil {
		out.Error = l.Err.Error()
		return out
	}
	out.Latest = l.Version
	out.ReleaseUrl = l.URL
	switch {
	case r.current == "":
		out.Error = "builds without a release tag can't be compared with releases"
	case semver.Compare(r.current, l.Version) < 0:
		out.State = krillv1.UpdateState_UPDATE_STATE_AVAILABLE
		out.Releases = l.Releases
	case b.Ahead > 0 || b.Dirty:
		out.State = krillv1.UpdateState_UPDATE_STATE_AHEAD
	default:
		out.State = krillv1.UpdateState_UPDATE_STATE_UP_TO_DATE
	}
	return out
}

func (r *Releases) latest(ctx context.Context, refresh bool) *Latest {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c := r.cached; c != nil && !refresh {
		ttl := cacheFor
		if c.Err != nil {
			ttl = retryFailure
		}
		if time.Since(c.CheckedAt) < ttl {
			return c
		}
	}
	l := r.fetch(ctx)
	l.CheckedAt = time.Now()
	r.cached = l
	return l
}

func (r *Releases) fetch(ctx context.Context) *Latest {
	var tags []struct {
		Name string `json:"name"`
	}
	if err := r.get(ctx, fmt.Sprintf("%s/repos/%s/tags?per_page=100", r.apiURL, Repo), func(b io.Reader) error {
		return json.NewDecoder(b).Decode(&tags)
	}); err != nil {
		return &Latest{Err: fmt.Errorf("list releases: %w", err)}
	}
	latest := ""
	for _, t := range tags {
		v := canonical(t.Name)
		if v == "" || semver.Prerelease(v) != "" {
			continue
		}
		if latest == "" || semver.Compare(v, latest) > 0 {
			latest = v
		}
	}
	if latest == "" {
		return &Latest{Err: errors.New("no releases are tagged yet")}
	}
	l := &Latest{Version: latest, URL: fmt.Sprintf("https://github.com/%s/blob/%s/CHANGELOG.md", Repo, latest)}
	if r.current == "" || semver.Compare(r.current, latest) >= 0 {
		return l
	}
	// The changelog is extra. Without it the panel still says an update is out.
	_ = r.get(ctx, fmt.Sprintf("%s/%s/%s/CHANGELOG.md", r.rawURL, Repo, latest), func(b io.Reader) error {
		all, err := ParseChangelog(b)
		l.Releases = Since(all, r.current)
		return err
	})
	return l
}

func (r *Releases) get(ctx context.Context, url string, read func(io.Reader) error) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	res, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub answered %s", res.Status)
	}
	return read(io.LimitReader(res.Body, maxBody))
}

var (
	releaseHeading = regexp.MustCompile(`^## \[([^\]]+)\](?:\s*-\s*(\S+))?`)
	sectionHeading = regexp.MustCompile(`^### (.+)`)
)

// ParseChangelog reads a Keep a Changelog file, skipping Unreleased.
func ParseChangelog(r io.Reader) ([]*krillv1.Release, error) {
	var (
		out     []*krillv1.Release
		release *krillv1.Release
		section *krillv1.ReleaseSection
	)
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if m := releaseHeading.FindStringSubmatch(line); m != nil {
			release, section = nil, nil
			if v := canonical(m[1]); v != "" {
				release = &krillv1.Release{Version: v, Date: m[2]}
				out = append(out, release)
			}
			continue
		}
		if release == nil {
			continue
		}
		if m := sectionHeading.FindStringSubmatch(line); m != nil {
			section = &krillv1.ReleaseSection{Title: strings.TrimSpace(m[1])}
			release.Sections = append(release.Sections, section)
			continue
		}
		if section == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "- "):
			section.Items = append(section.Items, strings.TrimSpace(line[2:]))
		case strings.HasPrefix(line, "  ") && strings.TrimSpace(line) != "" && len(section.Items) > 0:
			section.Items[len(section.Items)-1] += " " + strings.TrimSpace(line)
		}
	}
	return out, sc.Err()
}

// Since keeps the releases newer than current.
func Since(releases []*krillv1.Release, current string) []*krillv1.Release {
	var out []*krillv1.Release
	for _, r := range releases {
		if semver.Compare(r.GetVersion(), current) > 0 {
			out = append(out, r)
		}
	}
	return out
}
