package types

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// SuperAdminRoleCode identifies the built-in global administrator role available in every edition.
const SuperAdminRoleCode = "super_admin"

// AdminRoleSummary describes a role defined in the administrator role catalog.
type AdminRoleSummary struct {
	RoleCode    string `json:"role_code"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// AdminRoleDetail includes role metadata and its stored permission codes, including wildcards.
type AdminRoleDetail struct {
	AdminRoleSummary
	// IsAdmin identifies administrator roles in the shared role catalog.
	IsAdmin bool `json:"is_admin"`
	// IsBuiltin identifies a role supplied by the system.
	IsBuiltin bool `json:"is_builtin"`
	// CanDelete controls deletion of the role definition, not user assignments.
	CanDelete   bool              `json:"can_delete"`
	CreatedAt   time.Time         `json:"created_at"`
	UserID      int64             `json:"user_id"`
	Permissions []AdminPermission `json:"permissions"`
}

// AdminPermissionCheckRequest specifies route requirements; multiple permissions use OR semantics.
// The operator is taken exclusively from authenticated context.
type AdminPermissionCheckRequest struct {
	Permissions []AdminPermission `json:"permissions"`
}

// AdminPermissionCheckResult confirms successful authorization without exposing role details.
type AdminPermissionCheckResult struct {
	Allowed bool `json:"allowed"`
}

// AdminUser contains public identity fields and edition-supported administrator role summaries.
type AdminUser struct {
	UserID   int64              `json:"user_id"`
	UUID     string             `json:"uuid"`
	Username string             `json:"username"`
	Nickname string             `json:"nickname"`
	Avatar   string             `json:"avatar"`
	Roles    []AdminRoleSummary `json:"roles"`
}

// AdminUserListRequest filters a page of administrators by an optional role code.
// The server can restrict selection and role summaries to stored global administrator grants.
type AdminUserListRequest struct {
	RoleCode string
	Page     int
	Per      int
	// GlobalOnly restricts selection and role summaries to administrator roles carrying *.
	// It is supplied by the server and cannot be bound from client JSON.
	GlobalOnly bool `json:"-"`
}

// AdminUserRoleRequest identifies a role assignment and its authenticated operator.
// CurrentUser is supplied by the server, never bound from client input.
type AdminUserRoleRequest struct {
	UserID      int64
	RoleCode    string
	CurrentUser string
	// GlobalOnly requires global grants for role mutations and the remaining administrator check.
	// It is supplied by the server and cannot be bound from client JSON.
	GlobalOnly bool `json:"-"`
}

// AdminModuleCode identifies a top-level administration module.
type AdminModuleCode string

// AdminPermission identifies a permission required by an administration route.
// Module and submodule codes are independent of the route's URL path.
type AdminPermission string

const (
	// AdminModuleSystem groups system administration permissions.
	AdminModuleSystem AdminModuleCode = "system"
	// AdminModuleAccount groups account administration permissions.
	AdminModuleAccount AdminModuleCode = "account"
	// AdminModuleCompute groups compute administration permissions.
	AdminModuleCompute AdminModuleCode = "compute"
	// AdminModuleAPIGateway groups AI gateway administration permissions.
	AdminModuleAPIGateway AdminModuleCode = "apigateway"
	// AdminModuleAsset groups asset administration permissions.
	AdminModuleAsset AdminModuleCode = "asset"
	// AdminModuleMarketing groups marketing administration permissions.
	AdminModuleMarketing AdminModuleCode = "marketing"
	// AdminModuleAccounting groups accounting administration permissions.
	AdminModuleAccounting AdminModuleCode = "accounting"
	// AdminModuleLicense groups license administration permissions.
	AdminModuleLicense AdminModuleCode = "license"
	// AdminModuleAcademy groups academy administration permissions.
	AdminModuleAcademy AdminModuleCode = "academy"
)

const (
	// AdminPermissionAny requires active administrator membership in the shared store and EE/SaaS middleware.
	// CE PlatformAdmin upgrades this route-only requirement to AdminPermissionAll.
	// Authorization with Any preserves the route's original business permission checks in every edition.
	AdminPermissionAny AdminPermission = "any"
	// AdminPermissionAll requires the stored global wildcard grant, regardless of the role's name.
	// Concrete permissions and module wildcards do not satisfy this requirement.
	AdminPermissionAll AdminPermission = "*"

	// AdminSystemDashboard grants access to the system dashboard.
	AdminSystemDashboard AdminPermission = "system/dashboard"
	// AdminSystemConfig grants access to system configuration.
	AdminSystemConfig AdminPermission = "system/system_config"
	// AdminSystemBroadcast grants access to broadcast notifications.
	AdminSystemBroadcast AdminPermission = "system/broadcast"
	// AdminSystemErrorLog grants access to error logs.
	AdminSystemErrorLog AdminPermission = "system/error_log"

	// AdminAccountUser grants access to user administration.
	AdminAccountUser AdminPermission = "account/user"
	// AdminAccountAdmin grants access to administrator management.
	AdminAccountAdmin AdminPermission = "account/admin"
	// AdminAccountOrganization grants access to organization administration.
	AdminAccountOrganization AdminPermission = "account/organization"
	// AdminAccountFederation grants access to federation administration.
	AdminAccountFederation AdminPermission = "account/federation"

	// AdminComputeOverview grants access to the compute overview.
	AdminComputeOverview AdminPermission = "compute/overview"
	// AdminComputeSpec grants access to compute specifications.
	AdminComputeSpec AdminPermission = "compute/spec"
	// AdminComputeScenario grants access to compute scenarios.
	AdminComputeScenario AdminPermission = "compute/scenario"
	// AdminComputeCluster grants access to compute clusters.
	AdminComputeCluster AdminPermission = "compute/cluster"
	// AdminComputeNode grants access to compute nodes.
	AdminComputeNode AdminPermission = "compute/node"
	// AdminComputeInstance grants access to compute instances.
	AdminComputeInstance AdminPermission = "compute/instance"

	// AdminAPIGatewayGateway grants access to AI gateway administration.
	AdminAPIGatewayGateway AdminPermission = "apigateway/api_gateway"
	// AdminAPIGatewayPublicInference grants access to public inference administration.
	AdminAPIGatewayPublicInference AdminPermission = "apigateway/public_inference"
	// AdminAPIGatewayCommercialAPI grants access to commercial API administration.
	AdminAPIGatewayCommercialAPI AdminPermission = "apigateway/commercial_api"
	// AdminAPIGatewayTokenPlan grants access to token plan administration.
	AdminAPIGatewayTokenPlan AdminPermission = "apigateway/token_plan"
	// AdminAPIGatewayMCPGateway grants access to MCP gateway administration.
	AdminAPIGatewayMCPGateway AdminPermission = "apigateway/mcp_gateway"

	// AdminAssetTag grants access to tag administration.
	AdminAssetTag AdminPermission = "asset/tag"
	// AdminAssetModel grants access to model administration.
	AdminAssetModel AdminPermission = "asset/model"
	// AdminAssetDataset grants access to dataset administration.
	AdminAssetDataset AdminPermission = "asset/dataset"
	// AdminAssetCode grants access to code repository administration.
	AdminAssetCode AdminPermission = "asset/code"
	// AdminAssetSpace grants access to application space administration.
	AdminAssetSpace AdminPermission = "asset/space"
	// AdminAssetMCP grants access to MCP administration.
	AdminAssetMCP AdminPermission = "asset/mcp"
	// AdminAssetSkill grants access to skill administration.
	AdminAssetSkill AdminPermission = "asset/skill"
	// AdminAssetMirror grants access to mirror administration.
	AdminAssetMirror AdminPermission = "asset/mirror"
	// AdminAssetSync grants access to multi-source synchronization administration.
	AdminAssetSync AdminPermission = "asset/sync"
	// AdminAssetRuntimeFramework grants access to runtime framework administration.
	AdminAssetRuntimeFramework AdminPermission = "asset/runtime_framework"
	// AdminAssetSensitiveWord grants access to sensitive word administration.
	AdminAssetSensitiveWord AdminPermission = "asset/sensitive_word"
	// AdminAssetResourcePrice grants access to resource price administration.
	AdminAssetResourcePrice AdminPermission = "asset/resource_price"

	// AdminMarketingPhoto grants access to photo administration.
	AdminMarketingPhoto AdminPermission = "marketing/photo"
	// AdminMarketingDailyPaper grants access to paper administration.
	AdminMarketingDailyPaper AdminPermission = "marketing/daily_paper"
	// AdminMarketingCampaign grants access to campaign administration.
	AdminMarketingCampaign AdminPermission = "marketing/campaign"
	// AdminMarketingArticle grants access to article administration.
	AdminMarketingArticle AdminPermission = "marketing/article"
	// AdminMarketingLead grants access to lead administration.
	AdminMarketingLead AdminPermission = "marketing/lead"
	// AdminMarketingLeadForm grants access to lead form administration.
	AdminMarketingLeadForm AdminPermission = "marketing/lead_form"
	// AdminMarketingEmailGroup grants access to email group administration.
	AdminMarketingEmailGroup AdminPermission = "marketing/email_group"
	// AdminMarketingEmailSending grants access to email sending administration.
	AdminMarketingEmailSending AdminPermission = "marketing/email_sending"
	// AdminMarketingEAIArticle grants access to embodied AI article administration.
	AdminMarketingEAIArticle AdminPermission = "marketing/eai_article"
	// AdminMarketingEAIDevice grants access to embodied AI device administration.
	AdminMarketingEAIDevice AdminPermission = "marketing/eai_device"

	// AdminAccountingBusinessReport grants access to business reports.
	AdminAccountingBusinessReport AdminPermission = "accounting/business_report"
	// AdminAccountingConsumptionReport grants access to consumption reports.
	AdminAccountingConsumptionReport AdminPermission = "accounting/consumption_report"
	// AdminAccountingAPIOperationsReport grants access to API operations reports.
	AdminAccountingAPIOperationsReport AdminPermission = "accounting/api_operations_report"
	// AdminAccountingRechargeReport grants access to recharge reports.
	AdminAccountingRechargeReport AdminPermission = "accounting/recharge_report"
	// AdminAccountingComputeVoucher grants access to compute vouchers.
	AdminAccountingComputeVoucher AdminPermission = "accounting/compute_voucher"
	// AdminAccountingInvoice grants access to invoices.
	AdminAccountingInvoice AdminPermission = "accounting/invoice"

	// AdminLicenseManagement grants access to license issuance administration.
	AdminLicenseManagement AdminPermission = "license/management"

	// AdminAcademyCourse grants access to academy course administration.
	AdminAcademyCourse AdminPermission = "academy/course"
	// AdminAcademyCollection grants access to academy collection administration.
	AdminAcademyCollection AdminPermission = "academy/collection"
	// AdminAcademyUserPermission grants access to academy user permissions.
	AdminAcademyUserPermission AdminPermission = "academy/user_permission"
)

// ValidateAdminPermissions validates every route requirement before authorization or edition mapping.
// Any and All are special requirements; all other values must be concrete module/submodule paths.
// An empty list is syntactically valid and must be denied separately by authorization callers.
func ValidateAdminPermissions(required ...AdminPermission) error {
	for _, permission := range required {
		if permission == AdminPermissionAny || permission == AdminPermissionAll {
			continue
		}
		parts := strings.Split(string(permission), "/")
		if len(parts) < 2 {
			return fmt.Errorf("invalid admin permission %q", permission)
		}
		for _, part := range parts {
			if part == "" || part != strings.TrimSpace(part) || strings.Contains(part, "*") {
				return fmt.Errorf("invalid admin permission %q", permission)
			}
		}
	}
	return nil
}

type adminAuthorizationKey struct{}

// WithAdminAuthorization records that module or global authorization succeeded for this request.
// Only authorization middleware should call this function.
func WithAdminAuthorization(ctx context.Context) context.Context {
	return context.WithValue(ctx, adminAuthorizationKey{}, true)
}

// IsAdminAuthorized reports whether module or global authorization succeeded for this request.
// AdminPermissionAny membership checks do not set this business authorization marker.
func IsAdminAuthorized(ctx context.Context) bool {
	authorized, _ := ctx.Value(adminAuthorizationKey{}).(bool)
	return authorized
}
