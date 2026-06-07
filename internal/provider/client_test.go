package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestNewClient(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "valid https", baseURL: "https://api.strem.io", wantErr: false},
		{name: "valid with path", baseURL: "https://example.com/base", wantErr: false},
		{name: "missing scheme", baseURL: "api.strem.io", wantErr: true},
		{name: "missing host", baseURL: "https://", wantErr: true},
		{name: "empty", baseURL: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			c, err := newClient(tt.baseURL)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q, got nil", tt.baseURL)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if c.httpClient == nil {
				t.Fatal("expected httpClient to be initialised")
			}
		})
	}
}

// newTestClient points a client at a test server.
func newTestClient(t *testing.T, baseURL string) *client {
	t.Helper()
	c, err := newClient(baseURL)
	if err != nil {
		t.Fatalf("newClient: %v", err)
	}
	return c
}

// writeEnvelope writes a Stremio-style API envelope to the response.
func writeEnvelope(t *testing.T, w http.ResponseWriter, result any, apiErr any) {
	t.Helper()
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	payload := map[string]any{"result": json.RawMessage(raw)}
	if apiErr != nil {
		payload["error"] = apiErr
	}
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("encode envelope: %v", err)
	}
}

func TestRequestSuccess(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/login" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		if err := json.Unmarshal(body, &got); err != nil {
			t.Errorf("decode body: %v", err)
		}
		if got["authKey"] != "secret" {
			t.Errorf("expected authKey to be injected, got %v", got["authKey"])
		}
		writeEnvelope(t, w, map[string]any{"ok": true}, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	body, err := c.request(context.Background(), "login", map[string]any{"email": "a@b.c"}, "secret")
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if !strings.Contains(string(body), "\"ok\":true") {
		t.Fatalf("unexpected result body: %s", body)
	}
}

func TestRequestAPIError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, map[string]any{}, map[string]any{"message": "bad creds"})
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.request(context.Background(), "login", map[string]any{}, "")
	if err == nil {
		t.Fatal("expected error from API error envelope")
	}
	if !strings.Contains(err.Error(), "bad creds") {
		t.Fatalf("expected error to surface API message, got: %v", err)
	}
}

func TestRequestHTTPError(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, "boom")
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.request(context.Background(), "login", map[string]any{}, "")
	if err == nil {
		t.Fatal("expected error from HTTP 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Fatalf("expected status code in error, got: %v", err)
	}
}

func TestRequestEmptyResult(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{}`)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	_, err := c.request(context.Background(), "login", map[string]any{}, "")
	if err == nil {
		t.Fatal("expected error for missing result")
	}
}

func TestLogin(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, map[string]any{
			"authKey": "auth-123",
			"user":    map[string]any{"_id": "user-1", "email": "a@b.c"},
		}, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if err := c.Login(context.Background(), "a@b.c", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if c.authKey != "auth-123" || c.userID != "user-1" || c.email != "a@b.c" {
		t.Fatalf("login did not populate client: %+v", c)
	}
}

func TestLoginEmptyAuthKey(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, map[string]any{"authKey": ""}, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if err := c.Login(context.Background(), "a@b.c", "pw"); err == nil {
		t.Fatal("expected error for empty auth key")
	}
}

func TestInstalledAddons(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, map[string]any{
			"addons": []map[string]any{
				{
					"transportUrl": "https://addon.example/manifest.json",
					"manifest": map[string]any{
						"id":      "org.example",
						"name":    "Example",
						"version": "1.2.3",
						"types":   []any{"movie", "series"},
						"catalogs": []any{
							map[string]any{"type": "movie"},
							map[string]any{"type": "movie"},
							map[string]any{"type": "series"},
						},
						"resources": []any{"catalog", map[string]any{"name": "stream"}},
					},
				},
			},
		}, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.authKey = "auth-123"

	addons, err := c.InstalledAddons(context.Background())
	if err != nil {
		t.Fatalf("InstalledAddons: %v", err)
	}
	if len(addons) != 1 {
		t.Fatalf("expected 1 addon, got %d", len(addons))
	}
	got := addons[0]
	if got.AddonID != "org.example" || got.Name != "Example" || got.Version != "1.2.3" {
		t.Fatalf("unexpected addon metadata: %+v", got)
	}
	if !reflect.DeepEqual(got.Types, []string{"movie", "series"}) {
		t.Fatalf("unexpected types: %v", got.Types)
	}
	if !reflect.DeepEqual(got.CatalogTypes, []string{"movie", "series"}) {
		t.Fatalf("expected deduped catalog types, got: %v", got.CatalogTypes)
	}
	if !reflect.DeepEqual(got.Resources, []string{"catalog", "stream"}) {
		t.Fatalf("unexpected resources: %v", got.Resources)
	}
}

func TestInstalledAddonsNotAuthenticated(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, "https://api.strem.io")
	if _, err := c.InstalledAddons(context.Background()); err == nil {
		t.Fatal("expected error when not authenticated")
	}
}

func TestContinueWatchingFiltering(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"type": "movie", "state": map[string]any{"timeOffset": float64(120)}},                       // keep
		{"type": "other", "state": map[string]any{"timeOffset": float64(120)}},                       // drop: type other
		{"type": "series", "state": map[string]any{"timeOffset": float64(0)}},                        // drop: no offset
		{"type": "movie", "removed": true, "state": map[string]any{"timeOffset": 5.0}},               // drop: removed
		{"type": "movie", "removed": true, "temp": true, "state": map[string]any{"timeOffset": 5.0}}, // keep: temp
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, items, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.authKey = "auth-123"

	entries, err := c.ContinueWatching(context.Background(), 0)
	if err != nil {
		t.Fatalf("ContinueWatching: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 in-progress entries, got %d", len(entries))
	}
}

func TestWatchHistoryLimit(t *testing.T) {
	t.Parallel()

	items := []map[string]any{
		{"state": map[string]any{"timeWatched": float64(10)}},
		{"state": map[string]any{"timesWatched": float64(1)}},
		{"state": map[string]any{"lastWatched": "2026-01-01"}},
		{"state": map[string]any{}}, // drop: never watched
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, items, nil)
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	c.authKey = "auth-123"

	entries, err := c.WatchHistory(context.Background(), 2)
	if err != nil {
		t.Fatalf("WatchHistory: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected limit of 2, got %d", len(entries))
	}
}

func TestSetInstalledAddons(t *testing.T) {
	t.Parallel()

	var collectionSet map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": "org.example", "name": "Example"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/api/addonCollectionSet", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &collectionSet)
		writeEnvelope(t, w, map[string]any{"success": true}, nil)
	})

	c := newTestClient(t, srv.URL)
	c.authKey = "auth-123"

	if err := c.SetInstalledAddons(context.Background(), []string{srv.URL + "/manifest.json"}); err != nil {
		t.Fatalf("SetInstalledAddons: %v", err)
	}

	addons, ok := collectionSet["addons"].([]any)
	if !ok || len(addons) != 1 {
		t.Fatalf("expected 1 addon in collectionSet payload, got: %v", collectionSet["addons"])
	}
	entry := addons[0].(map[string]any)
	if entry["transportName"] != "Example" {
		t.Fatalf("expected transportName from manifest, got: %v", entry["transportName"])
	}
}

func TestInt64FromAny(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   any
		want int64
		ok   bool
	}{
		{in: int(5), want: 5, ok: true},
		{in: int32(7), want: 7, ok: true},
		{in: int64(9), want: 9, ok: true},
		{in: float64(12.9), want: 12, ok: true},
		{in: "nope", want: 0, ok: false},
		{in: nil, want: 0, ok: false},
	}
	for _, tt := range tests {
		got, ok := int64FromAny(tt.in)
		if got != tt.want || ok != tt.ok {
			t.Errorf("int64FromAny(%v) = (%d, %t), want (%d, %t)", tt.in, got, ok, tt.want, tt.ok)
		}
	}
}

func TestStringSliceFromAny(t *testing.T) {
	t.Parallel()

	got := stringSliceFromAny([]any{"a", "", "b", 3})
	if !reflect.DeepEqual(got, []string{"a", "b"}) {
		t.Fatalf("unexpected slice: %v", got)
	}
	if got := stringSliceFromAny("not-a-slice"); len(got) != 0 {
		t.Fatalf("expected empty slice, got %v", got)
	}
}

func TestResourceNamesFromManifest(t *testing.T) {
	t.Parallel()

	got := resourceNamesFromManifest([]any{
		"catalog",
		map[string]any{"name": "stream"},
		"catalog", // duplicate
		map[string]any{"name": ""},
	})
	if !reflect.DeepEqual(got, []string{"catalog", "stream"}) {
		t.Fatalf("unexpected resources: %v", got)
	}
}

func TestExtractHistoryEntries(t *testing.T) {
	t.Parallel()

	// Array of objects
	got := extractHistoryEntries([]any{
		map[string]any{"id": 1},
		"skip-me",
		map[string]any{"id": 2},
	})
	if len(got) != 2 {
		t.Fatalf("expected 2 entries from array, got %d", len(got))
	}

	// Nested under a known key
	got = extractHistoryEntries(map[string]any{
		"items": []any{map[string]any{"id": 1}},
	})
	if len(got) != 1 {
		t.Fatalf("expected 1 entry from nested key, got %d", len(got))
	}

	// Single object carrying state
	got = extractHistoryEntries(map[string]any{"state": map[string]any{}})
	if len(got) != 1 {
		t.Fatalf("expected single state object to be wrapped, got %d", len(got))
	}
}
