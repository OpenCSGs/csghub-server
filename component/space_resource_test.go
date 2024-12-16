package component

// func TestSpaceResourceComponent_Index(t *testing.T) {
// 	ctx := context.TODO()
// 	sc := initializeTestSpaceResourceComponent(ctx, t)

// 	sc.mocks.deployer.EXPECT().ListCluster(ctx).Return([]types.ClusterRes{
// 		{ClusterID: "c1"},
// 	}, nil)
// 	sc.mocks.stores.SpaceResourceMock().EXPECT().Index(ctx, "c1").Return(
// 		[]database.SpaceResource{
// 			{ID: 1, Name: "sr", Resources: `{"memory": "1000"}`},
// 		}, nil,
// 	)
// 	sc.mocks.deployer.EXPECT().GetClusterById(ctx, "c1").Return(&types.ClusterRes{}, nil)
// 	sc.mocks.stores.UserMock().EXPECT().FindByUsername(ctx, "user").Return(database.User{
// 		UUID: "uid",
// 	}, nil)

// 	data, err := sc.Index(ctx, "", 1)
// 	require.Nil(t, err)
// 	require.Equal(t, []types.SpaceResource{
// 		{
// 			ID: 1, Name: "sr", Resources: "{\"memory\": \"1000\"}",
// 			IsAvailable: false, Type: "cpu",
// 		},
// 		{
// 			ID: 0, Name: "", Resources: "{\"memory\": \"2000\"}", IsAvailable: true,
// 			Type: "cpu",
// 		},
// 	}, data)

// }

// func TestSpaceResourceComponent_Update(t *testing.T) {
// 	ctx := context.TODO()
// 	sc := initializeTestSpaceResourceComponent(ctx, t)

// 	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
// 		&database.SpaceResource{}, nil,
// 	)
// 	sc.mocks.stores.SpaceResourceMock().EXPECT().Update(ctx, database.SpaceResource{
// 		Name:      "n",
// 		Resources: "r",
// 	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: "r"}, nil)

// 	data, err := sc.Update(ctx, &types.UpdateSpaceResourceReq{
// 		ID:        1,
// 		Name:      "n",
// 		Resources: "r",
// 	})
// 	require.Nil(t, err)
// 	require.Equal(t, &types.SpaceResource{
// 		ID:        1,
// 		Name:      "n",
// 		Resources: "r",
// 	}, data)
// }

// func TestSpaceResourceComponent_Create(t *testing.T) {
// 	ctx := context.TODO()
// 	sc := initializeTestSpaceResourceComponent(ctx, t)

// 	sc.mocks.stores.SpaceResourceMock().EXPECT().Create(ctx, database.SpaceResource{
// 		Name:      "n",
// 		Resources: "r",
// 		ClusterID: "c",
// 	}).Return(&database.SpaceResource{ID: 1, Name: "n", Resources: "r"}, nil)

// 	data, err := sc.Create(ctx, &types.CreateSpaceResourceReq{
// 		Name:      "n",
// 		Resources: "r",
// 		ClusterID: "c",
// 	})
// 	require.Nil(t, err)
// 	require.Equal(t, &types.SpaceResource{
// 		ID:        1,
// 		Name:      "n",
// 		Resources: "r",
// 	}, data)
// }

// func TestSpaceResourceComponent_Delete(t *testing.T) {
// 	ctx := context.TODO()
// 	sc := initializeTestSpaceResourceComponent(ctx, t)

// 	sc.mocks.stores.SpaceResourceMock().EXPECT().FindByID(ctx, int64(1)).Return(
// 		&database.SpaceResource{}, nil,
// 	)
// 	sc.mocks.stores.SpaceResourceMock().EXPECT().Delete(ctx, database.SpaceResource{}).Return(nil)

// 	err := sc.Delete(ctx, 1)
// 	require.Nil(t, err)
// }
