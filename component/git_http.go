package component

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"os"
	"path"
	"strconv"

	"github.com/minio/minio-go/v7"
	"github.com/minio/sha256-simd"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type gitHTTPComponentImpl struct {
	gitServer          gitserver.GitServer
	config             *config.Config
	s3Client           s3.Client
	lfsMetaObjectStore database.LfsMetaObjectStore
	lfsLockStore       database.LfsLockStore
	repoStore          database.RepoStore
	userStore          database.UserStore
	repoComponent      RepoComponent
}

type GitHTTPComponent interface {
	InfoRefs(ctx context.Context, req types.InfoRefsReq) (io.Reader, error)
	GitUploadPack(ctx context.Context, req types.GitUploadPackReq) error
	GitReceivePack(ctx context.Context, req types.GitReceivePackReq) error
	LFSBatch(ctx context.Context, req types.BatchRequest) (*types.BatchResponse, error)
	LfsUpload(ctx context.Context, body io.ReadCloser, req types.UploadRequest) error
	LfsVerify(ctx context.Context, req types.VerifyRequest, p types.Pointer) error
	CreateLock(ctx context.Context, req types.LfsLockReq) (*database.LfsLock, error)
	ListLocks(ctx context.Context, req types.ListLFSLockReq) (*types.LFSLockList, error)
	UnLock(ctx context.Context, req types.UnlockLFSReq) (*database.LfsLock, error)
	VerifyLock(ctx context.Context, req types.VerifyLFSLockReq) (*types.LFSLockListVerify, error)
	LfsDownload(ctx context.Context, req types.DownloadRequest) (*url.URL, error)
}

func NewGitHTTPComponent(config *config.Config) (GitHTTPComponent, error) {
	c := &gitHTTPComponentImpl{}
	c.config = config
	var err error
	c.gitServer, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.s3Client, err = s3.NewMinio(config)
	if err != nil {
		newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	c.lfsMetaObjectStore = database.NewLfsMetaObjectStore()
	c.repoStore = database.NewRepoStore()
	c.lfsLockStore = database.NewLfsLockStore()
	c.userStore = database.NewUserStore()
	c.repoComponent, err = NewRepoComponentImpl(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *gitHTTPComponentImpl) InfoRefs(ctx context.Context, req types.InfoRefsReq) (io.Reader, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if req.Rpc == "git-receive-pack" {
		allowed, err := c.repoComponent.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
		if err != nil {
			return nil, ErrUnauthorized
		}
		if !allowed {
			return nil, ErrForbidden
		}
	} else {
		if repo.Private {
			allowed, err := c.repoComponent.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
			if err != nil {
				return nil, ErrUnauthorized
			}
			if !allowed {
				return nil, ErrForbidden
			}
		}
	}

	reader, err := c.gitServer.InfoRefsResponse(ctx, gitserver.InfoRefsReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Rpc:         req.Rpc,
		RepoType:    req.RepoType,
		GitProtocol: req.GitProtocol,
	})

	return reader, err
}

func (c *gitHTTPComponentImpl) GitUploadPack(ctx context.Context, req types.GitUploadPackReq) error {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	if repo.Private {
		allowed, err := c.repoComponent.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
		if err != nil {
			return ErrUnauthorized
		}
		if !allowed {
			return ErrForbidden
		}
	}
	err = c.gitServer.UploadPack(ctx, gitserver.UploadPackReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Request:     req.Request,
		RepoType:    req.RepoType,
		GitProtocol: req.GitProtocol,
		Writer:      req.Writer,
	})

	return err
}

func (c *gitHTTPComponentImpl) GitReceivePack(ctx context.Context, req types.GitReceivePackReq) error {
	_, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return ErrUnauthorized
	}

	allowed, err := c.repoComponent.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		return ErrUnauthorized
	}
	if !allowed {
		return ErrForbidden
	}
	err = c.gitServer.ReceivePack(ctx, gitserver.ReceivePackReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Request:     req.Request,
		RepoType:    req.RepoType,
		GitProtocol: req.GitProtocol,
		Writer:      req.Writer,
		UserId:      user.ID,
		Username:    user.Username,
	})

	return err
}

func (c *gitHTTPComponentImpl) lfsBatchDownloadInfo(ctx context.Context, req types.BatchRequest, repo *database.Repository) (*types.BatchResponse, error) {
	var objs []*types.ObjectResponse
	lfsFiles, err := c.lfsMetaObjectStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	exists := map[string]*database.LfsMetaObject{}
	for _, f := range lfsFiles {
		exists[f.Oid] = &f
	}

	for _, obj := range req.Objects {
		if _, ok := exists[obj.Oid]; !ok {
			objs = append(objs, &types.ObjectResponse{
				Error: &types.ObjectError{
					Code:    404,
					Message: "Object does not exist",
				},
			})
			continue
		}
		if !obj.Valid() {
			objs = append(objs, &types.ObjectResponse{
				Error: &types.ObjectError{},
			})
			continue
		}
		objectKey := path.Join("lfs", obj.RelativePath())
		resp := &types.ObjectResponse{
			Actions: map[string]*types.Link{},
			Pointer: obj,
		}
		reqParams := make(url.Values)
		url, err := c.s3Client.PresignedGetObject(ctx, c.config.S3.Bucket, objectKey, types.OssFileExpire, reqParams)
		if err != nil {
			objs = append(objs, &types.ObjectResponse{
				Error: &types.ObjectError{},
			})
			continue
		}
		resp.Actions["download"] = &types.Link{Href: url.String(), Header: map[string]string{}}
		objs = append(objs, resp)
	}
	return &types.BatchResponse{Objects: objs}, nil
}

func (c *gitHTTPComponentImpl) lfsBatchUploadInfo(ctx context.Context, req types.BatchRequest, repo *database.Repository) (*types.BatchResponse, error) {
	header := make(map[string]string)
	if len(req.Authorization) > 0 {
		header["Authorization"] = req.Authorization
	}

	var objs []*types.ObjectResponse
	lfsFiles, err := c.lfsMetaObjectStore.FindByRepoID(ctx, repo.ID)
	if err != nil {
		return nil, err
	}
	exists := map[string]*database.LfsMetaObject{}
	for _, f := range lfsFiles {
		exists[f.Oid] = &f
	}

	for _, obj := range req.Objects {
		if _, ok := exists[obj.Oid]; ok {
			continue
		}
		if !obj.Valid() {
			objs = append(objs, &types.ObjectResponse{
				Error: &types.ObjectError{},
			})
			continue
		}
		resp := &types.ObjectResponse{
			Actions: map[string]*types.Link{},
			Pointer: obj,
		}
		resp.Actions["upload"] = &types.Link{Href: c.buildUploadLink(req, obj), Header: header}
		verifyHeader := make(map[string]string)
		for key, value := range header {
			verifyHeader[key] = value
		}
		verifyHeader["Accept"] = types.LfsMediaType
		resp.Actions["verify"] = &types.Link{Href: c.buildVerifyLink(req), Header: verifyHeader}
		objs = append(objs, resp)
	}
	return &types.BatchResponse{Objects: objs}, nil
}

// https://gitlab.com/gitlab-org/gitlab-foss/-/blob/master/app/controllers/concerns/lfs_request.rb#L45
// Only return a 403 response if the user has download(read) access permission,
// otherwise return a 404 to avoid exposing the existence of the container.
// Return nil means user has required permission.
func (c *gitHTTPComponentImpl) lfsCheckAccess(ctx context.Context, req types.BatchRequest) error {
	switch req.Operation {
	case types.LFSBatchUpload:
		allowWrite, err := c.repoComponent.AllowWriteAccess(
			ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser,
		)
		if err != nil {
			return err
		}
		if allowWrite {
			return nil
		}
		allowRead, err := c.repoComponent.AllowReadAccess(
			ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser,
		)
		if err != nil {
			return err
		}
		if allowRead {
			return ErrForbidden
		}
		return ErrNotFound
	case types.LFSBatchDownload:
		allowRead, err := c.repoComponent.AllowReadAccess(
			ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser,
		)
		if err != nil {
			return err
		}
		if !allowRead {
			return ErrNotFound
		}
		return nil
	}
	return &HTTPError{
		StatusCode: 400,
		Message:    fmt.Errorf("invalid lfs batch operation: %s", req.Operation),
	}
}

func (c *gitHTTPComponentImpl) LFSBatch(ctx context.Context, req types.BatchRequest) (*types.BatchResponse, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	err = c.lfsCheckAccess(ctx, req)
	if err != nil {
		return nil, err
	}

	var resp *types.BatchResponse
	switch req.Operation {
	case types.LFSBatchUpload:
		resp, err = c.lfsBatchUploadInfo(ctx, req, repo)
	case types.LFSBatchDownload:
		resp, err = c.lfsBatchDownloadInfo(ctx, req, repo)
	default:
		return nil, &HTTPError{
			StatusCode: 400,
			Message:    fmt.Errorf("invalid lfs batch operation: %s", req.Operation),
		}
	}
	return resp, err
}

// https://github.com/minio/minio-go/issues/1082
func noSuchKey(err error) bool {
	if os.IsNotExist(err) {
		return true
	}
	minioErr := minio.ToErrorResponse(err)
	return minioErr.Code == "NoSuchKey"
}

func (c *gitHTTPComponentImpl) LfsUpload(ctx context.Context, body io.ReadCloser, req types.UploadRequest) error {
	var allowed bool
	defer body.Close()
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	allowed, err = c.repoComponent.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrPermissionDenied
	}

	pointer := types.Pointer{Oid: req.Oid, Size: req.Size}

	if !pointer.Valid() {
		return errors.New("invalid lfs oid")
	}

	objectKey := path.Join("lfs", pointer.RelativePath())
	_, err = c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if !noSuchKey(err) {
			return err
		}
	} else {
		// object already exists on minio,
		// we still need to read whole file and calculate sha256
		// to verify user has the file
		h := sha256.New()
		size, err := io.Copy(h, body)
		if err != nil {
			return err
		}

		checksum := hex.EncodeToString(h.Sum(nil))
		if size != pointer.Size || checksum != pointer.Oid {
			return errors.New("invalid lfs size or oid")
		}
		return nil
	}

	_, err = c.s3Client.UploadAndValidate(
		ctx,
		c.config.S3.Bucket,
		objectKey,
		body,
		req.Size,
	)
	if err != nil {
		return err
	}

	_, err = c.lfsMetaObjectStore.Create(ctx, database.LfsMetaObject{
		Oid:          pointer.Oid,
		Size:         pointer.Size,
		RepositoryID: repo.ID,
		Existing:     true,
	})
	return err
}

func (c *gitHTTPComponentImpl) LfsVerify(ctx context.Context, req types.VerifyRequest, p types.Pointer) error {
	objectKey := path.Join("lfs", p.RelativePath())
	fileInfo, err := c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		slog.Error("failed to stat object in s3", slog.Any("error", err))
		return fmt.Errorf("failed to stat object in s3, error: %w", err)
	}

	if fileInfo.Size != p.Size {
		return types.ErrSizeMismatch
	}

	return nil
}

func (c *gitHTTPComponentImpl) CreateLock(ctx context.Context, req types.LfsLockReq) (*database.LfsLock, error) {
	var (
		lock *database.LfsLock
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.repoComponent.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("Unable to check user write access:", slog.Any("error", err))
		return nil, err
	}

	if !allowed {
		return nil, ErrUnauthorized
	}
	lfsLock := database.LfsLock{
		Path:         req.Path,
		UserID:       user.ID,
		RepositoryID: repo.ID,
	}

	lock, err = c.lfsLockStore.FindByPath(ctx, lfsLock.RepositoryID, lfsLock.Path)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			lock, err = c.lfsLockStore.Create(ctx, lfsLock)
			if err != nil {
				return nil, ErrAlreadyExists
			}
			return lock, nil
		}
		return lock, fmt.Errorf("failed to find lfs lock, error: %w", err)
	}

	return lock, ErrAlreadyExists
}

func (c *gitHTTPComponentImpl) ListLocks(ctx context.Context, req types.ListLFSLockReq) (*types.LFSLockList, error) {
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	_, err = c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.repoComponent.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("Unable to check user write access:", slog.Any("error", err))
		return nil, err
	}

	if !allowed {
		return nil, ErrUnauthorized
	}

	if req.ID != 0 {
		l, err := c.lfsLockStore.FindByID(ctx, req.ID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return buildLFSLockList([]database.LfsLock{}), nil
			}
			return buildLFSLockList([]database.LfsLock{}), err
		}
		if l.RepositoryID != repo.ID {
			return buildLFSLockList([]database.LfsLock{}), nil
		}
		return buildLFSLockList([]database.LfsLock{*l}), nil
	}

	if req.Path != "" {
		l, err := c.lfsLockStore.FindByPath(ctx, repo.ID, req.Path)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return buildLFSLockList([]database.LfsLock{}), nil
			}
			return buildLFSLockList([]database.LfsLock{}), err
		}
		return buildLFSLockList([]database.LfsLock{*l}), nil
	}

	locks, err := c.lfsLockStore.FindByRepoID(ctx, repo.ID, req.Cursor, req.Limit)
	if err != nil {
		return buildLFSLockList([]database.LfsLock{}), err
	}

	next := ""
	if req.Limit > 0 && len(locks) == req.Limit {
		next = strconv.Itoa(req.Cursor + 1)
	}
	res := buildLFSLockList(locks)
	res.Next = next

	return res, nil
}

func (c *gitHTTPComponentImpl) UnLock(ctx context.Context, req types.UnlockLFSReq) (*database.LfsLock, error) {
	var (
		lock *database.LfsLock
		err  error
	)
	_, err = c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.repoComponent.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("Unable to check user write access:", slog.Any("error", err))
		return nil, err
	}

	if !allowed {
		return nil, ErrUnauthorized
	}

	lock, err = c.lfsLockStore.FindByID(ctx, req.ID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}

	if !req.Force && lock.UserID != user.ID {
		return nil, ErrPermissionDenied
	}
	err = c.lfsLockStore.RemoveByID(ctx, req.ID)
	if err != nil {
		return nil, err
	}

	return lock, nil
}

func (c *gitHTTPComponentImpl) VerifyLock(ctx context.Context, req types.VerifyLFSLockReq) (*types.LFSLockListVerify, error) {
	var (
		ourLocks   []*types.LFSLock
		theirLocks []*types.LFSLock
		res        types.LFSLockListVerify
	)
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.userStore.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.repoComponent.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		slog.Error("Unable to check user write access:", slog.Any("error", err))
		return nil, err
	}

	if !allowed {
		return nil, ErrUnauthorized
	}

	locks, err := c.lfsLockStore.FindByRepoID(ctx, repo.ID, req.Cursor, req.Limit)
	if err != nil {
		return &types.LFSLockListVerify{}, err
	}

	next := ""
	if req.Limit > 0 && len(locks) == req.Limit {
		next = strconv.Itoa(req.Cursor + 1)
	}
	res.Next = next

	for _, l := range locks {
		lo := &types.LFSLock{
			ID:       strconv.FormatInt(l.ID, 10),
			Path:     l.Path,
			LockedAt: l.CreatedAt,
			Owner: &types.LFSLockOwner{
				Name: l.User.Username,
			},
		}
		if l.UserID == user.ID {
			ourLocks = append(ourLocks, lo)
		} else {
			theirLocks = append(theirLocks, lo)
		}
	}
	res.Ours = ourLocks
	res.Theirs = theirLocks

	return &res, nil
}

func (c *gitHTTPComponentImpl) LfsDownload(ctx context.Context, req types.DownloadRequest) (*url.URL, error) {
	pointer := types.Pointer{Oid: req.Oid}
	repo, err := c.repoStore.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	allowed, err := c.repoComponent.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		return nil, fmt.Errorf("failed to check allowed, error: %w", err)
	}

	if !allowed {
		return nil, errors.New("you have no permission to access this repo")
	}

	_, err = c.lfsMetaObjectStore.FindByOID(ctx, repo.ID, pointer.Oid)
	if err != nil {
		return nil, fmt.Errorf("failed to find lfs meta object, error: %w", err)
	}
	objectKey := path.Join("lfs", pointer.RelativePath())

	reqParams := make(url.Values)
	if req.SaveAs != "" {
		// allow rename when download through content-disposition header
		reqParams.Set("response-content-disposition", fmt.Sprintf("attachment;filename=%s", req.SaveAs))
	}
	signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.config.S3.Bucket, objectKey, types.OssFileExpire, reqParams)
	if err != nil {
		return nil, err
	}
	return signedUrl, nil
}

func (c *gitHTTPComponentImpl) buildUploadLink(req types.BatchRequest, pointer types.Pointer) string {
	return c.config.APIServer.PublicDomain + "/" + path.Join(fmt.Sprintf("%ss", req.RepoType), url.PathEscape(req.Namespace), url.PathEscape(req.Name+".git"), "info/lfs/objects", url.PathEscape(pointer.Oid), strconv.FormatInt(pointer.Size, 10))

}

func (c *gitHTTPComponentImpl) buildVerifyLink(req types.BatchRequest) string {
	return c.config.APIServer.PublicDomain + "/" + path.Join(fmt.Sprintf("%ss", req.RepoType), url.PathEscape(req.Namespace), url.PathEscape(req.Name+".git"), "info/lfs/verify")
}

func buildLFSLockList(lfsLocks []database.LfsLock) *types.LFSLockList {
	if len(lfsLocks) == 0 {
		return &types.LFSLockList{
			Locks: []*types.LFSLock{},
		}
	}

	var locks []*types.LFSLock
	for _, l := range lfsLocks {
		locks = append(locks, &types.LFSLock{
			ID:       strconv.FormatInt(l.ID, 10),
			Path:     l.Path,
			LockedAt: l.CreatedAt,
			Owner: &types.LFSLockOwner{
				Name: l.User.Username,
			},
		})
	}
	return &types.LFSLockList{
		Locks: locks,
	}
}
