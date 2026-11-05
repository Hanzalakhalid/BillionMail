package whatsapp

import (
	"context"

	v1 "billionmail-core/api/whatsapp/v1"
	"billionmail-core/internal/service/secrets"
	"billionmail-core/internal/service/tenancy"
	"billionmail-core/internal/service/whatsappchannel"
)

func (c *ControllerV1) ListChannels(ctx context.Context, _ *v1.ListChannelsReq) (*v1.ListChannelsRes, error) {
	res := &v1.ListChannelsRes{}
	tenant, err := tenancy.Require(ctx)
	if err == nil {
		res.Data, err = whatsappchannel.List(ctx, tenant)
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) ConnectChannel(ctx context.Context, req *v1.ConnectChannelReq) (*v1.ConnectChannelRes, error) {
	res := &v1.ConnectChannelRes{}
	tenant, err := tenancy.Require(ctx)
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	store, err := secrets.NewConfiguredStore(ctx)
	if err == nil {
		res.Data, err = whatsappchannel.Connect(ctx, tenant, store, whatsappchannel.ConnectInput{
			WABAID: req.WABAID, BusinessName: req.BusinessName, AccessToken: req.AccessToken,
			Scopes: []string{"whatsapp_business_messaging", "whatsapp_business_management"},
			PhoneNumbers: []whatsappchannel.PhoneAsset{{
				PhoneNumberID: req.PhoneNumberID, DisplayPhoneNumber: req.DisplayPhone,
				VerifiedName: req.VerifiedName,
			}},
		})
	}
	if err != nil {
		res.SetError(err)
		return res, nil
	}
	res.SetSuccess("WhatsApp channel connected")
	return res, nil
}
