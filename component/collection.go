package component

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func NewCollectionComponent() (*CollectionComponent, error) {
	cc := &CollectionComponent{}
	cc.cs = database.NewCollectionStore()
	cc.rs = database.NewRepoStore()
	cc.us = database.NewUserStore()
	return cc, nil
}

type CollectionComponent struct {
	cs *database.CollectionStore
	rs *database.RepoStore
	us *database.UserStore
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
		UserID:      user.ID,
		Name:        input.Name,
		Nickname:    input.Nickname,
		Description: input.Description,
		Private:     input.Private,
		Theme:       input.Theme,
	}
	return cc.cs.CreateCollection(ctx, collection)
}

func (cc *CollectionComponent) GetCollection(ctx context.Context, id int64) (*types.Collection, error) {
	collection, err := cc.cs.GetCollection(ctx, id)
	if err != nil {
		return nil, err
	}
	var newCollection types.Collection
	temporaryVariable, _ := json.Marshal(collection)
	err = json.Unmarshal(temporaryVariable, &newCollection)
	if err != nil {
		return nil, err
	}
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
