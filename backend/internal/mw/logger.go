package mw

import (
	"feedsystem_video_go/internal/pkg/logger"
	"log/slog"

	"github.com/gin-gonic/gin"
)

func LoggerHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		rid, _ := c.Get("request_id")
		attrs := []slog.Attr{
			slog.Any("rid", rid),
			slog.String("path", c.Request.URL.Path),
			slog.String("method", c.Request.Method),
		}
		if len(c.Errors) != 0 {
			err := c.Errors.Last().Err
			attrs = append(attrs, slog.String("error", err.Error()))
		}
		n := c.Writer.Status()
		switch {
		case n >= 500:
			logger.Error("[GIN ServerError]", attrs...)
		case n >= 400:
			logger.Warn("[GIN ClientError]", attrs...)
		default:
			logger.Info("[GIN Success]", attrs...)
		}
	}
}
