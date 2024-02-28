package logscan

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var logPath string
var (
	repoStore *database.RepoStore
)

func init() {
	initCmd.Flags().StringVar(&logPath, "path", "", "log path of log file")
	Cmd.AddCommand(
		initCmd,
	)
}

var Cmd = &cobra.Command{
	Use:   "logscan",
	Short: "scan gitserver log to count the models and datasets downloads",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) (err error) {
		config, err := config.LoadConfig()
		if err != nil {
			return
		}

		dbConfig := database.DBConfig{
			Dialect: database.DatabaseDialect(config.Database.Driver),
			DSN:     config.Database.DSN,
		}

		database.InitDB(dbConfig)
		if err != nil {
			err = fmt.Errorf("initializing DB connection: %w", err)
			return
		}
		repoStore = database.NewRepoStore()
		return
	},
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var initCmd = &cobra.Command{
	Use:   "gitea",
	Short: "scan gitea log to count the models and datasets downloads",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(logPath)
		// Open log file
		file, err := os.Open(logPath)
		if err != nil {
			fmt.Println("Error opening file:", err)
			return err
		}
		defer file.Close()

		// use regexp to find the clone log
		// http clone log example: 2023/12/27 06:36:28 ...eb/routing/logger.go:102:func1() [I] router: completed GET /models_wayne0019/lwftest.git/info/refs?service=git-upload-pack
		httpPattern := regexp.MustCompile(`(\d{4}\/\d{2}\/\d{2}) \d{2}:\d{2}:\d{2}.*completed GET \/(models|datasets)_([\w_-]+\/[\w_-]+)\.git\/info\/refs\?service=git-upload-pack`)
		// ssh clone log example: 2023/12/27 06:38:04 ...eb/routing/logger.go:102:func1() [I] router: completed GET /api/internal/serv/command/15/models_zzz/test?mode=1&verb=git-upload-pack for 127.0.0.1:0, 200 OK in 3.7ms @ private/serv.go:79(private.ServCommand)
		sshPattern := regexp.MustCompile(`(\d{4}\/\d{2}\/\d{2}) \d{2}:\d{2}:\d{2}.*completed GET \/api\/internal\/serv\/command\/\d+\/(models|datasets)_([\w_-]+\/[\w_-]+)\?mode=1&verb=git-upload-pack`)

		// count clone action for each project
		projectCount := make(map[string]map[string]map[string]int64)

		// read log file line by line
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			httpMatches := httpPattern.FindStringSubmatch(line)
			if len(httpMatches) == 4 {
				date := httpMatches[1]
				repoType := httpMatches[2]
				repoPath := httpMatches[3]
				if projectCount[date] == nil {
					projectCount[date] = make(map[string]map[string]int64)
				}

				if projectCount[date][repoType] == nil {
					projectCount[date][repoType] = make(map[string]int64)
				}

				if _, ok := projectCount[date][repoType][repoPath]; ok {
					projectCount[date][repoType][repoPath]++
				} else {
					projectCount[date][repoType][repoPath] = 1
				}
			}
			sshMatches := sshPattern.FindStringSubmatch(line)
			if len(sshMatches) == 4 {
				date := sshMatches[1]
				repoType := sshMatches[2]
				repoPath := sshMatches[3]
				if projectCount[date] == nil {
					projectCount[date] = make(map[string]map[string]int64)
				}

				if projectCount[date][repoType] == nil {
					projectCount[date][repoType] = make(map[string]int64)
				}

				if _, ok := projectCount[date][repoType][repoPath]; ok {
					projectCount[date][repoType][repoPath]++
				} else {
					projectCount[date][repoType][repoPath] = 1
				}
			}
		}

		for date, typeMap := range projectCount {
			for repoTypeString, pathMap := range typeMap {
				for path, count := range pathMap {
					fmt.Printf("date: %s, type: %s, path: %s, count: %d\n", date, repoTypeString, path, count)
					repoType := getRepoTypeByTypeName(repoTypeString)
					spPath := strings.Split(path, "/")
					repo, err := repoStore.FindByPath(cmd.Context(), repoType, spPath[0], spPath[1])
					if err != nil {
						fmt.Printf("Error finding %s: %s, Date: %s, Count: %d error: %v\n", repoType, path, date, count, err)
						continue
					}
					pDate, _ := time.Parse("2006/01/02", date)
					err = repoStore.UpdateRepoCloneDownloads(cmd.Context(), repo, pDate, count)
					if err != nil {
						fmt.Printf("Error updating %s: %s, Date: %s, Count: %d error: %v\n", repoType, path, date, count, err)
					}
					fmt.Printf("Update secceed %s: %s, Date: %s, Count: %d error: %v\n", repoType, path, date, count, err)
				}
			}
		}
		// check error
		if err := scanner.Err(); err != nil {
			fmt.Println("Error reading file:", err)
			return err
		}
		return nil
	},
}

func getRepoTypeByTypeName(repoType string) types.RepositoryType {
	switch repoType {
	case "datasets":
		return types.DatasetRepo
	case "models":
		return types.ModelRepo
	case "codes":
		return types.CodeRepo
	case "spaces":
		return types.SpaceRepo
	default:
		return types.UnknownRepo
	}
}
