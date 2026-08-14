package component

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	commonutil "opencsg.com/csghub-server/common/utils/common"
)

const (
	repositoryPackageSyncTimeout = 10 * time.Minute
	repositoryPackageContentType = "application/zip"
)

type repositoryPackageSyncer struct {
	config             *config.Config
	repos              database.RepoStore
	git                gitserver.GitServer
	s3Client           s3.Client
	agentTemplateStore database.AgentTemplateStore
}

type repositoryPackageWriteFunc func(ctx context.Context, repoType types.RepositoryType, repoID int64, commitID string, archive []byte) error

type RepositoryPackageSyncer interface {
	SyncBranch(ctx context.Context, repoType types.RepositoryType, namespace, name, branch string) error
	SyncRepoBranch(ctx context.Context, repo *database.Repository, namespace, name, branch string) error
	RemoveRepoPackages(ctx context.Context, repoType types.RepositoryType, repoID int64) error
}

func NewRepositoryPackageSyncer(config *config.Config, repos database.RepoStore, git gitserver.GitServer, s3Client s3.Client) RepositoryPackageSyncer {
	return newRepositoryPackageSyncer(config, repos, git, s3Client, database.NewAgentTemplateStore())
}

func newRepositoryPackageSyncer(config *config.Config, repos database.RepoStore, git gitserver.GitServer, s3Client s3.Client, agentTemplateStore database.AgentTemplateStore) *repositoryPackageSyncer {
	return &repositoryPackageSyncer{
		config:             config,
		repos:              repos,
		git:                git,
		s3Client:           s3Client,
		agentTemplateStore: agentTemplateStore,
	}
}

func supportedRepositoryPackageType(repoType types.RepositoryType) bool {
	return repoType == types.SkillRepo || repoType == types.CodeRepo
}

func repositoryPackageTypePrefix(repoType types.RepositoryType) string {
	return fmt.Sprintf("%ss", repoType)
}

func repositoryPackageRepoPrefix(repoType types.RepositoryType, repoID int64) string {
	sha256Path := commonutil.SHA256(strconv.FormatInt(repoID, 10))
	return fmt.Sprintf("%s/%s/%s/%s/", repositoryPackageTypePrefix(repoType), sha256Path[0:2], sha256Path[2:4], sha256Path)
}

func repositoryPackageCommitObjectKey(repoType types.RepositoryType, repoID int64, commitID string) string {
	return repositoryPackageRepoPrefix(repoType, repoID) + commitID + ".zip"
}

func normalizeRepositoryPackageBranch(branch, defaultBranch string) string {
	branch = strings.TrimSpace(strings.TrimPrefix(branch, "refs/heads/"))
	if branch != "" {
		return branch
	}
	defaultBranch = strings.TrimSpace(defaultBranch)
	if defaultBranch != "" {
		return defaultBranch
	}
	return types.MainBranch
}

func repositoryPackageNamespaceAndName(repo *database.Repository, namespace, name string) (string, string) {
	if namespace != "" && name != "" {
		return namespace, name
	}
	if repo != nil && repo.Path != "" {
		parts := strings.SplitN(repo.Path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
	}
	return namespace, name
}

func repositoryPackagePath(namespace, name string) string {
	if namespace == "" || name == "" {
		return ""
	}
	return namespace + "/" + name
}

func (s *repositoryPackageSyncer) supportedRepositoryPackageType(ctx context.Context, repoType types.RepositoryType, repoPath string) (bool, error) {
	switch repoType {
	case types.SkillRepo:
		return true, nil
	case types.CodeRepo:
		if s == nil || s.agentTemplateStore == nil || strings.TrimSpace(repoPath) == "" {
			return false, nil
		}
		templates, err := s.agentTemplateStore.FindByTypeAndName(ctx, "csgclaw", strings.Trim(repoPath, "/"))
		if err != nil {
			return false, err
		}
		return len(templates) > 0, nil
	default:
		return false, nil
	}
}

func (s *repositoryPackageSyncer) supportedRepositoryPackage(ctx context.Context, repo *database.Repository, namespace, name string) (bool, error) {
	if repo == nil {
		return false, nil
	}
	namespace, name = repositoryPackageNamespaceAndName(repo, namespace, name)
	return s.supportedRepositoryPackageType(ctx, repo.RepositoryType, repositoryPackagePath(namespace, name))
}

func (s *repositoryPackageSyncer) supportedRepositoryPackageByID(ctx context.Context, repoType types.RepositoryType, repoID int64) (bool, error) {
	if repoType == types.SkillRepo {
		return true, nil
	}
	if repoType != types.CodeRepo || s == nil || s.repos == nil {
		return false, nil
	}
	repo, err := s.repos.FindById(ctx, repoID)
	if err != nil {
		return false, fmt.Errorf("failed to find repository for package write: %w", err)
	}
	if repo == nil || repo.RepositoryType != repoType {
		return false, nil
	}
	return s.supportedRepositoryPackageType(ctx, repoType, repo.Path)
}

func (s *repositoryPackageSyncer) resolveBranchHeadCommit(ctx context.Context, repoType types.RepositoryType, namespace, name, branch string) (string, error) {
	currentBranch, err := s.git.GetRepoBranchByName(ctx, gitserver.GetBranchReq{
		Namespace: namespace,
		Name:      name,
		Ref:       branch,
		RepoType:  repoType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to resolve current repository branch head: %w", err)
	}
	if currentBranch == nil || currentBranch.Commit.ID == "" {
		return "", fmt.Errorf("failed to resolve current repository branch head: empty commit id")
	}
	return currentBranch.Commit.ID, nil
}

func (s *repositoryPackageSyncer) SyncBranch(ctx context.Context, repoType types.RepositoryType, namespace, name, branch string) error {
	if s == nil || s.config == nil || s.repos == nil || s.git == nil || s.s3Client == nil {
		return nil
	}
	if !supportedRepositoryPackageType(repoType) {
		return nil
	}
	repo, err := s.repos.FindByPath(ctx, repoType, namespace, name)
	if err != nil {
		return fmt.Errorf("failed to find repository for package sync: %w", err)
	}
	return s.SyncRepoBranch(ctx, repo, namespace, name, branch)
}

func (s *repositoryPackageSyncer) SyncRepoBranch(ctx context.Context, repo *database.Repository, namespace, name, branch string) error {
	if s == nil || s.config == nil || s.git == nil || s.s3Client == nil || repo == nil {
		return nil
	}
	if !supportedRepositoryPackageType(repo.RepositoryType) {
		return nil
	}
	namespace, name = repositoryPackageNamespaceAndName(repo, namespace, name)
	if namespace == "" || name == "" {
		return fmt.Errorf("missing repository namespace or name for package sync")
	}
	syncCtx, cancel := context.WithTimeout(ctx, repositoryPackageSyncTimeout)
	defer cancel()

	supported, err := s.supportedRepositoryPackage(syncCtx, repo, namespace, name)
	if err != nil {
		return err
	}
	if !supported {
		return nil
	}

	branch = normalizeRepositoryPackageBranch(branch, repo.DefaultBranch)

	commitID, err := s.resolveBranchHeadCommit(syncCtx, repo.RepositoryType, namespace, name, branch)
	if err != nil {
		return err
	}

	archive, err := s.git.GetArchive(syncCtx, gitserver.GetArchiveReq{
		Namespace: namespace,
		Name:      name,
		Revision:  commitID,
		RepoType:  repo.RepositoryType,
	})
	if err != nil {
		return fmt.Errorf("failed to archive repository package: %w", err)
	}
	if err := writeRepositoryPackageToS3(syncCtx, s.config, s.s3Client, repo.RepositoryType, repo.ID, commitID, archive); err != nil {
		return err
	}
	return nil
}

func (s *repositoryPackageSyncer) WriteArchive(ctx context.Context, repoType types.RepositoryType, repoID int64, commitID string, archive []byte) error {
	if s == nil {
		return nil
	}
	supported, err := s.supportedRepositoryPackageByID(ctx, repoType, repoID)
	if err != nil {
		return err
	}
	if !supported {
		return nil
	}
	return writeRepositoryPackageToS3(ctx, s.config, s.s3Client, repoType, repoID, commitID, archive)
}

func writeRepositoryPackageToS3(ctx context.Context, cfg *config.Config, s3Client s3.Client, repoType types.RepositoryType, repoID int64, commitID string, archive []byte) error {
	if cfg == nil || s3Client == nil || !supportedRepositoryPackageType(repoType) {
		return nil
	}
	objectKey := repositoryPackageCommitObjectKey(repoType, repoID, commitID)
	_, err := s3Client.PutObject(ctx, cfg.S3.Bucket, objectKey, bytes.NewReader(archive), int64(len(archive)), minio.PutObjectOptions{
		ContentType: repositoryPackageContentType,
	})
	if err != nil {
		return fmt.Errorf("failed to upload repository package: %w", err)
	}
	return nil
}

func asyncWriteRepositoryPackageArchive(ctx context.Context, repoType types.RepositoryType, repoID int64, commitID string, archive []byte, write repositoryPackageWriteFunc, logMessage string) {
	if write == nil {
		return
	}
	go func(archiveData []byte) {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), repositoryPackageSyncTimeout)
		defer cancel()
		if err := write(writeCtx, repoType, repoID, commitID, archiveData); err != nil {
			slog.WarnContext(writeCtx, logMessage,
				slog.Any("error", err),
				slog.String("repo_type", string(repoType)),
				slog.Int64("repo_id", repoID),
				slog.String("commit_id", commitID),
			)
		}
	}(archive)
}

func (s *repositoryPackageSyncer) RemoveRepoPackages(ctx context.Context, repoType types.RepositoryType, repoID int64) error {
	if s == nil || s.config == nil || s.s3Client == nil {
		return nil
	}
	if !supportedRepositoryPackageType(repoType) {
		return nil
	}
	prefix := repositoryPackageRepoPrefix(repoType, repoID)
	objects := s.s3Client.ListObjects(ctx, s.config.S3.Bucket, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	removeErrors := s.s3Client.RemoveObjects(ctx, s.config.S3.Bucket, objects, minio.RemoveObjectsOptions{})
	for removeErr := range removeErrors {
		if removeErr.Err != nil {
			return fmt.Errorf("failed to remove repository package object %s: %w", removeErr.ObjectName, removeErr.Err)
		}
	}
	return nil
}

func readRepositoryPackageFromS3(ctx context.Context, cfg *config.Config, s3Client s3.Client, repoType types.RepositoryType, repoID int64, branch, commitID string) ([]byte, bool) {
	if cfg == nil || s3Client == nil || !supportedRepositoryPackageType(repoType) {
		return nil, false
	}
	objectKey := repositoryPackageCommitObjectKey(repoType, repoID, commitID)
	return readRepositoryPackageObjectFromS3(ctx, cfg, s3Client, objectKey)
}

func readRepositoryPackageObjectFromS3(ctx context.Context, cfg *config.Config, s3Client s3.Client, objectKey string) ([]byte, bool) {
	_, err := s3Client.StatObject(ctx, cfg.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if !noSuchKey(err) {
			slog.WarnContext(ctx, "failed to stat repository package in s3",
				slog.Any("error", err),
				slog.String("object_key", objectKey),
			)
		}
		return nil, false
	}
	object, err := s3Client.GetObject(ctx, cfg.S3.Bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		slog.WarnContext(ctx, "failed to read repository package from s3",
			slog.Any("error", err),
			slog.String("object_key", objectKey),
		)
		return nil, false
	}
	if object == nil {
		return nil, false
	}
	defer object.Close()
	archiveData, err := io.ReadAll(object)
	if err != nil {
		slog.WarnContext(ctx, "failed to read repository package body from s3",
			slog.Any("error", err),
			slog.String("object_key", objectKey),
		)
		return nil, false
	}
	return archiveData, true
}
