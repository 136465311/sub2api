package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRegisterCommonRoutesSEOFiles(t *testing.T) {
	t.Run("robots_txt_points_to_current_host_sitemap", func(t *testing.T) {
		router := gin.New()
		RegisterCommonRoutes(router)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/robots.txt", nil)
		req.Host = "sub2api.example"
		req.Header.Set("X-Forwarded-Proto", "https")
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "text/plain")
		assert.Contains(t, w.Body.String(), "User-agent: *")
		assert.Contains(t, w.Body.String(), "Disallow: /admin")
		assert.Contains(t, w.Body.String(), "Sitemap: https://sub2api.example/sitemap.xml")
	})

	t.Run("sitemap_xml_lists_public_indexable_routes", func(t *testing.T) {
		router := gin.New()
		RegisterCommonRoutes(router)

		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
		req.Host = "sub2api.example"
		req.Header.Set("X-Forwarded-Proto", "https")
		router.ServeHTTP(w, req)

		body := w.Body.String()

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/xml")
		assert.Contains(t, body, "<loc>https://sub2api.example/</loc>")
		assert.Contains(t, body, "<loc>https://sub2api.example/key-usage</loc>")
		assert.NotContains(t, body, "/login")
		assert.NotContains(t, body, "/admin")
		assert.True(t, strings.HasPrefix(body, "<?xml"))
	})
}
