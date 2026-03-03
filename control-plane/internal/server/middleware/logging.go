package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
)

// StructuredLogger is a Gin middleware that emits structured JSON request logs
// via the global zerolog logger. It replaces gin.Logger with machine-readable
// output suitable for VictoriaMetrics/Loki log ingestion.
func StructuredLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()

		event := logger.Logger.Info()
		if status >= 500 {
			event = logger.Logger.Error()
		} else if status >= 400 {
			event = logger.Logger.Warn()
		}

		event.
			Str("method", c.Request.Method).
			Str("path", path).
			Str("query", query).
			Int("status", status).
			Dur("latency", latency).
			Int("bytes", c.Writer.Size()).
			Str("client_ip", c.ClientIP()).
			Str("user_agent", c.Request.UserAgent())

		if errMsg := c.Errors.ByType(gin.ErrorTypePrivate).String(); errMsg != "" {
			event.Str("error", errMsg)
		}

		// Include IAM user info if present.
		if user := GetIAMUser(c); user != nil {
			event.Str("iam_user", user.Sub)
			if user.Organization != "" {
				event.Str("iam_org", user.Organization)
			}
		}

		event.Msg("http_request")
	}
}
