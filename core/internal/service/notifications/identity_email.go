package notifications

import (
	"billionmail-core/internal/service/identity"
	platformqueue "billionmail-core/internal/service/queue"
	"billionmail-core/internal/service/tenancy"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"net/mail"
	"net/smtp"
	"net/url"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	sestypes "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/google/uuid"
)

type Mailer interface {
	Send(context.Context, string, string, string) error
}

type SMTPMailer struct {
	Address string
	From    string
}

type SESMailer struct {
	Client interface {
		SendEmail(context.Context, *sesv2.SendEmailInput, ...func(*sesv2.Options)) (*sesv2.SendEmailOutput, error)
	}
	From string
}

func (m SESMailer) Send(ctx context.Context, to, subject, body string) error {
	if m.Client == nil || m.From == "" {
		return fmt.Errorf("SES client and sender are required")
	}
	if strings.ContainsAny(to+m.From+subject, "\r\n") {
		return fmt.Errorf("mail headers contain invalid newline characters")
	}
	if _, err := mail.ParseAddress(to); err != nil {
		return fmt.Errorf("recipient address is invalid")
	}
	_, err := m.Client.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(m.From),
		Destination:      &sestypes.Destination{ToAddresses: []string{to}},
		Content: &sestypes.EmailContent{Simple: &sestypes.Message{
			Subject: &sestypes.Content{Data: aws.String(subject), Charset: aws.String("UTF-8")},
			Body:    &sestypes.Body{Html: &sestypes.Content{Data: aws.String(body), Charset: aws.String("UTF-8")}},
		}},
	})
	return err
}

func (m SMTPMailer) Send(_ context.Context, to, subject, body string) error {
	if m.Address == "" || m.From == "" {
		return fmt.Errorf("SMTP address and sender are required")
	}
	if strings.ContainsAny(to+m.From+subject, "\r\n") {
		return fmt.Errorf("mail headers contain invalid newline characters")
	}
	toAddress, err := mail.ParseAddress(to)
	if err != nil || toAddress.Address != to {
		return fmt.Errorf("recipient address is invalid")
	}
	fromAddress, err := mail.ParseAddress(m.From)
	if err != nil || fromAddress.Address != m.From {
		return fmt.Errorf("sender address is invalid")
	}
	message := []byte("From: " + m.From + "\r\nTo: " + to + "\r\nSubject: " + subject +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n" + body)
	return smtp.SendMail(m.Address, nil, m.From, []string{to}, message)
}

type IdentityEmailHandler struct {
	Mailer      Mailer
	TokenSecret string
}

type identityEmailPayload struct {
	TokenID   uuid.UUID `json:"token_id"`
	Purpose   string    `json:"purpose"`
	Email     string    `json:"email"`
	PublicURL string    `json:"public_url"`
}

func (h IdentityEmailHandler) Handle(ctx context.Context, event platformqueue.Event) error {
	if h.Mailer == nil {
		return fmt.Errorf("identity mailer is required")
	}
	var email, base, token, path, subject, action string
	switch event.EventType {
	case "identity.email_verification.requested":
		var payload identityEmailPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		var err error
		token, _, err = identity.BuildActionToken(payload.TokenID, payload.Purpose, h.TokenSecret)
		if err != nil {
			return err
		}
		email, base = payload.Email, strings.TrimRight(payload.PublicURL, "/")
		path, subject, action = "/verify-email", "Verify your BillionMail email", "Verify email"
	case "identity.password_reset.requested":
		var payload identityEmailPayload
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		var err error
		token, _, err = identity.BuildActionToken(payload.TokenID, payload.Purpose, h.TokenSecret)
		if err != nil {
			return err
		}
		email, base = payload.Email, strings.TrimRight(payload.PublicURL, "/")
		path, subject, action = "/reset-password", "Reset your BillionMail password", "Reset password"
	case "company.invitation.created":
		var payload struct {
			InvitationID  uuid.UUID `json:"invitation_id"`
			Email         string    `json:"email"`
			AcceptBaseURL string    `json:"accept_base_url"`
		}
		if err := json.Unmarshal(event.Payload, &payload); err != nil {
			return err
		}
		var err error
		token, _, err = tenancy.BuildInvitationToken(payload.InvitationID, h.TokenSecret)
		if err != nil {
			return err
		}
		email, base = payload.Email, strings.TrimRight(payload.AcceptBaseURL, "/")
		path, subject, action = "/accept-invitation", "You are invited to a BillionMail workspace", "Accept invitation"
	default:
		return fmt.Errorf("unsupported identity email event %s", event.EventType)
	}
	if base == "" {
		return fmt.Errorf("public URL is required")
	}
	link := base + path + "?token=" + url.QueryEscape(token)
	body := `<p>` + html.EscapeString(subject) + `</p><p><a href="` + html.EscapeString(link) + `">` + action + `</a></p><p>If you did not request this, you can ignore this email.</p>`
	return h.Mailer.Send(ctx, email, subject, body)
}
