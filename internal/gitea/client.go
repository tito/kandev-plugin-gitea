package gitea

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	maxResponseBytes = 1 << 20 // 1 MiB
	defaultTimeout   = 30 * time.Second
)

type Client struct {
	baseURL    *url.URL
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) (*Client, error) {
	parsed, err := validateBaseURL(baseURL)
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: parsed,
		token:   token,
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
	}, nil
}

func validateBaseURL(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("gitea: base URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("gitea: invalid base URL: %w", err)
	}
	if parsed.Scheme != "https" {
		return nil, fmt.Errorf("gitea: base URL must use HTTPS")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("gitea: base URL must include a host")
	}
	if parsed.User != nil {
		return nil, fmt.Errorf("gitea: base URL must not include credentials")
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	if !strings.HasSuffix(parsed.Path, "/") {
		parsed.Path += "/"
	}
	return parsed, nil
}

func (c *Client) apiURL(path string, query url.Values) string {
	ref, _ := url.Parse("api/v1/" + strings.TrimPrefix(path, "/"))
	resolved := c.baseURL.ResolveReference(ref)
	if query != nil {
		resolved.RawQuery = query.Encode()
	}
	return resolved.String()
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("gitea: encode request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.apiURL(path, query), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("gitea: create request: %w", err)
	}
	req.Header.Set("Authorization", "token "+c.token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.httpClient.Do(req)
}

func (c *Client) get(ctx context.Context, path string, query url.Values, result any) error {
	resp, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, result)
}

func (c *Client) post(ctx context.Context, path string, body any, result any) error {
	resp, err := c.do(ctx, http.MethodPost, path, nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, result)
}

func (c *Client) patch(ctx context.Context, path string, body any, result any) error {
	resp, err := c.do(ctx, http.MethodPatch, path, nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return decodeResponse(resp, result)
}

func (c *Client) delete(ctx context.Context, path string) error {
	resp, err := c.do(ctx, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return readError(resp)
	}
	return nil
}

type Page[T any] struct {
	Items      []T
	TotalCount int
	NextPage   int
}

func getPage[T any](ctx context.Context, c *Client, path string, query url.Values, page, limit int) (Page[T], error) {
	if query == nil {
		query = url.Values{}
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("limit", strconv.Itoa(limit))

	resp, err := c.do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return Page[T]{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return Page[T]{}, readError(resp)
	}

	var items []T
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(&items); err != nil {
		return Page[T]{}, fmt.Errorf("gitea: decode response: %w", err)
	}

	totalCount := 0
	if tc := resp.Header.Get("X-Total-Count"); tc != "" {
		if n, err := strconv.Atoi(tc); err == nil {
			totalCount = n
		}
	}

	nextPage := 0
	if len(items) == limit {
		nextPage = page + 1
	}

	return Page[T]{Items: items, TotalCount: totalCount, NextPage: nextPage}, nil
}

func decodeResponse(resp *http.Response, result any) error {
	if resp.StatusCode >= 400 {
		return readError(resp)
	}
	if result == nil {
		return nil
	}
	return json.NewDecoder(io.LimitReader(resp.Body, maxResponseBytes)).Decode(result)
}

func readError(resp *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	var apiErr APIError
	if json.Unmarshal(body, &apiErr) != nil || apiErr.Message == "" {
		apiErr.Message = strings.TrimSpace(string(body))
		if apiErr.Message == "" {
			apiErr.Message = http.StatusText(resp.StatusCode)
		}
	}
	return mapHTTPError(resp.StatusCode, apiErr.Message)
}

func (c *Client) BaseURL() string {
	return strings.TrimSuffix(c.baseURL.String(), "/")
}

// SetHTTPClient replaces the underlying HTTP client; used by tests to trust a local TLS server.
func (c *Client) SetHTTPClient(httpClient *http.Client) {
	if httpClient != nil {
		c.httpClient = httpClient
	}
}
