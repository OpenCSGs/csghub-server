package middleware

import (
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/builder/store/database"
)

const gitSuffix = ".git"

func GitHTTPParamMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		namespace := c.Param("namespace")
		repo_type := strings.TrimSuffix(c.Param("repo_type"), "s")

		if namespace == "" {
			httpbase.BadRequest(c, "invalid repository namespace")
			return
		}

		if repo_type == "" {
			httpbase.BadRequest(c, "invalid repository repository type")
			return
		}

		if name == "" || strings.TrimSuffix(name, gitSuffix) == "" {
			httpbase.BadRequest(c, "invalid repository name")
			return
		}

		if strings.HasSuffix(name, gitSuffix) {
			c.Set("name", strings.TrimSuffix(name, gitSuffix))
		} else {
			c.Set("name", name)
		}
		c.Set("namespace", namespace)
		c.Set("repo_type", repo_type)

		c.Next()
	}
}

func ContentEncoding() gin.HandlerFunc {
	return func(c *gin.Context) {
		var (
			body io.ReadCloser
			err  error
		)

		contentEncoding := c.Request.Header.Get("Content-Encoding")
		switch contentEncoding {
		case "":
			body = c.Request.Body
		case "gzip":
			body, err = gzip.NewReader(c.Request.Body)
		default:
			err = fmt.Errorf("unsupported content encoding: %s", contentEncoding)
		}

		if err != nil {
			httpbase.BadRequest(c, err.Error())
			c.Abort()
			return
		}
		defer body.Close()

		c.Request.Body = body
		c.Request.Header.Del("Content-Encoding")

		c.Next()
	}
}

func GetCurrentUserFromHeader() gin.HandlerFunc {
	userStore := database.NewUserStore()
	return func(c *gin.Context) {
		authHeader := c.Request.Header.Get("Authorization")
		if authHeader != "" && !strings.HasPrefix(authHeader, "X-OPENCSG-Sync-Token") {
			var username, token string
			if strings.HasPrefix(authHeader, "Basic ") {
				authHeader = strings.TrimPrefix(authHeader, "Basic ")
				authInfo, err := base64.StdEncoding.DecodeString(authHeader)
				if err != nil {
					slog.Info("Failed to decode basic auth header", slog.Any("header", authHeader), slog.Any("error", err))
					c.Next()
					return
				}
				username = strings.Split(string(authInfo), ":")[0]
				token = strings.Split(string(authInfo), ":")[1]
				user, err := userStore.FindByGitAccessToken(c.Request.Context(), token)
				if err != nil {
					slog.Info("Failed to find user by git access token", slog.Any("header", authHeader), slog.Any("token", token), slog.Any("error", err))
					c.Next()
					return
				}
				if user.Username == username {
					httpbase.SetCurrentUser(c, username)
				}
			} else if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
				user, err := userStore.FindByGitAccessToken(c.Request.Context(), token)
				if err != nil {
					slog.Info("Failed to find user by git access token", slog.Any("header", authHeader), slog.Any("token", token), slog.Any("error", err))
					c.Next()
					return
				}
				httpbase.SetCurrentUser(c, user.Username)
			}
		}

		c.Next()
	}
}
