package database

import (
	"context"
	"fmt"
	"time"

	"github.com/looplab/fsm"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/common/errorx"
	"opencsg.com/csghub-server/common/types"
)

type mirrorTaskStoreImpl struct {
	db *DB
}

type MirrorTaskStore interface {
	CancelOtherTasksAndCreate(ctx context.Context, task MirrorTask) (MirrorTask, error)
	Create(ctx context.Context, task MirrorTask) (MirrorTask, error)
	Update(ctx context.Context, task MirrorTask) (MirrorTask, error)
	FindByMirrorID(ctx context.Context, mirrorID int64) (*MirrorTask, error)
	Delete(ctx context.Context, ID int64) error
	GetHighestPriorityByTaskStatus(ctx context.Context, status []types.MirrorTaskStatus) (MirrorTask, error)
	SetMirrorCurrentTaskID(ctx context.Context, task MirrorTask) error
	FindByID(ctx context.Context, ID int64) (*MirrorTask, error)
	ListByStatusWithPriority(ctx context.Context, status []types.MirrorTaskStatus, per, page int) ([]MirrorTask, error)
}

func NewMirrorTaskStore() MirrorTaskStore {
	return &mirrorTaskStoreImpl{
		db: defaultDB,
	}
}

func NewMirrorTaskStoreWithDB(db *DB) MirrorTaskStore {
	return &mirrorTaskStoreImpl{
		db: db,
	}
}

type MirrorTask struct {
	ID                 int64                  `bun:",pk,autoincrement" json:"id"`
	MirrorID           int64                  `bun:",notnull" json:"mirror_id"`
	Mirror             *Mirror                `bun:"rel:belongs-to,join:mirror_id=id" json:"mirror"`
	ErrorMessage       string                 `bun:",nullzero" json:"error_message"`
	Status             types.MirrorTaskStatus `bun:",notnull" json:"status"`
	RetryCount         int                    `bun:",notnull,default:0" json:"retry_count"`
	Payload            string                 `bun:"," json:"payload"`
	Priority           types.MirrorPriority   `bun:",notnull" json:"priority"`
	BeforeLastCommitID string                 `bun:"" json:"before_last_commit_id"`
	AfterLastCommitID  string                 `bun:"" json:"after_last_commit_id"`
	Progress           int                    `bun:"" json:"progress"`

	StartedAt  time.Time `bun:",nullzero"`
	FinishedAt time.Time `bun:",nullzero"`
	times
}

type MirrorTaskWithFSM struct {
	mirrorTask *MirrorTask
	from       types.MirrorTaskStatus
	fsm        *fsm.FSM
}

const (
	MirrorContinue = "continue"
	MirrorFail     = "fail"
	MirrorSuccess  = "success"
	MirrorRetry    = "retry"
	MirrorCancel   = "cancel"
	MirrorFatal    = "fatal"
)

func NewMirrorTaskWithFSM(mt *MirrorTask) MirrorTaskWithFSM {
	return MirrorTaskWithFSM{
		mirrorTask: mt,
		from:       mt.Status,
		fsm: fsm.NewFSM(
			string(mt.Status),
			fsm.Events{
				{
					Name: MirrorContinue,
					Src: []string{
						string(types.MirrorQueued),
					},
					Dst: string(types.MirrorRepoSyncStart),
				},
				{
					Name: MirrorContinue,
					Src: []string{
						string(types.MirrorRepoSyncFinished),
					},
					Dst: string(types.MirrorLfsSyncStart),
				},
				{
					Name: MirrorFail,
					Src: []string{
						string(types.MirrorRepoSyncStart),
					},
					Dst: string(types.MirrorRepoSyncFailed),
				},
				{
					Name: MirrorFail,
					Src: []string{
						string(types.MirrorLfsSyncStart),
					},
					Dst: string(types.MirrorLfsSyncFailed),
				},
				{
					Name: MirrorFatal,
					Src: []string{
						string(types.MirrorRepoSyncFailed),
					},
					Dst: string(types.MirrorRepoSyncFatal),
				},
				{
					Name: MirrorFatal,
					Src: []string{
						string(types.MirrorLfsSyncFailed),
					},
					Dst: string(types.MirrorLfsSyncFatal),
				},
				{
					Name: MirrorSuccess,
					Src: []string{
						string(types.MirrorRepoSyncStart),
					},
					Dst: string(types.MirrorRepoSyncFinished),
				},
				{
					Name: MirrorSuccess,
					Src: []string{
						string(types.MirrorLfsSyncStart),
					},
					Dst: string(types.MirrorLfsSyncFinished),
				},
				{
					Name: MirrorCancel,
					Src: []string{
						string(types.MirrorQueued),
						string(types.MirrorRepoSyncStart),
						string(types.MirrorRepoSyncFailed),
						string(types.MirrorRepoSyncFinished),
						string(types.MirrorRepoSyncFatal),
						string(types.MirrorLfsSyncStart),
						string(types.MirrorLfsSyncFailed),
						string(types.MirrorLfsSyncFinished),
						string(types.MirrorLfsSyncFatal),
					},
					Dst: string(types.MirrorCanceled),
				},
			},
			fsm.Callbacks{
				"entry_state": func(ctx context.Context, event *fsm.Event) {
					mt.Status = types.MirrorTaskStatus(event.Dst)
				},
			},
		),
	}
}

func (m *MirrorTaskWithFSM) SubmitEvent(ctx context.Context, event string) bool {
	return m.fsm.Event(ctx, event) == nil
}

func (m *MirrorTaskWithFSM) Current() string {
	return m.fsm.Current()
}

func (m *mirrorTaskStoreImpl) Create(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	err := m.db.Operator.Core.NewInsert().Model(&task).Scan(ctx, &task)
	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) Update(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	_, err := m.db.Operator.Core.NewUpdate().Model(&task).WherePK().Exec(ctx)
	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) FindByMirrorID(ctx context.Context, mirrorID int64) (*MirrorTask, error) {
	var task MirrorTask
	err := m.db.Operator.Core.NewSelect().Model(&task).Where("mirror_id = ?", mirrorID).Scan(ctx)
	return &task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) Delete(ctx context.Context, ID int64) error {
	var task MirrorTask
	task.ID = ID
	_, err := m.db.Operator.Core.NewDelete().Model(&task).WherePK().Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) GetHighestPriorityByTaskStatus(ctx context.Context, status []types.MirrorTaskStatus) (MirrorTask, error) {
	var task MirrorTask
	if len(status) == 0 {
		status = []types.MirrorTaskStatus{
			types.MirrorQueued,
		}
	}
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		err := m.db.Operator.Core.NewSelect().
			Model(&task).
			Relation("Mirror").
			Relation("Mirror.Repository").
			Where("mirror_task.status in (?)", bun.In(status)).
			OrderExpr("mirror_task.priority desc, mirror_task.updated_at desc").
			For("UPDATE OF mirror_task SKIP LOCKED").
			Scan(ctx)
		if err != nil {
			return err
		}
		tFSM := NewMirrorTaskWithFSM(&task)
		canContinue := tFSM.SubmitEvent(ctx, MirrorContinue)
		if !canContinue {
			return fmt.Errorf("mirror task status %s not allow to continue", task.Status)
		}
		task.Status = types.MirrorTaskStatus(tFSM.Current())
		_, err = m.db.Operator.Core.NewUpdate().Model(&task).WherePK().Exec(ctx)
		if err != nil {
			return err
		}
		var mirror Mirror
		mirror.ID = task.MirrorID
		_, err = m.db.Operator.Core.NewUpdate().Model(&mirror).WherePK().Set("current_task_id = ?", task.ID).Exec(ctx)
		if err != nil {
			return err
		}
		return nil
	})

	return task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) SetMirrorCurrentTaskID(ctx context.Context, task MirrorTask) error {
	var mirror Mirror
	mirror.ID = task.MirrorID
	mirror.CurrentTaskID = task.ID
	_, err := m.db.Operator.Core.NewUpdate().Model(&mirror).WherePK().Column("current_task_id").Exec(ctx)
	return errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) FindByID(ctx context.Context, ID int64) (*MirrorTask, error) {
	var task MirrorTask
	task.ID = ID
	err := m.db.Operator.Core.NewSelect().
		Model(&task).
		Relation("Mirror").
		Relation("Mirror.Repository").
		Where("mirror_task.id = ?", ID).
		Scan(ctx)
	return &task, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) ListByStatusWithPriority(ctx context.Context, status []types.MirrorTaskStatus, per, page int) ([]MirrorTask, error) {
	var tasks []MirrorTask
	err := m.db.Operator.Core.NewSelect().
		Model(&tasks).
		Relation("Mirror").
		Relation("Mirror.Repository").
		Where("mirror_task.status IN (?)", bun.In(status)).
		OrderExpr("mirror_task.priority DESC, mirror_task.created_at DESC").
		Limit(per).
		Offset((page - 1) * per).
		Scan(ctx)
	return tasks, errorx.HandleDBError(err, nil)
}

func (m *mirrorTaskStoreImpl) CancelOtherTasksAndCreate(ctx context.Context, task MirrorTask) (MirrorTask, error) {
	err := m.db.Operator.Core.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().
			Model(&task).
			Set("status = ?", types.MirrorCanceled).
			Where("mirror_task.mirror_id = ?", task.MirrorID).
			Exec(ctx)
		if err != nil {
			return err
		}

		err = tx.NewInsert().
			Model(&task).
			Scan(ctx, &task)
		if err != nil {
			return err
		}
		return nil
	})
	return task, errorx.HandleDBError(err, nil)
}
