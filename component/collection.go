package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type CollectionComponent interface {
	GetCollections(ctx context.Context, filter *types.CollectionFilter, per, page int) ([]types.Collection, int, error)
	CreateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error)
	GetCollection(ctx context.Context, currentUser string, id int64) (*types.Collection, error)
	// get non private repositories of the collection
	GetPublicRepos(collection types.Collection) []types.CollectionRepository
	UpdateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error)
	DeleteCollection(ctx context.Context, id int64, userName string) error
	AddReposToCollection(ctx context.Context, req types.UpdateCollectionReposReq) error
	RemoveReposFromCollection(ctx context.Context, req types.UpdateCollectionReposReq) error
	OrgCollections(ctx context.Context, req *types.OrgCollectionsReq) ([]types.Collection, int, error)
	UpdateCollectionRepo(ctx context.Context, req types.UpdateCollectionRepoReq) error
}

func NewCollectionComponent(config *config.Config) (CollectionComponent, error) {
	cc := &collectionComponentImpl{}
	cc.collectionStore = database.NewCollectionStore()
	cc.repoStore = database.NewRepoStore()
	cc.userStore = database.NewUserStore()
	cc.orgStore = database.NewOrgStore()
	cc.userLikesStore = database.NewUserLikesStore()
	cc.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	spaceComponent, err := NewSpaceComponent(config)
	if err != nil {
		return nil, err
	}
	cc.spaceComponent = spaceComponent
	return cc, nil
}

type collectionComponentImpl struct {
	collectionStore database.CollectionStore
	orgStore        database.OrgStore
	repoStore       database.RepoStore
	userStore       database.UserStore
	userLikesStore  database.UserLikesStore
	userSvcClient   rpc.UserSvcClient
	spaceComponent  SpaceComponent
}

func (cc *collectionComponentImpl) GetCollections(ctx context.Context, filter *types.CollectionFilter, per, page int) ([]types.Collection, int, error) {
	collections, total, err := cc.collectionStore.GetCollections(ctx, filter, per, page, true)
	if err != nil {
		return nil, 0, err
	}
	var newCollection []types.Collection
	temporaryVariable, _ := json.Marshal(collections)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, 0, err
	}
	return newCollection, total, nil

}

func (cc *collectionComponentImpl) CreateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	// find by user name
	user, err := cc.userStore.FindByUsername(ctx, input.Username)
	if err != nil {
		return nil, fmt.Errorf("cannot find user for collection, %w", err)
	}
	collection := database.Collection{
		Username:    user.Username,
		Namespace:   input.Namespace,
		UserID:      user.ID,
		Name:        input.Name,
		Nickname:    input.Nickname,
		Description: input.Description,
		Private:     input.Private,
		Theme:       input.Theme,
	}
	//for org case, no need user name
	if input.Namespace != "" {
		collection.Username = ""
	}

	return cc.collectionStore.CreateCollection(ctx, collection)
}

func (cc *collectionComponentImpl) GetCollection(ctx context.Context, currentUser string, id int64) (*types.Collection, error) {
	collection, err := cc.collectionStore.GetCollection(ctx, id)
	if err != nil {
		return nil, err
	}
	// find by user name
	avatar := ""
	if collection.Username != "" {
		user, err := cc.userStore.FindByUsername(ctx, collection.Username)
		if err != nil {
			return nil, fmt.Errorf("cannot find user for collection, %w", err)
		}
		avatar = user.Avatar
	} else if collection.Namespace != "" {
		org, err := cc.orgStore.FindByPath(ctx, collection.Namespace)
		if err != nil {
			return nil, fmt.Errorf("fail to get org info, path: %s, error: %w", collection.Namespace, err)
		}
		avatar = org.Logo
	}

	permission, err := cc.getUserCollectionPermission(ctx, currentUser, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}

	if !permission.CanRead {
		return nil, errorx.ErrUnauthorized
	}

	var newCollection types.Collection
	temporaryVariable, _ := json.Marshal(collection)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, err
	}

	// Get collection repositories with remarks
	collectionRepos, err := cc.collectionStore.GetCollectionRepos(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get collection repositories, error: %w", err)
	}
	repoRemarkMap := make(map[int64]string, len(collectionRepos))
	for _, cr := range collectionRepos {
		repoRemarkMap[cr.RepositoryID] = cr.Remark
	}

	likeExists, err := cc.userLikesStore.IsExistCollection(ctx, currentUser, id)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}
	if !permission.CanWrite {
		newCollection.Repositories = cc.GetPublicRepos(newCollection)
	}
	for i, repo := range newCollection.Repositories {
		if repo.RepositoryType == types.SpaceRepo && strings.Contains(repo.Path, "/") {
			namespace, name := repo.NamespaceAndName()
			_, status, _ := cc.spaceComponent.Status(ctx, namespace, name)
			newCollection.Repositories[i].Status = status
		}
		if remark, exists := repoRemarkMap[repo.ID]; exists {
			newCollection.Repositories[i].Remark = remark
		}
	}
	newCollection.UserLikes = likeExists
	newCollection.CanWrite = permission.CanWrite
	newCollection.CanManage = permission.CanAdmin
	newCollection.Avatar = avatar
	return &newCollection, nil
}

// get non private repositories of the collection
func (cc *collectionComponentImpl) GetPublicRepos(collection types.Collection) []types.CollectionRepository {
	var filtered []types.CollectionRepository
	for _, repo := range collection.Repositories {
		if !repo.Private {
			filtered = append(filtered, repo)
		}
	}
	return filtered
}

func (cc *collectionComponentImpl) UpdateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	collection, err := cc.collectionStore.GetCollection(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot find collection to update, %w", err)
	}
	collection.Name = input.Name
	collection.Nickname = input.Nickname
	collection.Description = input.Description
	collection.Private = input.Private
	collection.Theme = input.Theme
	collection.UpdatedAt = time.Now()
	return cc.collectionStore.UpdateCollection(ctx, *collection)
}

func (cc *collectionComponentImpl) DeleteCollection(ctx context.Context, id int64, userName string) error {
	// find by user name
	user, err := cc.userStore.FindByUsername(ctx, userName)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	return cc.collectionStore.DeleteCollection(ctx, id, user.ID)
}

func (cc *collectionComponentImpl) AddReposToCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	// find by user name
	user, err := cc.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	collection, err := cc.collectionStore.GetCollection(ctx, req.ID)
	if err != nil {
		return err
	}
	if collection.UserID != user.ID {
		return fmt.Errorf("no permission to operate this collection: %s", strconv.FormatInt(req.ID, 10))
	}
	var collectionRepos []database.CollectionRepository
	for _, repo := range req.RepoIDs {
		remark := ""
		if r, exists := req.Remarks[repo]; exists {
			remark = r
		}
		collectionRepos = append(collectionRepos, database.CollectionRepository{
			CollectionID: req.ID,
			RepositoryID: repo,
			Remark:       remark,
		})
	}
	err = cc.collectionStore.AddCollectionRepos(ctx, collectionRepos)
	if err != nil {
		// Check if the error is a duplicate key constraint violation
		if strings.Contains(err.Error(), "duplicate key value") {
			return errorx.RepoAlreadyInCollection(err, errorx.Ctx().Set("collection_id", strconv.FormatInt(req.ID, 10)))
		}
		if strings.Contains(err.Error(), "violates foreign key constraint") {
			return fmt.Errorf("repo not found: %v", req.RepoIDs)
		}
		return err
	}
	return err
}

func (cc *collectionComponentImpl) RemoveReposFromCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	// find by user name
	user, err := cc.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	collection, err := cc.collectionStore.GetCollection(ctx, req.ID)
	if err != nil {
		return err
	}
	if collection.UserID != user.ID {
		return fmt.Errorf("no permission to operate this collection: %s", strconv.FormatInt(req.ID, 10))
	}
	var collectionRepos []database.CollectionRepository
	for _, repo := range req.RepoIDs {
		collectionRepos = append(collectionRepos, database.CollectionRepository{
			CollectionID: req.ID,
			RepositoryID: repo,
		})
	}
	return cc.collectionStore.RemoveCollectionRepos(ctx, collectionRepos)
}

func (cc *collectionComponentImpl) getUserCollectionPermission(ctx context.Context, userName string, collection *database.Collection) (*types.UserRepoPermission, error) {
	if userName == "" {
		//anonymous user only has read permission to public repo
		return &types.UserRepoPermission{CanRead: !collection.Private, CanWrite: false, CanAdmin: false}, nil
	}

	namespace := collection.Namespace
	namespaceType := "user"
	if namespace == "" {
		//Compatibility old data
		namespace = collection.Username
	}
	if collection.Username != namespace {
		namespaceType = "org"
	}

	if namespaceType == "user" {
		//owner has full permission
		if userName == namespace {
			return &types.UserRepoPermission{
				CanRead:  true,
				CanWrite: true,
				CanAdmin: true,
			}, nil
		} else {
			//other user has read permission to pubic repo
			return &types.UserRepoPermission{
				CanRead: !collection.Private, CanWrite: false, CanAdmin: false,
			}, nil
		}
	} else {
		r, err := cc.userSvcClient.GetMemberRole(ctx, namespace, userName)
		if err != nil {
			return nil, fmt.Errorf("failed to get user '%s' member role of org '%s' when get user repo permission, error: %w", userName, namespace, err)
		}

		return &types.UserRepoPermission{
			CanRead:  r.CanRead() || !collection.Private,
			CanWrite: r.CanWrite(),
			CanAdmin: r.CanAdmin(),
		}, nil
	}
}

func (c *collectionComponentImpl) OrgCollections(ctx context.Context, req *types.OrgCollectionsReq) ([]types.Collection, int, error) {
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.Error("faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	collections, total, err := c.collectionStore.ByUserOrgs(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		return nil, 0, err
	}
	var newCollection []types.Collection
	temporaryVariable, _ := json.Marshal(collections)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, 0, err
	}
	return newCollection, total, nil

}

func (cc *collectionComponentImpl) UpdateCollectionRepo(ctx context.Context, req types.UpdateCollectionRepoReq) error {
	user, err := cc.userStore.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}

	collection, err := cc.collectionStore.GetCollection(ctx, req.ID)
	if err != nil {
		return err
	}
	if collection.UserID != user.ID {
		return errorx.ErrForbiddenMsg("no permission to operate this collection")
	}

	err = cc.collectionStore.UpdateCollectionRepo(ctx, database.CollectionRepository{
		CollectionID: req.ID,
		RepositoryID: req.RepoID,
		Remark:       req.Remark,
	})
	if err != nil {
		return err
	}
	return nil
}
