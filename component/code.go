package component

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

const codeGitattributesContent = modelGitattributesContent

func NewCodeComponent(config *config.Config) (*CodeComponent, error) {
	c := &CodeComponent{}
	var err error
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	c.cs = database.NewCodeStore()
	return c, nil
}

type CodeComponent struct {
	*RepoComponent
	cs *database.CodeStore
}

func (c *CodeComponent) Create(ctx context.Context, req *types.CreateCodeReq) (*types.Code, error) {
	var (
		nickname string
		tags     []types.RepoTag
	)

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	req.RepoType = types.CodeRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname
	_, dbRepo, err := c.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbCode := database.Code{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	code, err := c.cs.Create(ctx, dbCode)
	if err != nil {
		return nil, fmt.Errorf("failed to create database code, cause: %w", err)
	}

	// Create README.md file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   req.Readme,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  readmeFileName,
	}, types.CodeRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create README.md file, cause: %w", err)
	}

	// Create .gitattributes file
	err = c.git.CreateRepoFile(buildCreateFileReq(&types.CreateFileParams{
		Username:  dbRepo.User.Username,
		Email:     dbRepo.User.Email,
		Message:   initCommitMessage,
		Branch:    req.DefaultBranch,
		Content:   codeGitattributesContent,
		NewBranch: req.DefaultBranch,
		Namespace: req.Namespace,
		Name:      req.Name,
		FilePath:  gitattributesFileName,
	}, types.CodeRepo))
	if err != nil {
		return nil, fmt.Errorf("failed to create .gitattributes file, cause: %w", err)
	}

	for _, tag := range code.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	resCode := &types.Code{
		ID:           code.ID,
		Name:         code.Repository.Name,
		Nickname:     code.Repository.Nickname,
		Description:  code.Repository.Description,
		Likes:        code.Repository.Likes,
		Downloads:    code.Repository.DownloadCount,
		Path:         code.Repository.Path,
		RepositoryID: code.RepositoryID,
		Repository: types.Repository{
			HTTPCloneURL: code.Repository.HTTPCloneURL,
			SSHCloneURL:  code.Repository.SSHCloneURL,
		},
		Private: code.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.Name,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: code.CreatedAt,
		UpdatedAt: code.UpdatedAt,
	}

	return resCode, nil
}

func (c *CodeComponent) Index(ctx context.Context, username, search, sort string, tags []database.TagReq, per, page int) ([]types.Code, int, error) {
	var (
		user     database.User
		err      error
		resCodes []types.Code
	)
	if username == "" {
		slog.Info("get codes without current username")
	} else {
		user, err = c.user.FindByUsername(ctx, username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			slog.Error(newError.Error())
			return nil, 0, newError
		}
	}
	codes, total, err := c.cs.PublicToUser(ctx, &user, search, sort, tags, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public codes,error:%w", err)
		slog.Error(newError.Error())
		return nil, 0, newError
	}

	for _, data := range codes {
		resCodes = append(resCodes, types.Code{
			ID:           data.ID,
			Name:         data.Repository.Name,
			Nickname:     data.Repository.Nickname,
			Description:  data.Repository.Description,
			Likes:        data.Repository.Likes,
			Downloads:    data.Repository.DownloadCount,
			Path:         data.Repository.Path,
			RepositoryID: data.RepositoryID,
			Private:      data.Repository.Private,
			CreatedAt:    data.CreatedAt,
			UpdatedAt:    data.UpdatedAt,
		})
	}

	return resCodes, total, nil
}

func (c *CodeComponent) Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error) {
	req.RepoType = types.CodeRepo
	dbRepo, err := c.UpdateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	code, err := c.cs.FindByPath(ctx, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code, error: %w", err)
	}

	err = c.cs.Update(ctx, *code)
	if err != nil {
		return nil, fmt.Errorf("failed to update database code, error: %w", err)
	}

	resCode := &types.Code{
		ID:           code.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    code.CreatedAt,
		UpdatedAt:    code.UpdatedAt,
	}

	return resCode, nil
}

func (c *CodeComponent) Delete(ctx context.Context, namespace, name, currentUser string) error {
	code, err := c.cs.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find code, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.CodeRepo,
	}
	_, err = c.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of code, error: %w", err)
	}

	err = c.cs.Delete(ctx, *code)
	if err != nil {
		return fmt.Errorf("failed to delete database code, error: %w", err)
	}
	return nil
}

func (c *CodeComponent) Show(ctx context.Context, namespace, name, currentUser string) (*types.Code, error) {
	var tags []types.RepoTag
	code, err := c.cs.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code, error: %w", err)
	}

	if code.Repository.Private {
		if code.Repository.User.Username != currentUser {
			return nil, fmt.Errorf("failed to find code, error: %w", errors.New("the private code is not accessible to the current user"))
		}
	}

	for _, tag := range code.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.ShowName,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	resCode := &types.Code{
		ID:            code.ID,
		Name:          code.Repository.Name,
		Nickname:      code.Repository.Nickname,
		Description:   code.Repository.Description,
		Likes:         code.Repository.Likes,
		Downloads:     code.Repository.DownloadCount,
		Path:          code.Repository.Path,
		RepositoryID:  code.Repository.ID,
		DefaultBranch: code.Repository.DefaultBranch,
		Repository: types.Repository{
			HTTPCloneURL: code.Repository.HTTPCloneURL,
			SSHCloneURL:  code.Repository.SSHCloneURL,
		},
		Tags: tags,
		User: types.User{
			Username: code.Repository.User.Username,
			Nickname: code.Repository.User.Name,
			Email:    code.Repository.User.Email,
		},
		Private:   code.Repository.Private,
		CreatedAt: code.CreatedAt,
		UpdatedAt: code.UpdatedAt,
	}

	return resCode, nil
}

func (c *CodeComponent) Relations(ctx context.Context, namespace, name, current_user string) (*types.Relations, error) {
	model, err := c.cs.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	if model.Repository.Private {
		if model.Repository.User.Username != current_user {
			return nil, ErrUnauthorized
		}
	}

	return c.getRelations(ctx, model.RepositoryID, current_user)
}

func (c *CodeComponent) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.relatedRepos(ctx, repoID, currentUser)
	if err != nil {
		return nil, err
	}
	rels := new(types.Relations)
	modelRepos := res["model"]
	for _, repo := range modelRepos {
		rels.Models = append(rels.Models, &types.Model{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
		})
	}

	return rels, nil
}
