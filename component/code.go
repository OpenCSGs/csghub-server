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

const codeGitattributesContent = modelGitattributesContent

type CodeComponent interface {
	Create(ctx context.Context, req *types.CreateCodeReq) (*types.Code, error)
	Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Code, int, error)
	Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error)
	Delete(ctx context.Context, namespace, name, currentUser string) error
	Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Code, error)
	Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error)
	OrgCodes(ctx context.Context, req *types.OrgCodesReq) ([]types.Code, int, error)
	GetUploadUrl(ctx context.Context) (string, string, map[string]string, error)
}

func NewCodeComponent(config *config.Config) (CodeComponent, error) {
	c := &codeComponentImpl{}
	var err error
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	c.mirrorComponent, err = NewMirrorComponent(config)
	if err != nil {
		return nil, err
	}
	c.codeStore = database.NewCodeStore()
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

type codeComponentImpl struct {
	config        *config.Config
	repoComponent RepoComponent
	// mirrorComponent creates mirror records and workhub jobs for Git URL imports.
	mirrorComponent MirrorComponent
	codeStore       database.CodeStore
	repoStore       database.RepoStore
	userLikesStore  database.UserLikesStore
	gitServer       gitserver.GitServer
	userSvcClient   rpc.UserSvcClient
	recomStore      database.RecomStore
	s3Client        s3.Client
}

func (c *codeComponentImpl) GetUploadUrl(ctx context.Context) (string, string, map[string]string, error) {
	// Generate UUID
	uuid := uuid.New().String()

	// Build object key
	objectKey := fmt.Sprintf("codes/packages/%s", uuid)

	// Create a new post policy
	expires := time.Now().Add(24 * time.Hour)
	policy := minio.NewPostPolicy()
	err := policy.SetBucket(c.config.S3.Bucket)
	if err != nil {
		slog.WarnContext(ctx, "code set bucket failed", slog.String("error", err.Error()))
	}
	err = policy.SetKey(objectKey)
	if err != nil {
		slog.WarnContext(ctx, "code set key failed", slog.String("error", err.Error()))
	}
	err = policy.SetExpires(expires)
	if err != nil {
		slog.WarnContext(ctx, "code set expires failed", slog.String("error", err.Error()))
	}

	// Set content length range (1 byte to 10MB)
	err = policy.SetContentLengthRange(1, 10*1024*1024)
	if err != nil {
		slog.WarnContext(ctx, "code set content length range failed", slog.String("error", err.Error()))
	}

	// Generate presigned POST URL and form data
	url, formData, err := c.s3Client.PresignedPostPolicy(ctx, policy)
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to generate presigned post policy: %w", err)
	}

	// Return the upload URL, UUID, and form data
	return url.String(), uuid, formData, nil
}

// Create creates a code repository after normalizing any upstream mirror source and credentials.
func (c *codeComponentImpl) Create(ctx context.Context, req *types.CreateCodeReq) (*types.Code, error) {
	if req.GitURL != "" || req.GitUsername != "" || req.GitPassword != "" {
		sourceURL, username, accessToken, err := normalizeMirrorSource(
			req.GitURL, req.GitUsername, req.GitPassword,
		)
		if err != nil {
			return nil, err
		}
		req.GitURL = sourceURL
		req.GitUsername = username
		req.GitPassword = accessToken
	}

	var (
		nickname string
		tags     []types.RepoTag
	)

	if req.Nickname != "" {
		nickname = req.Nickname
	} else {
		nickname = req.Name
	}

	if req.DefaultBranch == "" {
		req.DefaultBranch = types.MainBranch
	}

	req.RepoType = types.CodeRepo
	req.Readme = generateReadmeData(req.License)
	req.Nickname = nickname

	// Start with README and .gitattributes files
	commitFiles := []types.CommitFile{
		{
			Content: req.Readme,
			Path:    types.ReadmeFileName,
		},
		{
			Content: codeGitattributesContent,
			Path:    types.GitattributesFileName,
		},
	}

	// Handle code package if SHA256 is provided
	if req.CodePackageSHA256 != "" {
		decompressedFiles, err := c.handleCodePackage(ctx, req.CodePackageSHA256)
		if err != nil {
			return nil, err
		}
		// Replace with decompressed files
		commitFiles = decompressedFiles
	}

	req.CommitFiles = commitFiles
	_, dbRepo, commitFilesReq, err := c.repoComponent.CreateRepo(ctx, req.CreateRepoReq)
	if err != nil {
		return nil, err
	}

	dbCode := database.Code{
		Repository:   dbRepo,
		RepositoryID: dbRepo.ID,
	}

	repoPath := path.Join(req.Namespace, req.Name)
	code, err := c.codeStore.CreateAndUpdateRepoPath(ctx, dbCode, repoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create database code, cause: %w", err)
	}

	if commitFilesReq != nil {
		_ = c.gitServer.CommitFiles(ctx, *commitFilesReq)
	}

	if err := c.createMirrorIfNeeded(ctx, req); err != nil {
		return nil, err
	}

	for _, tag := range code.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
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
		Repository:   common.BuildCloneInfo(c.config, code.Repository),
		Private:      code.Repository.Private,
		User: types.User{
			Username: dbRepo.User.Username,
			Nickname: dbRepo.User.NickName,
			Email:    dbRepo.User.Email,
		},
		Tags:      tags,
		CreatedAt: code.CreatedAt,
		UpdatedAt: code.UpdatedAt,
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.CodeRepo,
			RepoPath:  code.Repository.Path,
			Operation: types.OperationCreate,
			UserUUID:  dbRepo.User.UUID,
		}
		if err = c.repoComponent.SendAssetManagementMsg(notificationCtx, repoNotificationReq); err != nil {
			slog.Error("failed to send asset management notification message", slog.Any("req", repoNotificationReq), slog.Any("err", err))
		}
	}()

	return resCode, nil
}

func (c *codeComponentImpl) Index(ctx context.Context, filter *types.RepoFilter, per, page int, needOpWeight bool) ([]*types.Code, int, error) {
	var (
		err      error
		resCodes []*types.Code
	)
	repos, total, err := c.repoComponent.PublicToUser(ctx, types.CodeRepo, filter.Username, filter, per, page)
	if err != nil {
		newError := fmt.Errorf("failed to get public code repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}
	codes, err := c.codeStore.ByRepoIDs(ctx, repoIDs)
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
				break
			}
		}
		if code == nil {
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
				ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
				I18nKey:   tag.I18nKey,
				CreatedAt: tag.CreatedAt,
				UpdatedAt: tag.UpdatedAt,
			})
		}
		if code.Repository.Mirror.CurrentTask != nil {
			mirrorTaskStatus = code.Repository.Mirror.CurrentTask.Status
		}
		resCodes = append(resCodes, &types.Code{
			ID:               code.ID,
			Name:             repo.Name,
			Nickname:         repo.Nickname,
			Description:      repo.Description,
			Likes:            repo.Likes,
			Downloads:        repo.DownloadCount,
			Path:             repo.Path,
			RepositoryID:     repo.ID,
			Private:          repo.Private,
			CreatedAt:        code.CreatedAt,
			UpdatedAt:        repo.UpdatedAt,
			Tags:             tags,
			Source:           repo.Source,
			SyncStatus:       repo.SyncStatus,
			License:          repo.License,
			MirrorTaskStatus: mirrorTaskStatus,
		})
	}
	slog.Info("code.index")
	if needOpWeight {
		c.addOpWeightToCodes(ctx, repoIDs, resCodes)
	}

	return resCodes, total, nil
}

func (c *codeComponentImpl) Update(ctx context.Context, req *types.UpdateCodeReq) (*types.Code, error) {
	req.RepoType = types.CodeRepo
	dbRepo, err := c.repoComponent.UpdateRepo(ctx, req.UpdateRepoReq)
	if err != nil {
		return nil, err
	}

	code, err := c.codeStore.ByRepoID(ctx, dbRepo.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	//update times of code
	err = c.codeStore.Update(ctx, *code)
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

func (c *codeComponentImpl) Delete(ctx context.Context, namespace, name, currentUser string) error {
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find code, error: %w", err)
	}

	deleteDatabaseRepoReq := types.DeleteRepoReq{
		Username:  currentUser,
		Namespace: namespace,
		Name:      name,
		RepoType:  types.CodeRepo,
	}
	repo, err := c.repoComponent.DeleteRepo(ctx, deleteDatabaseRepoReq)
	if err != nil {
		return fmt.Errorf("failed to delete repo of code, error: %w", err)
	}

	err = c.codeStore.Delete(ctx, *code)
	if err != nil {
		return fmt.Errorf("failed to delete database code, error: %w", err)
	}

	go func() {
		notificationCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		repoNotificationReq := types.RepoNotificationReq{
			RepoType:  types.CodeRepo,
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

func (c *codeComponentImpl) Show(ctx context.Context, namespace, name, currentUser string, needOpWeight bool, needMultiSync bool) (*types.Code, error) {
	var (
		tags             []types.RepoTag
		mirrorTaskStatus types.MirrorTaskStatus
	)
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code, error: %w", err)
	}

	permission, err := c.repoComponent.GetUserRepoPermission(ctx, currentUser, code.Repository)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, errorx.ErrForbidden
	}

	ns, err := c.repoComponent.GetNameSpaceInfo(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to get namespace info for code, error: %w", err)
	}

	for _, tag := range code.Repository.Tags {
		tags = append(tags, types.RepoTag{
			Name:      tag.Name,
			Category:  tag.Category,
			Group:     tag.Group,
			BuiltIn:   tag.BuiltIn,
			ShowName:  tag.I18nKey, // ShowName:  tag.ShowName,
			I18nKey:   tag.I18nKey,
			CreatedAt: tag.CreatedAt,
			UpdatedAt: tag.UpdatedAt,
		})
	}

	likeExists, err := c.userLikesStore.IsExist(ctx, currentUser, code.Repository.ID)
	if err != nil {
		newError := fmt.Errorf("failed to check for the presence of the user likes,error:%w", err)
		return nil, newError
	}

	mirrorTaskStatus = c.repoComponent.GetMirrorTaskStatus(code.Repository)

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
		Repository:    common.BuildCloneInfo(c.config, code.Repository),
		Tags:          tags,
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
		License:    code.Repository.License,
		CanWrite:   permission.CanWrite,
		CanManage:  permission.CanAdmin,
		Namespace:  ns,
		MultiSource: types.MultiSource{
			HFPath:  code.Repository.HFPath,
			MSPath:  code.Repository.MSPath,
			CSGPath: code.Repository.CSGPath,
		},
		MirrorTaskStatus: mirrorTaskStatus,
		//RecomOpWeight: ,
	}
	if permission.CanAdmin {
		resCode.SensitiveCheckStatus = code.Repository.SensitiveCheckStatus.String()
	}
	if needOpWeight {
		c.addOpWeightToCodes(ctx, []int64{resCode.RepositoryID}, []*types.Code{resCode})
	}

	if needMultiSync {
		weightNames := []database.RecomWeightName{database.RecomWeightFreshness,
			database.RecomWeightDownloads,
			database.RecomWeightQuality,
			database.RecomWeightOp,
			database.RecomWeightTotal}
		c.addWeightsToCode(ctx, resCode.RepositoryID, resCode, weightNames)
	}
	return resCode, nil
}

func (c *codeComponentImpl) Relations(ctx context.Context, namespace, name, currentUser string) (*types.Relations, error) {
	code, err := c.codeStore.FindByPath(ctx, namespace, name)
	if err != nil {
		return nil, fmt.Errorf("failed to find code repo, error: %w", err)
	}

	allow, _ := c.repoComponent.AllowReadAccessRepo(ctx, code.Repository, currentUser)
	if !allow {
		return nil, errorx.ErrForbidden
	}

	return c.getRelations(ctx, code.RepositoryID, currentUser)
}

func (c *codeComponentImpl) getRelations(ctx context.Context, repoID int64, currentUser string) (*types.Relations, error) {
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

func (c *codeComponentImpl) OrgCodes(ctx context.Context, req *types.OrgCodesReq) ([]types.Code, int, error) {
	var resCodes []types.Code
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
	codes, total, err := c.codeStore.ByOrgPath(ctx, req.Namespace, req.PageSize, req.Page, onlyPublic)
	if err != nil {
		newError := fmt.Errorf("failed to get org codes,error:%w", err)
		slog.ErrorContext(ctx, newError.Error())
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
			UpdatedAt:    data.Repository.UpdatedAt,
		})
	}

	return resCodes, total, nil
}

func (c *codeComponentImpl) addWeightsToCode(ctx context.Context, repoID int64, resCode *types.Code, weightNames []database.RecomWeightName) {
	weights, err := c.recomStore.FindByRepoIDs(ctx, []int64{repoID})
	if err == nil {
		resCode.Scores = make([]types.WeightScore, 0)
		for _, weight := range weights {
			if slices.Contains(weightNames, weight.WeightName) {
				score := types.WeightScore{
					WeightName: string(weight.WeightName),
					Score:      weight.Score,
				}
				resCode.Scores = append(resCode.Scores, score)
			}
		}
	}
}

// handleCodePackage downloads and decompresses the code package
func (c *codeComponentImpl) handleCodePackage(ctx context.Context, sha256 string) ([]types.CommitFile, error) {
	// Download file from Minio
	objectKey := common.BuildCodePackageObjectKey(sha256)
	object, err := c.s3Client.GetObject(ctx, c.config.S3.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to download code package from Minio: %w", err)
	}
	if object == nil {
		return nil, fmt.Errorf("failed to download code package: object is nil")
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
		return decompressZipForCode(bytes.NewReader(zipContent), int64(len(zipContent)))
	} else if bytes.HasPrefix(magicBytes, []byte{0x1F, 0x8B, 0x08}) {
		// GZIP format (tar.gz) - use streaming decompression
		// Reset reader to start (including the magic bytes we already read)
		r := io.MultiReader(bytes.NewReader(magicBytes), bufReader)
		return decompressTarGzForCode(r)
	} else {
		return nil, fmt.Errorf("unknown file format for code package")
	}
}

// Constants for decompression limits
const (
	// MaxCodeDecompressedSize is the maximum total size of decompressed files (100MB)
	MaxCodeDecompressedSize = 100 * 1024 * 1024
	// MaxCodeIndividualFileSize is the maximum size of a single decompressed file (50MB)
	MaxCodeIndividualFileSize = 50 * 1024 * 1024
)

// decompressZipForCode decompresses a zip file and returns a list of CommitFile objects
func decompressZipForCode(reader io.ReaderAt, size int64) ([]types.CommitFile, error) {
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
		if file.UncompressedSize64 > uint64(MaxCodeIndividualFileSize) {
			return nil, fmt.Errorf("file too large: %s (size: %d bytes, max: %d bytes)", file.Name, file.UncompressedSize64, MaxCodeIndividualFileSize)
		}

		// Update total size
		totalSize += int64(file.UncompressedSize64)
		if totalSize > MaxCodeDecompressedSize {
			return nil, fmt.Errorf("total decompressed size too large (max: %d bytes)", MaxCodeDecompressedSize)
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

// decompressTarGzForCode decompresses a tar.gz or tgz file and returns a list of CommitFile objects
func decompressTarGzForCode(reader io.Reader) ([]types.CommitFile, error) {
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
		if header.Size > MaxCodeIndividualFileSize {
			return nil, fmt.Errorf("file too large: %s (size: %d bytes, max: %d bytes)", header.Name, header.Size, MaxCodeIndividualFileSize)
		}

		// Update total size
		totalSize += header.Size
		if totalSize > MaxCodeDecompressedSize {
			return nil, fmt.Errorf("total decompressed size too large (max: %d bytes)", MaxCodeDecompressedSize)
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

func (c *codeComponentImpl) createMirrorIfNeeded(ctx context.Context, req *types.CreateCodeReq) error {
	if req.GitURL == "" {
		return nil
	}

	mirrorReq := types.CreateMirrorReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		SourceUrl:   req.GitURL,
		Username:    req.GitUsername,
		AccessToken: req.GitPassword,
		CurrentUser: req.Username,
		RepoType:    types.CodeRepo,
		SyncLfs:     true,
	}

	_, err := c.mirrorComponent.CreateMirror(ctx, mirrorReq)
	if err != nil {
		return fmt.Errorf("failed to create mirror: %w", err)
	}

	return nil
}
