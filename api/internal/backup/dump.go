// Package backup dumps the database to the bucket with pg_dump, on request
// and on a daily schedule, and describes the database for the admin panel.
package backup

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

var ErrNoPgDump = errors.New("pg_dump is not installed on the API's host")

// Available reports why backups can't run, or nil when they can.
func Available() error {
	if _, err := exec.LookPath("pg_dump"); err != nil {
		return ErrNoPgDump
	}
	return nil
}

// connection moves the password out of a URL so it isn't on pg_dump's
// command line, where any process on the host can read it.
func connection(databaseURL string) (dsn, password string) {
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.User == nil {
		return databaseURL, ""
	}
	password, _ = u.User.Password()
	u.User = url.User(u.User.Username())
	return u.String(), password
}

// tail keeps the end of what's written to it, for pg_dump's error output.
type tail struct {
	buf bytes.Buffer
	max int
}

func (t *tail) Write(p []byte) (int, error) {
	t.buf.Write(p)
	if over := t.buf.Len() - t.max; over > 0 {
		t.buf.Next(over)
	}
	return len(p), nil
}

type counter struct {
	r io.Reader
	n int64
}

func (c *counter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// dump runs pg_dump in its custom format and passes its output to upload,
// returning the dump's size.
func dump(ctx context.Context, databaseURL string, upload func(context.Context, io.Reader) error) (int64, error) {
	path, err := exec.LookPath("pg_dump")
	if err != nil {
		return 0, ErrNoPgDump
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	dsn, password := connection(databaseURL)
	cmd := exec.CommandContext(ctx, path, "--format=custom", "--dbname="+dsn) //nolint:gosec // the URL comes from the API's environment
	cmd.Env = append(os.Environ(), "PGCONNECT_TIMEOUT=10")
	if password != "" {
		cmd.Env = append(cmd.Env, "PGPASSWORD="+password)
	}
	stderr := &tail{max: 4 << 10}
	cmd.Stderr = stderr
	out, err := cmd.StdoutPipe()
	if err != nil {
		return 0, fmt.Errorf("pg_dump output: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start pg_dump: %w", err)
	}
	r := &counter{r: out}
	uerr := upload(ctx, r)
	if uerr != nil {
		// pg_dump would otherwise block on a pipe nobody reads.
		cancel()
	}
	werr := cmd.Wait()
	if werr != nil {
		if msg := strings.TrimSpace(stderr.buf.String()); msg != "" && ctx.Err() == nil {
			werr = fmt.Errorf("pg_dump: %s", msg)
		} else {
			werr = fmt.Errorf("pg_dump: %w", werr)
		}
	}
	if uerr != nil {
		return 0, errors.Join(uerr, werr)
	}
	return r.n, werr
}
