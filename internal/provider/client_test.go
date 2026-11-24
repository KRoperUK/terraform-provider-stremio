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

	"github.com/hashicorp/terraform-plugin-framework/types"
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
		{name: "http loopback allowed", baseURL: "http://127.0.0.1:8080", wantErr: false},
		{name: "http rejected", baseURL: "http://api.strem.io", wantErr: true},
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
	if c.authKey != "auth-123" || c.userID != "user-1" {
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

func TestClientCloneIsolatesAuth(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var got map[string]any
		_ = json.Unmarshal(body, &got)
		email, _ := got["email"].(string)
		writeEnvelope(t, w, map[string]any{
			"authKey": "auth-for-" + email,
			"user":    map[string]any{"_id": "user-" + email, "email": email},
		}, nil)
	}))
	defer srv.Close()

	base := newTestClient(t, srv.URL)

	first := base.clone()
	second := base.clone()

	if err := first.Login(context.Background(), "a@example.com", "pw"); err != nil {
		t.Fatalf("first Login: %v", err)
	}
	if err := second.Login(context.Background(), "b@example.com", "pw"); err != nil {
		t.Fatalf("second Login: %v", err)
	}

	if first.authKey != "auth-for-a@example.com" {
		t.Fatalf("first client holds %q, want its own login", first.authKey)
	}
	if second.authKey != "auth-for-b@example.com" {
		t.Fatalf("second client holds %q, want its own login", second.authKey)
	}
	if base.authKey != "" {
		t.Fatalf("base client holds %q, want untouched", base.authKey)
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

func TestLibraryItemsSharedBetweenClones(t *testing.T) {
	t.Parallel()

	var datastoreCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		writeEnvelope(t, w, map[string]any{
			"authKey": "auth-1",
			"user":    map[string]any{"_id": "user-1", "email": "a@b.c"},
		}, nil)
	})
	mux.HandleFunc("/api/datastoreGet", func(w http.ResponseWriter, r *http.Request) {
		datastoreCalls++
		writeEnvelope(t, w, []map[string]any{
			{"type": "movie", "state": map[string]any{"timeWatched": float64(10)}},
		}, nil)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if err := c.Login(context.Background(), "a@b.c", "pw"); err != nil {
		t.Fatalf("Login: %v", err)
	}

	if _, err := c.WatchHistory(context.Background(), 0); err != nil {
		t.Fatalf("WatchHistory: %v", err)
	}
	if _, err := c.clone().ContinueWatching(context.Background(), 0); err != nil {
		t.Fatalf("ContinueWatching: %v", err)
	}

	if datastoreCalls != 1 {
		t.Fatalf("expected 1 library fetch shared between clones, got %d", datastoreCalls)
	}
}

func TestHistoryNegativeLimit(t *testing.T) {
	t.Parallel()

	c := newTestClient(t, "https://api.strem.io")
	c.authKey = "auth-123"

	if _, err := c.WatchHistory(context.Background(), -1); err == nil {
		t.Fatal("expected error for negative watch history limit")
	}
	if _, err := c.ContinueWatching(context.Background(), -1); err == nil {
		t.Fatal("expected error for negative continue watching limit")
	}
}

func TestSetInstalledAddonsPreservesOrder(t *testing.T) {
	t.Parallel()

	var collectionSet map[string]any

	mux := http.NewServeMux()
	for _, name := range []string{"m1", "m2", "m3"} {
		mux.HandleFunc("/"+name+".json", func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "org." + r.URL.Path, "name": "Name " + r.URL.Path})
		})
	}
	srv := httptest.NewServer(mux)
	defer srv.Close()

	mux.HandleFunc("/api/addonCollectionSet", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &collectionSet)
		writeEnvelope(t, w, map[string]any{"success": true}, nil)
	})

	c := newTestClient(t, srv.URL)
	c.authKey = "auth-123"

	urls := []string{srv.URL + "/m1.json", srv.URL + "/m2.json", srv.URL + "/m3.json"}
	if err := c.SetInstalledAddons(context.Background(), urls); err != nil {
		t.Fatalf("SetInstalledAddons: %v", err)
	}

	addons, ok := collectionSet["addons"].([]any)
	if !ok || len(addons) != 3 {
		t.Fatalf("expected 3 addons in collectionSet payload, got: %v", collectionSet["addons"])
	}
	for i, want := range urls {
		entry, ok := addons[i].(map[string]any)
		if !ok || entry["transportUrl"] != want {
			t.Fatalf("addon %d out of order, got: %v", i, addons[i])
		}
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

func TestFetchManifestTooLarge(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, (10<<20)+1))
	}))
	defer srv.Close()

	c := newTestClient(t, srv.URL)
	if _, err := c.fetchManifest(context.Background(), srv.URL); err == nil {
		t.Fatal("expected error for oversized manifest")
	} else if !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("expected size error, got: %v", err)
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
		{in: float32(2.5), want: 2, ok: true},
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

func TestSortAddonModelsByVersion(t *testing.T) {
	t.Parallel()

	objs := []addonModel{
		{Version: types.StringValue("1.10.0")},
		{Version: types.StringValue("1.9.0")},
		{Version: types.StringValue("1.2.0")},
	}
	sortAddonModels(objs, "version", false)

	want := []string{"1.2.0", "1.9.0", "1.10.0"}
	for i, version := range want {
		if objs[i].Version.ValueString() != version {
			t.Fatalf("position %d = %q, want %q", i, objs[i].Version.ValueString(), version)
		}
	}
}

func TestMapWatchHistoryEntryProgress(t *testing.T) {
	t.Parallel()

	mapped := mapWatchHistoryEntry(map[string]any{
		"_id":   "m1",
		"name":  "Example",
		"type":  "movie",
		"state": map[string]any{"timeOffset": float64(30), "duration": float64(120)},
	}, 0)
	if got := mapped.Progress.ValueFloat64(); got != 25 {
		t.Fatalf("progress = %v, want 25", got)
	}
	if !mapped.VideoID.IsNull() {
		t.Fatalf("video_id should be null when absent, got %q", mapped.VideoID.ValueString())
	}

	sparse := mapWatchHistoryEntry(map[string]any{}, 3)
	if got := sparse.ID.ValueString(); got != "entry-3" {
		t.Fatalf("id = %q, want entry-3 fallback", got)
	}
	if !sparse.TimeOffset.IsNull() || !sparse.Progress.IsNull() {
		t.Fatal("missing numbers should map to null")
	}
}
