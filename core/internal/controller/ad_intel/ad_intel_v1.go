package ad_intel

import (
	v1 "billionmail-core/api/ad_intel/v1"
	"billionmail-core/internal/service/ad_intel"
	"context"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
)

func (c *ControllerV1) GetConfig(ctx context.Context, req *v1.GetConfigReq) (res *v1.GetConfigRes, err error) {
	res = &v1.GetConfigRes{}

	cfg, err := ad_intel.LoadConfig(ctx)
	if err != nil {
		g.Log().Error(ctx, "load ad finder config: %v", err)
		res.SetError(gerror.New("Failed to load Ad Finder settings"))
		return res, nil
	}

	res.Data = cfg.Masked()
	res.SetSuccess("Success")
	return
}

func (c *ControllerV1) SaveConfig(ctx context.Context, req *v1.SaveConfigReq) (res *v1.SaveConfigRes, err error) {
	res = &v1.SaveConfigRes{}

	if err = ad_intel.SaveConfig(ctx, &req.Config); err != nil {
		res.SetError(gerror.Newf("Failed to save Ad Finder settings: %v", err))
		return res, nil
	}

	res.SetSuccess("Ad Finder settings saved")
	return
}

func (c *ControllerV1) Search(ctx context.Context, req *v1.SearchReq) (res *v1.SearchRes, err error) {
	res = &v1.SearchRes{}

	result, err := ad_intel.Search(ctx, req.Keyword, req.Country, req.Limit)
	if err != nil {
		res.SetError(gerror.Newf("Search failed: %v", err))
		return res, nil
	}

	res.Data = result
	res.SetSuccess("Ads fetched")
	return
}

func (c *ControllerV1) List(ctx context.Context, req *v1.ListReq) (res *v1.ListRes, err error) {
	res = &v1.ListRes{}

	q := g.DB().Model("bm_ad_advertisers")
	if req.Keyword != "" {
		q = q.Where("keyword", req.Keyword)
	}
	if req.Search != "" {
		like := "%" + req.Search + "%"
		q = q.Where("(page_name ILIKE ? OR website ILIKE ? OR ad_topic ILIKE ? OR ad_text ILIKE ?)", like, like, like, like)
	}

	total, err := q.Count()
	if err != nil {
		g.Log().Error(ctx, "count advertisers: %v", err)
		total = 0
	}

	list := make([]*v1.AdvertiserRow, 0)
	err = q.OrderDesc("score").OrderDesc("days_running").Page(req.Page, req.PageSize).Scan(&list)
	if err != nil {
		g.Log().Error(ctx, "list advertisers: %v", err)
	}
	for _, row := range list {
		row.AdLibraryUrl = ad_intel.AdLibraryPageURL(row.PageId, row.Country)
	}

	keywords, err := g.DB().Model("bm_ad_advertisers").Fields("keyword").Distinct().OrderAsc("keyword").Array()
	if err != nil {
		g.Log().Error(ctx, "list ad searches: %v", err)
	}
	res.Data.Keywords = make([]string, 0, len(keywords))
	for _, k := range keywords {
		res.Data.Keywords = append(res.Data.Keywords, k.String())
	}

	res.Data.Total = total
	res.Data.List = list
	res.SetSuccess("Success")
	return res, nil
}

func (c *ControllerV1) AddLead(ctx context.Context, req *v1.AddLeadReq) (res *v1.AddLeadRes, err error) {
	res = &v1.AddLeadRes{}

	count, err := g.DB().Model("bm_contact_groups").Where("id", req.GroupId).Count()
	if err != nil || count == 0 {
		res.SetError(gerror.New("Contact group not found"))
		return res, nil
	}

	email, err := ad_intel.AddToLeads(ctx, req.PageId, req.GroupId, req.Email)
	if err != nil {
		res.SetError(gerror.Newf("Could not add lead: %v", err))
		return res, nil
	}

	res.Data.Email = email
	res.SetSuccess("Added to leads")
	return
}
