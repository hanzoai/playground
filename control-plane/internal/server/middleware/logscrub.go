package middleware

import (
	"strings"

	"github.com/gin-gonic/gin"
)

const scrubContextKey = "logscrub_active"

// ScrubBody marks requests to sensitive endpoints so that request body logging
// is suppressed. Paths are matched with a prefix check against the provided
// list. Additionally, any path containing "/secrets/" or "/auth/" is
// unconditionally scrubbed.
//
// Downstream loggers should check IsScrubbed(c) before emitting request bodies.
func ScrubBody(paths ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		reqPath := c.Request.URL.Path

		// Always scrub secrets and auth paths.
		if strings.Contains(reqPath, "/secrets/") || strings.Contains(reqPath, "/auth/") {
			c.Set(scrubContextKey, true)
			c.Next()
			return
		}

		for _, p := range paths {
			if strings.HasPrefix(reqPath, p) || matchParamPath(reqPath, p) {
				c.Set(scrubContextKey, true)
				c.Next()
				return
			}
		}

		c.Next()
	}
}

// IsScrubbed returns true if the current request has been flagged for body scrubbing.
func IsScrubbed(c *gin.Context) bool {
	v, exists := c.Get(scrubContextKey)
	if !exists {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

// matchParamPath checks whether reqPath matches a parameterized route pattern
// like "/api/v1/spaces/:id/git/clone". The :param segments match any single
// path segment.
func matchParamPath(reqPath, pattern string) bool {
	reqParts := strings.Split(strings.Trim(reqPath, "/"), "/")
	patParts := strings.Split(strings.Trim(pattern, "/"), "/")

	if len(reqParts) != len(patParts) {
		return false
	}

	for i, pat := range patParts {
		if strings.HasPrefix(pat, ":") {
			continue // wildcard segment
		}
		if reqParts[i] != pat {
			return false
		}
	}
	return true
}
