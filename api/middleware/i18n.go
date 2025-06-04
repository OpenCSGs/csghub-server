package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/language"
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

// LocalizedErrorMiddleware international error message
func LocalizedErrorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		bw := &bodyWriter{body: bytes.NewBufferString(""), ResponseWriter: c.Writer}
		c.Writer = bw
		c.Next()

		statusCode := c.Writer.Status()
		if statusCode < 400 {
			_, err := bw.ResponseWriter.Write(bw.body.Bytes())
			if err != nil {
				slog.Error("statusCode < 400, LocalizedErrorMiddleware", slog.String("err", err.Error()))
			}
			return
		}

		messageID, exists := i18n.StatusCodeMessageMap[statusCode]
		if !exists {
			_, err := bw.ResponseWriter.Write(bw.body.Bytes())
			if err != nil {
				slog.Error("unhandled statusCode, LocalizedErrorMiddleware", slog.String("err", err.Error()))
			}
			return
		}
		// translate error msg
		lang := c.GetHeader("Accept-Language")

		templateData := make(map[string]interface{})
		if handlerName, ok := c.Get("Error-Handler-Name"); ok {
			if handlerNameStr, ok := handlerName.(string); ok {
				handlerNameStr = i18n.TranslateText(lang, string(i18n.I18nHandler)+"."+handlerNameStr, handlerNameStr)
				templateData[string(i18n.I18nHandler)] = handlerNameStr
			}
		}
		if c.Request != nil {
			methodStr := strings.ToLower(c.Request.Method)
			methodStr = i18n.TranslateText(lang, string(i18n.I18nMethod)+"."+methodStr, methodStr)
			templateData[string(i18n.I18nMethod)] = methodStr
		}

		message := i18n.TranslateTextWithData(lang, string(i18n.I18nError)+"."+messageID, templateData)

		updateObj := make(map[string]interface{})
		updateObj["msg"] = message
		updatedBody, err := json.Marshal(updateObj)
		if err != nil {
			slog.Error("marshal updated httpbase.R", slog.String("err", err.Error()))
		}
		_, err = bw.ResponseWriter.WriteString(string(updatedBody))
		if err != nil {
			slog.Error("write updated httpbase.R, LocalizedErrorMiddleware", slog.String("err", err.Error()))
		}
	}
}

type bodyWriter struct {
	gin.ResponseWriter
	body *bytes.Buffer
}

func (w *bodyWriter) Write(b []byte) (int, error) {
	return w.body.Write(b)
	// w.ResponseWriter.Write(b)
}
