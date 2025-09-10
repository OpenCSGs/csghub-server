package git

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"opencsg.com/csghub-server/builder/git/gitserver/gitaly"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
)

var req = &gitaly.ProjectStorageCloneRequest{}

func serve(server *http.Server) {
	http.Handle("/", http.FileServer(http.Dir(".")))

	log.Printf("Serving files on HTTP port: %s\n", "8100")
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		// unexpected error. port in use?
		log.Fatalf("ListenAndServe(): %v", err)
	}
}

func init() {
	// Adding flags based on the struct fields
	cloneProjectStorageCmd.Flags().StringVar(&req.CurrentGitalyAddress, "ca", "", "Current Gitaly address")
	cloneProjectStorageCmd.Flags().StringVar(&req.CurrentGitalyToken, "ct", "", "Current Gitaly token")
	cloneProjectStorageCmd.Flags().StringVar(&req.CurrentGitalyStorage, "cs", "", "Current Gitaly storage")
	cloneProjectStorageCmd.Flags().StringVar(&req.NewGitalyAddress, "na", "", "New Gitaly address")
	cloneProjectStorageCmd.Flags().StringVar(&req.NewGitalyToken, "nt", "", "New Gitaly token")
	cloneProjectStorageCmd.Flags().StringVar(&req.NewGitalyStorage, "ns", "", "New Gitaly storage")
	cloneProjectStorageCmd.Flags().StringVar(&req.FilesServer, "fs", "http://host.docker.internal:8100/", "Local files server address")
	cloneProjectStorageCmd.Flags().IntVar(&req.Concurrency, "parallel", 1, "Concurrency level for storage clone (default: 1)")
}

var cloneProjectStorageCmd = &cobra.Command{
	Use:   "clone-project-storage",
	Short: "clone project gitaly storage",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("failed to load config,%w", err)
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		if err := database.InitDB(dbConfig); err != nil {
			slog.Error("failed to initialize database", slog.Any("error", err))
			return fmt.Errorf("database initialization failed: %w", err)
		}
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		server := &http.Server{Addr: ":8100"}
		defer func() { _ = server.Close() }()
		go serve(server)
		ctx := cmd.Context()
		repoStore := database.NewRepoStore()
		var i int
		for {
			repos, err := repoStore.FindWithBatch(ctx, 200, i)
			i += 1
			if err != nil {
				slog.Error("get repositories failed", slog.Any("err", err))
				return
			}
			if len(repos) == 0 {
				break
			}
			slog.Info("get repos", "count", len(repos))
			helper, err := gitaly.NewCloneStorageHelper(req)
			if err != nil {
				slog.Error("create helper failed", slog.Any("err", err))
				return
			}
			g, ctx := errgroup.WithContext(ctx)
			if req.Concurrency == 0 {
				req.Concurrency = 1
			}
			g.SetLimit(req.Concurrency)
			for _, repo := range repos {
				g.Go(func() error {
					err := helper.CloneRepoStorage(ctx, repo.GitPath+".git", req)
					if err != nil {
						slog.Error("clone storage failed", "path", repo.GitPath, "error", err)
					} else {
						slog.Info("clone storage success", "path", repo.GitPath)
					}
					return nil
				})
			}
			err = g.Wait()
			if err != nil {
				slog.Error("clone storage failed", slog.Any("err", err))
			}
		}
	},
}
