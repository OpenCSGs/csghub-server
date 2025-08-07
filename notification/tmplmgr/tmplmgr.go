package tmplmgr

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"reflect"
	"strings"
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

func (t *TemplateManager) Format(scenario types.MessageScenario, channel types.MessageChannel, data any, lang string) (*types.TemplateOutput, error) {
	tmplPath := fmt.Sprintf("%s/%s.%s.tpl", string(scenario), string(channel), string(lang))
	// check cache
	cachedTmpl, found := t.cache.Load(tmplPath)
	if found {
		if cached, ok := cachedTmpl.(cachedTemplate); ok {
			output, err := t.executeTemplate(cached.template, data, cached.isDefaultTemplate)
			if err != nil {
				return nil, err
			}
			return output, nil
		}
	}

	// if not in cache, load from embedded templates, then store in cache
	var tmpl *template.Template
	var isDefaultTemplate bool
	tmpls, err := fs.Sub(Templates, "templates")
	if err != nil {
		return nil, fmt.Errorf("failed to load templates: %v", err)
	}

	// check whether the template exists, if not, use the default template
	if _, err := tmpls.Open(tmplPath); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			slog.Info("template file not found, using default template", "path", tmplPath)
			defaultTmpl, err := t.loadDefaultTemplate(channel, lang)
			if err != nil {
				return nil, err
			}
			tmpl = defaultTmpl
			isDefaultTemplate = true
		} else {
			return nil, fmt.Errorf("failed to open template file %s: %w", tmplPath, err)
		}
	} else {
		tmpl, err = template.ParseFS(tmpls, tmplPath)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template %s: %w", tmplPath, err)
		}
		isDefaultTemplate = false
	}

	t.cache.Store(tmplPath, cachedTemplate{
		template:          tmpl,
		isDefaultTemplate: isDefaultTemplate,
	})

	output, err := t.executeTemplate(tmpl, data, isDefaultTemplate)
	if err != nil {
		return nil, err
	}
	return output, nil
}

func (t *TemplateManager) executeTemplate(tmpl *template.Template, data any, isDefaultTemplate bool) (*types.TemplateOutput, error) {
	var buf bytes.Buffer

	var templateData any
	if isDefaultTemplate {
		templateData = t.normalizeTemplateData(data)
	} else {
		templateData = data
	}

	if err := tmpl.Execute(&buf, templateData); err != nil {
		return nil, fmt.Errorf("failed to execute template: %w", err)
	}
	return t.parseTemplateOutput(buf.String()), nil
}

func (t *TemplateManager) parseTemplateOutput(outputStr string) *types.TemplateOutput {
	parts := strings.SplitN(outputStr, "---", 2)
	if len(parts) != 2 {
		return &types.TemplateOutput{Content: outputStr}
	}
	title := strings.TrimSpace(parts[0])
	content := strings.TrimSpace(parts[1])
	return &types.TemplateOutput{Title: title, Content: content}
}

func (t *TemplateManager) normalizeTemplateData(data any) map[string]any {
	result := make(map[string]any)

	dataValue := reflect.ValueOf(data)
	if dataValue.Kind() == reflect.Map {
		if mapData, ok := data.(map[string]any); ok {
			return mapData
		}
	} else if dataValue.Kind() == reflect.Struct {
		dataType := dataValue.Type()
		for i := 0; i < dataValue.NumField(); i++ {
			field := dataValue.Field(i)
			fieldName := dataType.Field(i).Name
			result[fieldName] = field.Interface()
		}
	} else {
		result["Content"] = data
	}

	return result
}

func (t *TemplateManager) loadDefaultTemplate(channel types.MessageChannel, lang string) (*template.Template, error) {
	defaultTmplPath := fmt.Sprintf("_default/%s.%s.tpl", string(channel), string(lang))
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
