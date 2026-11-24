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
	"sync"
	"time"
)

type client struct {
	baseURL    *url.URL
	httpClient *http.Client
	authKey    string
	userID     string
	// libraries is shared across clones (one per provider instance): the
	// full library is fetched once per Terraform run and reused by every
	// data source instead of once per data source.
	libraries *libraryStore
}

// libraryStore caches the account library keyed by auth key. It lives
// behind a pointer so client clones (one per resource/data source) share
// it; entries are only added, never mutated, so readers are safe.
type libraryStore struct {
	mu     sync.Mutex
	byAuth map[string][]map[string]any
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
	AddonID      string
	TransportURL string
	Name         string
	Version      string
	Types        []string
	CatalogTypes []string
	Resources    []string
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

	// Credentials travel in request bodies, so anything but HTTPS would
	// send them in cleartext. Local test servers are exempt.
	if parsed.Scheme != "https" && !isLoopbackHost(parsed.Hostname()) {
		return nil, fmt.Errorf("base_url must use https, got %q", parsed.Scheme)
	}

	return &client{
		baseURL: parsed,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
		libraries: &libraryStore{byAuth: map[string][]map[string]any{}},
	}, nil
}

// clone returns an independent client sharing the transport. Auth state is
// per-copy: each resource or data source instance logs in on its own copy,
// so concurrent operations on different accounts never share credentials.
func (c *client) clone() *client {
	cp := *c
	return &cp
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

		var addonID string
		var name string
		var version string
		typesList := make([]string, 0)
		catalogTypes := make([]string, 0)
		resources := make([]string, 0)
		if manifest, ok := item["manifest"].(map[string]any); ok {
			if value, exists := manifest["id"].(string); exists {
				addonID = value
			}
			if value, exists := manifest["name"].(string); exists {
				name = value
			}
			if value, exists := manifest["version"].(string); exists {
				version = value
			}

			typesList = stringSliceFromAny(manifest["types"])

			catalogTypes = catalogTypesFromManifest(manifest["catalogs"])
			resources = resourceNamesFromManifest(manifest["resources"])
		}

		if addonID == "" {
			if value, exists := item["transportName"].(string); exists {
				addonID = value
			}
		}

		if name == "" {
			if value, exists := item["name"].(string); exists {
				name = value
			}
		}

		addOns = append(addOns, addon{
			AddonID:      addonID,
			TransportURL: transportURL,
			Name:         name,
			Version:      version,
			Types:        typesList,
			CatalogTypes: catalogTypes,
			Resources:    resources,
		})
	}

	return addOns, nil
}

func (c *client) SetInstalledAddons(ctx context.Context, transportURLs []string) error {
	if c.authKey == "" {
		return fmt.Errorf("not authenticated")
	}

	// Fetch manifests concurrently: each fetch is an independent HTTP
	// round trip, and lists grow with the user's collection. Results stay
	// in input order so the collection is deterministic.
	const maxManifestConcurrency = 4

	manifests := make([]map[string]any, len(transportURLs))
	fetchErrs := make([]error, len(transportURLs))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxManifestConcurrency)
	for i, transportURL := range transportURLs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			manifest, err := c.fetchManifest(ctx, transportURL)
			if err != nil {
				fetchErrs[i] = fmt.Errorf("unable to fetch addon manifest for %s: %w", transportURL, err)
				return
			}
			manifests[i] = manifest
		}()
	}
	wg.Wait()

	addons := make([]map[string]any, 0, len(transportURLs))
	for i, transportURL := range transportURLs {
		if fetchErrs[i] != nil {
			return fetchErrs[i]
		}

		transportName := ""
		if value, ok := manifests[i]["name"].(string); ok {
			transportName = value
		}

		addons = append(addons, map[string]any{
			"transportUrl":  transportURL,
			"transportName": transportName,
			"manifest":      manifests[i],
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
	if limit < 0 {
		return nil, fmt.Errorf("limit must not be negative, got %d", limit)
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
	if limit < 0 {
		return nil, fmt.Errorf("limit must not be negative, got %d", limit)
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

func (c *client) DeleteUser(ctx context.Context, password string) error {
	if c.authKey == "" {
		return fmt.Errorf("not authenticated")
	}

	_, err := c.request(ctx, "deleteUser", map[string]any{
		"password": password,
	}, c.authKey)
	if err != nil {
		return fmt.Errorf("stremio deleteUser error: %w", err)
	}

	return nil
}

func isLoopbackHost(host string) bool {
	return host == "localhost" || host == "127.0.0.1" || host == "::1"
}

// maxManifestBytes caps addon manifest downloads: transport URLs are
// user-supplied, and an unbounded read lets one bad URL exhaust memory.
const maxManifestBytes = 10 << 20 // 10 MiB

func (c *client) fetchManifest(ctx context.Context, transportURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, transportURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxManifestBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxManifestBytes)
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
	defer func() { _ = resp.Body.Close() }()

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

func (c *client) libraryItems(ctx context.Context) ([]map[string]any, error) {
	c.libraries.mu.Lock()
	if items, ok := c.libraries.byAuth[c.authKey]; ok {
		c.libraries.mu.Unlock()
		return items, nil
	}
	c.libraries.mu.Unlock()

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

	c.libraries.mu.Lock()
	c.libraries.byAuth[c.authKey] = items
	c.libraries.mu.Unlock()

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

func stringSliceFromAny(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(items))
	for _, item := range items {
		if itemValue, ok := item.(string); ok && itemValue != "" {
			result = append(result, itemValue)
		}
	}

	return result
}

func catalogTypesFromManifest(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		catalog, ok := item.(map[string]any)
		if !ok {
			continue
		}
		typeValue, _ := catalog["type"].(string)
		if typeValue == "" {
			continue
		}
		if _, exists := seen[typeValue]; exists {
			continue
		}
		seen[typeValue] = struct{}{}
		result = append(result, typeValue)
	}

	return result
}

func resourceNamesFromManifest(value any) []string {
	items, ok := value.([]any)
	if !ok {
		return []string{}
	}

	result := make([]string, 0, len(items))
	seen := map[string]struct{}{}
	for _, item := range items {
		var resourceName string
		switch converted := item.(type) {
		case string:
			resourceName = converted
		case map[string]any:
			resourceName, _ = converted["name"].(string)
		}

		if resourceName == "" {
			continue
		}
		if _, exists := seen[resourceName]; exists {
			continue
		}
		seen[resourceName] = struct{}{}
		result = append(result, resourceName)
	}

	return result
}
