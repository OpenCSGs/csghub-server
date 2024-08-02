package component

import (
	"context"
	"fmt"

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
	c.rs = database.NewRepoStore()
	return c, nil
}

type CodeComponent struct {
	*RepoComponent
	cs *database.CodeStore
	rs *database.RepoStore
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
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: code.CreatedAt,
		UpdatedAt: code.UpdatedAt,
	}

	return resCode, nil
}

func (c *CodeComponent) Index(ctx context.Context, filter *types.RepoFilter, per, page int) ([]types.Code, int, error) {
	var (
		user     database.User
		err      error
		resCodes []types.Code
	)
	if filter.Username != "" {
		user, err = c.user.FindByUsername(ctx, filter.Username)
		if err != nil {
			newError := fmt.Errorf("failed to get current user,error:%w", err)
			return nil, 0, newError
		}
	}
	repos, total, err := c.rs.PublicToUser(ctx, types.CodeRepo, user.ID, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public code repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	codes, err := c.cs.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get codes by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var code *database.Code
		for _, c := range codes {
			if c.RepositoryID == repo.ID {
				code = &c
				code.Repository = repo
				break
			}
		}
		var tags []types.RepoTag
		for _, tag := range repo.Tags {
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
		resCodes = append(resCodes, types.Code{
			ID:           code.ID,
			Name:         repo.Name,
			Nickname:     repo.Nickname,
			Description:  repo.Description,
			Likes:        repo.Likes,
			Downloads:    repo.DownloadCount,
			Path:         repo.Path,
			RepositoryID: repo.ID,
			Private:      repo.Private,
			CreatedAt:    code.CreatedAt,
			UpdatedAt:    repo.UpdatedAt,
			Tags:         tags,
			Source:       repo.Source,
			SyncStatus:   repo.SyncStatus,
		})
	}

	return resCodes, total, nil
}

func (c *CodeComponent) Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error) {
	req.RepoType = types.CodeRepo
	dbRepo, err := c.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	code, err := c.cs.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	//update times of code
	err = c.cs.Update(ctx, *code)
	if err != nil {
		return nil, fmt.Errorf("failed to update database code repo, error: %w", err)
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

	permission, err := c.getUserRepoPermission(ctx, currentUser, code.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	ns, err := c.getNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for code, error: %w", err)
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

	likeExists, err := c.uls.IsExist(ctx, currentUser, code.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
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
			Nickname: code.Repository.User.NickName,
			Email:    code.Repository.User.Email,
		},
		Private:    code.Repository.Private,
		CreatedAt:  code.CreatedAt,
		UpdatedAt:  code.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     code.Repository.Source,
		SyncStatus: code.Repository.SyncStatus,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
	}

	return resCode, nil
}

func (c *CodeComponent) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	code, err := c.cs.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	allow, _ := c.AllowReadAccessRepo(ctx, code.Repository, currentUser)
	if !allow {
		return nil, ErrUnauthorized
	}

	return c.getRelations(ctx, code.RepositoryID, currentUser)
}

func (c *CodeComponent) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.relatedRepos(ctx, repoID, currentUser)
	if err != nil {
		return nil, err
	}
	rels := new(types.Relations)
	modelRepos := res[types.ModelRepo]
	for _, repo := range modelRepos {
		rels.Models = append(rels.Models, &types.Model{
			Path:        repo.Path,
			Name:        repo.Name,
			Nickname:    repo.Nickname,
			Description: repo.Description,
			UpdatedAt:   repo.UpdatedAt,
			Private:     repo.Private,
			Downloads:   repo.DownloadCount,
		})
	}

	return rels, nil
}
