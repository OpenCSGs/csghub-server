package component

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

type GitHTTPComponent struct {
	git                gitserver.GitServer
	config             *config.Config
	s3Client           *minio.Client
	lfsMetaObjectStore *database.LfsMetaObjectStore
	lfsLockStore       *database.LfsLockStore
	repo               *database.RepoStore
	*RepoComponent
}

func NewGitHTTPComponent(config *config.Config) (*GitHTTPComponent, error) {
	c := &GitHTTPComponent{}
	c.config = config
	var err error
	c.git, err = git.NewGitServer(config)
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
	c.repo = database.NewRepoStore()
	c.lfsLockStore = database.NewLfsLockStore()
	c.RepoComponent, err = NewRepoComponent(config)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (c *GitHTTPComponent) InfoRefs(ctx context.Context, req types.InfoRefsReq) (io.Reader, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	if req.Rpc == "git-receive-pack" {
		allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
		if err != nil {
			return nil, ErrUnauthorized
		}
		if !allowed {
			return nil, ErrForbidden
		}
	} else {
		if repo.Private {
			allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
			if err != nil {
				return nil, ErrUnauthorized
			}
			if !allowed {
				return nil, ErrForbidden
			}
		}
	}

	reader, err := c.git.InfoRefsResponse(ctx, gitserver.InfoRefsReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Rpc:         req.Rpc,
		RepoType:    req.RepoType,
		GitProtocol: req.GitProtocol,
	})

	return reader, err
}

func (c *GitHTTPComponent) GitUploadPack(ctx context.Context, req types.GitUploadPackReq) error {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	if repo.Private {
		allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
		if err != nil {
			return ErrUnauthorized
		}
		if !allowed {
			return ErrForbidden
		}
	}
	err = c.git.UploadPack(ctx, gitserver.UploadPackReq{
		Namespace:   req.Namespace,
		Name:        req.Name,
		Request:     req.Request,
		RepoType:    req.RepoType,
		GitProtocol: req.GitProtocol,
		Writer:      req.Writer,
	})

	return err
}

func (c *GitHTTPComponent) GitReceivePack(ctx context.Context, req types.GitReceivePackReq) error {
	_, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return ErrUnauthorized
	}

	allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
	if err != nil {
		return ErrUnauthorized
	}
	if !allowed {
		return ErrForbidden
	}
	err = c.git.ReceivePack(ctx, gitserver.ReceivePackReq{
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

func (c *GitHTTPComponent) BuildObjectResponse(ctx context.Context, req types.BatchRequest, isUpload bool) (*types.BatchResponse, error) {
	var (
		respObjects []*types.ObjectResponse
		exists      bool
	)
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	for _, obj := range req.Objects {
		if !obj.Valid() {
			respObjects = append(respObjects, c.buildObjectResponse(ctx, req, obj, false, false, &types.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: "Oid or size are invalid",
			}))
			continue
		}
		objectKey := path.Join("lfs", obj.RelativePath())
		_, err := c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
		if err != nil {
			if os.IsNotExist(err) {
				exists = false
			}
			slog.Error("failed to check if lfs file exists", slog.String("oid", objectKey), slog.Any("error", err))
			exists = false
		} else {
			exists = true
		}

		lfsMetaObject, err := c.lfsMetaObjectStore.FindByOID(ctx, repo.ID, obj.Oid)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			slog.Error("failed to check if lfs file exists in database", slog.String("oid", objectKey), slog.Any("error", err))
			return nil, err
		}

		if lfsMetaObject != nil && obj.Size != lfsMetaObject.Size {
			respObjects = append(respObjects, c.buildObjectResponse(ctx, req, obj, false, false, &types.ObjectError{
				Code:    http.StatusUnprocessableEntity,
				Message: fmt.Sprintf("Object %s is not %d bytes", obj.Oid, obj.Size),
			}))
			continue
		}

		var responseObject *types.ObjectResponse
		if isUpload {
			var err *types.ObjectError
			// if !exists && setting.LFS.MaxFileSize > 0 && p.Size > setting.LFS.MaxFileSize {
			// 	err = &types.ObjectError{
			// 		Code:    http.StatusUnprocessableEntity,
			// 		Message: fmt.Sprintf("Size must be less than or equal to %d", setting.LFS.MaxFileSize),
			// 	}
			// }

			if exists && lfsMetaObject == nil {
				allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
				if err != nil {
					slog.Error("unable to check if user can wirte this repo", slog.String("lfs oid", obj.Oid), slog.Any("error", err))
					return nil, ErrUnauthorized
				}
				if allowed {
					_, err := c.lfsMetaObjectStore.Create(ctx, database.LfsMetaObject{
						Oid:          obj.Oid,
						Size:         obj.Size,
						RepositoryID: repo.ID,
						Existing:     true,
					})
					if err != nil {
						slog.Error("Unable to create LFS MetaObject [%s] for %s/%s. Error: %v", obj.Oid, req.Namespace, req.Name, err)
						return nil, err
					}
				} else {
					exists = false
				}
			}

			responseObject = c.buildObjectResponse(ctx, req, obj, false, !exists, err)
		} else {
			var err *types.ObjectError
			// if !exists || lfsMetaObject == nil {
			// 	err = &types.ObjectError{
			// 		Code:    http.StatusNotFound,
			// 		Message: http.StatusText(http.StatusNotFound),
			// 	}
			// }

			responseObject = c.buildObjectResponse(ctx, req, obj, true, false, err)
		}
		respObjects = append(respObjects, responseObject)
	}
	respobj := &types.BatchResponse{Objects: respObjects}
	return respobj, nil
}

func (c *GitHTTPComponent) buildObjectResponse(ctx context.Context, req types.BatchRequest, pointer types.Pointer, download, upload bool, err *types.ObjectError) *types.ObjectResponse {
	rep := &types.ObjectResponse{Pointer: pointer}
	if err != nil {
		rep.Error = err
	} else {
		rep.Actions = make(map[string]*types.Link)

		header := make(map[string]string)

		if len(req.Authorization) > 0 {
			header["Authorization"] = req.Authorization
		}

		if download {
			var link *types.Link
			reqParams := make(url.Values)
			objectKey := path.Join("lfs", pointer.RelativePath())
			url, err := c.s3Client.PresignedGetObject(ctx, c.config.S3.Bucket, objectKey, ossFileExpireSeconds, reqParams)
			if url != nil && err == nil {
				delete(header, "Authorization")
				link = &types.Link{Href: url.String(), Header: header}
			}
			if link == nil {
				link = &types.Link{Href: c.buildDownloadLink(req, pointer), Header: header}
			}
			rep.Actions["download"] = link
		}
		if upload {
			rep.Actions["upload"] = &types.Link{Href: c.buildUploadLink(req, pointer), Header: header}

			verifyHeader := make(map[string]string)
			for key, value := range header {
				verifyHeader[key] = value
			}

			verifyHeader["Accept"] = types.LfsMediaType

			rep.Actions["verify"] = &types.Link{Href: c.buildVerifyLink(req), Header: verifyHeader}
		}
	}
	return rep
}

func (c *GitHTTPComponent) LfsUpload(ctx context.Context, body io.ReadCloser, req types.UploadRequest) error {
	var exists bool
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	pointer := types.Pointer{Oid: req.Oid, Size: req.Size}

	if !pointer.Valid() {
		slog.Error("invalid lfs oid", slog.String("oid", req.Oid))
		return errors.New("invalid lfs oid")
	}

	objectKey := path.Join("lfs", pointer.RelativePath())
	_, err = c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
		}
		slog.Error("failed to check if lfs file exists", slog.String("oid", objectKey), slog.Any("error", err))
		exists = false
	} else {
		exists = true
	}
	uploadOrVerify := func() error {
		if exists {
			allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
			if err != nil {
				slog.Error("Unable to check if LFS MetaObject [%s] is allowed. Error: %v", pointer.Oid, err)
				return err
			}
			if !allowed {
				// The file exists but the user has no access to it.
				// The upload gets verified by hashing and size comparison to prove access to it.
				hash := sha256.New()
				written, err := io.Copy(hash, body)
				if err != nil {
					slog.Error("Error creating hash. Error: %v", err)
					return err
				}

				if written != pointer.Size {
					return types.ErrSizeMismatch
				}
				if hex.EncodeToString(hash.Sum(nil)) != pointer.Oid {
					return types.ErrHashMismatch
				}
			}
		} else {
			var (
				uploadErr  error
				uploadInfo minio.UploadInfo
			)
			uploadInfo, uploadErr = c.s3Client.PutObject(
				ctx,
				c.config.S3.Bucket,
				objectKey,
				body,
				req.Size,
				minio.PutObjectOptions{
					ContentType:           "application/octet-stream",
					SendContentMd5:        true,
					ConcurrentStreamParts: true,
					NumThreads:            5,
				})
			if uploadErr != nil {
				slog.Error("Error putting LFS MetaObject [%s] into content store. Error: %v", pointer.Oid, err)
			}
			if uploadInfo.Size != pointer.Size {
				uploadErr = types.ErrSizeMismatch
			}
			if uploadErr != nil {
				err := c.s3Client.RemoveObject(
					ctx,
					c.config.S3.Bucket,
					objectKey,
					minio.RemoveObjectOptions{},
				)
				if err != nil {
					slog.Error("Cleaning the LFS OID[%s] failed: %v", pointer.Oid, err)
				}
			}
		}
		_, err := c.lfsMetaObjectStore.Create(ctx, database.LfsMetaObject{
			Oid:          pointer.Oid,
			Size:         pointer.Size,
			RepositoryID: repo.ID,
			Existing:     true,
		})
		return err
	}
	defer body.Close()
	if err := uploadOrVerify(); err != nil {
		if errors.Is(err, types.ErrSizeMismatch) || errors.Is(err, types.ErrHashMismatch) {
			slog.Error("Upload does not match LFS MetaObject [%s]. Error: %v", pointer.Oid, err)
		} else {
			slog.Error("Error whilst uploadOrVerify LFS OID[%s]: %v", pointer.Oid, err)
		}
		if err = c.lfsMetaObjectStore.RemoveByOid(ctx, pointer.Oid, repo.ID); err != nil {
			slog.Error("Error whilst removing MetaObject for LFS OID[%s]: %v", pointer.Oid, err)
		}
		return err
	}

	return nil
}

func (c *GitHTTPComponent) LfsVerify(ctx context.Context, req types.VerifyRequest, p types.Pointer) error {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}

	// _, err = c.lfsMetaObjectStore.FindByOID(ctx, repo.ID, p.Oid)
	// if err != nil {
	// 	return fmt.Errorf("failed to find lfs meta object, error: %w", err)
	// }
	objectKey := path.Join("lfs", p.RelativePath())
	fileInfo, err := c.s3Client.StatObject(ctx, c.config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		slog.Error("failed to stat object in s3", slog.Any("error", err))
		return fmt.Errorf("failed to stat object in s3, error: %w", err)
	}

	if fileInfo.Size != p.Size {
		return types.ErrSizeMismatch
	}

	_, err = c.lfsMetaObjectStore.Create(ctx, database.LfsMetaObject{
		Oid:          p.Oid,
		Size:         p.Size,
		RepositoryID: repo.ID,
		Existing:     true,
	})
	if err != nil {
		return fmt.Errorf("failed to create lfs meta object in database: %w", err)
	}

	return nil
}

func (c *GitHTTPComponent) CreateLock(ctx context.Context, req types.LfsLockReq) (*database.LfsLock, error) {
	var (
		lock *database.LfsLock
	)
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
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

func (c *GitHTTPComponent) ListLocks(ctx context.Context, req types.ListLFSLockReq) (*types.LFSLockList, error) {
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	_, err = c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
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

func (c *GitHTTPComponent) UnLock(ctx context.Context, req types.UnlockLFSReq) (*database.LfsLock, error) {
	var (
		lock *database.LfsLock
		err  error
	)
	_, err = c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.AllowWriteAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
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

func (c *GitHTTPComponent) VerifyLock(ctx context.Context, req types.VerifyLFSLockReq) (*types.LFSLockListVerify, error) {
	var (
		ourLocks   []*types.LFSLock
		theirLocks []*types.LFSLock
		res        types.LFSLockListVerify
	)
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	user, err := c.user.FindByUsername(ctx, req.CurrentUser)
	if err != nil {
		return nil, ErrUnauthorized
	}

	allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
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

func (c *GitHTTPComponent) LfsDownload(ctx context.Context, req types.DownloadRequest) (*url.URL, error) {
	pointer := types.Pointer{Oid: req.Oid}
	repo, err := c.repo.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}
	allowed, err := c.AllowReadAccess(ctx, req.RepoType, req.Namespace, req.Name, req.CurrentUser)
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
	signedUrl, err := c.s3Client.PresignedGetObject(ctx, c.config.S3.Bucket, objectKey, ossFileExpireSeconds, reqParams)
	if err != nil {
		return nil, err
	}
	return signedUrl, nil
}

func (c *GitHTTPComponent) buildDownloadLink(req types.BatchRequest, pointer types.Pointer) string {
	return c.config.APIServer.PublicDomain + "/" + path.Join(fmt.Sprintf("%ss", req.RepoType), url.PathEscape(req.Namespace), url.PathEscape(req.Name+".git"), "info/lfs/objects", url.PathEscape(pointer.Oid))
}

// func (c *GitHTTPComponent) buildUploadLink(req types.BatchRequest, pointer types.Pointer) string {
// 	return c.config.APIServer.PublicDomain + "/" + path.Join(fmt.Sprintf("%ss", req.RepoType), url.PathEscape(req.Namespace), url.PathEscape(req.Name+".git"), "info/lfs/objects", url.PathEscape(pointer.Oid), strconv.FormatInt(pointer.Size, 10))
// }

func (c *GitHTTPComponent) buildUploadLink(req types.BatchRequest, pointer types.Pointer) string {
	objectKey := path.Join("lfs", pointer.RelativePath())
	u, err := c.s3Client.PresignedPutObject(context.Background(), c.config.S3.Bucket, objectKey, time.Hour*24)
	if err != nil {
		return c.config.APIServer.PublicDomain + "/" + path.Join(fmt.Sprintf("%ss", req.RepoType), url.PathEscape(req.Namespace), url.PathEscape(req.Name+".git"), "info/lfs/objects", url.PathEscape(pointer.Oid), strconv.FormatInt(pointer.Size, 10))
	}
	return u.String()
}

func (c *GitHTTPComponent) buildVerifyLink(req types.BatchRequest) string {
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

