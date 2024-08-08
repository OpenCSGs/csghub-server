package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewCollectionComponent(config *config.Config) (*CollectionComponent, error) {
	cc := &CollectionComponent{}
	cc.cs = database.NewCollectionStore()
	cc.rs = database.NewRepoStore()
	cc.us = database.NewUserStore()
	cc.uls = database.NewUserLikesStore()
	cc.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	return cc, nil
}

type CollectionComponent struct {
	cs            *database.CollectionStore
	rs            *database.RepoStore
	us            *database.UserStore
	uls           *database.UserLikesStore
	userSvcClient rpc.UserSvcClient
}

func (cc *CollectionComponent) GetCollections(ctx context.Context, filter *types.CollectionFilter, per, page int) ([]types.Collection, int, error) {
	collections, total, err := cc.cs.GetCollections(ctx, filter, per, page)
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

func (cc *CollectionComponent) CreateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	// find by user name
	user, err := cc.us.FindByUsername(ctx, input.Username)
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
	return cc.cs.CreateCollection(ctx, collection)
}

func (cc *CollectionComponent) GetCollection(ctx context.Context, currentUser string, id int64) (*types.Collection, error) {
	collection, err := cc.cs.GetCollection(ctx, id)
	if err != nil {
		return nil, err
	}

	permission, err := cc.getUserCollectionPermission(ctx, currentUser, collection)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}

	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	var newCollection types.Collection
	temporaryVariable, _ := json.Marshal(collection)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, err
	}
	likeExists, err := cc.uls.IsExistCollection(ctx, currentUser, id)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}
	newCollection.UserLikes = likeExists
	newCollection.CanWrite = permission.CanWrite
	newCollection.CanManage = permission.CanAdmin
	return &newCollection, nil
}

func (cc *CollectionComponent) UpdateCollection(ctx context.Context, input types.CreateCollectionReq) (*database.Collection, error) {
	collection, err := cc.cs.GetCollection(ctx, input.ID)
	if err != nil {
		return nil, fmt.Errorf("cannot find collection to update, %w", err)
	}
	collection.Name = input.Name
	collection.Nickname = input.Nickname
	collection.Description = input.Description
	collection.Private = input.Private
	collection.Theme = input.Theme
	collection.UpdatedAt = time.Now()
	return cc.cs.UpdateCollection(ctx, *collection)
}

func (cc *CollectionComponent) DeleteCollection(ctx context.Context, id int64, userName string) error {
	// find by user name
	user, err := cc.us.FindByUsername(ctx, userName)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	return cc.cs.DeleteCollection(ctx, id, user.ID)
}

func (cc *CollectionComponent) AddReposToCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	// find by user name
	user, err := cc.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	collection, err := cc.cs.GetCollection(ctx, req.ID)
	if err != nil {
		return err
	}
	if collection.UserID != user.ID {
		return fmt.Errorf("no permission to operate this collection: %s", strconv.FormatInt(req.ID, 10))
	}
	var collectionRepos []database.CollectionRepository
	for _, id := range req.RepoIDs {
		collectionRepos = append(collectionRepos, database.CollectionRepository{
			CollectionID: req.ID,
			RepositoryID: id,
		})
	}
	return cc.cs.AddCollectionRepos(ctx, collectionRepos)
}

func (cc *CollectionComponent) RemoveReposFromCollection(ctx context.Context, req types.UpdateCollectionReposReq) error {
	// find by user name
	user, err := cc.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return fmt.Errorf("cannot find user for collection, %w", err)
	}
	collection, err := cc.cs.GetCollection(ctx, req.ID)
	if err != nil {
		return err
	}
	if collection.UserID != user.ID {
		return fmt.Errorf("no permission to operate this collection: %s", strconv.FormatInt(req.ID, 10))
	}
	var collectionRepos []database.CollectionRepository
	for _, id := range req.RepoIDs {
		collectionRepos = append(collectionRepos, database.CollectionRepository{
			CollectionID: req.ID,
			RepositoryID: id,
		})
	}
	return cc.cs.RemoveCollectionRepos(ctx, collectionRepos)
}

func (cc *CollectionComponent) getUserCollectionPermission(ctx context.Context, userName string, collection *database.Collection) (*types.UserRepoPermission, error) {
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
