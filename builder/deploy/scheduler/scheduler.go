package scheduler

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"opencsg.com/csghub-server/component/reporter"

	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/deploy/imagebuilder"
	"opencsg.com/csghub-server/builder/deploy/imagerunner"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/redis"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	utilcommon "opencsg.com/csghub-server/common/utils/common"
)

type Scheduler interface {
	Run() error
	Queue(deployTaskID int64) error
}

// a Scheduler will run tasks in their arrival order
type FIFOScheduler struct {
	timeout time.Duration
	// parallel running tasks
	tasks chan Runner
	last  *database.DeployTask

	store               database.DeployTaskStore
	spaceStore          database.SpaceStore
	modelStore          database.ModelStore
	spaceResourcesStore database.SpaceResourceStore
	ib                  imagebuilder.Builder
	ir                  imagerunner.Runner

	nextLock    *sync.Mutex
	deployCfg   common.DeployConfig
	config      *config.Config
	redisLocker *redis.DistributedLocker
	logReporter reporter.LogCollector
	git         gitserver.GitServer
}

func NewFIFOScheduler(ib imagebuilder.Builder, ir imagerunner.Runner, c common.DeployConfig, logReporter reporter.LogCollector) (Scheduler, error) {
	s := &FIFOScheduler{}
	// TODO:allow config
	s.timeout = 30 * time.Minute
	s.store = database.NewDeployTaskStore()
	s.spaceStore = database.NewSpaceStore()
	s.modelStore = database.NewModelStore()
	s.spaceResourcesStore = database.NewSpaceResourceStore()
	// allow concurrent deployment tasks
	s.tasks = make(chan Runner, 100)
	// s.ib = imagebuilder.NewLocalBuilder()
	// s.ir = imagerunner.NewLocalRunner()
	s.ib = ib
	s.ir = ir
	s.nextLock = &sync.Mutex{}
	s.deployCfg = c
	//TODO: avoid load config, use config from params
	s.config, _ = config.LoadConfig()
	s.redisLocker = c.RedisLocker
	s.logReporter = logReporter

	gc, err := git.NewGitServer(s.config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}

	s.git = gc

	return s, nil
}

// Run will load tasks and run them currently
func (rs *FIFOScheduler) Run() error {
	slog.Info("FIFOScheduler run started")

	go func() {
		for count := 0; count <= cap(rs.tasks); count++ {
			_, err := rs.next()
			if err != nil {
				slog.Error("failed to get next task", "error", err)
				continue
			}
		}
	}()

	slog.Debug("scheduler try to loop through tasks channel")
	for t := range rs.tasks {
		go func(t Runner) {
			slog.Debug("dequeue a task to run", slog.Any("task", t.WatchID()))
			ctx, cancel := context.WithTimeout(context.Background(), rs.timeout)
			defer cancel()

			if err := t.Run(ctx); err != nil {
				slog.Error("failed to run task", slog.Any("error", err), slog.Any("task", t.WatchID()))
				rs.failDeployFollowingTasks(t.WatchID(), err.Error())
			}

			_, _ = rs.next()
		}(t)
	}

	return nil
}

func (rs *FIFOScheduler) Queue(deployTaskID int64) error {
	// simply trigger next task
	_, err := rs.next()

	return err
}

// run next task
func (rs *FIFOScheduler) next() (Runner, error) {
	var (
		deployTask *database.DeployTask
		t          Runner
		err        error
	)
	// get lock here to prevent concurrent access to the same task
	errLock := rs.redisLocker.GetDeployTaskSchedulerLock()
	if errLock != nil && errors.Is(errLock, redis.ErrLockAcquire) {
		slog.Debug("skip schedule deploy task due to fail getting lock", slog.Any("error", errLock))
		t = &sleepTask{
			du: 5 * time.Second,
		}
		rs.tasks <- t
		return t, nil
	}
	defer func() {
		if errLock != nil {
			slog.Warn("fail to getting lock in deploy task scheduler", slog.Any("error", errLock))
		} else {
			ok, err := rs.redisLocker.ReleaseDeployTaskSchedulerLock()
			if !ok || err != nil {
				slog.Error("failed to release deploy task scheduler lock", slog.Any("success", ok), slog.Any("error", err))
			}
		}
	}()

	rs.nextLock.Lock()
	slog.Debug("FIFOScheduler try to get next task", slog.Any("last", rs.last))
	defer rs.nextLock.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if rs.last == nil {
		deployTask, err = rs.store.GetNewTaskFirst(ctx)
		slog.Debug("GetNewTaskFirst", slog.Any("deploy_task", deployTask), slog.Any("error", err))
	} else {
		deployTask, err = rs.store.GetNewTaskAfter(ctx, rs.last.ID)
		slog.Debug("GetNewTaskAfter", slog.Any("deploy_task", deployTask), slog.Any("last", rs.last.ID), slog.Any("error", err))
	}
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Debug("no more tasks to run, schedule a sleeping task")
			// using a sleep task to pause the scheduler
		} else {
			slog.Error("FIFOScheduler cannot get next task by db error", slog.Any("error", err))
		}

		t = &sleepTask{
			du: 5 * time.Second,
		}
		rs.tasks <- t
		return t, nil
	}

	var repo RepoInfo

	if deployTask.Deploy.SpaceID > 0 {
		// handle space
		var s *database.Space
		s, err = rs.spaceStore.ByID(ctx, deployTask.Deploy.SpaceID)
		if err == nil {
			repoCloneInfo := utilcommon.BuildCloneInfo(rs.config, s.Repository)
			repo.Path = s.Repository.Path
			repo.Name = s.Repository.Name
			repo.Sdk = s.Sdk
			repo.SdkVersion = s.SdkVersion
			repo.DriverVersion = s.DriverVersion
			repo.HTTPCloneURL = repoCloneInfo.HTTPCloneURL
			repo.SpaceID = s.ID
			repo.RepoID = s.Repository.ID
			repo.UserName = s.Repository.User.Username
			repo.DeployID = deployTask.Deploy.ID
			repo.ModelID = 0
			repo.RepoType = string(types.SpaceRepo)
		}
	} else if deployTask.Deploy.ModelID > 0 {
		// handle model
		var m *database.Model
		m, err = rs.modelStore.ByID(ctx, deployTask.Deploy.ModelID)
		if err == nil {
			repo.Path = m.Repository.Path
			repo.Name = m.Repository.Name
			repo.ModelID = m.ID
			repo.RepoID = m.Repository.ID
			repo.UserName = m.Repository.User.Username
			repo.DeployID = deployTask.Deploy.ID
			repo.SpaceID = 0
			repo.RepoType = string(types.ModelRepo)
		}
	}

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			slog.Warn("cancel deploy task as repo not found", slog.Any("deploy_task", deployTask))
			// mark task as cancelled
			deployTask.Status = Cancelled
			deployTask.Message = "repo not found"
			err = rs.store.UpdateDeployTask(ctx, deployTask)
			if err != nil {
				slog.Error("update deploy task failed", "error", err)
			}
		}
		t = &sleepTask{
			du: 5 * time.Second,
		}
		rs.last = deployTask
		rs.tasks <- t
		return t, nil
	}
	// for build task
	if deployTask.TaskType == 0 {
		t, err = NewBuildRunner(rs.git, rs.ib, &repo, deployTask, rs.logReporter, rs.config.Runner.HearBeatIntervalInSec)
		if err != nil {
			slog.Error("failed to create build runner", slog.Any("error", err))
			return nil, err
		}
	} else {
		t = NewDeployRunner(rs.ir, &repo, deployTask, rs.deployCfg, rs.logReporter)
	}

	rs.last = deployTask
	rs.tasks <- t
	slog.Info("enqueue next task", slog.Any("task", t.WatchID()))
	return t, err
}

func (rs *FIFOScheduler) failDeployFollowingTasks(deploytaskID int64, reason string) {
	slog.Info("scheduler fail following tasks", slog.Any("deploy_task_id", deploytaskID))
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	t, _ := rs.store.GetDeployTask(ctx, deploytaskID)

	dps, err := rs.store.GetDeployTasksOfDeploy(ctx, t.DeployID)
	if err != nil {
		slog.Error("failed to get tasks of deploy when check build status", slog.Any("error", err),
			slog.Int64("deploy_id", t.DeployID))
		return
	}

	// update following tasks to be failed to stop scheduler to run it
	for _, dp := range dps {
		// fail current task
		if dp.ID == t.ID {
			dp.Status = BuildFailed
			dp.Message = reason
			continue
		}
		// tasks after current task
		if dp.ID > t.ID {
			dp.Status = Cancelled
			dp.Message = "cancel as previous task failed"
		} else {
			dp.Status = deployFailed
			dp.Message = reason
		}
	}
	if err := rs.store.UpdateInTx(ctx, nil, []string{"status", "message"}, nil, dps...); err != nil {
		slog.Error("failed update deploy status to `BuildFailed`", slog.Int64("deploy_task_id", t.ID), "error", err)
		return
	}

	deploy, err := rs.store.GetDeployByID(ctx, t.DeployID)
	if err != nil {
		slog.Error("failed to get deploy when check build status", slog.Any("error", err),
			slog.Int64("deploy_id", t.DeployID))
		return
	}
	deploy.Status = common.DeployFailed
	deploy.Message = reason
	rs.logReporter.Report(types.LogEntry{
		Message:  fmt.Sprintf("%s, task status: %d", reason, deploy.Status),
		Stage:    types.StageDeploy,
		Step:     types.StepDeployFailed,
		DeployID: strconv.FormatInt(t.DeployID, 10),
		Labels: map[string]string{
			types.LogLabelTypeKey:       types.LogLabelDeploy,
			types.StreamKeyDeployType:   strconv.Itoa(deploy.Type),
			types.StreamKeyDeployTaskID: strconv.FormatInt(deploytaskID, 10),
		},
	})
	if err := rs.store.UpdateDeploy(ctx, deploy); err != nil {
		slog.Error("failed update deploy status to `DeployFailed`", slog.Int64("deploy_id", t.DeployID), "error", err)
		return
	}

}
