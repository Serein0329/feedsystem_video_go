package mw

import (
	"feedsystem_video_go/internal/pkg/apierror"
	"feedsystem_video_go/internal/pkg/response"
	"fmt"
	"log/slog"
	"runtime/debug"

	"github.com/gin-gonic/gin"
)

func RecoveryHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				rid, _ := c.Get("request_id")
				msg := fmt.Sprintf("[PANIC] rid=%v path=%s method=%s panic=%v\n%s",
					rid,
					c.Request.URL.Path,
					c.Request.Method,
					r,
					string(debug.Stack()))
				slog.Info(msg)
				response.FailByErrCode(c, apierror.ErrInternalServer)
				c.Abort()
			}
		}()
		c.Next()
	}
}
