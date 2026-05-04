package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestScrubBody_SecretsPath(t *testing.T) {
	router := gin.New()
	router.Use(ScrubBody())

	var scrubbed bool
	router.GET("/api/v1/secrets/mykey", func(c *gin.Context) {
		scrubbed = IsScrubbed(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/secrets/mykey", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, scrubbed, "secrets path should be scrubbed")
}

func TestScrubBody_AuthPath(t *testing.T) {
	router := gin.New()
	router.Use(ScrubBody())

	var scrubbed bool
	router.POST("/v1/auth/token", func(c *gin.Context) {
		scrubbed = IsScrubbed(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/token", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, scrubbed, "auth path should be scrubbed")
}

func TestScrubBody_ExplicitPath(t *testing.T) {
	router := gin.New()
	router.Use(ScrubBody(
		"/api/v1/spaces/:id/git/clone",
		"/api/v1/spaces/:id/git/push",
	))

	var scrubbed bool
	router.POST("/api/v1/spaces/abc123/git/clone", func(c *gin.Context) {
		scrubbed = IsScrubbed(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/spaces/abc123/git/clone", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.True(t, scrubbed, "explicit clone path should be scrubbed")
}

func TestScrubBody_NormalPathNotScrubbed(t *testing.T) {
	router := gin.New()
	router.Use(ScrubBody(
		"/api/v1/spaces/:id/git/clone",
	))

	var scrubbed bool
	router.GET("/api/v1/nodes", func(c *gin.Context) {
		scrubbed = IsScrubbed(c)
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/nodes", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.False(t, scrubbed, "normal path should not be scrubbed")
}

func TestMatchParamPath(t *testing.T) {
	tests := []struct {
		name    string
		reqPath string
		pattern string
		want    bool
	}{
		{
			name:    "exact match with param",
			reqPath: "/api/v1/spaces/abc123/git/clone",
			pattern: "/api/v1/spaces/:id/git/clone",
			want:    true,
		},
		{
			name:    "different endpoint",
			reqPath: "/api/v1/spaces/abc123/git/push",
			pattern: "/api/v1/spaces/:id/git/clone",
			want:    false,
		},
		{
			name:    "length mismatch",
			reqPath: "/api/v1/spaces/abc123",
			pattern: "/api/v1/spaces/:id/git/clone",
			want:    false,
		},
		{
			name:    "multiple params",
			reqPath: "/api/v1/orgs/hanzo/secrets/dbpass",
			pattern: "/api/v1/orgs/:org/secrets/:key",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := matchParamPath(tt.reqPath, tt.pattern)
			assert.Equal(t, tt.want, got)
		})
	}
}
