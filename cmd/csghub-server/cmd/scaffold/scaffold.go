package scaffold

import (
	"fmt"
	"html/template"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// snakeToCamel converts snake_case to CamelCase
func snakeToCamel(s string) string {
	parts := strings.Split(s, "_")
	titleCase := cases.Title(language.English)
	for i, part := range parts {
		parts[i] = titleCase.String(part)
	}
	return strings.Join(parts, "")
}

// Template data structure for generating files
type TemplateData struct {
	Singular      string
	Plural        string
	LowerSingular string
	LowerPlural   string
	CamelSingular string
}

// Template strings
const (
	migrationTemplate = `package migrations

import (
	"context"

	"github.com/uptrace/bun"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS {{.LowerPlural}} (id SERIAL PRIMARY KEY, name VARCHAR(255) NOT NULL, description TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP, updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)", "{{.LowerPlural}}")
		return err
	}, func(ctx context.Context, db *bun.DB) error {
		_, err := db.ExecContext(ctx, "DROP TABLE IF EXISTS {{.LowerPlural}}", "{{.LowerPlural}}")
		return err
	})
}
`

	databaseTemplate = `package database

import (
	"context"

	"opencsg.com/csghub-server/common/errorx"
)

type {{.Singular}} struct {
	ID          int64     ` + "`" + `json:"id"` + "`" + `
	Name        string    ` + "`" + `json:"name"` + "`" + `
	Description string    ` + "`" + `json:"description"` + "`" + `
	times
}

type {{.Singular}}Store interface {
	Create(ctx context.Context, input {{.Singular}}) (*{{.Singular}}, error)
	Update(ctx context.Context, input {{.Singular}}) (*{{.Singular}}, error)
	Delete(ctx context.Context, id int64) error
	FindById(ctx context.Context, id int64) (*{{.Singular}}, error)
	FindAll(ctx context.Context, limit, offset int) ([]*{{.Singular}}, error)
}

type {{.Singular}}StoreImpl struct {
	db *DB
}

func New{{.Singular}}Store(db *DB) {{.Singular}}Store {
	return &{{.Singular}}StoreImpl{
		db: db,
	}
}

// for testing with mock db
func New{{.Singular}}StoreWithDB(db *DB) {{.Singular}}Store {
	return &{{.Singular}}StoreImpl{
		db: db,
	}
}

func (s *{{.Singular}}StoreImpl) Create(ctx context.Context, input {{.Singular}}) (*{{.Singular}}, error) {
	_, err := s.db.Core.NewInsert().Model(&input).Exec(ctx, &input)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &input, nil
}

func (s *{{.Singular}}StoreImpl) Update(ctx context.Context, input {{.Singular}}) (*{{.Singular}}, error) {
	_, err := s.db.Core.NewUpdate().Model(&input).WherePK().Exec(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &input, nil
}

func (s *{{.Singular}}StoreImpl) Delete(ctx context.Context, id int64) error {
	_, err := s.db.Core.NewDelete().Model((*{{.Singular}})(nil)).Where("id = ?", id).Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (s *{{.Singular}}StoreImpl) FindById(ctx context.Context, id int64) (*{{.Singular}}, error) {
	var {{.CamelSingular}} {{.Singular}}
	err := s.db.Core.NewSelect().Model(&{{.CamelSingular}}).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return &{{.CamelSingular}}, nil
}

func (s *{{.Singular}}StoreImpl) FindAll(ctx context.Context, limit, offset int) ([]*{{.Singular}}, error) {
	var {{.LowerPlural}} []*{{.Singular}}
	err := s.db.Core.NewSelect().Model(&{{.LowerPlural}}).Limit(limit).Offset(offset).Scan(ctx)
	if err != nil {
		return nil, errorx.HandleDBError(err, nil)
	}

	return {{.LowerPlural}}, nil
}
`

	componentTemplate = `package component

import (
	"context"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

type {{.Singular}}Component interface {
	Create{{.Singular}}(ctx context.Context, req *types.Create{{.Singular}}Req) (*types.{{.Singular}}, error)
	Update{{.Singular}}(ctx context.Context, req *types.Update{{.Singular}}Req) (*types.{{.Singular}}, error)
	Delete{{.Singular}}(ctx context.Context, id int64) error
	Get{{.Singular}}(ctx context.Context, id int64) (*types.{{.Singular}}, error)
	List{{.Singular}}(ctx context.Context, req *types.List{{.Singular}}Req) ([]*types.{{.Singular}}, int, error)
}

type {{.Singular}}ComponentImpl struct {
	{{.CamelSingular}}Store database.{{.Singular}}Store
}

func New{{.Singular}}Component({{.CamelSingular}}Store database.{{.Singular}}Store) {{.Singular}}Component {
	return &{{.Singular}}ComponentImpl{
		{{.CamelSingular}}Store: {{.CamelSingular}}Store,
	}
}

func (c *{{.Singular}}ComponentImpl) Create{{.Singular}}(ctx context.Context, req *types.Create{{.Singular}}Req) (*types.{{.Singular}}, error) {
	{{.CamelSingular}} := database.{{.Singular}}{
		Name:        req.Name,
		Description: req.Description,
	}

	created{{.Singular}}, err := c.{{.CamelSingular}}Store.Create(ctx, {{.CamelSingular}})
	if err != nil {
		return nil, err
	}

	return &types.{{.Singular}}{
		ID:          created{{.Singular}}.ID,
		Name:        created{{.Singular}}.Name,
		Description: created{{.Singular}}.Description,
		CreatedAt:   created{{.Singular}}.CreatedAt,
		UpdatedAt:   created{{.Singular}}.UpdatedAt,
	}, nil
}

func (c *{{.Singular}}ComponentImpl) Update{{.Singular}}(ctx context.Context, req *types.Update{{.Singular}}Req) (*types.{{.Singular}}, error) {
	{{.CamelSingular}} := database.{{.Singular}}{
		ID:          req.ID,
		Name:        req.Name,
		Description: req.Description,
	}

	updated{{.Singular}}, err := c.{{.CamelSingular}}Store.Update(ctx, {{.CamelSingular}})
	if err != nil {
		return nil, err
	}

	return &types.{{.Singular}}{
		ID:          updated{{.Singular}}.ID,
		Name:        updated{{.Singular}}.Name,
		Description: updated{{.Singular}}.Description,
		CreatedAt:   updated{{.Singular}}.CreatedAt,
		UpdatedAt:   updated{{.Singular}}.UpdatedAt,
	}, nil
}

func (c *{{.Singular}}ComponentImpl) Delete{{.Singular}}(ctx context.Context, id int64) error {
	return c.{{.CamelSingular}}Store.Delete(ctx, id)
}

func (c *{{.Singular}}ComponentImpl) Get{{.Singular}}(ctx context.Context, id int64) (*types.{{.Singular}}, error) {
	item, err := c.{{.CamelSingular}}Store.FindById(ctx, id)
	if err != nil {
		return nil, err
	}

	return &types.{{.Singular}}{
		ID:          item.ID,
		Name:        item.Name,
		Description: item.Description,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
	}, nil
}

func (c *{{.Singular}}ComponentImpl) List{{.Singular}}(ctx context.Context, req *types.List{{.Singular}}Req) ([]*types.{{.Singular}}, int, error) {
	items, err := c.{{.CamelSingular}}Store.FindAll(ctx, req.Limit, req.Offset)
	if err != nil {
		return nil, 0, err
	}

	result := make([]*types.{{.Singular}}, len(items))
	for i, item := range items {
		result[i] = &types.{{.Singular}}{
			ID:          item.ID,
			Name:        item.Name,
			Description: item.Description,
			CreatedAt:   item.CreatedAt,
			UpdatedAt:   item.UpdatedAt,
		}
	}

	// For simplicity, return len(items) as count
	// In a real scenario, you would query the database for the total count
	return result, len(items), nil
}
`

	handlerTemplate = `package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/common/types"
)

type {{.Singular}}Handler struct {
	{{.CamelSingular}}Component component.{{.Singular}}Component
}

func New{{.Singular}}Handler({{.CamelSingular}}Component component.{{.Singular}}Component) *{{.Singular}}Handler {
	return &{{.Singular}}Handler{ {{.CamelSingular}}Component: {{.CamelSingular}}Component }
}

func (h *{{.Singular}}Handler) Create{{.Singular}}(c *gin.Context) {
	var req types.Create{{.Singular}}Req
	if err := c.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	{{.CamelSingular}}, err := h.{{.CamelSingular}}Component.Create{{.Singular}}(c, &req)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, {{.CamelSingular}})
}

func (h *{{.Singular}}Handler) Update{{.Singular}}(c *gin.Context) {
	var req types.Update{{.Singular}}Req
	if err := c.ShouldBindJSON(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	{{.CamelSingular}}, err := h.{{.CamelSingular}}Component.Update{{.Singular}}(c, &req)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, {{.CamelSingular}})
}

func (h *{{.Singular}}Handler) Delete{{.Singular}}(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(c, "Invalid ID format")
		return
	}

	err = h.{{.CamelSingular}}Component.Delete{{.Singular}}(c, id)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, nil)
}

func (h *{{.Singular}}Handler) Get{{.Singular}}(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		httpbase.BadRequest(c, "Invalid ID format")
		return
	}

	{{.CamelSingular}}, err := h.{{.CamelSingular}}Component.Get{{.Singular}}(c, id)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, {{.CamelSingular}})
}

func (h *{{.Singular}}Handler) List{{.Singular}}(c *gin.Context) {
	var req types.List{{.Singular}}Req
	if err := c.ShouldBindQuery(&req); err != nil {
		httpbase.BadRequest(c, err.Error())
		return
	}

	items, count, err := h.{{.CamelSingular}}Component.List{{.Singular}}(c, &req)
	if err != nil {
		httpbase.ServerError(c, err)
		return
	}

	httpbase.OK(c, gin.H{
		"items": items,
		"count": count,
	})
}
`

	routeTemplate = `package router

import (
	"github.com/gin-gonic/gin"
	"opencsg.com/csghub-server/api/handler"
	"opencsg.com/csghub-server/component"
)

func Register{{.Plural}}Routes(router *gin.RouterGroup, {{.CamelSingular}}Component component.{{.Singular}}Component) {
	{{.Singular}}Handler := handler.New{{.Singular}}Handler({{.CamelSingular}}Component)

	{{.Singular}}Group := router.Group("/{{.LowerPlural}}")
	{
		{{.Singular}}Group.POST("", {{.Singular}}Handler.Create{{.Singular}})
		{{.Singular}}Group.PUT("/:id", {{.Singular}}Handler.Update{{.Singular}})
		{{.Singular}}Group.DELETE("/:id", {{.Singular}}Handler.Delete{{.Singular}})
		{{.Singular}}Group.GET("/:id", {{.Singular}}Handler.Get{{.Singular}})
		{{.Singular}}Group.GET("", {{.Singular}}Handler.List{{.Singular}})
	}
}
`

	typesTemplate = `package types

import (
	"time"
)

type {{.Singular}} struct {
	ID          int64     ` + "`" + `json:"id"` + "`" + `
	Name        string    ` + "`" + `json:"name"` + "`" + `
	Description string    ` + "`" + `json:"description"` + "`" + `
	CreatedAt   time.Time ` + "`" + `json:"created_at"` + "`" + `
	UpdatedAt   time.Time ` + "`" + `json:"updated_at"` + "`" + `
}

type Create{{.Singular}}Req struct {
	Name        string ` + "`" + `json:"name" binding:"required"` + "`" + `
	Description string ` + "`" + `json:"description"` + "`" + `
}

type Update{{.Singular}}Req struct {
	ID          int64  ` + "`" + `json:"id" binding:"required"` + "`" + `
	Name        string ` + "`" + `json:"name" binding:"required"` + "`" + `
	Description string ` + "`" + `json:"description"` + "`" + `
}

type List{{.Singular}}Req struct {
	Limit  int ` + "`" + `form:"limit,default=10"` + "`" + `
	Offset int ` + "`" + `form:"offset,default=0"` + "`" + `
}
`
)

var Cmd = &cobra.Command{
	Use:   "scaffold",
	Short: "generate CRUD files for a new entity",
	Long:  "scaffold generates route, component, handler, database, and migration files for a new entity",
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			slog.Error("entity name is required")
			_ = cmd.Help()
			return
		}

		entityName := args[0]
		generateFiles(entityName)
	},
}

func generateFiles(entityName string) {
	// Convert entity name to different cases
	singular := snakeToCamel(entityName)
	plural := singular + "s"
	lowerSingular := strings.ToLower(entityName)
	lowerPlural := lowerSingular + "s"
	// Generate camelCase for variables (e.g., repo_statistic -> repoStatistic)
	camelSingular := strings.ToLower(singular[:1]) + singular[1:]

	// Create template data
	data := TemplateData{
		Singular:      singular,
		Plural:        plural,
		LowerSingular: lowerSingular,
		LowerPlural:   lowerPlural,
		CamelSingular: camelSingular,
	}

	slog.Info(fmt.Sprintf("Generating CRUD files for entity: %s", singular))

	// Generate types file
	generateTypes(data)

	// Generate migration file
	generateMigration(data)

	// Generate database file
	generateDatabase(data)

	// Generate component file
	generateComponent(data)

	// Generate handler file
	generateHandler(data)

	// Generate route file
	generateRoute(data)

	slog.Info("CRUD files generated successfully")
}

// generateFile generates a file from a template
func generateFile(filePath, templateStr string, data TemplateData) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create template
	tmpl, err := template.New("").Parse(templateStr)
	if err != nil {
		return fmt.Errorf("failed to parse template: %w", err)
	}

	// Create file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Execute template
	if err := tmpl.Execute(file, data); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	return nil
}

func generateMigration(data TemplateData) {
	// Generate migration file name
	timestamp := time.Now().Format("20060102150405")
	fileName := fmt.Sprintf("%s_create_%s.go", timestamp, data.LowerPlural)
	filePath := filepath.Join("builder/store/database/migrations", fileName)

	// Generate migration file
	if err := generateFile(filePath, migrationTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create migration file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created migration file: %s", filePath))
}

func generateDatabase(data TemplateData) {
	// Generate database file path
	filePath := filepath.Join("builder/store/database", fmt.Sprintf("%s.go", data.LowerSingular))

	// Generate database file
	if err := generateFile(filePath, databaseTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create database file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created database file: %s", filePath))
}

func generateComponent(data TemplateData) {
	// Generate component file path
	filePath := filepath.Join("component", fmt.Sprintf("%s.go", data.LowerSingular))

	// Generate component file
	if err := generateFile(filePath, componentTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create component file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created component file: %s", filePath))
}

func generateHandler(data TemplateData) {
	// Generate handler file path
	filePath := filepath.Join("api/handler", fmt.Sprintf("%s.go", data.LowerSingular))

	// Generate handler file
	if err := generateFile(filePath, handlerTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create handler file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created handler file: %s", filePath))
}

func generateRoute(data TemplateData) {
	// Generate route file path
	filePath := filepath.Join("api/router", fmt.Sprintf("%s.go", data.LowerSingular))

	// Generate route file
	if err := generateFile(filePath, routeTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create route file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created route file: %s", filePath))
}

func generateTypes(data TemplateData) {
	// Generate types file path
	filePath := filepath.Join("common/types", fmt.Sprintf("%s.go", data.LowerSingular))

	// Generate types file
	if err := generateFile(filePath, typesTemplate, data); err != nil {
		slog.Error(fmt.Sprintf("Failed to create types file: %v", err))
		return
	}

	slog.Info(fmt.Sprintf("Created types file: %s", filePath))
}
