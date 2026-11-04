package metawebhook

import (
	"billionmail-core/internal/service/public"
	"billionmail-core/internal/service/whatsappwebhook"
	"errors"
	"net/http"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

func Verify(r *ghttp.Request) {
	challenge, ok := whatsappwebhook.VerifyChallenge(
		r.URL.Query().Get("hub.mode"), r.URL.Query().Get("hub.verify_token"),
		r.URL.Query().Get("hub.challenge"), public.MustGetDockerEnv("META_VERIFY_TOKEN", ""),
	)
	if !ok {
		r.Response.WriteStatus(http.StatusForbidden)
		return
	}
	r.Response.Header().Set("Content-Type", "text/plain; charset=utf-8")
	r.Response.Write(challenge)
}

func Receive(r *ghttp.Request) {
	body := r.GetBody()
	if len(body) > whatsappwebhook.MaxPayloadBytes {
		r.Response.WriteStatus(http.StatusRequestEntityTooLarge)
		return
	}
	signature := r.Header.Get("X-Hub-Signature-256")
	if err := whatsappwebhook.VerifySignature(body, signature, public.MustGetDockerEnv("META_APP_SECRET", "")); err != nil {
		r.Response.WriteStatus(http.StatusUnauthorized)
		return
	}
	_, err := whatsappwebhook.Ingest(r.GetCtx(), body, signature)
	switch {
	case err == nil:
		r.Response.WriteStatus(http.StatusOK)
	case errors.Is(err, whatsappwebhook.ErrUnknownPhoneNumber):
		// Acknowledge assets not owned by this platform without creating any
		// tenant data or causing Meta to retry them indefinitely.
		r.Response.WriteStatus(http.StatusOK)
	case errors.Is(err, whatsappwebhook.ErrInvalidPayload):
		r.Response.WriteStatus(http.StatusBadRequest)
	default:
		g.Log().Error(r.GetCtx(), "persist Meta webhook failed", err)
		r.Response.WriteStatus(http.StatusInternalServerError)
	}
}
