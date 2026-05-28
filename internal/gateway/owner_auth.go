package gateway

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	sessionCookieName = "nura_session"
	csrfCookieName    = "nura_csrf"
	csrfHeaderName    = "X-CSRF-Token"
)

type ownerAuth struct {
	bootstrapToken string
	sessionToken   string
	csrfToken      string
}

func newOwnerAuth() *ownerAuth {
	gen := func() string {
		b := make([]byte, 32)
		if _, err := rand.Read(b); err != nil {
			panic("crypto/rand: " + err.Error())
		}
		return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(b)
	}
	return &ownerAuth{
		bootstrapToken: gen(),
		sessionToken:   gen(),
		csrfToken:      gen(),
	}
}

func (oa *ownerAuth) handleLogin(c *gin.Context) {
	token := c.Query("token")
	if subtle.ConstantTimeCompare([]byte(token), []byte(oa.bootstrapToken)) != 1 {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     sessionCookieName,
		Value:    oa.sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     csrfCookieName,
		Value:    oa.csrfToken,
		Path:     "/",
		HttpOnly: false,
		SameSite: http.SameSiteLaxMode,
	})

	c.Redirect(http.StatusFound, "/")
}

func isExemptRoute(path string) bool {
	switch {
	case path == "/health", path == "/local/login":
		return true
	case strings.HasPrefix(path, "/static/"),
		strings.HasPrefix(path, "/share/"):
		return true
	default:
		return false
	}
}

func (oa *ownerAuth) requireSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isExemptRoute(c.Request.URL.Path) {
			c.Next()
			return
		}
		cookie, err := c.Cookie(sessionCookieName)
		if err != nil || subtle.ConstantTimeCompare([]byte(cookie), []byte(oa.sessionToken)) != 1 {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		c.Next()
	}
}

func (oa *ownerAuth) requireCSRF() gin.HandlerFunc {
	return func(c *gin.Context) {
		if isExemptRoute(c.Request.URL.Path) {
			c.Next()
			return
		}
		switch c.Request.Method {
		case http.MethodPost, http.MethodPut, http.MethodDelete:
			token := c.GetHeader(csrfHeaderName)
			if subtle.ConstantTimeCompare([]byte(token), []byte(oa.csrfToken)) != 1 {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
		}
		c.Next()
	}
}
