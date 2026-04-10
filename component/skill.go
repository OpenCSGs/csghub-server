package component

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/git/membership"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
)

const skillGitattributesContent = modelGitattributesContent

type SkillComponent interface {
	Create(ctx context.Context, req *types.CreateSkillReq) (*types.Skill, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Skill, int, error)
	Update(ctx context.Context, req *types.UpdateSkillReq) (*types.Skill, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Skill, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgSkills(ctx context.Context, req *types.OrgSkillsReq) ([]types.Skill, int, error)
	GetUploadUrl(ctx context.Context) (string, string, map[string]string, error)
}

func NewSkillComponent(config *config.Config) (SkillComponent, error) {
	c := &skillComponentImpl{}
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	c.skillStore = database.NewSkillStore()
	c.repoStore = database.NewRepoStore()
	c.recomStore = database.NewRecomStore()
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create git server, error: %w", err)
	}
	c.gitServer = gs
	c.config = config
	c.userLikesStore = database.NewUserLikesStore()
	c.userSvcClient = rpc.NewUserSvcHttpClient(fmt.Sprintf("%s:%d", config.User.Host, config.User.Port),
		rpc.AuthWithApiKey(config.APIToken))
	s3Client, err := s3.NewMinio(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create s3 client, error: %w", err)
	}
	c.s3Client = s3Client

	return c, nil
}

type skillComponentImpl struct {
	config         *config.Config
	repoComponent  RepoComponent
	skillStore     database.SkillStore
	repoStore      database.RepoStore
	userLikesStore database.UserLikesStore
	gitServer      gitserver.GitServer
	userSvcClient  rpc.UserSvcClient
	recomStore     database.RecomStore
	s3Client       s3.Client
}

func (c *skillComponentImpl) GetUploadUrl(ctx context.Context) (string, string, map[string]string, error) {
	// Generate UUID
	uuid := uuid.New().String()

	// Build object key
	objectKey := fmt.Sprintf("skills/packages/%s", uuid)

	// Create a new post policy
	expires := time.Now().Add(24 * time.Hour)
	policy := minio.NewPostPolicy()
	err := policy.SetBucket(c.config.S3.Bucket)
	if err != nil {
		slog.WarnContext(ctx, "skill set bucket failed", slog.String("error", err.Error()))
	}
	err = policy.SetKey(objectKey)
	if err != nil {
		slog.WarnContext(ctx, "skill set key failed", slog.String("error", err.Error()))
	}
	err = policy.SetExpires(expires)
	if err != nil {
		slog.WarnContext(ctx, "skill set expires failed", slog.String("error", err.Error()))
	}

	// Set content length range (1 byte to 10MB)
	err = policy.SetContentLengthRange(1, 10*1024*1024)
	if err != nil {
		slog.WarnContext(ctx, "skill set content length range failed", slog.String("error", err.Error()))
	}

	// Generate presigned POST URL and form data
	url, formData, err := c.s3Client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate presigned post policy: %w", err)
	}

	// Return the upload URL, UUID, and form data
	return url.String(), uuid, formData, nil
}

func (c *skillComponentImpl) Create(ctx context.Context, req *types.CreateSkillReq) (*types.Skill, error) {
	// Setup request with default values
	c.setupCreateRequest(req)

	// Start with README and .gitattributes files
	commitFiles := c.initializeCommitFiles(req)

	// Handle skill package if SHA256 is provided
	if req.SkillPackageSHA256 != "" {
		decompressedFiles, err := c.handleSkillPackage(ctx, req.SkillPackageSHA256)
		if err != nil {
			return nil, err
		}
		// Replace with decompressed files
		commitFiles = decompressedFiles
	}

	// Add any additional commit files from the request
	commitFiles = append(commitFiles, req.CommitFiles...)
	req.CommitFiles = commitFiles

	// Create repository first
	_, dbRepo, commitFilesReq, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	// Create skill record
	skill, err := c.createSkillRecord(ctx, req, dbRepo)
	if err != nil {
		return nil, err
	}

	// Commit files in batches
	if err := c.commitFilesInBatches(ctx, commitFilesReq); err != nil {
		return nil, err
	}

	// Create mirror if Git URL is provided
	if err := c.createMirrorIfNeeded(ctx, req); err != nil {
		return nil, err
	}

	// Build response
	resSkill := c.buildSkillResponse(skill, dbRepo)

	// Send notification
	c.sendCreateNotification(dbRepo, skill.Repository.Path)

	return resSkill, nil
}

// setupCreateRequest sets up the create request with default values
func (c *skillComponentImpl) setupCreateRequest(req *types.CreateSkillReq) {
	// Set nickname if not provided
	if req.Nickname == "" {
		req.Nickname = req.Name
	}

	// Set default branch if not provided
	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	// Set repo type and generate README
	req.RepoType = types.SkillRepo
	req.Readme = generateReadmeData(req.License)
}

// initializeCommitFiles initializes commit files with README, .gitattributes, and SKILL.md
func (c *skillComponentImpl) initializeCommitFiles(req *types.CreateSkillReq) []types.CommitFile {
	skillsContent := fmt.Sprintf(`---
name: %s
description: %s
---`, req.Name, req.Description)
	return []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: skillGitattributesContent,
			Path:    types.GitattributesFileName,
		},
		{
			Content: skillsContent,
			Path:    "SKILL.md",
		},
	}
}

// handleSkillPackage downloads and decompresses the skill package
func (c *skillComponentImpl) handleSkillPackage(ctx context.Context, sha256 string) ([]types.CommitFile, error) {
	// Download file from Minio
	objectKey := common.BuildSkillPackageObjectKey(sha256)
	object, err := c.s3Client.GetObject(ctx, c.config.S3.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download skill package from Minio: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("failed to download skill package: object is nil")
	}
	// Only defer close if object is not nil
	obj := object
	defer obj.Close()

	// Create a buffered reader to detect file format
	bufReader := bufio.NewReader(object)
	// Read first 8 bytes to detect file format
	magicBytes := make([]byte, 8)
	_, err = bufReader.Read(magicBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to read file header: %w", err)
	}

	// Decompress based on content detection (since objectKey has no extension)
	// Try to detect format from content
	if bytes.HasPrefix(magicBytes, []byte{0x50, 0x4B, 0x03, 0x04}) {
		// ZIP format - read entire file into memory
		// Reset reader to start (including the magic bytes we already read)
		r := io.MultiReader(bytes.NewReader(magicBytes), bufReader)
		zipContent, err := io.ReadAll(r)
		if err != nil {
			return nil, fmt.Errorf("failed to read zip file: %w", err)
		}
		return decompressZip(bytes.NewReader(zipContent), int64(len(zipContent)))
	} else if bytes.HasPrefix(magicBytes, []byte{0x1F, 0x8B, 0x08}) {
		// GZIP format (tar.gz) - use streaming decompression
		// Reset reader to start (including the magic bytes we already read)
		r := io.MultiReader(bytes.NewReader(magicBytes), bufReader)
		return decompressTarGz(r)
	} else {
		return nil, fmt.Errorf("unknown file format for skill package")
	}
}

// createSkillRecord creates a skill record in the database
func (c *skillComponentImpl) createSkillRecord(ctx context.Context, req *types.CreateSkillReq, dbRepo *database.Repository) (*database.Skill, error) {
	dbSkill := database.Skill{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	repoPath := path.Join(req.Namespace, req.Name)
	skill, err := c.skillStore.CreateAndUpdateRepoPath(ctx, dbSkill, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database skill, cause: %w", err)
	}

	return skill, nil
}

// commitFilesInBatches commits files in batches to avoid overloading the git server
func (c *skillComponentImpl) commitFilesInBatches(ctx context.Context, commitFilesReq *gitserver.CommitFilesReq) error {
	const batchSize = 50
	files := commitFilesReq.Files
	if len(files) == 0 {
		return nil
	}

	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		batch := files[i:end]
		batchReq := *commitFilesReq
		batchReq.Files = batch
		err := c.gitServer.CommitFiles(ctx, batchReq)
		if err != nil {
			return fmt.Errorf("failed to commit files: %w", err)
		}
	}

	return nil
}

// createMirrorIfNeeded creates a mirror if Git URL is provided
func (c *skillComponentImpl) createMirrorIfNeeded(ctx context.Context, req *types.CreateSkillReq) error {
	if req.GitURL == "" {
		return nil
	}

	// Create mirror task for the existing repo
	gitUrl := req.GitURL
	// Add username and password to git url if provided
	if req.GitUsername != "" && req.GitPassword != "" {
		// Parse the URL to add authentication
		parsedUrl, err := url.Parse(gitUrl)
		if err == nil {
			parsedUrl.User = url.UserPassword(req.GitUsername, req.GitPassword)
			gitUrl = parsedUrl.String()
		}
	}

	mirrorReq := types.CreateMirrorReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		SourceUrl:   gitUrl,
		Username:    req.GitUsername,
		AccessToken: req.GitPassword,
		CurrentUser: req.Username,
		RepoType:    types.SkillRepo,
		Interval:    "24h",
		SyncLfs:     true,
	}

	_, err := c.repoComponent.CreateMirror(ctx, mirrorReq)
	if err != nil {
		return fmt.Errorf("failed to create mirror: %w", err)
	}

	return nil
}

// buildSkillResponse builds the skill response object
func (c *skillComponentImpl) buildSkillResponse(skill *database.Skill, dbRepo *database.Repository) *types.Skill {
	var tags []types.RepoTag
	for _, tag := range skill.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	return &types.Skill{
		ID:           skill.ID,
		Name:         skill.Repository.Name,
		Nickname:     skill.Repository.Nickname,
		Description:  skill.Repository.Description,
		Likes:        skill.Repository.Likes,
		Downloads:    skill.Repository.DownloadCount,
		Path:         skill.Repository.Path,
		RepositoryID: skill.RepositoryID,
		Repository:   common.BuildCloneInfo(c.config, skill.Repository),
		Private:      skill.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: skill.CreatedAt,
		UpdatedAt: skill.UpdatedAt,
	}
}

// sendCreateNotification sends a notification about skill creation
func (c *skillComponentImpl) sendCreateNotification(dbRepo *database.Repository, repoPath string) {
	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SkillRepo,
			RepoPath:  repoPath,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err := c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()
}

func (c *skillComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Skill, int, error) {
	var (
		err       error
		resSkills []*types.Skill
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.SkillRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public skill repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	skills, err := c.skillStore.ByRepoIDs(ctx, repoIDs)
	if err != nil {
		newError := fmt.Errorf("failed to get skills by repo ids,error:%w", err)
		return nil, 0, newError
	}

	//loop through repos to keep the repos in sort order
	for _, repo := range repos {
		var skill *database.Skill
		for _, s := range skills {
			if s.RepositoryID == repo.ID {
				skill = &s
				break
			}
		}
		if skill == nil {
			continue
		}
		var (
			tags             []types.RepoTag
			mirrorTaskStatus types.MirrorTaskStatus
		)
		for _, tag := range repo.Tags {
			tags = append(tags, types.RepoTag{
				Name:      tag.Name,
				Category:  tag.Category,
				Group:     tag.Group,
				BuiltIn:   tag.BuiltIn,
				ShowName:  tag.I18nKey,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		if skill.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = skill.Repository.Mirror.CurrentTask.Status
		}
		resSkills = append(resSkills, &types.Skill{
			ID:               skill.ID,
			Name:             repo.Name,
			Nickname:         repo.Nickname,
			Description:      repo.Description,
			Likes:            repo.Likes,
			Downloads:        repo.DownloadCount,
			Path:             repo.Path,
			RepositoryID:     repo.ID,
			Private:          repo.Private,
			CreatedAt:        skill.CreatedAt,
			UpdatedAt:        repo.UpdatedAt,
			Tags:             tags,
			Source:           repo.Source,
			SyncStatus:       repo.SyncStatus,
			License:          repo.License,
			MirrorTaskStatus: mirrorTaskStatus,
		})
	}
	slog.Info("skill.index")
	if needOpWeight {
		c.addOpWeightToSkills(ctx, repoIDs, resSkills)
	}

	return resSkills, total, nil
}

func (c *skillComponentImpl) Update(ctx context.Context, req *types.UpdateSkillReq) (*types.Skill, error) {
	req.RepoType = types.SkillRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	skill, err := c.skillStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill repo, error: %w", err)
	}

	//update times of skill
	err = c.skillStore.Update(ctx, *skill)
	if err != nil {
		return nil, fmt.Errorf("failed to update database skill repo, error: %w", err)
	}

	resSkill := &types.Skill{
		ID:           skill.ID,
		Name:         dbRepo.Name,
		Nickname:     dbRepo.Nickname,
		Description:  dbRepo.Description,
		Likes:        dbRepo.Likes,
		Downloads:    dbRepo.DownloadCount,
		Path:         dbRepo.Path,
		RepositoryID: dbRepo.ID,
		Private:      dbRepo.Private,
		CreatedAt:    skill.CreatedAt,
		UpdatedAt:    skill.UpdatedAt,
	}

	return resSkill, nil
}

func (c *skillComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find skill, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.SkillRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of skill, error: %w", err)
	}

	err = c.skillStore.Delete(ctx, *skill)
	if err != nil {
		return fmt.Errorf("failed to delete database skill, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.SkillRepo,
			RepoPath:  repo.Path,
			Operation: types.OperationDelete,
			UserUUID:  repo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.ErrorContext(ctx, "failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return nil
}

func (c *skillComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Skill, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
	)
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, skill.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for skill, error: %w", err)
	}

	for _, tag := range skill.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, skill.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus = c.repoComponent.GetMirrorTaskStatus(skill.Repository)

	resSkill := &types.Skill{
		ID:            skill.ID,
		Name:          skill.Repository.Name,
		Nickname:      skill.Repository.Nickname,
		Description:   skill.Repository.Description,
		Likes:         skill.Repository.Likes,
		Downloads:     skill.Repository.DownloadCount,
		Path:          skill.Repository.Path,
		RepositoryID:  skill.Repository.ID,
		DefaultBranch: skill.Repository.DefaultBranch,
		Repository:    common.BuildCloneInfo(c.config, skill.Repository),
		Tags:          tags,
		User: types.User{
			Username: skill.Repository.User.Username,
			Nickname: skill.Repository.User.NickName,
			Email:    skill.Repository.User.Email,
		},
		Private:    skill.Repository.Private,
		CreatedAt:  skill.CreatedAt,
		UpdatedAt:  skill.Repository.UpdatedAt,
		UserLikes:  likeExists,
		Source:     skill.Repository.Source,
		SyncStatus: skill.Repository.SyncStatus,
		License:    skill.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
		MultiSource: types.MultiSource{
			HFPath:  skill.Repository.HFPath,
			MSPath:  skill.Repository.MSPath,
			CSGPath: skill.Repository.CSGPath,
		},
		MirrorTaskStatus: mirrorTaskStatus,
	}
	if permission.CanAdmin {
		resSkill.SensitiveCheckStatus = skill.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		c.addOpWeightToSkills(ctx, []int64{resSkill.RepositoryID}, []*types.Skill{resSkill})
	}

	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToSkill(ctx, resSkill.RepositoryID, resSkill, weightNames)
	}
	return resSkill, nil
}

func (c *skillComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	skill, err := c.skillStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find skill repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, skill.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrForbidden
	}

	return c.getRelations(ctx, skill.RepositoryID, currentUser)
}

func (c *skillComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
	res, err := c.repoComponent.RelatedRepos(ctx, repoID, currentUser)
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

func (c *skillComponentImpl) OrgSkills(ctx context.Context, req *types.OrgSkillsReq) ([]types.Skill, int, error) {
	var resSkills []types.Skill
	var err error
	r := membership.RoleUnknown
	if req.CurrentUser != "" {
		r, err = c.userSvcClient.GetMemberRole(ctx, req.Namespace, req.CurrentUser)
		// log error, and treat user as unknown role in org
		if err != nil {
			slog.ErrorContext(ctx, "faild to get member role",
				slog.String("org", req.Namespace), slog.String("user", req.CurrentUser),
				slog.String("error", err.Error()))
		}
	}
	onlyPublic := !r.CanRead()
	skills, total, err := c.skillStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org skills,error:%w", err)
		slog.ErrorContext(ctx, newError.Error())
		return nil, 0, newError
	}

	for _, data := range skills {
		resSkills = append(resSkills, types.Skill{
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
			UpdatedAt:    data.Repository.UpdatedAt,
		})
	}

	return resSkills, total, nil
}

func (c *skillComponentImpl) addWeightsToSkill(ctx context.Context, repoID int64, resSkill *types.Skill, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resSkill.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resSkill.Scores = append(resSkill.Scores, score)
			}
		}
	}
}

func (c *skillComponentImpl) addOpWeightToSkills(ctx context.Context, repoIDs []int64, resSkills []*types.Skill) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, repoIDs)
	if err != nil {
		return
	}
	weightMap := make(map[int64]map[string]float64)
	for _, weight := range weights {
		if _, ok := weightMap[weight.RepositoryID]; !ok {
			weightMap[weight.RepositoryID] = make(map[string]float64)
		}
		weightMap[weight.RepositoryID][string(weight.WeightName)] = weight.Score
	}

	for _, skill := range resSkills {
		if weight, ok := weightMap[skill.RepositoryID]; ok {
			skill.RecomOpWeight = int(weight["op"])
		}
	}
}

// Constants for decompression limits
const (
	// MaxDecompressedSize is the maximum total size of decompressed files (100MB)
	MaxDecompressedSize = 100 * 1024 * 1024
	// MaxIndividualFileSize is the maximum size of a single decompressed file (50MB)
	MaxIndividualFileSize = 50 * 1024 * 1024
)

// decompressZip decompresses a zip file and returns a list of CommitFile objects
func decompressZip(reader io.ReaderAt, size int64) ([]types.CommitFile, error) {
	zipReader, err := zip.NewReader(reader, size)
	if err != nil {
		return nil, err
	}

	var commitFiles []types.CommitFile
	var totalSize int64

	for _, file := range zipReader.File {
		if file.FileInfo().IsDir() {
			continue
		}

		// Skip .git directory and files
		if strings.Contains(file.Name, "/.git/") || strings.HasPrefix(file.Name, ".git/") || file.Name == ".git" {
			continue
		}

		// Check individual file size
		if file.UncompressedSize64 > uint64(MaxIndividualFileSize) {
			return nil, fmt.Errorf("file too large: %s (size: %d bytes, max: %d bytes)", file.Name, file.UncompressedSize64, MaxIndividualFileSize)
		}

		// Update total size
		totalSize += int64(file.UncompressedSize64)
		if totalSize > MaxDecompressedSize {
			return nil, fmt.Errorf("total decompressed size too large (max: %d bytes)", MaxDecompressedSize)
		}

		// Normalize file path to prevent path traversal attacks
		normalizedPath := filepath.Clean(file.Name)
		// Ensure the path doesn't contain ".." or absolute paths
		if filepath.IsAbs(normalizedPath) || strings.Contains(normalizedPath, "..") {
			return nil, fmt.Errorf("invalid file path: %s", file.Name)
		}

		f, err := file.Open()
		if err != nil {
			return nil, err
		}

		fileContent, err := io.ReadAll(f)
		f.Close()
		if err != nil {
			return nil, err
		}

		commitFiles = append(commitFiles, types.CommitFile{
			Content: string(fileContent),
			Path:    normalizedPath,
		})
	}

	return commitFiles, nil
}

// decompressTarGz decompresses a tar.gz or tgz file and returns a list of CommitFile objects
func decompressTarGz(reader io.Reader) ([]types.CommitFile, error) {
	gzipReader, err := gzip.NewReader(reader)
	if err != nil {
		return nil, err
	}
	defer gzipReader.Close()

	tarReader := tar.NewReader(gzipReader)
	var commitFiles []types.CommitFile
	var totalSize int64

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Typeflag == tar.TypeDir {
			continue
		}

		// Skip .git directory and files
		if strings.Contains(header.Name, "/.git/") || strings.HasPrefix(header.Name, ".git/") || header.Name == ".git" {
			continue
		}

		// Check individual file size
		if header.Size > MaxIndividualFileSize {
			return nil, fmt.Errorf("file too large: %s (size: %d bytes, max: %d bytes)", header.Name, header.Size, MaxIndividualFileSize)
		}

		// Update total size
		totalSize += header.Size
		if totalSize > MaxDecompressedSize {
			return nil, fmt.Errorf("total decompressed size too large (max: %d bytes)", MaxDecompressedSize)
		}

		// Normalize file path to prevent path traversal attacks
		normalizedPath := filepath.Clean(header.Name)
		// Ensure the path doesn't contain ".." or absolute paths
		if filepath.IsAbs(normalizedPath) || strings.Contains(normalizedPath, "..") {
			return nil, fmt.Errorf("invalid file path: %s", header.Name)
		}

		fileContent, err := io.ReadAll(tarReader)
		if err != nil {
			return nil, err
		}

		commitFiles = append(commitFiles, types.CommitFile{
			Content: string(fileContent),
			Path:    normalizedPath,
		})
	}

	return commitFiles, nil
}
