package v1

import (
	"billionmail-core/utility/types/api_v1"
	"github.com/gogf/gf/v2/frame/g"
)

type RegisterReq struct {
	g.Meta      `path:"/v1/auth/register" method:"post" tags:"Identity" summary:"Register user and company"`
	Username    string `json:"username" v:"required|min-length:3|max-length:64"`
	Email       string `json:"email" v:"required|email|max-length:320"`
	Password    string `json:"password" v:"required|min-length:12|max-length:128"`
	CompanyName string `json:"company_name" v:"required|max-length:160"`
	Timezone    string `json:"timezone" d:"UTC"`
	Locale      string `json:"locale" d:"en"`
}
type RegisterRes struct {
	api_v1.StandardRes
	Data struct {
		UserID string `json:"user_id"`
	} `json:"data"`
}

type VerifyEmailReq struct {
	g.Meta `path:"/v1/auth/verify-email" method:"post" tags:"Identity" summary:"Verify email"`
	Token  string `json:"token" v:"required|min-length:32|max-length:128"`
}
type VerifyEmailRes struct{ api_v1.StandardRes }

type ForgotPasswordReq struct {
	g.Meta `path:"/v1/auth/forgot-password" method:"post" tags:"Identity" summary:"Request password reset"`
	Email  string `json:"email" v:"required|email|max-length:320"`
}
type ForgotPasswordRes struct{ api_v1.StandardRes }

type ResetPasswordReq struct {
	g.Meta   `path:"/v1/auth/reset-password" method:"post" tags:"Identity" summary:"Reset password"`
	Token    string `json:"token" v:"required|min-length:32|max-length:128"`
	Password string `json:"password" v:"required|min-length:12|max-length:128"`
}
type ResetPasswordRes struct{ api_v1.StandardRes }
