package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/i18n"
)

func ModifyAcceptLanguageMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Check if the Accept-Language header is set
		acceptLanguage := c.GetHeader("Accept-Language")
		if acceptLanguage == "" {
			acceptLanguage = "en-US"
		}
		var lang language.Tag
		tags, _, err := language.ParseAcceptLanguage(acceptLanguage)
		if err != nil {
			slog.Error("parse Accept-Language header, LocalizedErrorMiddleware", slog.String("err", err.Error()))
			lang = language.AmericanEnglish // default to English if parsing fails
		} else {
			lang, _, _ = i18n.Matcher.Match(tags...)
			if lang == language.Und {
				lang = language.AmericanEnglish // default to English if no match found
			}
		}
		c.Request.Header.Set("Accept-Language", lang.String())
		c.Next()
	}
}

var skipRoutes = []string{
	"/healthz",
	"/csg",
	"/hf",
}

func shouldSkip(c *gin.Context) bool {
	// Check if the request is for a static file or a health check endpoint
	path := c.Request.URL.Path
	// Check for path prefixes
	for _, route := range skipRoutes {
		if strings.HasPrefix(path, route) {
			return true
		}
	}
	return false
}

// LocalizedErrorMiddleware international error message
func LocalizedErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bw := &bodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		if shouldSkip(c) {
			bw.isSkipped = true // Mark as skipped if the route is in the skip list
		}
		c.Writer = bw
		c.Next()

		statusCode := c.Writer.Status()
		if statusCode < 400 || bw.isSkipped {
			return
		}
		// translate error msg
		lang := c.GetHeader("Accept-Language")

		respBytes := bw.body.Bytes()
		var respObj httpbase.R
		err := json.Unmarshal(respBytes, &respObj)
		if err != nil {
			slog.Error("unmarshal original httpbase.R, LocalizedErrorMiddleware",
				slog.String("url", c.Request.URL.Path),
				slog.String("err", err.Error()),
				slog.String("resp", string(respBytes)),
			)
			_, err = bw.writeInternal(respBytes)
			if err != nil {
				slog.Error("unmarshal original resp failed, write original resp, LocalizedErrorMiddleware",
					slog.String("url", c.Request.URL.Path),
					slog.String("err", err.Error()),
				)
			}
			return
		}
		code := respObj.Code
		if errorx.IsValidErrorCode(code) {
			translatedMsg, ok := i18n.TranslateText(lang, "error."+code, code)
			if !ok {
				slog.Error("can not translate error code",
					slog.String("url", c.Request.URL.Path),
					slog.String("msg", respObj.Msg),
					slog.String("code", respObj.Code),
					slog.String("lang", lang),
				)
			} else {
				respObj.Msg = fmt.Sprintf("%s: %s", code, translatedMsg)
			}
		}

		updatedBody, err := json.Marshal(respObj)
		if err != nil {
			slog.Error("marshal updated httpbase.R",
				slog.String("url", c.Request.URL.Path),
				slog.String("err", err.Error()),
			)
			_, err = bw.writeInternal(respBytes)
			if err != nil {
				slog.Error("marshal updated httpbase.R failed, write origin resp, LocalizedErrorMiddleware",
					slog.String("url", c.Request.URL.Path),
					slog.String("err", err.Error()),
				)
			}
			return
		}
		bw.ResponseWriter.Header().Set("Content-Length", strconv.Itoa(len(updatedBody)))
		_, err = bw.writeInternal(updatedBody)
		if err != nil {
			slog.Error("write updated httpbase.R, LocalizedErrorMiddleware",
				slog.String("url", c.Request.URL.Path),
				slog.String("err", err.Error()),
			)
		}
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	body      *bytes.Buffer
	isSkipped bool // Flag to indicate if the route is skipped
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	if w.Status() == 200 || w.isSkipped {
		return w.ResponseWriter.Write(b) // If status is 200, write directly to the ResponseWriter
	} else {
		return w.body.Write(b) // If status is not 200, write to the buffer
	}
}

func (w *bodyWriter) writeInternal(b []byte) (int, error) {
	return w.ResponseWriter.Write(b)
}
