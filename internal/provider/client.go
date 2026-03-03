package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type client struct {
	baseURL    *url.URL
	httpClient *http.Client
	authKey    string
	userID     string
	email      string
}

type loginResult struct {
	AuthKey string `json:"authKey"`
	User    struct {
		ID    string `json:"_id"`
		Email string `json:"email"`
	} `json:"user"`
}

type addonCollectionResult struct {
	Addons []map[string]any `json:"addons"`
}

type addon struct {
	TransportURL string
	Name         string
}

type apiEnvelope struct {
	Result json.RawMessage `json:"result"`
	Error  any             `json:"error"`
}

func newClient(baseURL string) (*client, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("base_url must include scheme and host")
	}

	return &client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}, nil
}

func (c *client) Register(ctx context.Context, email, password string) error {
	payload := map[string]any{
		"email":    email,
		"password": password,
	}

	body, err := c.request(ctx, "register", payload, "")
	if err != nil {
		return err
	}

	_ = body

	return c.Login(ctx, email, password)
}

func (c *client) Login(ctx context.Context, email, password string) error {
	payload := map[string]any{
		"email":    email,
		"password": password,
	}

	body, err := c.request(ctx, "login", payload, "")
	if err != nil {
		return err
	}

	var out loginResult
	if err := json.Unmarshal(body, &out); err != nil {
		return err
	}

	if out.AuthKey == "" {
		return fmt.Errorf("empty auth key in login response")
	}

	c.authKey = out.AuthKey
	c.userID = out.User.ID
	c.email = out.User.Email

	return nil
}

func (c *client) InstalledAddons(ctx context.Context) ([]addon, error) {
	if c.authKey == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	body, err := c.request(ctx, "addonCollectionGet", map[string]any{}, c.authKey)
	if err != nil {
		return nil, err
	}

	var out addonCollectionResult
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, err
	}

	addOns := make([]addon, 0, len(out.Addons))
	for _, item := range out.Addons {
		transportURL, _ := item["transportUrl"].(string)

		var name string
		if manifest, ok := item["manifest"].(map[string]any); ok {
			if value, exists := manifest["name"].(string); exists {
				name = value
			}
		}

		if name == "" {
			if value, exists := item["name"].(string); exists {
				name = value
			}
		}

		addOns = append(addOns, addon{
			TransportURL: transportURL,
			Name:         name,
		})
	}

	return addOns, nil
}

func (c *client) SetInstalledAddons(ctx context.Context, transportURLs []string) error {
	if c.authKey == "" {
		return fmt.Errorf("not authenticated")
	}

	addons := make([]map[string]any, 0, len(transportURLs))
	for _, transportURL := range transportURLs {
		manifest, err := c.fetchManifest(ctx, transportURL)
		if err != nil {
			return fmt.Errorf("unable to fetch addon manifest for %s: %w", transportURL, err)
		}

		transportName := ""
		if value, ok := manifest["name"].(string); ok {
			transportName = value
		}

		addons = append(addons, map[string]any{
			"transportUrl":  transportURL,
			"transportName": transportName,
			"manifest":      manifest,
		})
	}

	_, err := c.request(ctx, "addonCollectionSet", map[string]any{
		"addons": addons,
	}, c.authKey)
	if err != nil {
		return err
	}

	return nil
}

func (c *client) WatchHistory(ctx context.Context, limit int64) ([]map[string]any, error) {
	if c.authKey == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	items, err := c.libraryItems(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		state, _ := item["state"].(map[string]any)
		timeOffset, _ := int64FromAny(state["timeOffset"])
		timeWatched, _ := int64FromAny(state["timeWatched"])
		timesWatched, _ := int64FromAny(state["timesWatched"])
		lastWatched, _ := state["lastWatched"].(string)

		if timeOffset > 0 || timeWatched > 0 || timesWatched > 0 || lastWatched != "" {
			entries = append(entries, item)
		}
	}
	if limit > 0 && int64(len(entries)) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

func (c *client) ContinueWatching(ctx context.Context, limit int64) ([]map[string]any, error) {
	if c.authKey == "" {
		return nil, fmt.Errorf("not authenticated")
	}

	items, err := c.libraryItems(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]map[string]any, 0, len(items))
	for _, item := range items {
		state, _ := item["state"].(map[string]any)
		typeValue, _ := item["type"].(string)
		removed, _ := boolFromAny(item["removed"])
		temp, _ := boolFromAny(item["temp"])
		timeOffset, _ := int64FromAny(state["timeOffset"])

		if typeValue == "other" {
			continue
		}
		if removed && !temp {
			continue
		}
		if timeOffset <= 0 {
			continue
		}

		entries = append(entries, item)
	}
	if limit > 0 && int64(len(entries)) > limit {
		entries = entries[:limit]
	}

	return entries, nil
}

func (c *client) fetchManifest(ctx context.Context, transportURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transportURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("manifest request failed with status %d", resp.StatusCode)
	}

	var manifest map[string]any
	if err := json.Unmarshal(body, &manifest); err != nil {
		return nil, err
	}

	return manifest, nil
}

func (c *client) request(ctx context.Context, method string, params map[string]any, authKey string) ([]byte, error) {
	payload := map[string]any{}
	for key, value := range params {
		payload[key] = value
	}
	payload["authKey"] = authKey

	requestBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	fullURL := strings.TrimRight(c.baseURL.String(), "/") + "/api/" + method

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fullURL, bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("request failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var envelope apiEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, err
	}

	if envelope.Error != nil {
		return nil, fmt.Errorf("stremio %s error: %v", method, envelope.Error)
	}

	if len(envelope.Result) == 0 {
		return nil, fmt.Errorf("stremio %s response has no result", method)
	}

	return envelope.Result, nil
}

func extractHistoryEntries(payload any) []map[string]any {
	switch value := payload.(type) {
	case []any:
		entries := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if object, ok := item.(map[string]any); ok {
				entries = append(entries, object)
			}
		}
		return entries
	case map[string]any:
		for _, key := range []string{"items", "history", "entries", "results", "rows", "videos"} {
			if nested, ok := value[key]; ok {
				entries := extractHistoryEntries(nested)
				if len(entries) > 0 {
					return entries
				}
			}
		}

		if _, ok := value["state"]; ok {
			return []map[string]any{value}
		}

		entries := make([]map[string]any, 0, len(value))
		for _, item := range value {
			if object, ok := item.(map[string]any); ok {
				entries = append(entries, object)
			}
		}
		return entries
	default:
		return []map[string]any{}
	}
}

func (c *client) libraryItems(ctx context.Context) ([]map[string]any, error) {
	body, err := c.request(ctx, "datastoreGet", map[string]any{
		"collection": "libraryItem",
		"ids":        []string{},
		"all":        true,
	}, c.authKey)
	if err != nil {
		return nil, err
	}

	var items []map[string]any
	if err := json.Unmarshal(body, &items); err != nil {
		return nil, err
	}

	return items, nil
}

func int64FromAny(value any) (int64, bool) {
	switch converted := value.(type) {
	case int:
		return int64(converted), true
	case int32:
		return int64(converted), true
	case int64:
		return converted, true
	case float32:
		return int64(converted), true
	case float64:
		return int64(converted), true
	default:
		return 0, false
	}
}

func boolFromAny(value any) (bool, bool) {
	converted, ok := value.(bool)
	return converted, ok
}
