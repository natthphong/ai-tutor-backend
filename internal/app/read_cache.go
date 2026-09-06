package app

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"time"
)

func (a *App) invalidateUser(uid string) {
	if a.Cache == nil {
		return
	}
	a.cacheMu.Lock()
	a.cacheEpoch[uid]++
	a.cacheMu.Unlock()
	_ = a.Cache.DeletePrefix(context.Background(), "user:"+uid+":")
}
func (a *App) readCache(c *fiber.Ctx) error {
	uid := user(c).ID
	path := c.Path()
	base := "/ai-tutor/api/v2"
	ttl := time.Duration(a.Cfg.CacheTTLSeconds) * time.Second
	allowed := path == base+"/curriculum" || path == base+"/daily-plan" || path == base+"/library" || path == base+"/progress" || path == base+"/scenarios" || path == base+"/review"
	if a.Cache == nil || ttl <= 0 {
		return c.Next()
	}
	if c.Method() != "GET" || !allowed {
		e := c.Next()
		if c.Method() != "GET" || (c.Method() == "GET" && len(path) > len(base+"/sessions/") && path[:len(base+"/sessions/")] == base+"/sessions/") {
			a.invalidateUser(uid)
		}
		return e
	}
	a.cacheMu.Lock()
	epoch := a.cacheEpoch[uid]
	a.cacheMu.Unlock()
	key := fmt.Sprintf("user:%s:%d:%s", uid, epoch, path)
	if b, ok, e := a.Cache.Get(c.UserContext(), key); e == nil && ok {
		c.Set("X-App-Cache", "HIT")
		c.Type("json")
		return c.Send(b)
	}
	c.Set("X-App-Cache", "MISS")
	e := c.Next()
	if e == nil && c.Response().StatusCode() == 200 {
		_ = a.Cache.Set(c.UserContext(), key, c.Response().Body(), ttl)
	}
	return e
}
