package whatsapp

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

var testCfg = &Config{
	PhoneNumberId:     "pn-1",
	BusinessAccountId: "waba-1",
	AccessToken:       "token-1",
	ApiVersion:        "v23.0",
}

// fakeGraph points graphBaseURL at a test server for the duration of the test.
func fakeGraph(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	old := graphBaseURL
	graphBaseURL = srv.URL
	t.Cleanup(func() { graphBaseURL = old })
}

func TestSendTemplate_WithParams(t *testing.T) {
	var body map[string]any
	fakeGraph(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/v23.0/pn-1/messages", r.URL.Path)
		assert.Equal(t, "Bearer token-1", r.Header.Get("Authorization"))
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`{"messaging_product":"whatsapp","messages":[{"id":"wamid.1"}]}`))
	})

	id, err := sendTemplate(context.Background(), testCfg, "923001234567",
		Template{Name: "appointment_reminder", Language: "en", Params: []string{"Ali", "4 PM"}})

	require.NoError(t, err)
	assert.Equal(t, "wamid.1", id)
	assert.Equal(t, "whatsapp", body["messaging_product"])
	assert.Equal(t, "923001234567", body["to"])
	assert.Equal(t, "template", body["type"])
	tpl := body["template"].(map[string]any)
	assert.Equal(t, "appointment_reminder", tpl["name"])
	assert.Equal(t, map[string]any{"code": "en"}, tpl["language"])
	assert.Equal(t, []any{map[string]any{
		"type": "body",
		"parameters": []any{
			map[string]any{"type": "text", "text": "Ali"},
			map[string]any{"type": "text", "text": "4 PM"},
		},
	}}, tpl["components"])
}

func TestSendTemplate_NoParamsOmitsComponents(t *testing.T) {
	var body map[string]any
	fakeGraph(t, func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(raw, &body))
		_, _ = w.Write([]byte(`{"messages":[{"id":"wamid.2"}]}`))
	})

	_, err := sendTemplate(context.Background(), testCfg, "923001234567", Template{Name: "hello_world", Language: "en_US"})

	require.NoError(t, err)
	assert.NotContains(t, body["template"].(map[string]any), "components")
}

func TestSendTemplate_GraphError(t *testing.T) {
	fakeGraph(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"Template name does not exist in the translation","code":132001}}`))
	})

	_, err := sendTemplate(context.Background(), testCfg, "923001234567", Template{Name: "x", Language: "en"})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "132001")
	assert.Contains(t, err.Error(), "Template name does not exist")
}

func TestSendTemplate_MissingConfig(t *testing.T) {
	_, err := sendTemplate(context.Background(), &Config{}, "923001234567", Template{Name: "x"})
	assert.Error(t, err)

	_, err = sendTemplate(context.Background(), testCfg, "923001234567", Template{})
	assert.Error(t, err, "template name is required")
}

func TestListTemplates(t *testing.T) {
	fakeGraph(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v23.0/waba-1/message_templates", r.URL.Path)
		_, _ = w.Write([]byte(`{"data":[{"name":"appointment_reminder","language":"en","status":"APPROVED","category":"UTILITY",
			"components":[{"type":"HEADER","text":"Reminder"},{"type":"BODY","text":"Hi {{1}}, see you at {{2}}"}]}]}`))
	})

	list, err := listTemplates(context.Background(), testCfg)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, TemplateInfo{
		Name: "appointment_reminder", Language: "en", Status: "APPROVED", Category: "UTILITY",
		Body: "Hi {{1}}, see you at {{2}}",
	}, list[0])
}

func TestListTemplates_NeedsBusinessAccount(t *testing.T) {
	_, err := listTemplates(context.Background(), &Config{AccessToken: "t"})
	assert.Error(t, err)
}

func TestConfigMasked(t *testing.T) {
	cfg := &Config{PhoneNumberId: "pn", AccessToken: "secret"}

	m := cfg.Masked()

	assert.Equal(t, SecretMask, m.AccessToken)
	assert.Equal(t, "pn", m.PhoneNumberId)
	assert.Equal(t, "secret", cfg.AccessToken, "original must stay unmasked")
}
