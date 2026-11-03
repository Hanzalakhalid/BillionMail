package video_outreach

import (
	"billionmail-core/internal/model/entity"
	vo "billionmail-core/internal/service/video_outreach"
	"context"
	"html/template"
	"net/http"
	"regexp"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
)

const landingPageHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Personalized Video for {{.Name}}</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;background:#0f172a;color:#e2e8f0;min-height:100vh;display:flex;align-items:center;justify-content:center}
.container{max-width:800px;width:100%;padding:2rem}
h1{font-size:1.5rem;font-weight:600;margin-bottom:1.5rem;text-align:center}
.video-wrap{position:relative;border-radius:12px;overflow:hidden;box-shadow:0 25px 50px -12px rgba(0,0,0,0.5);background:#1e293b}
video{width:100%;display:block}
.cta{display:block;margin:2rem auto 0;padding:.875rem 2rem;background:#3b82f6;color:#fff;border:none;border-radius:8px;font-size:1rem;font-weight:500;cursor:pointer;text-align:center;text-decoration:none;transition:background .2s}
.cta:hover{background:#2563eb}
</style>
</head>
<body>
<div class="container">
<h1>A quick message for you, {{.Name}}</h1>
<div class="video-wrap">
<video controls playsinline{{if .Thumb}} poster="{{.Thumb}}"{{end}}>
<source src="{{.Video}}" type="video/mp4">
Your browser does not support video.
</video>
</div>
<a class="cta" href="{{.CTA}}">Book a Call</a>
</div>
{{if .Token}}<script>
(function(){
var video=document.querySelector("video"),sent=false;
video.addEventListener("play",function(){
if(sent)return;sent=true;
var url="/landing/video/event?e=play&t="+encodeURIComponent({{.Token}});
if(navigator.sendBeacon){navigator.sendBeacon(url)}else{fetch(url,{method:"POST",keepalive:true})}
});
})();
</script>{{end}}
</body>
</html>`

var landingTmpl = template.Must(template.New("landing").Parse(landingPageHTML))

var tokenPattern = regexp.MustCompile(`^[a-f0-9]{16,64}$`)

// contactByToken finds the contact a landing page token belongs to.
func contactByToken(ctx context.Context, token string) (entity.Contact, bool) {
	var contact entity.Contact
	if !tokenPattern.MatchString(token) {
		return contact, false
	}
	err := g.DB().Model("bm_contacts").
		Where("attribs->>'"+vo.AttrVideoToken+"' = ?", token).
		Limit(1).
		Scan(&contact)
	if err != nil {
		g.Log().Warningf(ctx, "look up video token: %v", err)
	}
	return contact, err == nil && contact.Id > 0
}

// ServeLandingPage renders the personalized video landing page.
// Public route — no auth required. With ?t=<token> the page is built from the contact and
// views, plays and Book a Call clicks are tracked; ?video=&name=&thumb=&cta= render it untracked.
func ServeLandingPage(r *ghttp.Request) {
	data := map[string]string{
		"Video": r.GetQuery("video").String(),
		"Thumb": r.GetQuery("thumb").String(),
		"Name":  r.GetQuery("name").String(),
		"CTA":   r.GetQuery("cta").String(),
		"Token": "",
	}

	if token := r.GetQuery("t").String(); token != "" {
		contact, ok := contactByToken(r.Context(), token)
		if !ok || contact.Attribs[vo.AttrVideoURL] == "" {
			r.Response.WriteStatus(http.StatusNotFound, "video not found")
			return
		}

		name := contact.Attribs["name"]
		if name == "" {
			name, _, _ = strings.Cut(contact.Email, "@")
		}
		data["Name"] = name
		data["Video"] = contact.Attribs[vo.AttrVideoURL]
		data["Thumb"] = contact.Attribs[vo.AttrThumbnailURL]
		data["CTA"] = "/landing/video/go?t=" + token
		data["Token"] = token

		vo.RecordEvent(r.Context(), contact.Id, vo.EventView)
	}

	if data["Video"] == "" || data["Name"] == "" {
		r.Response.WriteStatus(http.StatusBadRequest, "missing video or name param")
		return
	}
	// Without a separate thumbnail the video URL is stored as one; it is not an image
	if data["Thumb"] == data["Video"] {
		data["Thumb"] = ""
	}
	if data["CTA"] == "" {
		data["CTA"] = "#"
	}

	r.Response.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = landingTmpl.Execute(r.Response.Writer, data)
}

// ServeLandingCTA records a Book a Call click and redirects to the contact's booking link.
func ServeLandingCTA(r *ghttp.Request) {
	token := r.GetQuery("t").String()
	contact, ok := contactByToken(r.Context(), token)
	if !ok {
		r.Response.WriteStatus(http.StatusNotFound, "video not found")
		return
	}

	vo.RecordEvent(r.Context(), contact.Id, vo.EventCTA)

	// Only http(s) links stored by the admin are followed, never a URL from the request
	target := contact.Attribs[vo.AttrCtaURL]
	if !strings.HasPrefix(target, "https://") && !strings.HasPrefix(target, "http://") {
		target = "/landing/video?t=" + token
	}
	r.Response.RedirectTo(target, http.StatusFound)
}

// ServeLandingEvent records events reported by the landing page script (currently video plays).
func ServeLandingEvent(r *ghttp.Request) {
	if r.GetQuery("e").String() == vo.EventPlay {
		if contact, ok := contactByToken(r.Context(), r.GetQuery("t").String()); ok {
			vo.RecordEvent(r.Context(), contact.Id, vo.EventPlay)
		}
	}
	r.Response.WriteHeader(http.StatusNoContent)
}
