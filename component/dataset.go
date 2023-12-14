package component

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"slices"
	"strings"

	"opencsg.com/starhub-server/builder/gitserver"
	"opencsg.com/starhub-server/builder/store/database"
	"opencsg.com/starhub-server/common/config"
	"opencsg.com/starhub-server/common/types"
	"opencsg.com/starhub-server/component/tagparser"
)

const datasetGitattributesContent = `*.7z filter=lfs diff=lfs merge=lfs -text
*.arrow filter=lfs diff=lfs merge=lfs -text
*.bin filter=lfs diff=lfs merge=lfs -text
*.bz2 filter=lfs diff=lfs merge=lfs -text
*.ckpt filter=lfs diff=lfs merge=lfs -text
*.ftz filter=lfs diff=lfs merge=lfs -text
*.gz filter=lfs diff=lfs merge=lfs -text
*.h5 filter=lfs diff=lfs merge=lfs -text
*.joblib filter=lfs diff=lfs merge=lfs -text
*.lfs.* filter=lfs diff=lfs merge=lfs -text
*.lz4 filter=lfs diff=lfs merge=lfs -text
*.mlmodel filter=lfs diff=lfs merge=lfs -text
*.model filter=lfs diff=lfs merge=lfs -text
*.msgpack filter=lfs diff=lfs merge=lfs -text
*.npy filter=lfs diff=lfs merge=lfs -text
*.npz filter=lfs diff=lfs merge=lfs -text
*.onnx filter=lfs diff=lfs merge=lfs -text
*.ot filter=lfs diff=lfs merge=lfs -text
*.parquet filter=lfs diff=lfs merge=lfs -text
*.pb filter=lfs diff=lfs merge=lfs -text
*.pickle filter=lfs diff=lfs merge=lfs -text
*.pkl filter=lfs diff=lfs merge=lfs -text
*.pt filter=lfs diff=lfs merge=lfs -text
*.pth filter=lfs diff=lfs merge=lfs -text
*.rar filter=lfs diff=lfs merge=lfs -text
*.safetensors filter=lfs diff=lfs merge=lfs -text
saved_model/**/* filter=lfs diff=lfs merge=lfs -text
*.tar.* filter=lfs diff=lfs merge=lfs -text
*.tar filter=lfs diff=lfs merge=lfs -text
*.tflite filter=lfs diff=lfs merge=lfs -text
*.tgz filter=lfs diff=lfs merge=lfs -text
*.wasm filter=lfs diff=lfs merge=lfs -text
*.xz filter=lfs diff=lfs merge=lfs -text
*.zip filter=lfs diff=lfs merge=lfs -text
*.zst filter=lfs diff=lfs merge=lfs -text
*tfevents* filter=lfs diff=lfs merge=lfs -text
# Audio files - uncompressed
*.pcm filter=lfs diff=lfs merge=lfs -text
*.sam filter=lfs diff=lfs merge=lfs -text
*.raw filter=lfs diff=lfs merge=lfs -text
# Audio files - compressed
*.aac filter=lfs diff=lfs merge=lfs -text
*.flac filter=lfs diff=lfs merge=lfs -text
*.mp3 filter=lfs diff=lfs merge=lfs -text
*.ogg filter=lfs diff=lfs merge=lfs -text
*.wav filter=lfs diff=lfs merge=lfs -text
# Image files - uncompressed
*.bmp filter=lfs diff=lfs merge=lfs -text
*.gif filter=lfs diff=lfs merge=lfs -text
*.png filter=lfs diff=lfs merge=lfs -text
*.tiff filter=lfs diff=lfs merge=lfs -text
# Image files - compressed
*.jpg filter=lfs diff=lfs merge=lfs -text
*.jpeg filter=lfs diff=lfs merge=lfs -text
*.webp filter=lfs diff=lfs merge=lfs -text

`

const (
	initCommitMessage = "initial commit"
)

func NewDatasetComponent(config *config.Config) (*DatasetComponent, error) {
	c := &DatasetComponent{}
	c.ds = database.NewDatasetStore()
	c.ns = database.NewNamespaceStore()
	c.us = database.NewUserStore()
	c.ts = database.NewTagStore()
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
	ds *database.DatasetStore
	ns *database.NamespaceStore
	us *database.UserStore
	ts *database.TagStore
	gs gitserver.GitServer
}

func (c *DatasetComponent) CreateFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	slog.Debug("creating file get request", slog.String("namespace", req.NameSpace), slog.String("filepath", req.FilePath))
	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}
	//TODO:check sensitive content of file

	fileCategoryTagMap := make(map[string][]string)
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		fileCategoryTagMap, err = tagparser.MetaTags(req.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to parse metadata, error: %w", err)
		}
	}
	libTag := tagparser.LibraryTag(fileName)
	if libTag != "" {
		fileCategoryTagMap["Library"] = append(fileCategoryTagMap["Library"], libTag)
	}
	slog.Debug("File tags parsed", slog.Any("tags", fileCategoryTagMap))

	//compare with system predefined categories and tags
	var predefinedTags []*database.Tag
	//TODO:load from cache
	predefinedTags, err = c.ts.AllDatasetTags(ctx)
	if err != nil {
		slog.Error("Failed to get predefined tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get predefined tags, error: %w", err)
	}
	var tags []*database.Tag
	tags, err = c.prepareTags(ctx, predefinedTags, fileCategoryTagMap)
	if err != nil {
		slog.Error("Failed to process tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to process tags, error: %w", err)
	}
	var repoTags []*database.RepositoryTag
	repoTags, err = c.ds.SetTags(ctx, req.NameSpace, req.Name, tags)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	respTags := make([]types.CreateFileResp_Tag, 0, len(repoTags))
	for _, tag := range repoTags {
		respTags = append(respTags, types.CreateFileResp_Tag{Name: tag.Tag.Name, Category: tag.Tag.Category})
	}
	resp := &types.CreateFileResp{
		Tags: respTags,
	}

	return resp, err
}

func (c *DatasetComponent) prepareTags(ctx context.Context, predefinedTags []*database.Tag, categoryTagMap map[string][]string) ([]*database.Tag, error) {
	var tagsNeed []*database.Tag
	if len(categoryTagMap) == 0 {
		slog.Debug("No category tags to compare with predefined tags")
		return tagsNeed, nil
	}

	var tagsToCreate []*database.Tag
	for category, tagNames := range categoryTagMap {
		for _, tagName := range tagNames {
			//is predefined tag, or "Other" tag created before
			if !slices.ContainsFunc(predefinedTags, func(t *database.Tag) bool {
				match := strings.EqualFold(t.Name, tagName) && (strings.EqualFold(t.Category, category) ||
					strings.EqualFold(t.Category, "Other"))

				if match {
					tagsNeed = append(tagsNeed, t)
				}
				return match
			}) {
				//all unkown tags belongs to category "Other" and will be created later
				category = "Other"
				tagsToCreate = append(tagsToCreate, &database.Tag{
					Category: category,
					Name:     tagName,
					Scope:    database.DatasetTagScope,
				})
			}
		}
	}
	//remove duplicated tag info, make sure the same tag will be created once
	tagsToCreate = slices.CompactFunc(tagsToCreate, func(t1, t2 *database.Tag) bool {
		return t1.Name == t2.Name && t1.Category == t2.Category
	})

	if len(tagsToCreate) == 0 {
		return tagsNeed, nil
	}

	err := c.ts.SaveTags(ctx, tagsToCreate)
	if err != nil {
		return nil, err
	}

	return append(tagsNeed, tagsToCreate...), nil

}

func (c *DatasetComponent) Create(ctx context.Context, req *types.CreateDatasetReq) (dataset *database.Dataset, err error) {
	_, err = c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("Namespace does not exist")
	}

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("User does not exist")
	}

	dataset, repo, err := c.gs.CreateDatasetRepo(req)
	if err == nil {
		err = c.ds.Create(ctx, dataset, repo, user.ID)
		if err != nil {
			return
		}
	}

	err = c.gs.CreateDatasetFile(createGitattributesReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(createReadmeReq(req, user))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	return
}

func createGitattributesReq(r *types.CreateDatasetReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(datasetGitattributesContent)),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  ".gitattributes",
	}
}

func createReadmeReq(r *types.CreateDatasetReq, user database.User) *types.CreateFileReq {
	return &types.CreateFileReq{
		Username:  user.Username,
		Email:     user.Email,
		Message:   initCommitMessage,
		Branch:    r.DefaultBranch,
		Content:   base64.StdEncoding.EncodeToString([]byte(generateReadmeData(r.License))),
		NewBranch: r.DefaultBranch,
		NameSpace: r.Namespace,
		Name:      r.Name,
		FilePath:  "README.md",
	}
}

func generateReadmeData(license string) string {
	return `
---
license: ` + license + `
---
	`
}
