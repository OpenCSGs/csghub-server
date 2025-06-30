package i18n

import (
	"log/slog"
	"net/http"
	"reflect"
	text_tmpl "text/template"

	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/nicksnyder/go-i18n/v2/i18n/template"
)

func TranslateText(lang, messageID, defaultMessage string) (string, bool) {
	localizer, ok := LocalizerMap[lang]
	if !ok {
		slog.Error("Language not supported", slog.String("lang", lang))
		return defaultMessage, false
	}
	message, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
	})
	if err != nil {
		return defaultMessage, false
	}
	return message, true
}

// TranslateTextWithData for translate with template
func TranslateTextWithData(lang string, messageID string, templateData map[string]interface{}) string {
	localizer, ok := LocalizerMap[lang]
	if !ok {
		return messageID
	}
	tmplParser := &template.TextParser{
		Funcs: text_tmpl.FuncMap{
			// default template
			"default": func(defaultValue, value interface{}) interface{} {
				if value == nil {
					return defaultValue
				}
				if s, ok := value.(string); ok && s == "" {
					return defaultValue
				}
				return value
			},
		},
	}

	msg, err := localizer.Localize(&i18n.LocalizeConfig{
		MessageID:      messageID,
		TemplateData:   templateData,
		TemplateParser: tmplParser,
	})

	if err != nil {
		return messageID
	}

	return msg
}

type I18nOptionsKey string
type I18nOptionsValue string

const (
	I18nMethod  I18nOptionsKey = "method"
	I18nHandler I18nOptionsKey = "handler"
	I18nError   I18nOptionsKey = "error"
)

var StatusCodeMessageMap = map[int]string{
	http.StatusBadRequest:          "BadRequest",
	http.StatusUnauthorized:        "Unauthorized",
	http.StatusForbidden:           "Forbidden",
	http.StatusNotFound:            "NotFound",
	http.StatusMethodNotAllowed:    "MethodNotAllowed",
	http.StatusInternalServerError: "InternalServerError",
	http.StatusServiceUnavailable:  "ServiceUnavailable",
}

// according to type of v, recuresively translate the value
func TranslateInterface(v interface{}, lang string) interface{} {
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Struct:
		// if changed, return a new struct
		modified := false
		newStruct := reflect.New(val.Type()).Elem()
		newStruct.Set(val)

		t := val.Type()
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			fieldValue := val.Field(i)

			if field.PkgPath == "" {
				if field.Type.Kind() == reflect.String {
					jsonTag := field.Tag.Get("json")
					i18nTag := field.Tag.Get("i18n")
					if jsonTag != "" && i18nTag != "" && fieldValue.String() != "" {
						oldValue := fieldValue.String()
						// Translate the string field using the i18n tag
						// If the i18n tag is not set, use the field name as the default value
						newValue, _ := TranslateText(lang, i18nTag+"."+oldValue, oldValue)

						newStruct.Field(i).SetString(newValue)
						// val.Elem().Field(i).SetString(newValue)
						modified = true
					}
				} else if fieldValue.CanInterface() {
					newFieldValue := TranslateInterface(fieldValue.Interface(), lang)
					if !reflect.DeepEqual(newFieldValue, fieldValue.Interface()) && newStruct.Field(i).CanSet() {
						newStruct.Field(i).Set(reflect.ValueOf(newFieldValue))
						// val.Field(i).Set(reflect.ValueOf(newFieldValue))
						modified = true
					}
				}
			}
		}
		if modified {
			return newStruct.Interface()
		}
		return v

	case reflect.Slice:
		modified := false
		newSlice := reflect.MakeSlice(val.Type(), val.Len(), val.Cap())

		reflect.Copy(newSlice, val)

		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.CanInterface() {
				newElem := TranslateInterface(elem.Interface(), lang)

				if !reflect.DeepEqual(newElem, elem.Interface()) && newSlice.Index(i).CanSet() {
					newSlice.Index(i).Set(reflect.ValueOf(newElem))
					modified = true
				}
			}
		}

		if modified {
			return newSlice.Interface()
		}
		return v
	case reflect.Array:
		modified := false
		newArray := reflect.New(val.Type()).Elem()
		reflect.Copy(newArray, val)
		for i := 0; i < val.Len(); i++ {
			elem := val.Index(i)
			if elem.CanInterface() {
				newElem := TranslateInterface(elem.Interface(), lang)
				if !reflect.DeepEqual(newElem, elem.Interface()) && newArray.Index(i).CanSet() {
					newArray.Index(i).Set(reflect.ValueOf(newElem))
					modified = true
				}
			}
		}
		if modified {
			return newArray.Interface()
		}
		return v
	case reflect.Map:
		modified := false
		newMap := reflect.MakeMap(val.Type())

		for _, key := range val.MapKeys() {
			mapValue := val.MapIndex(key)
			newMap.SetMapIndex(key, mapValue)
		}

		for _, key := range val.MapKeys() {
			mapValue := val.MapIndex(key)
			if mapValue.CanInterface() {
				newMapValue := TranslateInterface(mapValue.Interface(), lang)

				if !reflect.DeepEqual(newMapValue, mapValue.Interface()) {
					newMap.SetMapIndex(key, reflect.ValueOf(newMapValue))
					modified = true
				}
			}
		}

		if modified {
			return newMap.Interface()
		}
		return v
	case reflect.Pointer:
		if val.IsNil() {
			return v
		}
		newElem := TranslateInterface(val.Elem().Interface(), lang)

		if reflect.ValueOf(newElem).Type() != val.Elem().Type() {
			return v
		}

		if reflect.DeepEqual(newElem, val.Elem().Interface()) {
			return v
		}

		newPtr := reflect.New(reflect.TypeOf(newElem))
		newPtr.Elem().Set(reflect.ValueOf(newElem))
		return newPtr.Interface()
	default:
		return v
	}
}
