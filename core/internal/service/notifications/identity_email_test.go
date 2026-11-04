package notifications

import (
	platformqueue "billionmail-core/internal/service/queue"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/google/uuid"
)

type recordingMailer struct{ to, subject, body string }

func (m *recordingMailer) Send(_ context.Context, to, subject, body string) error {
	m.to, m.subject, m.body = to, subject, body
	return nil
}

func TestIdentityEmailHandlerBuildsTokenOnlyAtDelivery(t *testing.T) {
	mailer := &recordingMailer{}
	payload, _ := json.Marshal(identityEmailPayload{
		TokenID: uuid.New(), Purpose: "verify-email", Email: "owner@example.test", PublicURL: "https://app.example.test/",
	})
	handler := IdentityEmailHandler{Mailer: mailer, TokenSecret: strings.Repeat("s", 32)}
	err := handler.Handle(context.Background(), platformqueue.Event{
		EventType: "identity.email_verification.requested", Payload: payload,
	})
	if err != nil {
		t.Fatal(err)
	}
	if mailer.to != "owner@example.test" || !strings.Contains(mailer.body, "https://app.example.test/verify-email?token=") {
		t.Fatalf("unexpected email: %+v", mailer)
	}
	if strings.Contains(string(payload), "?token=") {
		t.Fatal("raw token leaked into durable outbox payload")
	}
}

func TestSMTPMailerRejectsHeaderInjectionBeforeConnecting(t *testing.T) {
	err := (SMTPMailer{Address: "127.0.0.1:1", From: "noreply@example.test"}).Send(
		context.Background(), "victim@example.test\r\nBcc: attacker@example.test", "Subject", "body")
	if err == nil || !strings.Contains(err.Error(), "newline") {
		t.Fatalf("header injection was not rejected: %v", err)
	}
}

type recordingSES struct{ input *sesv2.SendEmailInput }

func (s *recordingSES) SendEmail(_ context.Context, input *sesv2.SendEmailInput, _ ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error) {
	s.input = input
	return &sesv2.SendEmailOutput{}, nil
}

func TestSESMailerUsesStructuredMessage(t *testing.T) {
	client := &recordingSES{}
	err := (SESMailer{Client: client, From: "noreply@example.test"}).Send(
		context.Background(), "owner@example.test", "Verify account", "<p>Verify</p>")
	if err != nil {
		t.Fatal(err)
	}
	if client.input == nil || len(client.input.Destination.ToAddresses) != 1 || client.input.Content.Simple == nil {
		t.Fatalf("unexpected SES request: %+v", client.input)
	}
}
