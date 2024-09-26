package component

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

var (
	MainBranch     string = "main"
	ConfigFileName string = "config.json"
	ScanLock       sync.Mutex
)

type RuntimeArchitectureComponent struct {
	r   *RepoComponent
	ras *database.RuntimeArchitecturesStore
}

func NewRuntimeArchitectureComponent(config *config.Config) (*RuntimeArchitectureComponent, error) {
	c := &RuntimeArchitectureComponent{}
	c.ras = database.NewRuntimeArchitecturesStore()
	repo, err := NewRepoComponent(config)
	if err != nil {
		return nil, fmt.Errorf("fail to create repo component, %w", err)
	}
	c.r = repo
	return c, nil
}

func (c *RuntimeArchitectureComponent) ListByRuntimeFrameworkID(ctx context.Context, id int64) ([]database.RuntimeArchitecture, error) {
	archs, err := c.ras.ListByRuntimeFrameworkID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("list runtime arch failed, %w", err)
	}
	return archs, nil
}

func (c *RuntimeArchitectureComponent) SetArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error) {
	_, err := c.r.rtfm.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime framework id, %w", err)
	}
	var failedArchs []string
	for _, arch := range architectures {
		if len(strings.Trim(arch, " ")) < 1 {
			continue
		}
		err := c.ras.Add(ctx, database.RuntimeArchitecture{
			RuntimeFrameworkID: id,
			ArchitectureName:   strings.Trim(arch, " "),
		})
		if err != nil {
			failedArchs = append(failedArchs, arch)
		}
	}
	return failedArchs, nil
}

func (c *RuntimeArchitectureComponent) DeleteArchitectures(ctx context.Context, id int64, architectures []string) ([]string, error) {
	_, err := c.r.rtfm.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime framework id, %w", err)
	}
	var failedDeletes []string
	for _, arch := range architectures {
		if len(strings.Trim(arch, " ")) < 1 {
			continue
		}
		err := c.ras.DeleteByRuntimeIDAndArchName(ctx, id, strings.Trim(arch, " "))
		if err != nil {
			failedDeletes = append(failedDeletes, arch)
		}
	}
	return failedDeletes, nil
}

func (c *RuntimeArchitectureComponent) ScanArchitecture(ctx context.Context, id int64, scanType int, models []string) error {
	frame, err := c.r.rtfm.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("invalid runtime framework id, %w", err)
	}
	archs, err := c.ras.ListByRuntimeFrameworkID(ctx, id)
	if err != nil {
		return fmt.Errorf("list runtime arch failed, %w", err)
	}
	var archMap map[string]string = make(map[string]string)
	for _, arch := range archs {
		archMap[arch.ArchitectureName] = arch.ArchitectureName
	}

	if ScanLock.TryLock() {
		go func() {
			slog.Info("scan models to update runtime frameworks started")
			defer ScanLock.Unlock()
			if scanType == 0 || scanType == 2 {
				err := c.scanExistModels(ctx, types.ScanReq{
					FrameID:   id,
					FrameType: frame.Type,
					ArchMap:   archMap,
					Models:    models,
				})
				if err != nil {
					slog.Any("scan old models failed", slog.Any("error", err))
				}
			}

			if scanType == 0 || scanType == 1 {
				err := c.scanNewModels(ctx, types.ScanReq{
					FrameID:   id,
					FrameType: frame.Type,
					ArchMap:   archMap,
					Models:    models,
				})
				if err != nil {
					slog.Any("scan new models failed", slog.Any("error", err))
				}
			}
			slog.Info("scan models to update runtime frameworks done")
		}()
	} else {
		return fmt.Errorf("architecture scan is already in progress")
	}
	return nil
}

func (c *RuntimeArchitectureComponent) scanNewModels(ctx context.Context, req types.ScanReq) error {
	repos, err := c.r.repo.GetRepoWithoutRuntimeByID(ctx, req.FrameID, req.Models)
	if err != nil {
		return fmt.Errorf("failed to get repos without runtime by ID, %w", err)
	}
	if repos == nil {
		return nil
	}
	for _, repo := range repos {
		fields := strings.Split(repo.Path, "/")
		arch, err := c.GetArchitectureFromConfig(ctx, fields[0], fields[1])
		if err != nil {
			slog.Warn("did not to get arch for create relation", slog.Any("ConfigFileName", ConfigFileName), slog.Any("repo", repo.Path), slog.Any("error", err))
			continue
		}
		if len(arch) < 1 {
			continue
		}
		_, exist := req.ArchMap[arch]
		if !exist {
			continue
		}
		err = c.r.rrtfms.Add(ctx, req.FrameID, repo.ID, req.FrameType)
		if err != nil {
			slog.Warn("fail to create relation", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
		}
	}
	return nil
}

func (c *RuntimeArchitectureComponent) scanExistModels(ctx context.Context, req types.ScanReq) error {
	repos, err := c.r.repo.GetRepoWithRuntimeByID(ctx, req.FrameID, req.Models)
	if err != nil {
		return fmt.Errorf("fail to get repos with runtime by ID, %w", err)
	}
	if repos == nil {
		return nil
	}
	for _, repo := range repos {
		fields := strings.Split(repo.Path, "/")
		arch, err := c.GetArchitectureFromConfig(ctx, fields[0], fields[1])
		if err != nil {
			slog.Warn("did not to get arch for remove relation", slog.Any("ConfigFileName", ConfigFileName), slog.Any("repo", repo.Path), slog.Any("error", err))
			continue
		}
		if len(arch) < 1 {
			continue
		}
		_, exist := req.ArchMap[arch]
		if exist {
			continue
		}
		err = c.r.rrtfms.Delete(ctx, req.FrameID, repo.ID, req.FrameType)
		if err != nil {
			slog.Warn("fail to remove relation", slog.Any("repo", repo.Path), slog.Any("frameid", req.FrameID), slog.Any("error", err))
		}
	}
	return nil
}

func (c *RuntimeArchitectureComponent) GetArchitectureFromConfig(ctx context.Context, namespace, name string) (string, error) {
	content, err := c.getConfigContent(ctx, namespace, name)
	if err != nil {
		return "", fmt.Errorf("fail to read config.json for relation, %w", err)
	}
	var config struct {
		Architectures []string `json:"architectures"`
	}
	if err := json.Unmarshal([]byte(content), &config); err != nil {
		return "", fmt.Errorf("fail to unmarshal config, %w", err)
	}
	slog.Debug("unmarshal config", slog.Any("config", config))
	if config.Architectures == nil {
		return "", nil
	}
	if len(config.Architectures) < 1 {
		return "", nil
	}
	slog.Debug("architectures of config", slog.Any("Architectures", config.Architectures))
	return config.Architectures[0], nil
}

func (c *RuntimeArchitectureComponent) getConfigContent(ctx context.Context, namespace, name string) (string, error) {
	content, err := c.r.git.GetRepoFileRaw(ctx, gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      name,
		Ref:       MainBranch,
		Path:      ConfigFileName,
		RepoType:  types.ModelRepo,
	})
	if err != nil {
		return "", fmt.Errorf("get RepoFileRaw for relation, %w", err)
	}
	return content, nil
}
