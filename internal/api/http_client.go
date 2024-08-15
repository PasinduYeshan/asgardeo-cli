package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/shashimalcse/asgardeo-cli/internal/config"
	"go.uber.org/zap"
)

type httpClient struct {
	client   *http.Client
	baseUrl  *url.URL
	basepath string
	token    string
	logger   *zap.Logger
}

func NewHTTPClientAPI(cfg *config.Config, tenantDomain string, logger *zap.Logger) (*httpClient, error) {
	tenant, err := cfg.GetTenant(tenantDomain)
	if err != nil {
		logger.Error("failed to get tenant while creating http client", zap.Error(err))
		return nil, err
	}
	basepath := "t/" + tenant.Name + "/api/server/v1"
	u, err := url.Parse("https://api.asgardeo.io/")
	if err != nil {
		logger.Error("failed to parse base URL while creating http client", zap.Error(err))
		return nil, err
	}
	return &httpClient{client: &http.Client{}, basepath: basepath, baseUrl: u, token: tenant.GetAccessToken(), logger: logger}, nil
}

func (c *httpClient) Request(ctx context.Context, method, uri string, payload interface{}) error {
	request, err := c.NewRequest(ctx, method, uri, payload)
	if err != nil {
		return fmt.Errorf("failed to create a new request: %w", err)
	}
	response, err := c.Do(request)
	if err != nil {
		c.logger.Error("failed to send the request with http client", zap.String("method", method), zap.String("uri", uri), zap.Error(err))
		return fmt.Errorf("failed to send the request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusBadRequest {
		c.logger.Error("received an error response from the server", zap.String("method", method), zap.String("uri", uri), zap.Int("status_code", response.StatusCode))
		return newError(response)
	}
	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return fmt.Errorf("failed to read the response body: %w", err)
	}
	if len(responseBody) > 0 && string(responseBody) != "{}" {
		if err = json.Unmarshal(responseBody, &payload); err != nil {
			return fmt.Errorf("failed to unmarshal response payload: %w", err)
		}
	}
	return nil
}

func (c *httpClient) NewRequest(ctx context.Context, method, uri string, payload interface{}) (*http.Request, error) {
	const nullBody = "null\n"
	var body bytes.Buffer
	if payload != nil {
		if err := json.NewEncoder(&body).Encode(payload); err != nil {
			return nil, fmt.Errorf("encoding request payload failed: %w", err)
		}
	}
	if body.String() == nullBody {
		body.Reset()
	}
	request, err := http.NewRequestWithContext(ctx, method, uri, &body)
	if err != nil {
		return nil, err
	}
	request.Header.Add("Content-Type", "application/json")
	return request, nil
}

func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	ctx := req.Context()
	req.Header.Set("Authorization", "Bearer "+c.token)
	response, err := c.client.Do(req)
	if err != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return nil, err
		}
	}
	return response, nil
}

func (c *httpClient) URI(path ...string) string {
	baseURL := &url.URL{
		Scheme: c.baseUrl.Scheme,
		Host:   c.baseUrl.Host,
		Path:   c.basepath + "/",
	}
	const escapedForwardSlash = "%2F"
	var escapedPath []string
	for _, unescapedPath := range path {
		defaultPathEscaped := url.PathEscape(unescapedPath)
		escapedPath = append(
			escapedPath,
			strings.ReplaceAll(defaultPathEscaped, "/", escapedForwardSlash),
		)
	}
	return baseURL.String() + strings.Join(escapedPath, "/")
}
