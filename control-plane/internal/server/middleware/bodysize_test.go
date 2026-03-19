package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestMaxBodySize_AllowsSmallBody(t *testing.T) {
	router := gin.New()
	router.Use(MaxBodySize(1024))
	router.POST("/api/v1/test", func(c *gin.Context) {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, string(body))
	})

	body := bytes.NewBufferString("hello")
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", body)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello", w.Body.String())
}

func TestMaxBodySize_RejectsOversizedBody(t *testing.T) {
	router := gin.New()
	router.Use(MaxBodySize(10)) // 10 bytes max
	router.POST("/api/v1/test", func(c *gin.Context) {
		_, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.String(http.StatusRequestEntityTooLarge, "body too large")
			return
		}
		c.String(http.StatusOK, "ok")
	})

	// Send a body larger than 10 bytes
	largeBody := strings.NewReader(strings.Repeat("x", 100))
	req := httptest.NewRequest(http.MethodPost, "/api/v1/test", largeBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
}

func TestMaxBodySize_AllowsGETWithoutBody(t *testing.T) {
	router := gin.New()
	router.Use(MaxBodySize(10))
	router.GET("/api/v1/test", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}
