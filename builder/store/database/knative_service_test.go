package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"opencsg.com/csghub-server/builder/deploy/common"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestKnativeServiceStore_Get(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewKnativeServiceWithDB(db)
	err := store.Add(ctx, &database.KnativeService{
		Name:      "test",
		Status:    corev1.ConditionTrue,
		Code:      common.Running,
		ClusterID: "cluster1",
	})
	require.Nil(t, err)
	err = store.Add(ctx, &database.KnativeService{
		Name:      "test2",
		Status:    corev1.ConditionTrue,
		Code:      common.Running,
		ClusterID: "cluster1",
	})
	require.Nil(t, err)
	err = store.Add(ctx, &database.KnativeService{
		Name:      "test3",
		Status:    corev1.ConditionTrue,
		Code:      common.Running,
		ClusterID: "cluster2",
	})
	require.Nil(t, err)
	ks, err := store.Get(ctx, "test", "cluster1")
	require.Nil(t, err)
	require.Equal(t, "test", ks.Name)
	list, err := store.GetByCluster(ctx, "cluster1")
	require.Nil(t, err)
	require.Equal(t, 2, len(list))
}

func TestKnativeServiceStore_Delete(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewKnativeServiceWithDB(db)
	err := store.Add(ctx, &database.KnativeService{
		Name:      "test",
		Status:    corev1.ConditionTrue,
		Code:      common.Running,
		ClusterID: "cluster1",
	})
	require.Nil(t, err)
	err = store.Delete(ctx, "cluster1", "test")
	require.Nil(t, err)
	_, err = store.Get(ctx, "test", "cluster1")
	require.NotNil(t, err)
}

func TestKnativeServiceStore_Update(t *testing.T) {
	db := tests.InitTestDB()
	defer db.Close()
	ctx := context.TODO()

	store := database.NewKnativeServiceWithDB(db)
	err := store.Add(ctx, &database.KnativeService{
		ID:        1,
		Name:      "test",
		Status:    corev1.ConditionFalse,
		Code:      common.Deploying,
		ClusterID: "cluster1",
	})
	require.Nil(t, err)
	err = store.Update(ctx, &database.KnativeService{
		ID:        1,
		Name:      "test",
		Status:    corev1.ConditionTrue,
		Code:      common.Running,
		ClusterID: "cluster1",
	})
	require.Nil(t, err)
	ks, err := store.Get(ctx, "test", "cluster1")
	require.Nil(t, err)
	require.Equal(t, corev1.ConditionTrue, ks.Status)
	require.Equal(t, common.Running, ks.Code)
}
