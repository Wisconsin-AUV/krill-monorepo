package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Resend sends email through the Resend API.
type Resend struct {
	apiKey  string
	from    string
	client  *http.Client
	baseURL string
}

func NewResend(apiKey, from string) *Resend {
	return &Resend{
		apiKey:  apiKey,
		from:    from,
		client:  &http.Client{Timeout: 10 * time.Second},
		baseURL: "https://api.resend.com",
	}
}

func (r *Resend) Send(ctx context.Context, m Message) error {
	body, err := json.Marshal(map[string]any{
		"from":    r.from,
		"to":      []string{m.To},
		"subject": m.Subject,
		"html":    m.HTML,
		"text":    m.Text,
	})
	if err != nil {
		return fmt.Errorf("encode email: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, r.baseURL+"/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build email request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "krill")
	res, err := r.client.Do(req)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 1024))
		return fmt.Errorf("send email: resend returned %s: %s", res.Status, bytes.TrimSpace(detail))
	}
	return nil
}
