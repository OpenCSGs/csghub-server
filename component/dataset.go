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
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	//TODO:check sensitive content of file
	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		return c.createReadmeFile(ctx, req)
	} else {
		return c.createLibraryFile(ctx, req)
	}
}

func (c *DatasetComponent) createReadmeFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var err error
	var repoTags []*database.RepositoryTag
	repoTags, err = c.updateMetaTags(ctx, req.NameSpace, req.Name, req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}

	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create dataset file, cause: %w", err)
	}

	respTags := make([]types.CreateFileResp_Tag, 0, len(repoTags))
	for _, tag := range repoTags {
		respTags = append(respTags, types.CreateFileResp_Tag{
			Name: tag.Tag.Name, Category: tag.Tag.Category,
			Scope: string(tag.Tag.Scope), Group: tag.Tag.Group,
		})
	}
	resp := &types.CreateFileResp{
		Tags: respTags,
	}

	return resp, err
}

func (c *DatasetComponent) createLibraryFile(ctx context.Context, req *types.CreateFileReq) (*types.CreateFileResp, error) {
	var err error
	resp := &types.CreateFileResp{}

	newLibTagName := tagparser.LibraryTag(req.FilePath)
	//TODO:load from cache
	allTags, err := c.ts.AllDatasetTags(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all tags, error: %w", err)
	}
	var newLibTag *database.Tag
	for _, t := range allTags {
		if t.Name == newLibTagName {
			newLibTag = t
			break
		}
	}
	err = c.ds.SetLibraryTag(ctx, req.NameSpace, req.Name, nil, newLibTag)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", req.NameSpace),
			slog.String("name", req.Name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}
	//TODO:reload all tags of dataset ?
	respTags := make([]types.CreateFileResp_Tag, 0)
	respTags = append(respTags, types.CreateFileResp_Tag{
		Name: newLibTag.Name, Category: newLibTag.Category,
		Scope: string(newLibTag.Scope), Group: newLibTag.Group,
	})
	resp.Tags = respTags

	err = c.gs.CreateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	return resp, err
}
func (c *DatasetComponent) UpdateFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("update file get request", slog.String("namespace", req.NameSpace), slog.String("filePath", req.FilePath),
		slog.String("origin_path", req.OriginPath))

	var err error
	_, err = c.ns.FindByPath(ctx, req.NameSpace)
	if err != nil {
		return nil, fmt.Errorf("fail to check namespace, cause: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("fail to check user, cause: %w", err)
	}
	//TODO:check sensitive content of file

	fileName := filepath.Base(req.FilePath)
	if fileName == "README.md" {
		slog.Debug("file is readme", slog.String("content", req.Content))
		return c.updateReadmeFile(ctx, req)
	} else {
		slog.Debug("file is not readme", slog.String("filePath", req.FilePath), slog.String("originPath", req.OriginPath))
		return c.updateLibraryFile(ctx, req)
	}
}

func (c *DatasetComponent) updateLibraryFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	var err error
	resp := &types.UpdateFileResp{}

	isFileRenamed := req.FilePath != req.OriginPath
	//need to handle tag change only if file renamed
	if isFileRenamed {
		oldLibTagName := tagparser.LibraryTag(req.OriginPath)
		newLibTagName := tagparser.LibraryTag(req.FilePath)
		if newLibTagName != oldLibTagName {
			//TODO:load from cache
			allTags, err := c.ts.AllDatasetTags(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get all tags, error: %w", err)
			}
			var oldLibTag, newLibTag *database.Tag
			for _, t := range allTags {
				if t.Name == oldLibTagName {
					oldLibTag = t
				} else if t.Name == newLibTagName {
					newLibTag = t
				}
			}
			err = c.ds.SetLibraryTag(ctx, req.NameSpace, req.Name, oldLibTag, newLibTag)
			if err != nil {
				slog.Error("failed to set dataset's tags", slog.String("namespace", req.NameSpace),
					slog.String("name", req.Name), slog.Any("error", err))
				return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
			}
			//TODO:reload all tags of dataset ?
			respTags := make([]types.CreateFileResp_Tag, 0)
			respTags = append(respTags, types.CreateFileResp_Tag{
				Name: newLibTag.Name, Category: newLibTag.Category,
				Scope: string(newLibTag.Scope), Group: newLibTag.Group,
			})
			resp.Tags = respTags
		}
	}

	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, err
	}

	return resp, err
}

func (c *DatasetComponent) updateMetaTags(ctx context.Context, namespace, name, content string) ([]*database.RepositoryTag, error) {
	fileCategoryTagMap, err := tagparser.MetaTags(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse metadata, cause: %w", err)
	}
	slog.Debug("File tags parsed", slog.Any("tags", fileCategoryTagMap))

	var predefinedTags []*database.Tag
	//TODO:load from cache
	predefinedTags, err = c.ts.AllDatasetTags(ctx)
	if err != nil {
		slog.Error("Failed to get predefined tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get predefined tags, cause: %w", err)
	}

	var metaTags []*database.Tag
	metaTags, err = c.prepareMetaTags(ctx, predefinedTags, fileCategoryTagMap)
	if err != nil {
		slog.Error("Failed to process tags", slog.Any("error", err))
		return nil, fmt.Errorf("failed to process tags, cause: %w", err)
	}
	categories, err := c.ts.AllDatasetCategories(ctx)
	if err != nil {
		slog.Error("Failed to get categories", slog.Any("error", err))
		return nil, fmt.Errorf("failed to get categories, cause: %w", err)
	}
	var libCategory database.TagCategory
	for _, c := range categories {
		if c.Name == "library" {
			libCategory = c
			break
		}
	}
	var repoTags []*database.RepositoryTag
	repoTags, err = c.ds.SetMetaTags(ctx, namespace, name, metaTags, libCategory)
	if err != nil {
		slog.Error("failed to set dataset's tags", slog.String("namespace", namespace),
			slog.String("name", name), slog.Any("error", err))
		return nil, fmt.Errorf("failed to set dataset's tags, cause: %w", err)
	}

	return repoTags, nil
}

func (c *DatasetComponent) updateReadmeFile(ctx context.Context, req *types.UpdateFileReq) (*types.UpdateFileResp, error) {
	slog.Debug("file is readme", slog.String("content", req.Content))
	var err error
	var repoTags []*database.RepositoryTag
	repoTags, err = c.updateMetaTags(ctx, req.NameSpace, req.Name, req.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to update meta tags, cause: %w", err)
	}
	err = c.gs.UpdateDatasetFile(req)
	if err != nil {
		return nil, fmt.Errorf("failed to update dataset file, cause: %w", err)
	}

	//TODO:reload all tags of dataset ?
	respTags := make([]types.CreateFileResp_Tag, 0, len(repoTags))
	for _, tag := range repoTags {
		respTags = append(respTags, types.CreateFileResp_Tag{
			Name: tag.Tag.Name, Category: tag.Tag.Category,
			Scope: string(tag.Tag.Scope), Group: tag.Tag.Group,
		})
	}
	resp := &types.UpdateFileResp{
		Tags: respTags,
	}

	return resp, err
}

func (c *DatasetComponent) prepareMetaTags(ctx context.Context, predefinedTags []*database.Tag, categoryTagMap map[string][]string) ([]*database.Tag, error) {
	var err error
	var tagsNeed []*database.Tag
	if len(categoryTagMap) == 0 {
		slog.Debug("No category tags to compare with predefined tags")
		return tagsNeed, nil
	}

	/*Rules for meta tags here:
	- if any tag is found in the predefined tags, accept it
	- if any tag is not found in the predefined tags but of category "Tasks", add it to "Other" category
	- if any tag is not found in the predefined tags and not of category "Tasks", ignore it
	*/
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
				//all unkown tags of category "Tasks" belongs to category "Other" and will be created later
				if strings.EqualFold(category, "Tasks") {
					continue
				}
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

	err = c.ts.SaveTags(ctx, tagsToCreate)
	if err != nil {
		return nil, err
	}

	return append(tagsNeed, tagsToCreate...), nil

}

func (c *DatasetComponent) Create(ctx context.Context, req *types.CreateDatasetReq) (dataset *database.Dataset, err error) {
	_, err = c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, errors.New("namespace does not exist")
	}

	user, err := c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, errors.New("user does not exist")
	}

	dataset, repo, err := c.gs.CreateDatasetRepo(req)
	if err != nil {
		return nil, fmt.Errorf("failed to create git dataset repository, cause: %w", err)
	}

	dataset, err = c.ds.Create(ctx, dataset, repo, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create database dataset, cause: %w", err)
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

func (c *DatasetComponent) Index(ctx context.Context, search, sort, tag string, per, page int) ([]database.Dataset, int, error) {
	datasets, total, err := c.ds.Public(ctx, search, sort, tag, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public datasets,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}
	return datasets, total, nil
}

func (c *DatasetComponent) Update(ctx context.Context, req *types.UpdateDatasetReq) (*database.Dataset, error) {
	_, err := c.ns.FindByPath(ctx, req.Namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to find namespace, error: %w", err)
	}

	_, err = c.us.FindByUsername(ctx, req.Username)
	if err != nil {
		return nil, fmt.Errorf("failed to find user, error: %w", err)
	}

	dataset, err := c.ds.FindyByPath(ctx, req.Namespace, req.OriginName)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	err = c.gs.UpdateDatasetRepo(req.Namespace, req.OriginName, dataset, dataset.Repository, req)
	if err != nil {
		return nil, fmt.Errorf("failed to update git dataset repository, error: %w", err)
	}

	err = c.ds.Update(ctx, dataset, dataset.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to update database dataset, error: %w", err)
	}

	return dataset, nil
}

func (c *DatasetComponent) Delete(ctx context.Context, namespace, name string) error {
	_, err := c.ds.FindyByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find dataset, error: %w", err)
	}
	err = c.gs.DeleteDatasetRepo(namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete git dataset repository, error: %w", err)
	}

	err = c.ds.Delete(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to delete database dataset, error: %w", err)
	}
	return nil
}

func (c *DatasetComponent) Detail(ctx context.Context, namespace, name string) (*types.DatasetDetail, error) {
	_, err := c.ds.FindyByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	detail, err := c.gs.GetDatasetDetail(namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset detail, error: %w", err)
	}

	return detail, nil
}

func (c *DatasetComponent) Commits(ctx context.Context, req *types.GetCommitsReq) ([]*types.Commit, error) {
	dataset, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	commits, err := c.gs.GetDatasetCommits(req.Namespace, req.Name, req.Ref, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository commits, error: %w", err)
	}
	return commits, nil
}

func (c *DatasetComponent) LastCommit(ctx context.Context, req *types.GetCommitsReq) (*types.Commit, error) {
	dataset, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}

	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	commit, err := c.gs.GetDatasetLastCommit(req.Namespace, req.Name, req.Ref)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository last commit, error: %w", err)
	}
	return commit, nil
}

func (c *DatasetComponent) FileRaw(ctx context.Context, req *types.GetFileReq) (string, error) {
	dataset, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return "", fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	raw, err := c.gs.GetDatasetFileRaw(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return "", fmt.Errorf("failed to get git dataset repository file raw, error: %w", err)
	}
	return raw, nil
}

func (c *DatasetComponent) Branches(ctx context.Context, req *types.GetBranchesReq) ([]*types.DatasetBranch, error) {
	_, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	bs, err := c.gs.GetDatasetBranches(req.Namespace, req.Name, req.Per, req.Page)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository branches, error: %w", err)
	}
	return bs, nil
}

func (c *DatasetComponent) Tags(ctx context.Context, req *types.GetTagsReq) ([]database.Tag, error) {
	_, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	tags, err := c.ds.Tags(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to get dataset tags, error: %w", err)
	}
	return tags, nil
}

func (c *DatasetComponent) Tree(ctx context.Context, req *types.GetFileReq) ([]*types.File, error) {
	dataset, err := c.ds.FindyByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find dataset, error: %w", err)
	}
	if req.Ref == "" {
		req.Ref = dataset.Repository.DefaultBranch
	}
	tree, err := c.gs.GetDatasetFileTree(req.Namespace, req.Name, req.Ref, req.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to get git dataset repository file tree, error: %w", err)
	}
	return tree, nil
}
