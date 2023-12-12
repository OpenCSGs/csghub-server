package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component/tagparser"
)

func NewDatasetComponent(config *config.Config) (*DatasetComponent, error) {
	c := &DatasetComponent{}
	c.ns = database.NewNamespaceStore()
	c.us = database.NewUserStore()
	var err error
	c.gs, err = gitserver.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return c, nil
}

type DatasetComponent struct {
	// ds *database.DatasetStore
	ns *database.NamespaceStore
	us *database.UserStore
	gs gitserver.GitServer
}

func (c *DatasetComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) error {
	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return errors.New("namespace does not exist")
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return errors.New("user does not exist")
	}
	//TODO:check sensitive content of file

	categoryTagMap := make(map[string][]string)
	if req.Name == "README.md" {
		categoryTagMap, err = tagparser.MetaTags(req.Content)
		if err != nil {
			return fmt.Errorf("failed to parse metadata, error: %w", err)
		}
	}
	libTag := tagparser.LibraryTag(req.Name)
	if libTag != "" {
		categoryTagMap["Library"] = append(categoryTagMap["Library"], libTag)
	}
	slog.Debug("File tags parsed", slog.Any("tags", categoryTagMap))
	// for category, tagNames := range categoryTagMap {
	// 	for _, tagName := range tagNames {
	// 		tags = append(tags, database.Tag{Name: tagName, Category: category})
	// 	}
	// }
	//TODO:compare with system predefined categories and tags

	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return err
	}

	return err
}
