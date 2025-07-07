package tmplmgr

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"reflect"
	"sync"
	"text/template"

	"opencsg.com/csghub-server/common/types"
)

//go:embed templates templates/_default
var Templates embed.FS

type TemplateManager struct {
	cache sync.Map
}

type cachedTemplate struct {
	template          *template.Template
	isDefaultTemplate bool
}

func NewTemplateManager() *TemplateManager {
	return &TemplateManager{
		cache: sync.Map{},
	}
}

func (t *TemplateManager) Format(scenario types.MessageScenario, channel types.MessageChannel, data any) (string, error) {
	// {scenario}/{channel}.tpl
	tmplPath := fmt.Sprintf("%s/%s.tpl", string(scenario), string(channel))

	// check cache
	cachedTmpl, found := t.cache.Load(tmplPath)
	if found {
		if cached, ok := cachedTmpl.(cachedTemplate); ok {
			return t.executeTemplate(cached.template, data, cached.isDefaultTemplate)
		}
	}

	// if not in cache, load from embedded templates, then store in cache
	var tmpl *template.Template
	var isDefaultTemplate bool
	tmpls, err := fs.Sub(Templates, "templates")
	if err != nil {
		return "", fmt.Errorf("failed to load templates: %v", err)
	}

	// check whether the template exists, if not, use the default template
	if _, err := tmpls.Open(tmplPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Info("template file not found, using default template", "path", tmplPath)
			defaultTmpl, err := t.loadDefaultTemplate(channel)
			if err != nil {
				return "", err
			}
			tmpl = defaultTmpl
			isDefaultTemplate = true
		} else {
			return "", fmt.Errorf("failed to open template file %s: %w", tmplPath, err)
		}
	} else {
		tmpl, err = template.ParseFS(tmpls, tmplPath)
		if err != nil {
			return "", fmt.Errorf("failed to parse template %s: %w", tmplPath, err)
		}
		isDefaultTemplate = false
	}

	t.cache.Store(tmplPath, cachedTemplate{
		template:          tmpl,
		isDefaultTemplate: isDefaultTemplate,
	})

	return t.executeTemplate(tmpl, data, isDefaultTemplate)
}

func (t *TemplateManager) executeTemplate(tmpl *template.Template, data any, isDefaultTemplate bool) (string, error) {
	var buf bytes.Buffer

	var templateData any
	if isDefaultTemplate {
		templateData = t.convertStructToMap(data)
	} else {
		templateData = data
	}

	if err := tmpl.Execute(&buf, templateData); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}
	return buf.String(), nil
}

func (t *TemplateManager) convertStructToMap(data any) map[string]any {
	result := make(map[string]any)

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() == reflect.Struct {
		dataType := dataValue.Type()
		for i := 0; i < dataValue.NumField(); i++ {
			field := dataValue.Field(i)
			fieldName := dataType.Field(i).Name

			// Skip if the field value type is struct
			if field.Kind() == reflect.Struct {
				continue
			}

			fieldValue := field.Interface()
			result[fieldName] = fieldValue
		}
	} else if dataValue.Kind() == reflect.Map {
		// If data is already a map, return it directly
		if mapData, ok := data.(map[string]any); ok {
			return mapData
		}
		iter := dataValue.MapRange()
		for iter.Next() {
			key := iter.Key()
			value := iter.Value()
			if key.Kind() == reflect.String {
				result[key.String()] = value.Interface()
			}
		}
	} else {
		result["Content"] = data
	}

	return result
}

func (t *TemplateManager) loadDefaultTemplate(channel types.MessageChannel) (*template.Template, error) {
	defaultTmplPath := fmt.Sprintf("_default/%s.tpl", string(channel))
	tmpls, err := fs.Sub(Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %v", err)
	}

	if _, err := tmpls.Open(defaultTmplPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("default template file not found: %s", defaultTmplPath)
		}
		return nil, fmt.Errorf("failed to open default template file %s: %w", defaultTmplPath, err)
	}

	tmpl, err := template.ParseFS(tmpls, defaultTmplPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse default template %s: %w", defaultTmplPath, err)
	}
	return tmpl, nil
}
