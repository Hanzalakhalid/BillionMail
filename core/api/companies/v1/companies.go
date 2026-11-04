package v1

import (
	"billionmail-core/internal/service/tenancy"
	"billionmail-core/utility/types/api_v1"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/companies/list" method:"get" tags:"Companies" summary:"List active company memberships"`
}
type ListRes struct {
	api_v1.StandardRes
	Data struct {
		Companies       []tenancy.Company `json:"companies"`
		ActiveCompanyID string            `json:"active_company_id"`
	} `json:"data"`
}

type CreateReq struct {
	g.Meta   `path:"/v1/companies/create" method:"post" tags:"Companies" summary:"Create a company workspace"`
	Name     string `json:"name" v:"required|max-length:160"`
	Slug     string `json:"slug" v:"required|max-length:80"`
	Timezone string `json:"timezone" d:"UTC"`
	Locale   string `json:"locale" d:"en"`
}
type CreateRes struct {
	api_v1.StandardRes
	Data tenancy.Company `json:"data"`
}

type SwitchReq struct {
	g.Meta        `path:"/v1/companies/switch" method:"post" tags:"Companies" summary:"Switch active company"`
	Authorization string `json:"authorization" in:"header"`
	CompanyID     string `json:"company_id" v:"required|uuid"`
}
type SwitchRes struct {
	api_v1.StandardRes
	Data struct {
		Token           string `json:"token"`
		TTL             int64  `json:"ttl"`
		ActiveCompanyID string `json:"active_company_id"`
		Role            string `json:"role"`
	} `json:"data"`
}

type MembersReq struct {
	g.Meta `path:"/v1/companies/members/list" method:"get" tags:"Companies" summary:"List company members"`
}
type MembersRes struct {
	api_v1.StandardRes
	Data struct {
		Members []tenancy.Member `json:"members"`
	} `json:"data"`
}

type InvitationsReq struct {
	g.Meta `path:"/v1/companies/invitations/list" method:"get" tags:"Companies" summary:"List company invitations"`
}
type InvitationsRes struct {
	api_v1.StandardRes
	Data struct {
		Invitations []tenancy.Invitation `json:"invitations"`
	} `json:"data"`
}

type InviteReq struct {
	g.Meta `path:"/v1/companies/invitations/create" method:"post" tags:"Companies" summary:"Invite a company member"`
	Email  string `json:"email" v:"required|email|max-length:320"`
	Role   string `json:"role" v:"required|in:admin,manager,agent,viewer"`
}
type InviteRes struct {
	api_v1.StandardRes
	Data tenancy.Invitation `json:"data"`
}

type AcceptInvitationReq struct {
	g.Meta `path:"/v1/companies/invitations/accept" method:"post" tags:"Companies" summary:"Accept a company invitation"`
	Token  string `json:"token" v:"required|min-length:32|max-length:128"`
}
type AcceptInvitationRes struct {
	api_v1.StandardRes
	Data struct {
		CompanyID string `json:"company_id"`
	} `json:"data"`
}

type ChangeRoleReq struct {
	g.Meta       `path:"/v1/companies/members/change_role" method:"post" tags:"Companies" summary:"Change member role"`
	MembershipID string `json:"membership_id" v:"required|uuid"`
	Role         string `json:"role" v:"required|in:admin,manager,agent,viewer"`
}
type ChangeRoleRes struct{ api_v1.StandardRes }

type RemoveMemberReq struct {
	g.Meta       `path:"/v1/companies/members/remove" method:"post" tags:"Companies" summary:"Remove company member"`
	MembershipID string `json:"membership_id" v:"required|uuid"`
}
type RemoveMemberRes struct{ api_v1.StandardRes }

type RevokeInvitationReq struct {
	g.Meta       `path:"/v1/companies/invitations/revoke" method:"post" tags:"Companies" summary:"Revoke company invitation"`
	InvitationID string `json:"invitation_id" v:"required|uuid"`
}
type RevokeInvitationRes struct{ api_v1.StandardRes }
