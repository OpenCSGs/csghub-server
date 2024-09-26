package git

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var generateLfsMetaObjectsCmd = &cobra.Command{
	Use:   "generate-lfs-meta-objects",
	Short: "the cmd to generate lfs meta objects",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			return fmt.Errorf("initializing DB connection: %w", err)
		}
		ctx := context.WithValue(cmd.Context(), "config", config)
		cmd.SetContext(ctx)
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		ctx := cmd.Context()
		config, ok := ctx.Value("config").(*config.Config)
		if !ok {
			slog.Error("config not found in context")
			return
		}

		if config.GitServer.Type == types.GitServerTypeGitea {
			return
		}

		s3Client, err := s3.NewMinio(config)
		if err != nil {
			newError := fmt.Errorf("fail to init s3 client for code,error:%w", err)
			slog.Error(newError.Error())
			return
		}

		lfsMetaObjectStore := database.NewLfsMetaObjectStore()
		repoStore := database.NewRepoStore()

		gitServer, err := git.NewGitServer(config)
		if err != nil {
			newError := fmt.Errorf("fail to create git server,error:%w", err)
			slog.Error(newError.Error())
			return
		}

		var i int
		for {
			queryCtx, queryCancel := context.WithTimeout(context.Background(), time.Second*15)
			defer queryCancel()
			repos, err := repoStore.FindWithBatch(queryCtx, 1000, i)
			i += 1
			if err != nil {
				slog.Error("fail to batch get repositories", slog.Any("err", err))
				return
			}
			if len(repos) == 0 {
				break
			}
			for _, repo := range repos {
				err := fetchAllPointersForRepo(config, gitServer, s3Client, lfsMetaObjectStore, repo)
				if err != nil {
					slog.Error("fail to fetch all pointers for repository", slog.Any("err", err))
					continue
				}
			}
		}
	},
}

func fetchAllPointersForRepo(config *config.Config, gitServer gitserver.GitServer, s3Client *minio.Client, lfsMetaObjectStore *database.LfsMetaObjectStore, repo database.Repository) error {
	namespace := strings.Split(repo.Path, "/")[0]
	name := strings.Split(repo.Path, "/")[1]
	ref := repo.DefaultBranch
	if ref == "" {
		ref = "main"
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*5)
	defer cancel()
	slog.Info("start to fetch all pointers for repository", slog.String("namespace", namespace), slog.String("name", name), slog.String("ref", ref))
	lfsPointers, err := gitServer.GetRepoAllLfsPointers(ctx, gitserver.GetRepoAllFilesReq{
		Namespace: namespace,
		Name:      name,
		Ref:       ref,
		RepoType:  repo.RepositoryType,
	})
	if err != nil {
		return err
	}

	for _, lfsPointer := range lfsPointers {
		pointer := types.Pointer{
			Oid:  lfsPointer.FileOid,
			Size: lfsPointer.FileSize,
		}
		checkAndUpdateLfsMetaObjects(config, s3Client, lfsMetaObjectStore, repo, &pointer)
	}
	return nil
}

func checkAndUpdateLfsMetaObjects(config *config.Config, s3Client *minio.Client, lfsMetaObjectStore *database.LfsMetaObjectStore, repo database.Repository, pointer *types.Pointer) {
	var exists bool
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	objectKey := path.Join("lfs", pointer.RelativePath())
	_, err := s3Client.StatObject(ctx, config.S3.Bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if os.IsNotExist(err) {
			exists = false
		}
		slog.Error("failed to check if lfs file exists", slog.String("oid", objectKey), slog.Any("error", err))
		exists = false
	} else {
		exists = true
	}
	slog.Info("lfs file exists", slog.Bool("exists", exists))
	lfsMetaObjectStore.UpdateOrCreate(ctx, database.LfsMetaObject{
		Oid:          pointer.Oid,
		Size:         pointer.Size,
		RepositoryID: repo.ID,
		Existing:     exists,
	})
}
