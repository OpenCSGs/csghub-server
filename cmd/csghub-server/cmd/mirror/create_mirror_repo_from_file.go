package mirror

import (
	"bufio"
	"context"
	"encoding/csv"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

var (
	filePath string
	lfs      bool
)

func init() {
	createMirrorRepoFromFile.Flags().StringVar(&filePath, "file", "", "the path of the file")
	createMirrorRepoFromFile.Flags().BoolVar(&lfs, "lfs", false, "sync lfs file")
}

var createMirrorRepoFromFile = &cobra.Command{
	Use:   "create-mirror-from-file",
	Short: "the cmd to create mirror repository from file",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		if filePath == "" {
			return fmt.Errorf("empty file path")
		}
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
		if !config.Saas {
			return
		}

		c, err := component.NewMirrorComponent(config)
		if err != nil {
			slog.Error("failed to create mirror component", "err", err)
			return
		}
		mirrorSourceStore := database.NewMirrorSourceStore()
		mirrorSource, err := mirrorSourceStore.FindByName(ctx, "huggingface")
		if err != nil {
			slog.Error("failed to find mirror source, Please create mirror source first", "err", err)
			return
		}

		absPath, err := filepath.Abs(filePath)
		if err != nil {
			slog.Error("error getting absolute path:", "err", err)
			return
		}
		file, err := os.Open(absPath)
		if err != nil {
			slog.Error("error opening file:", "err", err)
			return
		}
		defer file.Close()

		reader := csv.NewReader(bufio.NewReader(file))
		reader.TrimLeadingSpace = true

		if _, err := reader.Read(); err != nil {
			slog.Error("error reading header:", "err", err)
			return
		}

		for {
			record, err := reader.Read()
			if err != nil {
				slog.Error("error reading data:", "err", err)
				break
			}

			var sourceGitCloneUrl string

			repoType := record[4]
			repoPath := record[1]
			sourceNamespace := strings.Split(repoPath, "/")[0]
			sourceName := strings.Split(repoPath, "/")[1]
			if repoType == "model" {
				sourceGitCloneUrl = fmt.Sprintf("https://huggingface.co/%s", repoPath)
			} else {
				sourceGitCloneUrl = fmt.Sprintf("https://huggingface.co/%s", fmt.Sprintf("%ss/%s", repoType, repoPath))
			}

			req := types.CreateMirrorRepoReq{
				SourceNamespace:   sourceNamespace,
				SourceName:        sourceName,
				MirrorSourceID:    mirrorSource.ID,
				RepoType:          types.RepositoryType(repoType),
				DefaultBranch:     "main",
				SourceGitCloneUrl: sourceGitCloneUrl,
				SyncLfs:           lfs,
			}
			fmt.Println(req)
			_, err = c.CreateMirrorRepo(ctx, req)
			if err != nil {
				slog.Error("error creating mirror:", "err", err)
				continue
			}
			slog.Info("create mirror successfully", slog.String("source_url", sourceGitCloneUrl))
		}
	},
}
