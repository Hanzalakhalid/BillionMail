package sms

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"billionmail-core/internal/model/entity"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizePhone(t *testing.T) {
	cases := []struct {
		in, cc, want string
	}{
		{"03001234567", "92", "+923001234567"},
		{"0300-123 4567", "92", "+923001234567"},
		{"+92 300 1234567", "92", "+923001234567"},
		{"00923001234567", "", "+923001234567"},
		{"923001234567", "", "+923001234567"},
		{"+1 (415) 555-0100", "92", "+14155550100"},
		{"0300 1234567", "+92", "+923001234567"},
	}
	for _, c := range cases {
		got, err := NormalizePhone(c.in, c.cc)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestNormalizePhone_Invalid(t *testing.T) {
	_, err := NormalizePhone("03001234567", "")
	assert.Error(t, err, "local number without country code")

	_, err = NormalizePhone("12345", "92")
	assert.Error(t, err, "too short")

	_, err = NormalizePhone("", "92")
	assert.Error(t, err, "empty")
}

func TestPersonalize(t *testing.T) {
	c := entity.Contact{Email: "ali@example.com", Attribs: map[string]string{"name": "Ali"}}

	got := Personalize("Hi {name}, your email is {email}. {missing}!", c)

	assert.Equal(t, "Hi Ali, your email is ali@example.com. !", got)
}

func TestSendGeneric_GET(t *testing.T) {
	var gotQuery map[string][]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		gotQuery = r.URL.Query()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	cfg := &Config{
		Provider:      ProviderGeneric,
		GenericMethod: "GET",
		GenericURL:    srv.URL + "/send?key={api_key}&to={to_digits}&mask={sender}&msg={message}",
		GenericApiKey: "k&1",
		GenericSender: "Brand",
	}

	err := send(context.Background(), cfg, "+923001234567", "Hello & welcome")

	require.NoError(t, err)
	assert.Equal(t, "k&1", gotQuery["key"][0])
	assert.Equal(t, "923001234567", gotQuery["to"][0])
	assert.Equal(t, "Brand", gotQuery["mask"][0])
	assert.Equal(t, "Hello & welcome", gotQuery["msg"][0])
}

func TestSendGeneric_POSTJSON(t *testing.T) {
	var gotBody, gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody, gotType = string(b), r.Header.Get("Content-Type")
	}))
	defer srv.Close()

	cfg := &Config{
		Provider:           ProviderGeneric,
		GenericMethod:      "POST",
		GenericURL:         srv.URL,
		GenericContentType: "application/json",
		GenericBody:        `{"to":"{to}","text":"{message}"}`,
	}

	err := send(context.Background(), cfg, "+923001234567", `Say "hi"`)

	require.NoError(t, err)
	assert.Equal(t, "application/json", gotType)
	assert.Equal(t, `{"to":"+923001234567","text":"Say \"hi\""}`, gotBody)
}

func TestSendGeneric_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("bad api key"))
	}))
	defer srv.Close()

	cfg := &Config{Provider: ProviderGeneric, GenericURL: srv.URL}

	err := send(context.Background(), cfg, "+923001234567", "x")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "HTTP 401")
	assert.Contains(t, err.Error(), "bad api key")
}

func TestSendTwilio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		assert.True(t, ok)
		assert.Equal(t, "AC123", user)
		assert.Equal(t, "secret", pass)
		assert.Equal(t, "/2010-04-01/Accounts/AC123/Messages.json", r.URL.Path)
		require.NoError(t, r.ParseForm())
		assert.Equal(t, "+923001234567", r.PostForm.Get("To"))
		assert.Equal(t, "+15550001111", r.PostForm.Get("From"))
		assert.Equal(t, "Hello", r.PostForm.Get("Body"))
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	old := twilioBaseURL
	twilioBaseURL = srv.URL
	defer func() { twilioBaseURL = old }()

	cfg := &Config{
		Provider:         ProviderTwilio,
		TwilioAccountSid: "AC123",
		TwilioAuthToken:  "secret",
		TwilioFrom:       "+15550001111",
	}

	require.NoError(t, send(context.Background(), cfg, "+923001234567", "Hello"))
}

func TestSendTwilio_MissingCredentials(t *testing.T) {
	err := send(context.Background(), &Config{Provider: ProviderTwilio}, "+923001234567", "Hello")
	assert.Error(t, err)
}

func TestConfigMasked(t *testing.T) {
	cfg := &Config{TwilioAuthToken: "secret", GenericApiKey: "key", TwilioAccountSid: "AC1"}

	m := cfg.Masked()

	assert.Equal(t, SecretMask, m.TwilioAuthToken)
	assert.Equal(t, SecretMask, m.GenericApiKey)
	assert.Equal(t, "AC1", m.TwilioAccountSid)
	assert.Equal(t, "secret", cfg.TwilioAuthToken, "original must stay unmasked")
}
