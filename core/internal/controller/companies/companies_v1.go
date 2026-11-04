package companies

import (
	v1 "billionmail-core/api/companies/v1"
	"billionmail-core/internal/service/public"
	"billionmail-core/internal/service/rbac"
	"billionmail-core/internal/service/tenancy"
	"context"
	"fmt"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/google/uuid"
)

func (c *ControllerV1) List(ctx context.Context, _ *v1.ListReq) (*v1.ListRes, error) {
	res := &v1.ListRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	if err = tenancy.Authorize(tenant, "company.read"); err != nil {
		return nil, err
	}
	companies, err := tenancy.ListCompanies(ctx, tenant.UserID)
	if err != nil {
		return nil, err
	}
	res.Data.Companies = companies
	res.Data.ActiveCompanyID = tenant.CompanyID.String()
	res.SetSuccess("Company workspaces retrieved")
	return res, nil
}

func (c *ControllerV1) Members(ctx context.Context, _ *v1.MembersReq) (*v1.MembersRes, error) {
	res := &v1.MembersRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	members, err := tenancy.ListMembers(ctx, tenant)
	if err != nil {
		return nil, err
	}
	res.Data.Members = members
	res.SetSuccess("Company members retrieved")
	return res, nil
}

func (c *ControllerV1) Invitations(ctx context.Context, _ *v1.InvitationsReq) (*v1.InvitationsRes, error) {
	res := &v1.InvitationsRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	invitations, err := tenancy.ListInvitations(ctx, tenant)
	if err != nil {
		return nil, err
	}
	res.Data.Invitations = invitations
	res.SetSuccess("Company invitations retrieved")
	return res, nil
}

func (c *ControllerV1) Invite(ctx context.Context, req *v1.InviteReq) (*v1.InviteRes, error) {
	res := &v1.InviteRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	baseURL := public.MustGetDockerEnv("SAAS_PUBLIC_URL", "")
	invitation, err := tenancy.InviteMember(ctx, tenant, req.Email, req.Role, baseURL, rbac.JWT().Secret)
	if err != nil {
		return nil, err
	}
	res.Data = invitation
	res.SetSuccess("Company invitation queued")
	return res, nil
}

func (c *ControllerV1) AcceptInvitation(ctx context.Context, req *v1.AcceptInvitationReq) (*v1.AcceptInvitationRes, error) {
	res := &v1.AcceptInvitationRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	user, err := tenancy.UserByID(ctx, tenant.UserID)
	if err != nil {
		return nil, err
	}
	companyID, err := tenancy.AcceptInvitation(ctx, user, req.Token)
	if err != nil {
		return nil, err
	}
	res.Data.CompanyID = companyID.String()
	res.SetSuccess("Invitation accepted")
	return res, nil
}

func (c *ControllerV1) ChangeRole(ctx context.Context, req *v1.ChangeRoleReq) (*v1.ChangeRoleRes, error) {
	res := &v1.ChangeRoleRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	membershipID, err := uuid.Parse(req.MembershipID)
	if err != nil {
		return nil, fmt.Errorf("invalid membership ID")
	}
	if err = tenancy.ChangeMemberRole(ctx, tenant, membershipID, req.Role); err != nil {
		return nil, err
	}
	res.SetSuccess("Member role updated")
	return res, nil
}

func (c *ControllerV1) RemoveMember(ctx context.Context, req *v1.RemoveMemberReq) (*v1.RemoveMemberRes, error) {
	res := &v1.RemoveMemberRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	membershipID, err := uuid.Parse(req.MembershipID)
	if err != nil {
		return nil, fmt.Errorf("invalid membership ID")
	}
	if err = tenancy.RemoveMember(ctx, tenant, membershipID); err != nil {
		return nil, err
	}
	res.SetSuccess("Company member removed")
	return res, nil
}

func (c *ControllerV1) RevokeInvitation(ctx context.Context, req *v1.RevokeInvitationReq) (*v1.RevokeInvitationRes, error) {
	res := &v1.RevokeInvitationRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	invitationID, err := uuid.Parse(req.InvitationID)
	if err != nil {
		return nil, fmt.Errorf("invalid invitation ID")
	}
	if err = tenancy.RevokeInvitation(ctx, tenant, invitationID); err != nil {
		return nil, err
	}
	res.SetSuccess("Company invitation revoked")
	return res, nil
}

func (c *ControllerV1) Create(ctx context.Context, req *v1.CreateReq) (*v1.CreateRes, error) {
	res := &v1.CreateRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	user, err := tenancy.UserByID(ctx, tenant.UserID)
	if err != nil {
		return nil, err
	}
	company, err := tenancy.CreateCompany(ctx, user, req.Name, req.Slug, req.Timezone, req.Locale)
	if err != nil {
		return nil, err
	}
	res.Data = company
	res.SetSuccess("Company workspace created")
	return res, nil
}

func (c *ControllerV1) Switch(ctx context.Context, req *v1.SwitchReq) (*v1.SwitchRes, error) {
	res := &v1.SwitchRes{}
	current, err := tenancy.Require(ctx)
	if err != nil {
		return nil, err
	}
	companyID, err := uuid.Parse(req.CompanyID)
	if err != nil {
		return nil, fmt.Errorf("invalid company ID")
	}
	target, err := tenancy.Membership(ctx, current.UserID, companyID)
	if err != nil {
		return nil, err
	}
	target.AccountID = current.AccountID
	claims, err := rbac.JWT().ParseToken(req.Authorization)
	if err != nil || claims.SessionID == "" {
		return nil, fmt.Errorf("company switching requires a session access token")
	}
	sessionID, err := uuid.Parse(claims.SessionID)
	if err != nil {
		return nil, fmt.Errorf("invalid session")
	}
	if err = tenancy.MoveSessionCompany(ctx, sessionID, current.UserID, companyID); err != nil {
		return nil, err
	}
	roles, err := rbac.Account().GetAccountRoles(ctx, current.AccountID)
	if err != nil {
		return nil, err
	}
	roleNames := make([]string, 0, len(roles))
	for _, role := range roles {
		roleNames = append(roleNames, role.RoleName)
	}
	token, _, err := rbac.JWT().GenerateTenantToken(current.AccountID, claims.Username, roleNames, target, sessionID)
	if err != nil {
		return nil, err
	}
	res.Data.Token = token
	res.Data.TTL = gconv.Int64(rbac.JWT().AccessExpiry.Seconds())
	res.Data.ActiveCompanyID = companyID.String()
	res.Data.Role = target.Role
	res.SetSuccess("Active company switched")
	return res, nil
}
