package health

import (
	"context"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/net/ghttp"
	"net/http"
	"time"
)

func Live(r *ghttp.Request) {
	r.Response.WriteHeader(http.StatusOK)
	r.Response.WriteJson(g.Map{"status": "ok", "service": "billionmail-core", "time": time.Now().UTC()})
}

func Ready(r *ghttp.Request) {
	ctx, cancel := context.WithTimeout(r.GetCtx(), 2*time.Second)
	defer cancel()
	checks := g.Map{"postgres": "ok", "redis": "ok"}
	status := http.StatusOK
	if _, err := g.DB().Exec(ctx, "SELECT 1"); err != nil {
		checks["postgres"] = "unavailable"
		status = http.StatusServiceUnavailable
	}
	if _, err := g.Redis().Do(ctx, "PING"); err != nil {
		checks["redis"] = "unavailable"
		status = http.StatusServiceUnavailable
	}
	state := "ok"
	if status != http.StatusOK {
		state = "unavailable"
	}
	r.Response.WriteHeader(status)
	r.Response.WriteJson(g.Map{"status": state, "checks": checks})
}
