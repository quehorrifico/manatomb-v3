package forge

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	origin, secret string
	http           *http.Client
}

func NewClient(enabled bool, origin, secret string) (*Client, error) {
	if !enabled {
		return nil, nil
	}
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || len(secret) < 32 {
		return nil, errors.New("CPU play disabled: requires FORGE_SERVICE_URL origin and FORGE_SERVICE_SECRET (32+ characters)")
	}
	return &Client{origin: origin, secret: secret, http: &http.Client{Timeout: 12 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}, nil
}
func (c *Client) Request(ctx context.Context, owner int64, method, path string, body []byte) (int, []byte, error) {
	if c == nil {
		return 503, nil, errors.New("CPU play is disabled")
	}
	if !strings.HasPrefix(path, "/v1/sessions") && !(method == "GET" && strings.HasPrefix(path, "/v1/defaults/") && validDefault(strings.TrimPrefix(path, "/v1/defaults/"))) {
		return 400, nil, errors.New("invalid engine path")
	}
	req, err := http.NewRequestWithContext(ctx, method, c.origin+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.secret)
	req.Header.Set("X-ManaTomb-Owner", strconv.FormatInt(owner, 10))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return 503, nil, err
	}
	defer res.Body.Close()
	data, err := io.ReadAll(io.LimitReader(res.Body, (2<<20)+1))
	if len(data) > 2<<20 {
		return 502, nil, errors.New("invalid engine response")
	}
	return res.StatusCode, data, err
}
