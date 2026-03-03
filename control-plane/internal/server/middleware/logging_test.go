package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/hanzoai/playground/control-plane/internal/logger"
	"github.com/stretchr/testify/require"
)

func TestStructuredLoggerSetsStatus(t *testing.T) {
	logger.InitLogger(false)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(StructuredLogger())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestStructuredLoggerDoesNotPanic(t *testing.T) {
	logger.InitLogger(true)
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(StructuredLogger())
	router.GET("/500", func(c *gin.Context) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "bad"})
	})
	router.GET("/404", func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "missing"})
	})

	for _, path := range []string{"/500", "/404"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		w := httptest.NewRecorder()
		require.NotPanics(t, func() { router.ServeHTTP(w, req) })
	}
}
