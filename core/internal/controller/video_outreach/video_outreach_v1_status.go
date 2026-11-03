package video_outreach

import (
	v1 "billionmail-core/api/video_outreach/v1"
	"billionmail-core/internal/model/entity"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetVideoStatus(ctx context.Context, req *v1.GetVideoStatusReq) (res *v1.GetVideoStatusRes, err error) {
	res = &v1.GetVideoStatusRes{}

	var contact entity.Contact
	err = g.DB().Model("bm_contacts").
		Where("email", req.ContactEmail).
		Where("group_id", req.GroupID).
		OrderDesc("create_time").
		Limit(1).
		Scan(&contact)
	if err != nil {
		res.SetError(gerror.New("Failed to look up contact"))
		return
	}
	if contact.Id == 0 {
		res.SetError(gerror.New("Contact not found"))
		return
	}

	res.Data = toVideoStatus(contact)
	res.SetSuccess("Video status retrieved")
	return
}
