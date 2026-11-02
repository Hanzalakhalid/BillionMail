package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeOneSignal starts a server that records the last request body and answers with reply.
func fakeOneSignal(t *testing.T, status int, reply string) (got *map[string]any, auth *string) {
	t.Helper()
	var body map[string]any
	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/notifications", r.URL.Path)
		assert.Equal(t, "push", r.URL.Query().Get("c"))
		authHeader = r.Header.Get("Authorization")
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		w.WriteHeader(status)
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)

	old := oneSignalBaseURL
	oneSignalBaseURL = srv.URL
	t.Cleanup(func() { oneSignalBaseURL = old })
	return &body, &authHeader
}

var testCfg = &Config{AppId: "app-1", RestApiKey: "key-1", DefaultUrl: "https://example.com"}

func TestSendOneSignal_AllSubscribers(t *testing.T) {
	body, auth := fakeOneSignal(t, http.StatusOK, `{"id":"notif-1"}`)

	id, err := sendOneSignal(context.Background(), testCfg, Notification{Title: "Sale", Message: "50% off"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "notif-1", id)
	assert.Equal(t, "Key key-1", *auth)
	b := *body
	assert.Equal(t, "app-1", b["app_id"])
	assert.Equal(t, "push", b["target_channel"])
	assert.Equal(t, map[string]any{"en": "Sale"}, b["headings"])
	assert.Equal(t, map[string]any{"en": "50% off"}, b["contents"])
	assert.Equal(t, []any{"Total Subscriptions"}, b["included_segments"])
	assert.Equal(t, "https://example.com", b["url"], "falls back to default url")
	assert.NotContains(t, b, "include_aliases")
}

func TestSendOneSignal_ExternalIdsAndImage(t *testing.T) {
	body, _ := fakeOneSignal(t, http.StatusOK, `{"id":"notif-2"}`)

	n := Notification{Message: "Hi", Url: "https://shop.example/sale", Image: "https://cdn.example/a.png"}
	_, err := sendOneSignal(context.Background(), testCfg, n, []string{"ali@test.com"})

	require.NoError(t, err)
	b := *body
	assert.Equal(t, map[string]any{"external_id": []any{"ali@test.com"}}, b["include_aliases"])
	assert.NotContains(t, b, "included_segments")
	assert.NotContains(t, b, "headings")
	assert.Equal(t, "https://shop.example/sale", b["url"])
	assert.Equal(t, "https://cdn.example/a.png", b["chrome_web_image"])
	assert.Equal(t, "https://cdn.example/a.png", b["big_picture"])
}

func TestSendOneSignal_NoSubscribers(t *testing.T) {
	fakeOneSignal(t, http.StatusOK, `{"id":"","errors":["All included players are not subscribed"]}`)

	_, err := sendOneSignal(context.Background(), testCfg, Notification{Message: "Hi"}, nil)

	require.Error(t, err)
	assert.ErrorIs(t, err, errNoRecipients)
	assert.Contains(t, err.Error(), "All included players are not subscribed")
}

func TestSendOneSignal_FallsBackToLegacySegment(t *testing.T) {
	var segments []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		seg := body["included_segments"].([]any)[0]
		segments = append(segments, seg)
		if seg == "Subscribed Users" {
			_, _ = w.Write([]byte(`{"id":"legacy-1"}`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"","errors":["All included players are not subscribed"]}`))
	}))
	defer srv.Close()
	old := oneSignalBaseURL
	oneSignalBaseURL = srv.URL
	defer func() { oneSignalBaseURL = old }()

	id, err := sendOneSignal(context.Background(), testCfg, Notification{Message: "Hi"}, nil)

	require.NoError(t, err)
	assert.Equal(t, "legacy-1", id)
	assert.Equal(t, []any{"Total Subscriptions", "Subscribed Users"}, segments)
}

func TestSendOneSignal_HTTPErrorDoesNotRetry(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	old := oneSignalBaseURL
	oneSignalBaseURL = srv.URL
	defer func() { oneSignalBaseURL = old }()

	_, err := sendOneSignal(context.Background(), testCfg, Notification{Message: "Hi"}, nil)

	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestSendOneSignal_HTTPError(t *testing.T) {
	fakeOneSignal(t, http.StatusForbidden, `{"errors":["Access denied"]}`)

	_, err := sendOneSignal(context.Background(), testCfg, Notification{Message: "Hi"}, nil)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 403")
	assert.Contains(t, err.Error(), "Access denied")
}

func TestSendOneSignal_MissingCredentials(t *testing.T) {
	_, err := sendOneSignal(context.Background(), &Config{}, Notification{Message: "Hi"}, nil)
	assert.Error(t, err)
}

func TestOneSignalErrors_ObjectForm(t *testing.T) {
	msg := oneSignalErrors(json.RawMessage(`{"invalid_aliases":{"external_id":["x"]}}`))
	assert.Contains(t, msg, "invalid_aliases")
}

func TestChunk(t *testing.T) {
	assert.Equal(t, [][]string{{"a", "b"}, {"c", "d"}, {"e"}}, chunk([]string{"a", "b", "c", "d", "e"}, 2))
	assert.Equal(t, [][]string{{"a"}}, chunk([]string{"a"}, 2))
}

func TestConfigMasked(t *testing.T) {
	cfg := &Config{AppId: "app", RestApiKey: "secret"}

	m := cfg.Masked()

	assert.Equal(t, SecretMask, m.RestApiKey)
	assert.Equal(t, "app", m.AppId)
	assert.Equal(t, "secret", cfg.RestApiKey, "original must stay unmasked")
}
