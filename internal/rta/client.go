package rta

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"runtime"
	"strings"

	"github.com/openshift-online/rosa-trusted-actions/internal/openapi"
)

const basePath = "/api/v0/trusted-actions"

type Client struct {
	base      string
	token     string
	http      *http.Client
	userAgent string
}

type ClientOption func(*Client)

func WithToken(token string) ClientOption {
	return func(c *Client) { c.token = token }
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) { c.http = hc }
}

func NewClient(serverURL string, opts ...ClientOption) (*Client, error) {
	if serverURL == "" {
		return nil, fmt.Errorf("server URL is required; set --server or RTA_SERVER")
	}
	serverURL = strings.TrimRight(serverURL, "/")

	c := &Client{
		base:      serverURL + basePath,
		http:      http.DefaultClient,
		userAgent: userAgent(),
	}
	for _, o := range opts {
		o(c)
	}
	return c, nil
}

func userAgent() string {
	ua := "rta/" + Version

	/* If this ever becomes a rosactl plugin, use:
	if hostVersion := os.Getenv("ROSACTL_HOST_VERSION"); hostVersion != "" {
		ua += " rosactl/" + hostVersion
	} */

	ua += " (" + runtime.GOOS + "/" + runtime.GOARCH + ")"
	return ua
}

func (c *Client) Catalog(ctx context.Context) (*openapi.TrustedActionCatalog, error) {
	var catalog openapi.TrustedActionCatalog
	if err := c.get(ctx, "/", nil, &catalog); err != nil {
		return nil, err
	}
	return &catalog, nil
}

func (c *Client) Describe(ctx context.Context, action string) (*openapi.TrustedAction, error) {
	var ta openapi.TrustedAction
	if err := c.get(ctx, "/"+url.PathEscape(action), nil, &ta); err != nil {
		return nil, err
	}
	return &ta, nil
}

func (c *Client) CreateExecution(ctx context.Context, action string, req openapi.ExecutionRequest) (*openapi.Execution, error) {
	var exec openapi.Execution
	if err := c.post(ctx, "/"+url.PathEscape(action)+"/run", req, &exec); err != nil {
		return nil, err
	}
	return &exec, nil
}

func (c *Client) ListExecutions(ctx context.Context, params url.Values) (*openapi.ExecutionList, error) {
	var list openapi.ExecutionList
	if err := c.get(ctx, "/runs", params, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *Client) GetExecution(ctx context.Context, id string) (*openapi.Execution, error) {
	var exec openapi.Execution
	if err := c.get(ctx, "/runs/"+url.PathEscape(id), nil, &exec); err != nil {
		return nil, err
	}
	return &exec, nil
}

func (c *Client) GetExecutionOutput(ctx context.Context, id string) (*openapi.ExecutionOutput, error) {
	var output openapi.ExecutionOutput
	if err := c.get(ctx, "/runs/"+url.PathEscape(id)+"/output", nil, &output); err != nil {
		return nil, err
	}
	return &output, nil
}

func (c *Client) ListAuditEntries(ctx context.Context, params url.Values) (*openapi.AuditList, error) {
	var list openapi.AuditList
	if err := c.get(ctx, "/audit", params, &list); err != nil {
		return nil, err
	}
	return &list, nil
}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	u := c.base + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body any, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request body: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", req.Method, req.URL.Path, err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s: read response: %w", req.Method, req.URL.Path, err)
	}

	if resp.StatusCode >= 400 {
		return parseAPIError(req, resp.StatusCode, data)
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("%s %s: decode response: %w", req.Method, req.URL.Path, err)
		}
	}
	return nil
}

type APIError struct {
	Method     string
	Path       string
	StatusCode int
	Code       string
	Reason     string
}

func (e *APIError) Error() string {
	if e.Reason != "" {
		return fmt.Sprintf("%s %s: %d %s: %s", e.Method, e.Path, e.StatusCode, e.Code, e.Reason)
	}
	return fmt.Sprintf("%s %s: %d %s", e.Method, e.Path, e.StatusCode, http.StatusText(e.StatusCode))
}

func parseAPIError(req *http.Request, statusCode int, body []byte) error {
	apiErr := &APIError{
		Method:     req.Method,
		Path:       req.URL.Path,
		StatusCode: statusCode,
	}
	var errResp openapi.Error
	if json.Unmarshal(body, &errResp) == nil && errResp.Code != "" {
		apiErr.Code = errResp.Code
		apiErr.Reason = errResp.Reason
	}
	return apiErr
}