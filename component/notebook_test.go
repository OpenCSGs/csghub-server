package component

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/types"
)

// --- Test ---

func TestNotebookComponentImpl_CreateNotebook(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	username := "testuser"
	nc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, username).Return(database.User{ID: 1, Username: username}, nil)
	nc.mocks.stores.RuntimeFrameworkMock().EXPECT().FindEnabledByID(ctx, int64(1)).Return(&database.RuntimeFramework{ID: 1, FrameName: "rf1", FrameImage: "abc/notebook:latest"}, nil)
	nc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(&database.SpaceResource{ID: 1, ClusterID: "1", Name: "sr1", Resources: `{"memory": "foo"}`}, nil)
	nc.mocks.components.repo.EXPECT().CheckAccountAndResource(ctx, username, "1", int64(0), &database.SpaceResource{ID: 1, ClusterID: "1", Name: "sr1", Resources: `{"memory": "foo"}`}).Return(nil)
	nc.mocks.deployer.EXPECT().Deploy(ctx, types.DeployRepo{
		DeployName:       "notebook-deploy",
		Hardware:         "{\"memory\": \"foo\"}",
		ClusterID:        "1",
		SKU:              "1",
		Type:             types.NotebookType,
		ImageID:          "abc/notebook:latest",
		RuntimeFramework: "rf1",
		UserID:           1,
		Annotation:       `{"hub-deploy-user":"testuser"}`,
		MinReplica:       0,
		MaxReplica:       1,
		SecureLevel:      2,
	}).Return(int64(123), nil)

	res, err := nc.CreateNotebook(ctx, &types.CreateNotebookReq{
		CurrentUser:        username,
		DeployName:         "notebook-deploy",
		ResourceID:         1,
		RuntimeFrameworkID: 1,
	})
	require.Nil(t, err)
	require.Equal(t, int64(123), res.ID)
}

func TestNotebookComponentImpl_GetNotebookByID(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	deploy := &database.Deploy{
		ID:         1,
		DeployName: "notebook-deploy",
		SvcName:    "notebook-svc",
		ClusterID:  "1",
		Status:     23,
	}
	nc.mocks.components.repo.EXPECT().CheckDeployPermissionForUser(ctx, types.DeployActReq{
		DeployID:    1,
		CurrentUser: "user",
	}).Return(&database.User{}, deploy, nil)
	nc.mocks.components.repo.EXPECT().GenerateEndpoint(ctx, deploy).Return("endpoint", "")
	req := &types.GetNotebookReq{
		ID:          1,
		CurrentUser: "user",
	}
	res, err := nc.GetNotebook(ctx, req)
	require.Nil(t, err)
	require.Equal(t, int64(1), res.ID)
	require.Equal(t, "notebook-deploy", res.DeployName)
	require.Equal(t, "notebook-svc", res.SvcName)
	require.Equal(t, "1", res.ClusterID)
	require.Equal(t, "Running", res.Status)
}
func TestNotebookComponentImpl_DeleteNotebook_Success(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        10,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    10,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Purge(ctx, types.DeployRepo{
			SpaceID:   0,
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(nil)

	nc.mocks.stores.DeployTaskMock().EXPECT().
		DeleteDeployByID(ctx, user.ID, deploy.ID).
		Return(nil)

	err := nc.DeleteNotebook(ctx, &types.DeleteNotebookReq{
		ID:          10,
		CurrentUser: "testuser",
	})
	require.NoError(t, err)
}

func TestNotebookComponentImpl_DeleteNotebook_PermissionDenied(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    10,
			CurrentUser: "testuser",
		}).
		Return(nil, nil, errors.New("permission denied"))

	err := nc.DeleteNotebook(ctx, &types.DeleteNotebookReq{
		ID:          10,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestNotebookComponentImpl_DeleteNotebook_PurgeFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        10,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    10,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Purge(ctx, types.DeployRepo{
			SpaceID:   0,
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(errors.New("purge error"))

	nc.mocks.stores.DeployTaskMock().EXPECT().
		DeleteDeployByID(ctx, user.ID, deploy.ID).
		Return(nil)

	err := nc.DeleteNotebook(ctx, &types.DeleteNotebookReq{
		ID:          10,
		CurrentUser: "testuser",
	})
	require.NoError(t, err)
}

func TestNotebookComponentImpl_DeleteNotebook_DeleteDeployFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        10,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    10,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Purge(ctx, types.DeployRepo{
			SpaceID:   0,
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(nil)

	nc.mocks.stores.DeployTaskMock().EXPECT().
		DeleteDeployByID(ctx, user.ID, deploy.ID).
		Return(errors.New("delete error"))

	err := nc.DeleteNotebook(ctx, &types.DeleteNotebookReq{
		ID:          10,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot delete notebook")
}
func TestNotebookComponentImpl_UpdateNotebook_Success(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:            20,
		SvcName:       "notebook-svc",
		ClusterID:     "1",
		OrderDetailID: 0,
	}
	resource := &database.SpaceResource{
		ID:        2,
		ClusterID: "1",
		Resources: `{"memory": "2Gi", "replicas": 1}`,
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    20,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.stores.SpaceResourceMock().EXPECT().
		FindByID(ctx, int64(2)).
		Return(resource, nil)

	nc.mocks.components.repo.EXPECT().
		CheckAccountAndResource(ctx, "testuser", "1", int64(0), resource).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		UpdateDeploy(ctx, &types.DeployUpdateReq{
			ResourceID: &resource.ID,
		}, deploy).
		Return(nil)

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          20,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.NoError(t, err)
}

func TestNotebookComponentImpl_UpdateNotebook_PermissionDenied(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    21,
			CurrentUser: "testuser",
		}).
		Return(nil, nil, errors.New("permission denied"))

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          21,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot find deploy for notebook")
}

func TestNotebookComponentImpl_UpdateNotebook_DeployRunning(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        22,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    22,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(true, nil)

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          22,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "stop deploy first")
}

func TestNotebookComponentImpl_UpdateNotebook_ResourceNotFound(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        23,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    23,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.stores.SpaceResourceMock().EXPECT().
		FindByID(ctx, int64(2)).
		Return(nil, errors.New("resource not found"))

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          23,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot find resource")
}

func TestNotebookComponentImpl_UpdateNotebook_ResourceUnavailable(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:            24,
		SvcName:       "notebook-svc",
		ClusterID:     "1",
		OrderDetailID: 0,
	}
	resource := &database.SpaceResource{
		ID:        2,
		ClusterID: "1",
		Resources: `{"memory": "2Gi", "replicas": 1}`,
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    24,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.stores.SpaceResourceMock().EXPECT().
		FindByID(ctx, int64(2)).
		Return(resource, nil)

	nc.mocks.components.repo.EXPECT().
		CheckAccountAndResource(ctx, "testuser", "1", int64(0), resource).
		Return(errors.New("resource unavailable"))

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          24,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "resource is not available")
}

func TestNotebookComponentImpl_UpdateNotebook_MultiHostNotSupported(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:            25,
		SvcName:       "notebook-svc",
		ClusterID:     "1",
		OrderDetailID: 0,
	}
	resource := &database.SpaceResource{
		ID:        2,
		ClusterID: "1",
		Resources: `{"memory": "2Gi", "replicas": 2}`,
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    25,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.stores.SpaceResourceMock().EXPECT().
		FindByID(ctx, int64(2)).
		Return(resource, nil)

	nc.mocks.components.repo.EXPECT().
		CheckAccountAndResource(ctx, "testuser", "1", int64(0), resource).
		Return(nil)

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          25,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
}

func TestNotebookComponentImpl_UpdateNotebook_UpdateDeployFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:            26,
		SvcName:       "notebook-svc",
		ClusterID:     "1",
		OrderDetailID: 0,
	}
	resource := &database.SpaceResource{
		ID:        2,
		ClusterID: "1",
		Resources: `{"memory": "2Gi", "replicas": 1}`,
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    26,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.stores.SpaceResourceMock().EXPECT().
		FindByID(ctx, int64(2)).
		Return(resource, nil)

	nc.mocks.components.repo.EXPECT().
		CheckAccountAndResource(ctx, "testuser", "1", int64(0), resource).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		UpdateDeploy(ctx, &types.DeployUpdateReq{
			ResourceID: &resource.ID,
		}, deploy).
		Return(errors.New("update failed"))

	err := nc.UpdateNotebook(ctx, &types.UpdateNotebookReq{
		ID:          26,
		CurrentUser: "testuser",
		ResourceID:  2,
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "update failed")
}
func TestNotebookComponentImpl_StartNotebook_Success(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        30,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    30,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.deployer.EXPECT().
		StartDeploy(ctx, deploy).
		Return(nil)

	err := nc.StartNotebook(ctx, &types.StartNotebookReq{
		ID:          30,
		CurrentUser: "testuser",
	})
	require.NoError(t, err)
}

func TestNotebookComponentImpl_StartNotebook_PermissionDenied(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    31,
			CurrentUser: "testuser",
		}).
		Return(nil, nil, errors.New("permission denied"))

	err := nc.StartNotebook(ctx, &types.StartNotebookReq{
		ID:          31,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "cannot find deploy for notebook")
}

func TestNotebookComponentImpl_StartNotebook_AlreadyStarted(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        32,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    32,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(true, nil)

	err := nc.StartNotebook(ctx, &types.StartNotebookReq{
		ID:          32,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "deploy already started")
}

func TestNotebookComponentImpl_StartNotebook_ExistCheckFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        33,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    33,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, errors.New("exist check error"))

	err := nc.StartNotebook(ctx, &types.StartNotebookReq{
		ID:          33,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "check notebook exists")
}

func TestNotebookComponentImpl_StartNotebook_StartDeployFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        34,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    34,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, types.DeployRepo{
			DeployID:  deploy.ID,
			SvcName:   deploy.SvcName,
			ClusterID: deploy.ClusterID,
		}).
		Return(false, nil)

	nc.mocks.deployer.EXPECT().
		StartDeploy(ctx, deploy).
		Return(errors.New("start error"))

	err := nc.StartNotebook(ctx, &types.StartNotebookReq{
		ID:          34,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to start notebook")
}
func TestNotebookComponentImpl_StopNotebook_Success(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        40,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    40,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	nc.mocks.deployer.EXPECT().
		Stop(ctx, deployRepo).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, deployRepo).
		Return(false, nil)

	nc.mocks.stores.DeployTaskMock().EXPECT().
		StopDeployByID(ctx, user.ID, deploy.ID).
		Return(nil)

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          40,
		CurrentUser: "testuser",
	})
	require.NoError(t, err)
}

func TestNotebookComponentImpl_StopNotebook_PermissionDenied(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    41,
			CurrentUser: "testuser",
		}).
		Return(nil, nil, errors.New("permission denied"))

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          41,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "permission denied")
}

func TestNotebookComponentImpl_StopNotebook_StopFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        42,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    42,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	nc.mocks.deployer.EXPECT().
		Stop(ctx, deployRepo).
		Return(errors.New("stop error"))

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          42,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to stop notebook")
}

func TestNotebookComponentImpl_StopNotebook_ExistCheckFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        43,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    43,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	nc.mocks.deployer.EXPECT().
		Stop(ctx, deployRepo).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, deployRepo).
		Return(false, errors.New("exist check error"))

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          43,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "exist check error")
}

func TestNotebookComponentImpl_StopNotebook_StillExistsAfterStop(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        44,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    44,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	nc.mocks.deployer.EXPECT().
		Stop(ctx, deployRepo).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, deployRepo).
		Return(true, nil)

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          44,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to stop notebook instance")
}

func TestNotebookComponentImpl_StopNotebook_StopDeployByIDFails(t *testing.T) {
	ctx := context.TODO()
	nc := initializeTestNotebookComponent(ctx, t)
	user := &database.User{ID: 1, Username: "testuser"}
	deploy := &database.Deploy{
		ID:        45,
		SvcName:   "notebook-svc",
		ClusterID: "1",
	}

	nc.mocks.components.repo.EXPECT().
		CheckDeployPermissionForUser(ctx, types.DeployActReq{
			DeployID:    45,
			CurrentUser: "testuser",
		}).
		Return(user, deploy, nil)

	deployRepo := types.DeployRepo{
		DeployID:  deploy.ID,
		SvcName:   deploy.SvcName,
		ClusterID: deploy.ClusterID,
	}

	nc.mocks.deployer.EXPECT().
		Stop(ctx, deployRepo).
		Return(nil)

	nc.mocks.deployer.EXPECT().
		Exist(ctx, deployRepo).
		Return(false, nil)

	nc.mocks.stores.DeployTaskMock().EXPECT().
		StopDeployByID(ctx, user.ID, deploy.ID).
		Return(errors.New("db error"))

	err := nc.StopNotebook(ctx, &types.StopNotebookReq{
		ID:          45,
		CurrentUser: "testuser",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "fail to stop notebook instance")
}
