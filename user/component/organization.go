package component

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/riverqueue/river"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rebac"
	rebacfactory "opencsg.com/csghub-server/builder/rebac/factory"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/workhub"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

const organizationDeletionBestEffortTimeout = 5 * time.Second

type OrganizationComponent interface {
	Create(ctx context.Context, req *types.CreateOrgReq) (*types.Organization, error)
	Index(ctx context.Context, search string, per, page int, orgType, verifyStatus, tag string) ([]types.Organization, int, error)
	// ListUserOrgs returns organizations belonging to a user.
	// Deprecated: the user organization list endpoint is being retired; use the current organization APIs instead.
	ListUserOrgs(ctx context.Context, req *types.ListUserOrgsReq) ([]types.Organization, int, error)
	// ListCurrentUserWritableNamespaces returns namespaces where the current user can write.
	ListCurrentUserWritableNamespaces(ctx context.Context, currentUser string) ([]types.WritableNamespace, error)
	Get(ctx context.Context, orgName string) (*types.Organization, error)
	GetByUUID(ctx context.Context, uuid string) (*types.Organization, error)
	Delete(ctx context.Context, req *types.DeleteOrgReq) error
	Update(ctx context.Context, req *types.EditOrgReq) (*database.Organization, error)
}

// NewOrganizationComponent uses the effective edition mode for organization lists and writes.
func NewOrganizationComponent(config *config.Config) (OrganizationComponent, error) {
	c := &organizationComponentImpl{config: config}
	authorizer, err := rebacfactory.NewAuthorizer()
	if err != nil {
		return nil, fmt.Errorf("fail to create ReBAC authorizer: %w", err)
	}
	c.rebac = authorizer
	deletionJobClient, err := newRepositoryDeletionJobClient()
	if err != nil {
		return nil, err
	}
	organizationDeletionJobClient, err := newOrganizationDeletionJobClient()
	if err != nil {
		return nil, err
	}
	c.orgStore = database.NewOrgStoreWithDeletionJobClients(config.IsHierarchicalOrganization(), deletionJobClient, organizationDeletionJobClient)
	c.memberStore = database.NewMemberStore()
	c.nsStore = database.NewNamespaceStore()
	c.userStore = database.NewUserStore()
	c.tagStore = database.NewTagStore()
	c.repositoryAuthorizations = database.NewRepositoryAuthorizationStore()
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
	// repositoryAuthorizations stores direct grants that use an organization as subject.
	repositoryAuthorizations database.RepositoryAuthorizationStore
}

// OrganizationDeletionWorker retries external organization cleanup after the
// database deletion transaction has committed.
type OrganizationDeletionWorker struct {
	river.WorkerDefaults[workhub.OrganizationDeletionArgs]
	sso                      rpc.SSOInterface
	rebac                    rebac.Authorizer
	repositoryAuthorizations database.RepositoryAuthorizationStore
}

// NewOrganizationDeletionWorker creates the durable organization cleanup worker.
func NewOrganizationDeletionWorker(sso rpc.SSOInterface, authorizer rebac.Authorizer, authorizations database.RepositoryAuthorizationStore) *OrganizationDeletionWorker {
	return &OrganizationDeletionWorker{sso: sso, rebac: authorizer, repositoryAuthorizations: authorizations}
}

// NewOrganizationDeletionWorkClient creates a work client for asynchronous organization deletion cleanup.
func NewOrganizationDeletionWorkClient(
	ctx context.Context,
	dsn string,
	sso rpc.SSOInterface,
	authorizer rebac.Authorizer,
	authorizations database.RepositoryAuthorizationStore,
	maxWorkers int,
) (workhub.WorkClient, error) {
	if maxWorkers <= 0 {
		maxWorkers = 2
	}
	worker := NewOrganizationDeletionWorker(sso, authorizer, authorizations)
	riverConfig := &river.Config{
		Queues: map[string]river.QueueConfig{
			workhub.OrganizationDeletionQueue: {MaxWorkers: maxWorkers},
		},
		Workers: workhub.NewWorkerRegistry(workhub.WorkerOverrides{
			OrganizationDeletion: worker,
		}),
	}
	return workhub.NewWorkClient(ctx, dsn, riverConfig)
}

// Work performs an idempotent cleanup attempt. Every independent cleanup stage
// is attempted so one unavailable dependency does not prevent the others.
func (w *OrganizationDeletionWorker) Work(ctx context.Context, job *river.Job[workhub.OrganizationDeletionArgs]) error {
	if job == nil || len(job.Args.OrganizationUUIDs) == 0 {
		return fmt.Errorf("organization deletion job requires organization UUIDs")
	}
	if w.rebac == nil {
		return fmt.Errorf("organization deletion worker ReBAC authorizer is required")
	}
	var jobID int64
	var attempt, maxAttempts int
	if job.JobRow != nil {
		jobID = job.ID
		attempt = job.Attempt
		maxAttempts = job.MaxAttempts
	}
	slog.InfoContext(ctx, "starting organization deletion worker job",
		slog.Int64("job_id", jobID),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", maxAttempts),
		slog.Any("organization_ids", job.Args.OrganizationIDs),
		slog.Any("organization_uuids", job.Args.OrganizationUUIDs),
		slog.Int("hierarchy_relationships_count", len(job.Args.DeletedHierarchyRelationships)),
		slog.Int("rebac_cleanups_count", len(job.Args.DeletedReBACRelationships)),
	)
	err := cleanupDeletedOrganization(ctx, w.sso, w.rebac, w.repositoryAuthorizations, job.Args)
	if err != nil {
		slog.ErrorContext(ctx, "organization deletion worker job failed",
			slog.Int64("job_id", jobID),
			slog.Int("attempt", attempt),
			slog.Any("organization_uuids", job.Args.OrganizationUUIDs),
			slog.Any("error", err),
		)
		return err
	}
	slog.InfoContext(ctx, "completed organization deletion worker job successfully",
		slog.Int64("job_id", jobID),
		slog.Int("attempt", attempt),
		slog.Any("organization_uuids", job.Args.OrganizationUUIDs),
	)
	return nil
}

// Timeout bounds one River cleanup attempt.
func (w *OrganizationDeletionWorker) Timeout(*river.Job[workhub.OrganizationDeletionArgs]) time.Duration {
	return workhub.OrganizationDeletionJobTimeout
}

// cleanupDeletedOrganization removes all external identities and relationships
// represented by one immutable deletion snapshot.
func cleanupDeletedOrganization(
	ctx context.Context,
	sso rpc.SSOInterface,
	authorizer rebac.Authorizer,
	authorizations database.RepositoryAuthorizationStore,
	args workhub.OrganizationDeletionArgs,
) error {
	var cleanupErrors []error
	for _, organizationID := range args.OrganizationIDs {
		if err := deleteDirectRepositoryAuthorizations(ctx, authorizations, authorizer, types.RepoAuthSubjectOrganization, organizationID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete organization %d repository authorizations: %w", organizationID, err))
		}
	}
	for _, organizationUUID := range args.OrganizationUUIDs {
		if err := deleteOrganizationSSOUser(ctx, sso, organizationUUID); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Errorf("delete organization %s from SSO: %w", organizationUUID, err))
		}
	}
	hierarchyRelationships := make([]types.OrganizationHierarchyRelationship, 0, len(args.DeletedHierarchyRelationships))
	for _, relationship := range args.DeletedHierarchyRelationships {
		hierarchyRelationships = append(hierarchyRelationships, types.OrganizationHierarchyRelationship{
			ParentOrganizationUUID: relationship.ParentOrganizationUUID,
			ChildOrganizationUUID:  relationship.ChildOrganizationUUID,
		})
	}
	if err := deleteOrganizationHierarchyRelationshipsForDeletion(ctx, authorizer, hierarchyRelationships); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete organization hierarchy relationships: %w", err))
	}
	rebacCleanups := make([]types.OrganizationReBACCleanup, 0, len(args.DeletedReBACRelationships))
	for _, cleanup := range args.DeletedReBACRelationships {
		rebacCleanups = append(rebacCleanups, types.OrganizationReBACCleanup{
			OrganizationUUID: cleanup.OrganizationUUID,
			NamespaceUUID:    cleanup.NamespaceUUID,
			UserUUIDs:        cleanup.UserUUIDs,
		})
	}
	if err := deleteOrganizationReBACRelationships(ctx, authorizer, rebacCleanups); err != nil {
		cleanupErrors = append(cleanupErrors, fmt.Errorf("delete organization member and namespace relationships: %w", err))
	}
	return errors.Join(cleanupErrors...)
}

// newOrganizationDeletionArgs converts database cleanup metadata into the
// stable River payload shared by the fast path and durable worker.
func newOrganizationDeletionArgs(
	organizationIDs []int64,
	organizationUUIDs []string,
	hierarchyRelationships []types.OrganizationHierarchyRelationship,
	rebacCleanups []types.OrganizationReBACCleanup,
) workhub.OrganizationDeletionArgs {
	args := workhub.OrganizationDeletionArgs{
		OrganizationIDs:   organizationIDs,
		OrganizationUUIDs: organizationUUIDs,
	}
	for _, relationship := range hierarchyRelationships {
		args.DeletedHierarchyRelationships = append(args.DeletedHierarchyRelationships, workhub.OrganizationHierarchyRelationshipArgs{
			ParentOrganizationUUID: relationship.ParentOrganizationUUID,
			ChildOrganizationUUID:  relationship.ChildOrganizationUUID,
		})
	}
	for _, cleanup := range rebacCleanups {
		args.DeletedReBACRelationships = append(args.DeletedReBACRelationships, workhub.OrganizationReBACCleanupArgs{
			OrganizationUUID: cleanup.OrganizationUUID,
			NamespaceUUID:    cleanup.NamespaceUUID,
			UserUUIDs:        cleanup.UserUUIDs,
		})
	}
	return args
}

// cleanupDeletedOrganizationBestEffort runs the low-latency cleanup path after
// commit. River remains responsible for eventual completion regardless of the result.
func cleanupDeletedOrganizationBestEffort(
	ctx context.Context,
	sso rpc.SSOInterface,
	authorizer rebac.Authorizer,
	authorizations database.RepositoryAuthorizationStore,
	jobID int64,
	args workhub.OrganizationDeletionArgs,
) {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), organizationDeletionBestEffortTimeout)
	defer cancel()
	if err := cleanupDeletedOrganization(cleanupCtx, sso, authorizer, authorizations, args); err != nil {
		slog.WarnContext(ctx, "organization external cleanup deferred to River",
			slog.Int64("job_id", jobID),
			slog.Any("organization_uuids", args.OrganizationUUIDs),
			slog.Any("error", err))
	}
}

// deleteOrganizationSSOUser removes an organization identity from SSO.
func deleteOrganizationSSOUser(ctx context.Context, sso rpc.SSOInterface, organizationUUID string) error {
	if sso == nil || organizationUUID == "" {
		return nil
	}
	if err := sso.DeleteUser(ctx, organizationUUID); err != nil {
		slog.ErrorContext(ctx, "failed to delete organization from SSO", slog.String("organization_uuid", organizationUUID), slog.Any("error", err))
		return err
	}
	return nil
}

// deleteOrganizationSSOUserBestEffort compensates a failed organization
// creation without masking the original database error.
func deleteOrganizationSSOUserBestEffort(ctx context.Context, sso rpc.SSOInterface, organizationUUID string) {
	if err := deleteOrganizationSSOUser(ctx, sso, organizationUUID); err != nil {
		slog.ErrorContext(ctx, "failed to compensate organization SSO user", slog.Any("error", err))
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
		Name:           req.Name,
		Nickname:       req.Nickname,
		Description:    req.Description,
		Homepage:       req.Homepage,
		Logo:           req.Logo,
		OrgType:        req.OrgType,
		Verified:       req.Verified,
		IsRoot:         true,
		IsHierarchical: false,
		User:           &user,
		UserID:         user.ID,
		UUID:           uuid.New(),
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
		Name:           dbOrg.Name,
		Nickname:       dbOrg.Nickname,
		Description:    dbOrg.Description,
		Homepage:       dbOrg.Homepage,
		Logo:           dbOrg.Logo,
		OrgType:        dbOrg.OrgType,
		Verified:       dbOrg.Verified,
		IsRoot:         dbOrg.IsRoot,
		IsHierarchical: dbOrg.IsHierarchical,
		UUID:           dbOrg.UUID,
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

// ListUserOrgs returns organizations belonging to the requested user.
// Deprecated: the user organization list endpoint is being retired; use the current organization APIs instead.
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

// ListCurrentUserWritableNamespaces returns namespaces authorized by OpenFGA with can_write.
func (c *organizationComponentImpl) ListCurrentUserWritableNamespaces(ctx context.Context, currentUser string) ([]types.WritableNamespace, error) {
	if currentUser == "" {
		return nil, fmt.Errorf("current user is required")
	}
	if c.rebac == nil {
		return nil, fmt.Errorf("ReBAC authorizer is nil")
	}
	user, err := c.userStore.FindByUsername(ctx, currentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to find current user, error: %w", err)
	}

	objects, err := c.rebac.ListObjects(ctx, rebac.ListObjectsRequest{
		Subject:     rebac.UserSubject(user.UUID),
		Relation:    rebac.NamespaceCanWrite,
		ObjectType:  rebac.ObjectTypeNamespace,
		Consistency: rebac.ConsistencyHigher,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list current user writable namespaces, error: %w", err)
	}

	namespaceUUIDs := make([]string, 0, len(objects.Objects))
	seen := make(map[string]struct{}, len(objects.Objects))
	for _, object := range objects.Objects {
		if object.Type != rebac.ObjectTypeNamespace || object.ID == "" {
			continue
		}
		if _, exists := seen[object.ID]; exists {
			continue
		}
		seen[object.ID] = struct{}{}
		namespaceUUIDs = append(namespaceUUIDs, object.ID)
	}
	if len(namespaceUUIDs) == 0 {
		return []types.WritableNamespace{}, nil
	}

	namespaces, err := c.nsStore.FindByUUIDs(ctx, namespaceUUIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load writable namespaces, error: %w", err)
	}
	namespaceByUUID := make(map[string]database.Namespace, len(namespaces))
	userUUIDs := make([]string, 0, len(namespaces))
	organizationUUIDs := make([]string, 0, len(namespaces))
	for _, namespace := range namespaces {
		if namespace.UUID == "" {
			continue
		}
		namespaceByUUID[namespace.UUID] = namespace
		switch namespace.NamespaceType {
		case database.UserNamespace:
			userUUIDs = append(userUUIDs, namespace.UUID)
		case database.OrgNamespace:
			organizationUUIDs = append(organizationUUIDs, namespace.UUID)
		}
	}

	usersByUUID := make(map[string]*database.User, len(userUUIDs))
	if len(userUUIDs) > 0 {
		users, err := c.userStore.FindByUUIDs(ctx, userUUIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to load writable namespace users, error: %w", err)
		}
		for _, entity := range users {
			if entity != nil {
				usersByUUID[entity.UUID] = entity
			}
		}
	}

	organizationsByUUID := make(map[string]database.Organization, len(organizationUUIDs))
	if len(organizationUUIDs) > 0 {
		organizations, err := c.orgStore.FindByUUIDs(ctx, organizationUUIDs)
		if err != nil {
			return nil, fmt.Errorf("failed to load writable namespace organizations, error: %w", err)
		}
		for _, organization := range organizations {
			organizationsByUUID[organization.UUID.String()] = organization
		}
	}

	result := make([]types.WritableNamespace, 0, len(namespaceByUUID))
	for _, namespaceUUID := range namespaceUUIDs {
		namespace, exists := namespaceByUUID[namespaceUUID]
		if !exists {
			continue
		}
		switch namespace.NamespaceType {
		case database.UserNamespace:
			entity, exists := usersByUUID[namespaceUUID]
			if !exists {
				continue
			}
			name := entity.NickName
			if name == "" {
				name = entity.Username
			}
			entityUUID := entity.UUID
			if entityUUID == "" {
				entityUUID = namespaceUUID
			}
			result = append(result, types.WritableNamespace{Path: namespace.Path, Type: string(database.UserNamespace), Name: name, UUID: entityUUID})
		case database.OrgNamespace:
			entity, exists := organizationsByUUID[namespaceUUID]
			if !exists {
				continue
			}
			name := entity.Nickname
			if name == "" {
				name = entity.Name
			}
			entityUUID := entity.UUID.String()
			if entityUUID == uuid.Nil.String() {
				entityUUID = namespaceUUID
			}
			result = append(result, types.WritableNamespace{Path: namespace.Path, Type: string(database.OrgNamespace), Name: name, UUID: entityUUID})
		}
	}
	return result, nil
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
			Name:           dborg.Name,
			Nickname:       dborg.Nickname,
			Description:    dborg.Description,
			Homepage:       dborg.Homepage,
			Logo:           dborg.Logo,
			OrgType:        dborg.OrgType,
			Verified:       dborg.Verified,
			IsRoot:         dborg.IsRoot,
			IsHierarchical: dborg.IsHierarchical,
			VerifyStatus:   string(dborg.VerifyStatus),
			UUID:           dborg.UUID,
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
		Name:           dborg.Name,
		Nickname:       dborg.Nickname,
		Description:    dborg.Description,
		Homepage:       dborg.Homepage,
		Logo:           dborg.Logo,
		OrgType:        dborg.OrgType,
		Verified:       dborg.Verified,
		IsRoot:         dborg.IsRoot,
		IsHierarchical: dborg.IsHierarchical,
		UUID:           dborg.UUID,
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
		Name:           dborg.Name,
		Nickname:       dborg.Nickname,
		Description:    dborg.Description,
		Homepage:       dborg.Homepage,
		Logo:           dborg.Logo,
		OrgType:        dborg.OrgType,
		Verified:       dborg.Verified,
		IsRoot:         dborg.IsRoot,
		IsHierarchical: dborg.IsHierarchical,
		UUID:           dborg.UUID,
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

// Delete soft-deletes a legacy organization, durably schedules external
// cleanup, and performs one best-effort cleanup attempt after commit.
func (c *organizationComponentImpl) Delete(ctx context.Context, req *types.DeleteOrgReq) error {
	organization, err := c.orgStore.FindForDeletion(ctx, database.OrganizationDeletionLookup{Path: req.Name})
	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, errorx.ErrDatabaseNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("find organization for deletion: %w", err)
	}
	if organization.IsHierarchical {
		return errorx.ReqParamInvalid(errors.New("hierarchy organizations must be deleted through the hierarchy organization API"), nil)
	}
	if !organization.DeletedAt.IsZero() {
		return nil
	}
	canAdmin, err := c.checkNamespaceAdminPermission(ctx, req.Name, req.CurrentUser)
	if err != nil {
		return err
	}
	if !canAdmin {
		return errorx.ErrOrganizationManageForbidden
	}
	result, err := c.orgStore.Delete(ctx, req.Name)
	if err != nil {
		return fmt.Errorf("failed to delete database organizations: %w", err)
	}
	if !result.AlreadyDeleted {
		cleanupDeletedOrganizationBestEffort(ctx, c.sso, c.rebac, c.repositoryAuthorizations, result.OrganizationJobID,
			newOrganizationDeletionArgs([]int64{organization.ID}, []string{organization.UUID.String()}, nil, result.DeletedReBACRelationships))
	}
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
