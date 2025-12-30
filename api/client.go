package api

import (
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
	defaultPublicBaseURL = "https://api.playtomic.io/v1"
	defaultAPIBaseURL    = "https://api.playtomic.io/v1"
	defaultAuthBaseURL   = "https://api.playtomic.io/v3"
	defaultUserAgent     = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3"
	defaultRequestedWith = "com.playtomic.web"
)

type Client struct {
	HTTP          *http.Client
	PublicBaseURL string
	APIBaseURL    string
	AuthBaseURL   string
	UserAgent     string
	RequestedWith string
	AccessToken   string
}

func NewClient() *Client {
	return &Client{
		HTTP:          &http.Client{Timeout: 15 * time.Second},
		PublicBaseURL: defaultPublicBaseURL,
		APIBaseURL:    defaultAPIBaseURL,
		AuthBaseURL:   defaultAuthBaseURL,
		UserAgent:     defaultUserAgent,
		RequestedWith: defaultRequestedWith,
	}
}

func (c *Client) GetTenants(ctx context.Context, lat, lon float64, radius int) ([]Tenant, error) {
	q := url.Values{}
	q.Set("sport_id", "PADEL")
	q.Set("coordinate", fmt.Sprintf("%.6f,%.6f", lat, lon))
	q.Set("radius", strconv.Itoa(radius))

	req, err := c.newPublicRequest(ctx, http.MethodGet, "/tenants", q)
	if err != nil {
		return nil, err
	}

	var tenants []Tenant
	if err := c.doJSON(req, &tenants); err != nil {
		return nil, err
	}
	return tenants, nil
}

func (c *Client) GetTenant(ctx context.Context, tenantID string) (Tenant, error) {
	path := "/tenants/" + url.PathEscape(tenantID)
	req, err := c.newPublicRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return Tenant{}, err
	}

	var tenant Tenant
	if err := c.doJSON(req, &tenant); err != nil {
		return Tenant{}, err
	}
	return tenant, nil
}

func (c *Client) GetResources(ctx context.Context, tenantID string) ([]Resource, error) {
	path := "/tenants/" + url.PathEscape(tenantID) + "/resources"
	req, err := c.newPublicRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}

	var resources []Resource
	if err := c.doJSON(req, &resources); err != nil {
		return nil, err
	}
	return resources, nil
}

func (c *Client) GetAvailability(ctx context.Context, tenantID string, start, end time.Time) ([]AvailabilityResource, error) {
	q := url.Values{}
	q.Set("sport_id", "PADEL")
	q.Set("tenant_id", tenantID)
	q.Set("start_min", start.Format("2006-01-02T15:04:05"))
	q.Set("start_max", end.Format("2006-01-02T15:04:05"))

	req, err := c.newPublicRequest(ctx, http.MethodGet, "/availability", q)
	if err != nil {
		return nil, err
	}

	var availability []AvailabilityResource
	if err := c.doJSON(req, &availability); err != nil {
		return nil, err
	}
	return availability, nil
}

func (c *Client) Geocode(ctx context.Context, query string) (float64, float64, error) {
	endpoint := "https://nominatim.openstreetmap.org/search"
	q := url.Values{}
	q.Set("format", "json")
	q.Set("limit", "1")
	q.Set("q", query)
	urlStr := endpoint + "?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return 0, 0, fmt.Errorf("geocode failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var results []struct {
		Lat string `json:"lat"`
		Lon string `json:"lon"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return 0, 0, err
	}
	if len(results) == 0 {
		return 0, 0, fmt.Errorf("no results for %q", query)
	}
	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return 0, 0, err
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return 0, 0, err
	}
	return lat, lon, nil
}

func (c *Client) newPublicRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	return c.newRequest(ctx, c.PublicBaseURL, method, path, query, nil, false)
}

func (c *Client) newAPIRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	return c.newRequest(ctx, c.APIBaseURL, method, path, query, nil, true)
}

func (c *Client) newAuthRequest(ctx context.Context, method, path string, query url.Values) (*http.Request, error) {
	return c.newRequest(ctx, c.AuthBaseURL, method, path, query, nil, false)
}

func (c *Client) newRequest(ctx context.Context, baseURL, method, path string, query url.Values, body io.Reader, useAuth bool) (*http.Request, error) {
	base, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}
	path = strings.TrimPrefix(path, "/")
	base.Path = strings.TrimSuffix(base.Path, "/") + "/" + path
	if query != nil {
		base.RawQuery = query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, base.String(), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.RequestedWith != "" {
		req.Header.Set("X-Requested-With", c.RequestedWith)
	}
	if useAuth && c.AccessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.AccessToken)
	}
	return req, nil
}

func (c *Client) doJSON(req *http.Request, dest any) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	if dest == nil {
		return nil
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(dest); err != nil {
		if err == io.EOF {
			return nil
		}
		return err
	}
	return nil
}

func (c *Client) doStatus(req *http.Request) error {
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed: %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}
	return nil
}
