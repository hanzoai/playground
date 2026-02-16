//go:build embedded

// UI embedding and route registration for Playground (embedded build).

package client

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

//go:embed dist/* dist/**
var UIFiles embed.FS

// RegisterUIRoutes registers the UI routes with the Gin engine.
// The UI is served at the root path /.
func RegisterUIRoutes(router *gin.Engine) {
	// Create a sub-filesystem that strips the "dist" prefix
	uiFS, err := fs.Sub(UIFiles, "dist")
	if err != nil {
		panic("Failed to create UI filesystem: " + err.Error())
	}

	fileServer := http.FileServer(http.FS(uiFS))

	// Serve root
	router.GET("/", func(c *gin.Context) {
		serveIndex(c)
	})

	// Redirect legacy /ui paths to root
	router.GET("/ui", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/")
	})
	router.GET("/ui/*filepath", func(c *gin.Context) {
		path := c.Param("filepath")
		if path == "/" || path == "" {
			c.Redirect(http.StatusMovedPermanently, "/")
		} else {
			c.Redirect(http.StatusMovedPermanently, path)
		}
	})

	// Serve known static asset directories from the Vite build
	router.GET("/assets/*filepath", func(c *gin.Context) {
		fileServer.ServeHTTP(c.Writer, c.Request)
	})

	// Serve specific root-level static files
	for _, name := range []string{"favicon.svg", "favicon.ico", "robots.txt", "manifest.json"} {
		name := name
		router.GET("/"+name, func(c *gin.Context) {
			fileServer.ServeHTTP(c.Writer, c.Request)
		})
	}

	// SPA fallback — serve index.html for all routes that don't match API or static files
	router.NoRoute(func(c *gin.Context) {
		path := c.Request.URL.Path

		// Don't intercept API routes
		if strings.HasPrefix(path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "endpoint not found"})
			return
		}

		// Check if it looks like a static asset request
		if isStaticAssetPath(path) {
			c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
			return
		}

		// SPA fallback — serve index.html
		serveIndex(c)
	})
}

func serveIndex(c *gin.Context) {
	indexHTML, err := UIFiles.ReadFile("dist/index.html")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load UI"})
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, string(indexHTML))
}

func isStaticAssetPath(path string) bool {
	p := strings.ToLower(path)
	exts := []string{".js", ".css", ".html", ".ico", ".png", ".jpg", ".jpeg", ".gif",
		".svg", ".woff", ".woff2", ".ttf", ".eot", ".map", ".json", ".xml", ".txt"}
	for _, ext := range exts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// IsUIEmbedded checks if UI files are embedded in the binary.
func IsUIEmbedded() bool {
	_, err := UIFiles.ReadFile("dist/index.html")
	return err == nil
}
