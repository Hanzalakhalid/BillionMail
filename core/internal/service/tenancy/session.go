package tenancy

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

var ErrInvalidRefreshToken = errors.New("invalid or expired refresh token")

type Session struct {
	ID              uuid.UUID  `json:"id"`
	UserID          uuid.UUID  `json:"user_id"`
	TokenFamily     uuid.UUID  `json:"token_family"`
	ActiveCompanyID uuid.UUID  `json:"active_company_id"`
	ExpiresAt       time.Time  `json:"expires_at"`
	RotatedAt       *time.Time `json:"rotated_at"`
	RevokedAt       *time.Time `json:"revoked_at"`
}

func newOpaqueToken() (string, []byte, error) {
	raw := make([]byte, 48)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func CreateSession(ctx context.Context, userID, companyID uuid.UUID, userAgent, ip string, ttl time.Duration) (Session, string, error) {
	token, hash, err := newOpaqueToken()
	if err != nil {
		return Session{}, "", err
	}
	session := Session{ID: uuid.New(), UserID: userID, TokenFamily: uuid.New(), ActiveCompanyID: companyID, ExpiresAt: time.Now().UTC().Add(ttl)}
	data := g.Map{
		"id": session.ID, "user_id": session.UserID, "token_family": session.TokenFamily,
		"refresh_token_hash": hash, "active_company_id": session.ActiveCompanyID,
		"user_agent": userAgent, "expires_at": session.ExpiresAt,
	}
	if parsed := net.ParseIP(ip); parsed != nil {
		data["ip_address"] = parsed.String()
	}
	if _, err = g.DB().Model("auth_sessions").Data(data).Insert(); err != nil {
		return Session{}, "", fmt.Errorf("create auth session: %w", err)
	}
	return session, token, nil
}

func RotateSession(ctx context.Context, token string, ttl time.Duration) (Session, string, error) {
	hash := sha256.Sum256([]byte(token))
	newToken, newHash, err := newOpaqueToken()
	if err != nil {
		return Session{}, "", err
	}
	var next Session
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var current Session
		if err := tx.Model("auth_sessions").Where("refresh_token_hash = decode(?, 'hex')", hex.EncodeToString(hash[:])).LockUpdate().Scan(&current); err != nil {
			return err
		}
		if current.ID == uuid.Nil {
			return ErrInvalidRefreshToken
		}
		if current.RotatedAt != nil || current.RevokedAt != nil || time.Now().UTC().After(current.ExpiresAt) {
			_, _ = tx.Model("auth_sessions").Data(g.Map{"revoked_at": time.Now().UTC(), "revoke_reason": "refresh_token_reuse"}).Where("token_family", current.TokenFamily).WhereNull("revoked_at").Update()
			return ErrInvalidRefreshToken
		}
		now := time.Now().UTC()
		if _, err := tx.Model("auth_sessions").Data(g.Map{"rotated_at": now, "last_used_at": now}).Where("id", current.ID).Update(); err != nil {
			return err
		}
		next = Session{ID: uuid.New(), UserID: current.UserID, TokenFamily: current.TokenFamily, ActiveCompanyID: current.ActiveCompanyID, ExpiresAt: now.Add(ttl)}
		_, err := tx.Model("auth_sessions").Data(g.Map{
			"id": next.ID, "user_id": next.UserID, "token_family": next.TokenFamily,
			"refresh_token_hash": newHash, "active_company_id": next.ActiveCompanyID,
			"expires_at": next.ExpiresAt,
		}).Insert()
		return err
	})
	if err != nil {
		return Session{}, "", err
	}
	return next, newToken, nil
}

func RevokeSession(ctx context.Context, sessionID uuid.UUID, reason string) error {
	result, err := g.DB().Model("auth_sessions").Data(g.Map{"revoked_at": time.Now().UTC(), "revoke_reason": reason}).Where("id", sessionID).WhereNull("revoked_at").Update()
	if err != nil {
		return err
	}
	if rows, _ := result.RowsAffected(); rows == 0 {
		return ErrInvalidRefreshToken
	}
	return nil
}

func MoveSessionCompany(ctx context.Context, sessionID, userID, companyID uuid.UUID) error {
	if _, err := Membership(ctx, userID, companyID); err != nil {
		return err
	}
	_, err := g.DB().Model("auth_sessions").Data(g.Map{"active_company_id": companyID, "last_used_at": time.Now().UTC()}).Where("id", sessionID).Where("user_id", userID).WhereNull("revoked_at").Update()
	return err
}
