package routes

import (
	"encoding/xml"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	userAIUploadsDir = "/app/data/uploads/user_ai"
	sitemapLastMod   = "2026-06-27"
)

var sitemapPublicPaths = []struct {
	Path     string
	Priority string
}{
	{Path: "/", Priority: "1.0"},
	{Path: "/key-usage", Priority: "0.7"},
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc        string `xml:"loc"`
	LastMod    string `xml:"lastmod"`
	ChangeFreq string `xml:"changefreq"`
	Priority   string `xml:"priority"`
}

// RegisterCommonRoutes registers health, public metadata, and static upload routes.
func RegisterCommonRoutes(r *gin.Engine) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	r.GET("/robots.txt", serveRobotsTxt)
	r.GET("/sitemap.xml", serveSitemapXML)

	// Claude Code probe logging endpoint: ignore and return 200.
	r.POST("/api/event_logging/batch", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	// Setup status endpoint used by the frontend after setup has completed.
	r.GET("/setup/status", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"needs_setup": false,
				"step":        "completed",
			},
		})
	})

	r.StaticFS("/uploads/user_ai", gin.Dir(userAIUploadsDir, false))
}

func serveRobotsTxt(c *gin.Context) {
	baseURL := requestBaseURL(c)
	lines := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /admin",
		"Disallow: /api/",
		"Disallow: /v1/",
		"Disallow: /v1beta/",
		"Disallow: /backend-api/",
		"Disallow: /auth/",
		"Disallow: /login",
		"Disallow: /register",
		"Disallow: /email-verify",
		"Disallow: /forgot-password",
		"Disallow: /reset-password",
		"Disallow: /payment/",
		"Sitemap: " + baseURL + "/sitemap.xml",
	}

	c.Data(http.StatusOK, "text/plain; charset=utf-8", []byte(strings.Join(lines, "\n")+"\n"))
}

func serveSitemapXML(c *gin.Context) {
	baseURL := requestBaseURL(c)
	urls := make([]sitemapURL, 0, len(sitemapPublicPaths))
	for _, item := range sitemapPublicPaths {
		urls = append(urls, sitemapURL{
			Loc:        baseURL + item.Path,
			LastMod:    sitemapLastMod,
			ChangeFreq: "weekly",
			Priority:   item.Priority,
		})
	}

	payload, err := xml.MarshalIndent(sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}, "", "  ")
	if err != nil {
		c.String(http.StatusInternalServerError, "failed to generate sitemap")
		return
	}

	body := append([]byte(xml.Header), payload...)
	body = append(body, '\n')
	c.Header("Cache-Control", "public, max-age=3600")
	c.Data(http.StatusOK, "application/xml; charset=utf-8", body)
}

func requestBaseURL(c *gin.Context) string {
	if c == nil || c.Request == nil {
		return ""
	}

	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	if proto := strings.ToLower(firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Proto"))); proto == "http" || proto == "https" {
		scheme = proto
	}

	host := strings.TrimSpace(c.Request.Host)
	if forwardedHost := firstForwardedHeaderValue(c.GetHeader("X-Forwarded-Host")); forwardedHost != "" {
		host = forwardedHost
	}
	if host == "" {
		host = "localhost"
	}

	return scheme + "://" + host
}

func firstForwardedHeaderValue(value string) string {
	parts := strings.Split(value, ",")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}
