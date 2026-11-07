package v1

import (
	"billionmail-core/internal/service/products"
	"github.com/gogf/gf/v2/frame/g"
)

type ListReq struct {
	g.Meta `path:"/v1/products/list" method:"get" tags:"Products" summary:"List workspace products"`
	Cursor string `json:"cursor"`
	Limit  int    `json:"limit" d:"30" v:"between:1,100"`
}
type ListRes = products.Page
type GetReq struct {
	g.Meta `path:"/v1/products/get" method:"get" tags:"Products" summary:"Get a product"`
	ID     string `json:"id" v:"required"`
}
type GetRes = products.Product
type SaveReq struct {
	g.Meta          `path:"/v1/products/save" method:"post" tags:"Products" summary:"Create an immutable product version"`
	ID              string   `json:"id"`
	Revision        int64    `json:"revision"`
	Name            string   `json:"name"`
	SKU             string   `json:"sku"`
	Description     string   `json:"description"`
	Benefits        []string `json:"benefits"`
	URL             string   `json:"url"`
	Price           string   `json:"price"`
	Currency        string   `json:"currency"`
	AssetVersionIDs []string `json:"asset_version_ids"`
}
type SaveRes = products.Product
type VersionsReq struct {
	g.Meta `path:"/v1/products/versions" method:"get" tags:"Products" summary:"List product version history"`
	ID     string `json:"id" v:"required"`
	Before int64  `json:"before"`
}
type VersionsRes struct {
	Items      []products.Version `json:"items"`
	NextBefore int64              `json:"next_before"`
}
type DeleteReq struct {
	g.Meta `path:"/v1/products/delete" method:"post" tags:"Products" summary:"Remove a product from the catalog"`
	ID     string `json:"id" v:"required"`
}
type DeleteRes struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}
