package executors

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	mockdb "opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

func TestImageBuilderExecutor_ProcessEvent(t *testing.T) {
	const taskID int64 = 1
	const imagePath = "812e1575865e1bc394351b38ff16f00737ef0bb5"

	tests := []struct {
		name         string
		workflow     v1alpha1.WorkflowPhase
		deployStatus int
		taskStatus   int
		wantDeploy   int
		wantTask     int
		wantWrite    bool
	}{
		{"WorkflowPending", v1alpha1.WorkflowPending, 0, common.TaskStatusBuildPending, 0, common.TaskStatusBuildPending, false},
		{"WorkflowRunning", v1alpha1.WorkflowRunning, common.BuildInQueue, common.TaskStatusBuildInQueue, common.Building, common.TaskStatusBuildInProgress, true},
		{"WorkflowSucceeded", v1alpha1.WorkflowSucceeded, common.Building, 0, common.BuildSuccess, common.TaskStatusBuildSucceed, true},
		{"WorkflowFailed", v1alpha1.WorkflowFailed, common.Building, 0, common.BuildFailed, common.TaskStatusBuildFailed, true},
		{"WorkflowSucceeded not building", v1alpha1.WorkflowSucceeded, 0, 0, 0, 0, false},
		{"WorkflowFailed not building", v1alpha1.WorkflowFailed, 0, 0, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			deploy := &database.Deploy{ID: taskID, Status: tt.deployStatus}
			task := &database.DeployTask{ID: taskID, DeployID: taskID, Status: tt.taskStatus, Deploy: deploy}
			store := mockdb.NewMockDeployTaskStore(t)
			store.EXPECT().GetDeployTask(ctx, taskID).Return(task, nil)
			store.EXPECT().GetLastTaskByType(ctx, taskID, task.TaskType).Return(task, nil)
			if tt.wantWrite {
				store.EXPECT().UpdateInTx(mock.Anything, []string{"status", "image_id"}, []string{"status", "message"}, deploy, task).Return(nil).Once()
			}

			data, err := json.Marshal(types.ImageBuilderEvent{
				DeployId: "1", TaskId: taskID, Status: string(tt.workflow), ImagetPath: imagePath,
			})
			require.NoError(t, err)
			event := &types.WebHookRecvEvent{
				WebHookHeader: types.WebHookHeader{EventType: types.RunnerBuilderChange},
				Data:          data,
			}
			err = (&imagebuilderExecutorImpl{store: store}).ProcessEvent(ctx, event)
			require.NoError(t, err)
			require.Equal(t, tt.wantTask, task.Status)
			require.Equal(t, tt.wantDeploy, deploy.Status)
			if tt.workflow == v1alpha1.WorkflowSucceeded && tt.wantWrite {
				require.Equal(t, imagePath, deploy.ImageID)
			}
		})
	}
}
