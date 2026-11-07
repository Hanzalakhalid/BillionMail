package v1

import (
	"billionmail-core/internal/service/assets"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/assets/list" method:"get" tags:"Assets" summary:"List workspace asset library"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit" d:"30" v:"between:1,100"`
}
type ListRes = assets.Page
type BeginUploadReq struct {
	g.Meta `path:"/v1/assets/uploads" method:"post" tags:"Assets" summary:"Create a quarantined upload form"`
	assets.UploadInput
}
type BeginUploadRes = assets.Upload
type CompleteUploadReq struct {
	g.Meta    `path:"/v1/assets/uploads/complete" method:"post" tags:"Assets" summary:"Queue file validation"`
	VersionID string `json:"version_id" v:"required"`
}
type CompleteUploadRes = assets.Job
type DownloadReq struct {
	g.Meta    `path:"/v1/assets/download" method:"get" tags:"Assets" summary:"Get an expiring validated asset URL"`
	VersionID string `json:"version_id" v:"required"`
}
type DownloadRes struct {
	URL string `json:"url"`
}
type DeleteReq struct {
	g.Meta `path:"/v1/assets/delete" method:"post" tags:"Assets" summary:"Remove an asset from the workspace library"`
	ID     string `json:"id" v:"required"`
}
type DeleteRes struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
