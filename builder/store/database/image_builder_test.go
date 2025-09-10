package database_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/tests"
)

func TestImagebuilderStore_CRUD(t *testing.T) {
	db := tests.InitTestDB()                       // Initialize test database
	defer db.Close()                               // Close database after test
	ctx := context.TODO()                          // Create context
	imd := database.NewImageBuilderStoreWithDB(db) // Create store instance with test database

	newWork := &database.ImageBuilderWork{
		WorkName:            "test-work",
		WorkStatus:          "Pending",
		Message:             "",
		ImagePath:           "test:v1",
		BuildId:             "test",
		PodName:             "test-pod",
		ClusterID:           "test-cluster",
		Namespace:           "test-namespace",
		InitContainerStatus: "Running",
		InitContainerLog:    "Init log",
		MainContainerLog:    "Main log",
	}

	// Test Create method
	createdWork, err := imd.Create(ctx, newWork)
	require.NoError(t, err)           // Verify that there is no error
	require.NotNil(t, createdWork.ID) // Verify that the returned work ID is not nil

	// Test FindByWorkName method
	foundWork, err := imd.FindByWorkName(ctx, newWork.WorkName)
	require.NoError(t, err)                                    // Verify that there is no error
	require.Equal(t, newWork.WorkName, foundWork.WorkName)     // Validate work name
	require.Equal(t, newWork.WorkStatus, foundWork.WorkStatus) // Verify work status

	// Test UpdateByWorkName method
	newWork.WorkStatus = "InProgress" // Update work status
	updatedWork, err := imd.UpdateByWorkName(ctx, newWork)
	require.NoError(t, err)                                      // Verify that there is no error
	require.Equal(t, newWork.WorkStatus, updatedWork.WorkStatus) // Verify updated work status

	// Test QueryStatusByBuildID method
	statusWork, err := imd.QueryStatusByBuildID(ctx, newWork.BuildId)
	require.NoError(t, err)                                     // Verify that there is no error
	require.Equal(t, newWork.WorkStatus, statusWork.WorkStatus) // Verify work status by build ID

	foundWork, err = imd.FindByImagePath(ctx, newWork.ImagePath)
	require.NoError(t, err)                                  // Verify that there is no error
	require.Equal(t, newWork.ImagePath, foundWork.ImagePath) // Validate image path
}
