package companies

import (
	"billionmail-core/api/companies/v1"
	"context"
)

type ICompaniesV1 interface {
	List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error)
	Create(ctx context.Context, req *v1.CreateReq) (res *v1.CreateRes, err error)
	Switch(ctx context.Context, req *v1.SwitchReq) (res *v1.SwitchRes, err error)
	Members(ctx context.Context, req *v1.MembersReq) (res *v1.MembersRes, err error)
	Invitations(ctx context.Context, req *v1.InvitationsReq) (res *v1.InvitationsRes, err error)
	Invite(ctx context.Context, req *v1.InviteReq) (res *v1.InviteRes, err error)
	AcceptInvitation(ctx context.Context, req *v1.AcceptInvitationReq) (res *v1.AcceptInvitationRes, err error)
	ChangeRole(ctx context.Context, req *v1.ChangeRoleReq) (res *v1.ChangeRoleRes, err error)
	RemoveMember(ctx context.Context, req *v1.RemoveMemberReq) (res *v1.RemoveMemberRes, err error)
	RevokeInvitation(ctx context.Context, req *v1.RevokeInvitationReq) (res *v1.RevokeInvitationRes, err error)
}
