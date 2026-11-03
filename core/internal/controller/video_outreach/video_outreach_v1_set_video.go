package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	"billionmail-core/internal/model/entity"
	"billionmail-core/internal/service/domains"
	vo "billionmail-core/internal/service/video_outreach"
	"context"
	"encoding/json"
	"strings"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) SetVideo(ctx context.Context, req *v1.SetVideoReq) (res *v1.SetVideoRes, err error) {
	res = &v1.SetVideoRes{}

	var contact entity.Contact
	err = g.DB().Model("bm_contacts").
		Where("email", req.ContactEmail).
		Where("group_id", req.GroupId).
		Limit(1).
		Scan(&contact)
	if err != nil || contact.Id == 0 {
		res.SetError(gerror.New("Contact not found"))
		return res, nil
	}

	name := contact.Attribs["name"]
	if name == "" {
		name, _, _ = strings.Cut(contact.Email, "@")
	}

	landing := vo.BuildLandingURL(domains.GetBaseURL(), name, req.VideoUrl, req.ThumbnailUrl, req.CtaUrl)

	// A video is "ready" once all three assets exist; without a thumbnail the video itself stands in
	thumb := req.ThumbnailUrl
	if thumb == "" {
		thumb = req.VideoUrl
	}
	update, _ := json.Marshal(map[string]string{
		vo.AttrVideoURL:       req.VideoUrl,
		vo.AttrThumbnailURL:   thumb,
		vo.AttrLandingPageURL: landing,
	})

	_, err = g.DB().Exec(ctx, `UPDATE bm_contacts SET attribs = COALESCE(attribs, '{}'::jsonb) || CAST(? AS jsonb) WHERE id = ?`,
		string(update), contact.Id)
	if err != nil {
		g.Log().Error(ctx, "set video for contact %d: %v", contact.Id, err)
		res.SetError(gerror.New("Failed to save video"))
		return res, nil
	}

	res.Data.LandingPageUrl = landing
	res.SetSuccess("Video saved")
	return
}
