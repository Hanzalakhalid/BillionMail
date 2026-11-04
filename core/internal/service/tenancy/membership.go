package tenancy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gogf/gf/v2/database/gdb"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

type Member struct {
	MembershipID uuid.UUID  `json:"membership_id"`
	UserID       uuid.UUID  `json:"user_id"`
	Username     string     `json:"username"`
	Email        string     `json:"email"`
	Role         string     `json:"role"`
	Status       string     `json:"status"`
	JoinedAt     *time.Time `json:"joined_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Invitation struct {
	ID         uuid.UUID  `json:"id"`
	CompanyID  uuid.UUID  `json:"company_id"`
	Email      string     `json:"email"`
	Role       string     `json:"role"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at"`
	RevokedAt  *time.Time `json:"revoked_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

var memberRoles = map[string]bool{"owner": true, "admin": true, "manager": true, "agent": true, "viewer": true}

func ListMembers(ctx context.Context, tenant TenantContext) ([]Member, error) {
	if err := Authorize(tenant, "member.read"); err != nil {
		return nil, err
	}
	members := make([]Member, 0)
	err := g.DB().Model("company_members cm").
		InnerJoin("users u", "u.id=cm.user_id").
		Where("cm.company_id", tenant.CompanyID).
		Fields("cm.id membership_id,cm.user_id,u.username,COALESCE(u.email,'') email,cm.role,cm.status,cm.joined_at,cm.created_at").
		Order("cm.created_at ASC").Scan(&members)
	return members, err
}

func ListInvitations(ctx context.Context, tenant TenantContext) ([]Invitation, error) {
	if err := Authorize(tenant, "member.read"); err != nil {
		return nil, err
	}
	invitations := make([]Invitation, 0)
	err := g.DB().Model("company_invitations").Where("company_id", tenant.CompanyID).
		Fields("id,company_id,email,role,expires_at,accepted_at,revoked_at,created_at").
		Order("created_at DESC").Scan(&invitations)
	return invitations, err
}

func BuildInvitationToken(invitationID uuid.UUID, secret string) (string, []byte, error) {
	if invitationID == uuid.Nil || len(secret) < 32 {
		return "", nil, fmt.Errorf("invitation ID and a secret of at least 32 characters are required")
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(invitationID[:])
	raw := append(invitationID[:], mac.Sum(nil)...)
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func InviteMember(ctx context.Context, tenant TenantContext, email, role, acceptBaseURL, tokenSecret string) (Invitation, error) {
	if err := Authorize(tenant, "member.manage"); err != nil {
		return Invitation{}, err
	}
	email = strings.ToLower(strings.TrimSpace(email))
	role = strings.ToLower(strings.TrimSpace(role))
	if email == "" || !strings.Contains(email, "@") {
		return Invitation{}, fmt.Errorf("valid invitation email is required")
	}
	if !memberRoles[role] || role == "owner" {
		return Invitation{}, fmt.Errorf("invitation role must be admin, manager, agent, or viewer")
	}
	if tenant.Role != "owner" && role == "admin" {
		return Invitation{}, ErrPermissionDenied
	}
	invitation := Invitation{ID: uuid.New(), CompanyID: tenant.CompanyID, Email: email, Role: role, ExpiresAt: time.Now().UTC().Add(7 * 24 * time.Hour), CreatedAt: time.Now().UTC()}
	_, tokenHash, err := BuildInvitationToken(invitation.ID, tokenSecret)
	if err != nil {
		return Invitation{}, err
	}
	err = g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var existing int
		existing, err = tx.Model("company_members cm").InnerJoin("users u", "u.id=cm.user_id").Where("cm.company_id", tenant.CompanyID).Where("LOWER(u.email) = ?", email).Count()
		if err != nil {
			return err
		}
		if existing > 0 {
			return fmt.Errorf("user is already a company member")
		}
		_, _ = tx.Model("company_invitations").Data(g.Map{"revoked_at": time.Now().UTC()}).Where("company_id", tenant.CompanyID).Where("LOWER(email) = ?", email).WhereNull("accepted_at").WhereNull("revoked_at").Update()
		if _, err = tx.Model("company_invitations").Data(g.Map{
			"id": invitation.ID, "company_id": tenant.CompanyID, "email": email, "role": role,
			"token_hash": tokenHash, "invited_by": tenant.UserID, "expires_at": invitation.ExpiresAt,
		}).Insert(); err != nil {
			return err
		}
		payload, _ := json.Marshal(g.Map{"invitation_id": invitation.ID, "email": email, "role": role, "accept_base_url": acceptBaseURL})
		if _, err = tx.Model("outbox_events").Data(g.Map{
			"id": uuid.New(), "company_id": tenant.CompanyID, "aggregate_type": "company_invitation",
			"aggregate_id": invitation.ID.String(), "event_type": "company.invitation.created",
			"payload": payload, "idempotency_key": invitation.ID.String(),
		}).Insert(); err != nil {
			return err
		}
		_, err = tx.Model("audit_logs").Data(g.Map{
			"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "member.invited",
			"resource_type": "company_invitation", "resource_id": invitation.ID.String(),
			"metadata": g.Map{"email": email, "role": role},
		}).Insert()
		return err
	})
	return invitation, err
}

func AcceptInvitation(ctx context.Context, user User, token string) (uuid.UUID, error) {
	hash := sha256.Sum256([]byte(strings.TrimSpace(token)))
	var acceptedCompany uuid.UUID
	err := g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		var invitation Invitation
		if err := tx.Model("company_invitations").Where("token_hash = decode(?, 'hex')", hex.EncodeToString(hash[:])).LockUpdate().Scan(&invitation); err != nil {
			return err
		}
		if invitation.ID == uuid.Nil || invitation.AcceptedAt != nil || invitation.RevokedAt != nil || time.Now().UTC().After(invitation.ExpiresAt) {
			return fmt.Errorf("invitation is invalid or expired")
		}
		var storedUser struct {
			Email  string `json:"email"`
			Status string `json:"status"`
		}
		if err := tx.Model("users").Where("id", user.ID).Scan(&storedUser); err != nil {
			return err
		}
		if storedUser.Status != "active" || !strings.EqualFold(strings.TrimSpace(storedUser.Email), invitation.Email) {
			return fmt.Errorf("invitation email does not match the signed-in user")
		}
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", invitation.CompanyID.String()); err != nil {
			return err
		}
		now := time.Now().UTC()
		if _, err := tx.Model("company_members").Data(g.Map{
			"id": uuid.New(), "company_id": invitation.CompanyID, "user_id": user.ID,
			"role": invitation.Role, "status": "active", "joined_at": now,
		}).OnConflict("company_id,user_id").Save(); err != nil {
			return err
		}
		if _, err := tx.Model("company_invitations").Data(g.Map{"accepted_at": now}).Where("id", invitation.ID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("users").Data(g.Map{"membership_version": gdb.Raw("membership_version+1"), "updated_at": now}).Where("id", user.ID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("audit_logs").Data(g.Map{
			"company_id": invitation.CompanyID, "actor_user_id": user.ID, "action": "member.invitation_accepted",
			"resource_type": "company_invitation", "resource_id": invitation.ID.String(),
		}).Insert(); err != nil {
			return err
		}
		acceptedCompany = invitation.CompanyID
		return nil
	})
	return acceptedCompany, err
}

func ChangeMemberRole(ctx context.Context, tenant TenantContext, membershipID uuid.UUID, role string) error {
	if err := Authorize(tenant, "member.manage"); err != nil {
		return err
	}
	role = strings.ToLower(strings.TrimSpace(role))
	if !memberRoles[role] || role == "owner" {
		return fmt.Errorf("owner role changes require ownership transfer")
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var member struct {
			UserID uuid.UUID `json:"user_id"`
			Role   string    `json:"role"`
		}
		if err := tx.Model("company_members").Where("id", membershipID).Where("company_id", tenant.CompanyID).LockUpdate().Scan(&member); err != nil {
			return err
		}
		if member.UserID == uuid.Nil || member.Role == "owner" {
			return ErrPermissionDenied
		}
		if tenant.Role != "owner" && (member.Role == "admin" || role == "admin") {
			return ErrPermissionDenied
		}
		if _, err := tx.Model("company_members").Data(g.Map{"role": role, "updated_at": time.Now().UTC()}).Where("id", membershipID).Update(); err != nil {
			return err
		}
		if _, err := tx.Model("users").Data(g.Map{"membership_version": gdb.Raw("membership_version+1")}).Where("id", member.UserID).Update(); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{
			"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "member.role_changed",
			"resource_type": "company_membership", "resource_id": membershipID.String(),
			"metadata": g.Map{"previous_role": member.Role, "new_role": role, "user_id": member.UserID},
		}).Insert()
		return err
	})
}

func RemoveMember(ctx context.Context, tenant TenantContext, membershipID uuid.UUID) error {
	if err := Authorize(tenant, "member.manage"); err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		var member struct {
			UserID uuid.UUID `json:"user_id"`
			Role   string    `json:"role"`
		}
		if err := tx.Model("company_members").Where("id", membershipID).Where("company_id", tenant.CompanyID).LockUpdate().Scan(&member); err != nil {
			return err
		}
		if member.UserID == uuid.Nil || member.Role == "owner" || member.UserID == tenant.UserID {
			return ErrPermissionDenied
		}
		if tenant.Role != "owner" && member.Role == "admin" {
			return ErrPermissionDenied
		}
		if _, err := tx.Model("company_members").Where("id", membershipID).Delete(); err != nil {
			return err
		}
		if _, err := tx.Model("users").Data(g.Map{"membership_version": gdb.Raw("membership_version+1")}).Where("id", member.UserID).Update(); err != nil {
			return err
		}
		_, err := tx.Model("audit_logs").Data(g.Map{
			"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "member.removed",
			"resource_type": "company_membership", "resource_id": membershipID.String(),
			"metadata": g.Map{"role": member.Role, "user_id": member.UserID},
		}).Insert()
		return err
	})
}

func RevokeInvitation(ctx context.Context, tenant TenantContext, invitationID uuid.UUID) error {
	if err := Authorize(tenant, "member.manage"); err != nil {
		return err
	}
	return g.DB().Transaction(ctx, func(_ context.Context, tx gdb.TX) error {
		if _, err := tx.Exec("SELECT set_config('app.company_id', ?, true)", tenant.CompanyID.String()); err != nil {
			return err
		}
		result, err := tx.Model("company_invitations").Data(g.Map{"revoked_at": time.Now().UTC()}).Where("id", invitationID).Where("company_id", tenant.CompanyID).WhereNull("accepted_at").WhereNull("revoked_at").Update()
		if err != nil {
			return err
		}
		rows, _ := result.RowsAffected()
		if rows == 0 {
			return fmt.Errorf("active invitation not found")
		}
		_, err = tx.Model("audit_logs").Data(g.Map{
			"company_id": tenant.CompanyID, "actor_user_id": tenant.UserID, "action": "member.invitation_revoked",
			"resource_type": "company_invitation", "resource_id": invitationID.String(),
		}).Insert()
		return err
	})
}
