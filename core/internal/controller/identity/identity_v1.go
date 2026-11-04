package identity

import (
	v1 "billionmail-core/api/identity/v1"
	"billionmail-core/internal/service/identity"
	"billionmail-core/internal/service/public"
	"billionmail-core/internal/service/rbac"
	"context"
)

func publicURL() string { return public.MustGetDockerEnv("SAAS_PUBLIC_URL", "") }

func (c *ControllerV1) Register(ctx context.Context, req *v1.RegisterReq) (*v1.RegisterRes, error) {
	res := &v1.RegisterRes{}
	userID, err := identity.Register(ctx, identity.Registration{
		Username: req.Username, Email: req.Email, Password: req.Password, CompanyName: req.CompanyName,
		Timezone: req.Timezone, Locale: req.Locale,
	}, rbac.JWT().Secret, publicURL())
	if err != nil {
		return nil, err
	}
	res.Data.UserID = userID.String()
	res.SetSuccess("Registration successful; check your email to verify the account")
	return res, nil
}

func (c *ControllerV1) VerifyEmail(ctx context.Context, req *v1.VerifyEmailReq) (*v1.VerifyEmailRes, error) {
	res := &v1.VerifyEmailRes{}
	if err := identity.VerifyEmail(ctx, req.Token); err != nil {
		return nil, err
	}
	res.SetSuccess("Email verified")
	return res, nil
}

func (c *ControllerV1) ForgotPassword(ctx context.Context, req *v1.ForgotPasswordReq) (*v1.ForgotPasswordRes, error) {
	res := &v1.ForgotPasswordRes{}
	// Always return the same response to prevent account enumeration.
	_ = identity.RequestPasswordReset(ctx, req.Email, rbac.JWT().Secret, publicURL())
	res.SetSuccess("If the account exists, password reset instructions have been queued")
	return res, nil
}

func (c *ControllerV1) ResetPassword(ctx context.Context, req *v1.ResetPasswordReq) (*v1.ResetPasswordRes, error) {
	res := &v1.ResetPasswordRes{}
	if err := identity.ResetPassword(ctx, req.Token, req.Password); err != nil {
		return nil, err
	}
	res.SetSuccess("Password reset successful")
	return res, nil
}
