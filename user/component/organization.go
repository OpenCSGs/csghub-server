package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rebac"
	rebacfactory "opencsg.com/csghub-server/builder/rebac/factory"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type OrganizationComponent interface {
	Create(ctx context.Context, req *types.CreateOrgReq) (*types.Organization, error)
	Index(ctx context.Context, search string, per, page int, orgType, verifyStatus, tag string) ([]types.Organization, int, error)
	ListUserOrgs(ctx context.Context, req *types.ListUserOrgsReq) ([]types.Organization, int, error)
	Get(ctx context.Context, orgName string) (*types.Organization, error)
	GetByUUID(ctx context.Context, uuid string) (*types.Organization, error)
	Delete(ctx context.Context, req *types.DeleteOrgReq) error
	Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error)
}

func NewOrganizationComponent(config *config.Config) (OrganizationComponent, error) {
	c := &organizationComponentImpl{config: config}
	authorizer, err := rebacfactory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("fail to create ReBAC authorizer: %w", err)
	}
	c.rebac = authorizer
	c.orgStore = database.NewOrgStore(config)
	c.memberStore = database.NewMemberStore()
	c.nsStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.tagStore = database.NewTagStore()
	c.gs, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.sso, err = rpc.NewSSOClient(config)
	if err != nil {
		newError := fmt.Errorf("fail to create sso client,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type organizationComponentImpl struct {
	orgStore    database.OrgStore
	memberStore database.MemberStore
	nsStore     database.NamespaceStore
	userStore   database.UserStore
	tagStore    database.TagStore
	gs          gitserver.GitServer
	// rebac synchronizes the organization's namespace relationship tuple.
	rebac rebac.Authorizer

	sso    rpc.SSOInterface
	config *config.Config
}

// deleteOrganizationSSOUserBestEffort removes an organization identity from
// SSO once. The database result remains authoritative when the remote cleanup
// fails, so the error is logged instead of replacing the original result.
func deleteOrganizationSSOUserBestEffort(ctx context.Context, sso rpc.SSOInterface, organizationUUID string) {
	if sso == nil || organizationUUID == "" {
		return
	}
	if err := sso.DeleteUser(ctx, organizationUUID); err != nil {
		slog.ErrorContext(ctx, "failed to delete organization from SSO", slog.String("organization_uuid", organizationUUID), slog.Any("error", err))
	}
}

func (c *organizationComponentImpl) Create(ctx context.Context, req *types.CreateOrgReq) (*types.Organization, error) {
	user, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	es, err := c.nsStore.Exists(ctx, req.Name)
	if err != nil {
		return nil, err
	}
	if es {
		return nil, errorx.NamespaceAlreadyExists(req.Name)
	}

	// check sso user existence, org will be created as a user in sso service
	exists, err := c.sso.IsExistByName(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to check organization name existence in sso service, error: %w", err)
	}
	if exists {
		return nil, errorx.NamespaceAlreadyExists(req.Name)
	}

	// validate tags before any side-effectful operations (Git, SSO, DB)
	// tags must belong to organization scope and industry category
	if len(req.TagIDs) > 0 {
		err = c.tagStore.CheckTagIDsExistInScope(ctx, req.TagIDs, types.OrganizationTagScope, string(types.IndustryCategory))
		if err != nil {
			if errors.Is(err, database.ErrTagIDsNotFoundInScope) {
				return nil, errorx.TagIDsNotFound(req.TagIDs)
			}
			return nil, fmt.Errorf("failed to check tag IDs, error: %w", err)
		}
	}

	dbOrg := &database.Organization{
		Name:        req.Name,
		Nickname:    req.Nickname,
		Description: req.Description,
		Homepage:    req.Homepage,
		Logo:        req.Logo,
		OrgType:     req.OrgType,
		Verified:    req.Verified,
		IsRoot:      true,
		IsUnit:      false,
		User:        &user,
		UserID:      user.ID,
		UUID:        uuid.New(),
	}

	exist, err := c.nsStore.ExistsByUUID(ctx, dbOrg.UUID.String())
	if err != nil {
		return nil, err
	}

	// use the org uuid as the namespace uuid by default
	newNSUUID := dbOrg.UUID.String()
	if exist {
		// generate a new uuid if the uuid already exists
		newNSUUID = uuid.New().String()
	}

	namespace := &database.Namespace{
		Path:          dbOrg.Name,
		UserID:        user.ID,
		UUID:          newNSUUID,
		NamespaceType: database.OrgNamespace,
	}

	// create sso user before db write, so if sso fails db stays clean
	err = c.sso.CreateUser(ctx, &rpc.SSOCreateUserInfo{
		Name:     dbOrg.Name,
		Nickname: dbOrg.Nickname,
		UUID:     dbOrg.UUID.String(),
		Password: uuid.New().String(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed create sso user for organization, error: %w", err)
	}

	err = c.orgStore.CreateWithRelations(ctx, dbOrg, namespace, req.TagIDs)
	if err != nil {
		deleteOrganizationSSOUserBestEffort(ctx, c.sso, dbOrg.UUID.String())
		return nil, err
	}
	if err := ensureNamespaceRelationship(ctx, c.rebac, *namespace, dbOrg.UUID.String()); err != nil {
		return nil, fmt.Errorf("synchronize organization namespace to ReBAC: %w", err)
	}
	if err := reconcileOrganizationMemberRelationships(
		ctx,
		c.rebac,
		dbOrg.UUID.String(),
		[]string{user.UUID},
		desiredOrganizationMemberRoles([]string{user.UUID}, types.UserAdmin),
	); err != nil {
		return nil, fmt.Errorf("synchronize organization administrator to ReBAC: %w", err)
	}

	org := &types.Organization{
		Name:        dbOrg.Name,
		Nickname:    dbOrg.Nickname,
		Description: dbOrg.Description,
		Homepage:    dbOrg.Homepage,
		Logo:        dbOrg.Logo,
		OrgType:     dbOrg.OrgType,
		Verified:    dbOrg.Verified,
		IsRoot:      dbOrg.IsRoot,
		IsUnit:      dbOrg.IsUnit,
		UUID:        dbOrg.UUID,
		Namespace: &types.Namespace{
			Path: dbOrg.Name,
			Type: string(namespace.NamespaceType),
			UUID: namespace.UUID,
		},
	}
	// Load tags for the response using a separate variable to avoid shadowing.
	if loadTags, loadErr := c.orgStore.GetOrganizationTags(ctx, dbOrg.ID); loadErr != nil {
		slog.WarnContext(ctx, "failed to get organization tags", slog.String("error", loadErr.Error()))
	} else {
		c.appendTags(&org.Tags, loadTags)
	}
	return org, nil
}

func (c *organizationComponentImpl) Index(ctx context.Context, search string, per, page int, orgType, verifyStatus, tag string) ([]types.Organization, int, error) {
	dborgs, total, err := c.orgStore.Search(ctx, search, per, page, orgType, verifyStatus, tag)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list all organizations, error: %w", err)
	}
	orgs, err := c.toOrgList(ctx, dborgs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load organization tags, error: %w", err)
	}
	return orgs, total, nil
}

func (c *organizationComponentImpl) ListUserOrgs(ctx context.Context, req *types.ListUserOrgsReq) ([]types.Organization, int, error) {
	var (
		err    error
		total  int
		dborgs []database.Organization
	)

	if req.Username == "" {
		return nil, 0, fmt.Errorf("username is required")
	}

	u, err := c.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find user, error: %w", err)
	}

	dborgs, total, err = c.orgStore.SearchUserBelongOrgs(ctx, u.ID, req.Search, req.Per, req.Page, req.OrgType, req.VerifyStatus, req.Role, req.Tag)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get user organizations, error: %w", err)
	}

	orgs, err := c.toOrgList(ctx, dborgs)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to load organization tags, error: %w", err)
	}
	return orgs, total, nil
}

func (c *organizationComponentImpl) toOrgList(ctx context.Context, dborgs []database.Organization) ([]types.Organization, error) {
	// Collect org IDs and batch load tags
	orgIDs := make([]int64, len(dborgs))
	for i, dborg := range dborgs {
		orgIDs[i] = dborg.ID
	}
	tagMap, err := c.orgStore.GetOrganizationTagsByOrgIDs(ctx, orgIDs)
	if err != nil {
		slog.WarnContext(ctx, "failed to batch load organization tags", slog.String("error", err.Error()))
	}
	// fallback to empty map if batch load fails
	if tagMap == nil {
		tagMap = make(map[int64][]database.Tag)
	}

	var orgs []types.Organization
	for _, dborg := range dborgs {
		org := types.Organization{
			Name:         dborg.Name,
			Nickname:     dborg.Nickname,
			Description:  dborg.Description,
			Homepage:     dborg.Homepage,
			Logo:         dborg.Logo,
			OrgType:      dborg.OrgType,
			Verified:     dborg.Verified,
			IsRoot:       dborg.IsRoot,
			IsUnit:       dborg.IsUnit,
			VerifyStatus: string(dborg.VerifyStatus),
			UUID:         dborg.UUID,
		}
		if dborg.Namespace != nil {
			org.Namespace = &types.Namespace{
				Path: dborg.Namespace.Path,
				Type: string(dborg.Namespace.NamespaceType),
				UUID: dborg.Namespace.UUID,
			}
		}
		if tags, ok := tagMap[dborg.ID]; ok {
			c.appendTags(&org.Tags, tags)
		}
		orgs = append(orgs, org)
	}
	return orgs, nil
}

func (c *organizationComponentImpl) Get(ctx context.Context, orgName string) (*types.Organization, error) {
	dborg, err := c.orgStore.FindByPath(ctx, orgName)
	if err != nil {
		return nil, fmt.Errorf("failed to get organizations by name, error: %w", err)
	}
	org := &types.Organization{
		Name:        dborg.Name,
		Nickname:    dborg.Nickname,
		Description: dborg.Description,
		Homepage:    dborg.Homepage,
		Logo:        dborg.Logo,
		OrgType:     dborg.OrgType,
		Verified:    dborg.Verified,
		IsRoot:      dborg.IsRoot,
		IsUnit:      dborg.IsUnit,
		UUID:        dborg.UUID,
	}
	if dborg.Namespace != nil {
		org.Namespace = &types.Namespace{
			Path: dborg.Namespace.Path,
			Type: string(dborg.Namespace.NamespaceType),
			UUID: dborg.Namespace.UUID,
		}
	}
	// Load tags
	if tags, err := c.orgStore.GetOrganizationTags(ctx, dborg.ID); err != nil {
		slog.WarnContext(ctx, "failed to get organization tags", slog.String("error", err.Error()))
	} else {
		c.appendTags(&org.Tags, tags)
	}
	return org, nil
}

func (c *organizationComponentImpl) GetByUUID(ctx context.Context, uuid string) (*types.Organization, error) {
	dborg, err := c.orgStore.FindByUUID(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to get organization by uuid, error: %w", err)
	}
	if dborg == nil {
		return nil, errorx.ErrDatabaseNoRows
	}
	org := &types.Organization{
		Name:        dborg.Name,
		Nickname:    dborg.Nickname,
		Description: dborg.Description,
		Homepage:    dborg.Homepage,
		Logo:        dborg.Logo,
		OrgType:     dborg.OrgType,
		Verified:    dborg.Verified,
		IsRoot:      dborg.IsRoot,
		IsUnit:      dborg.IsUnit,
		UUID:        dborg.UUID,
	}
	if dborg.Namespace != nil {
		org.Namespace = &types.Namespace{
			Path: dborg.Namespace.Path,
			Type: string(dborg.Namespace.NamespaceType),
			UUID: dborg.Namespace.UUID,
		}
	}
	// Load tags
	if tags, err := c.orgStore.GetOrganizationTags(ctx, dborg.ID); err != nil {
		slog.WarnContext(ctx, "failed to get organization tags", slog.String("error", err.Error()))
	} else {
		c.appendTags(&org.Tags, tags)
	}
	return org, nil
}

func (c *organizationComponentImpl) Delete(ctx context.Context, req *types.DeleteOrgReq) error {
	canAdmin, err := c.checkNamespaceAdminPermission(ctx, req.Name, req.CurrentUser)
	if err != nil {
		slog.ErrorContext(ctx, "failed to check namespace permission",
			slog.String("namespace", req.Name), slog.String("user", req.CurrentUser),
			slog.Any("error", err))
	}
	if !canAdmin {
		return fmt.Errorf("current user does not have permission to edit the organization, current user: %s", req.CurrentUser)
	}
	organization, err := c.orgStore.FindByPath(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find database organization, error: %w", err)
	}
	if organization.IsUnit {
		return errorx.ReqParamInvalid(
			errors.New("hierarchy organizations must be deleted through the hierarchy organization API"),
			nil,
		)
	}
	if c.memberStore == nil {
		return fmt.Errorf("organization member store is required")
	}
	userUUIDs, err := c.memberStore.UserUUIDsByOrganizationID(ctx, organization.ID)
	if err != nil {
		return fmt.Errorf("load organization members for ReBAC cleanup: %w", err)
	}
	if organization.Namespace == nil || organization.Namespace.UUID == "" {
		return fmt.Errorf("organization %q namespace UUID is required for ReBAC cleanup", organization.Name)
	}
	cleanup := types.OrganizationReBACCleanup{
		OrganizationUUID: organization.UUID.String(),
		NamespaceUUID:    organization.Namespace.UUID,
		UserUUIDs:        userUUIDs,
	}
	err = c.orgStore.Delete(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to delete database organizations, error: %w", err)
	}
	if err := deleteOrganizationReBACRelationships(ctx, c.rebac, []types.OrganizationReBACCleanup{cleanup}); err != nil {
		return fmt.Errorf("sync deleted organization to ReBAC: %w", err)
	}
	deleteOrganizationSSOUserBestEffort(ctx, c.sso, organization.UUID.String())
	return nil
}

func (c *organizationComponentImpl) Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error) {
	canAdmin, err := c.checkNamespaceAdminPermission(ctx, req.Name, req.CurrentUser)
	if err != nil {
		slog.ErrorContext(ctx, "failed to check namespace permission",
			slog.String("namespace", req.Name), slog.String("user", req.CurrentUser),
			slog.Any("error", err))
	}
	if !canAdmin {
		return nil, fmt.Errorf("current user does not have permission to edit the organization, current user: %s", req.CurrentUser)
	}
	org, err := c.orgStore.FindByPath(ctx, req.Name)
	if err != nil {
		return nil, fmt.Errorf("organization does not exists, error: %w", err)
	}

	if req.Nickname != nil {
		org.Nickname = *req.Nickname
	}
	if req.Logo != nil {
		org.Logo = *req.Logo
	}
	if req.Homepage != nil {
		org.Homepage = *req.Homepage
	}
	if req.Verified != nil {
		org.Verified = *req.Verified
	}
	if req.OrgType != nil {
		org.OrgType = *req.OrgType
	}
	if req.Description != nil {
		org.Description = *req.Description
	}

	if len(req.TagIDs) > 0 {
		err = c.tagStore.CheckTagIDsExistInScope(ctx, req.TagIDs, types.OrganizationTagScope, string(types.IndustryCategory))
		if err != nil {
			if errors.Is(err, database.ErrTagIDsNotFoundInScope) {
				return nil, errorx.TagIDsNotFound(req.TagIDs)
			}
			return nil, fmt.Errorf("failed to check tag IDs, error: %w", err)
		}
	}

	err = c.orgStore.Update(ctx, &org)
	if err != nil {
		return nil, fmt.Errorf("failed to update database organization, error: %w", err)
	}

	// Update organization tags if provided
	if req.TagIDs != nil {
		err = c.orgStore.SetOrganizationTags(ctx, org.ID, req.TagIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to set organization tags, error: %w", err)
		}
	}

	//skip update git server
	if req.Nickname == nil && req.Description == nil {
		return &org, nil
	}
	var gitEditReq types.EditOrgReq
	gitEditReq.Name = org.Name
	gitEditReq.Nickname = &org.Nickname
	gitEditReq.Description = &org.Description
	return &org, err
}

// checkNamespaceAdminPermission checks whether a user can administer an organization namespace.
func (c *organizationComponentImpl) checkNamespaceAdminPermission(ctx context.Context, namespacePath, userName string) (bool, error) {
	user, err := c.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return false, fmt.Errorf("find user %q for namespace permission: %w", userName, err)
	}
	namespace, err := c.nsStore.FindByPath(ctx, namespacePath)
	if err != nil {
		return false, fmt.Errorf("find namespace %q for permission: %w", namespacePath, err)
	}
	decision, err := c.rebac.Check(ctx, rebac.CheckRequest{
		Subject:     rebac.UserSubject(user.UUID),
		Relation:    rebac.NamespaceCanAdmin,
		Object:      rebac.NamespaceObject(namespace.UUID),
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return false, fmt.Errorf("check namespace admin permission: %w", err)
	}
	return decision.Allowed, nil
}

// appendTags converts database.Tag slice to types.RepoTag and appends to dst.
func (c *organizationComponentImpl) appendTags(dst *[]types.RepoTag, tags []database.Tag) {
	for _, t := range tags {
		*dst = append(*dst, types.RepoTag{
			ID:        t.ID,
			Name:      t.Name,
			Category:  t.Category,
			Group:     t.Group,
			BuiltIn:   t.BuiltIn,
			Scope:     t.Scope,
			ShowName:  t.I18nKey,
			I18nKey:   t.I18nKey,
			CreatedAt: t.CreatedAt,
			UpdatedAt: t.UpdatedAt,
		})
	}
}
