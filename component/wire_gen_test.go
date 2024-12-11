// Code generated by Wire. DO NOT EDIT.

//go:generate go run -mod=mod github.com/google/wire/cmd/wire
//go:build !wireinject
// +build !wireinject

package component

import (
	"context"
	"github.com/stretchr/testify/mock"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/accounting"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/deploy"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/git/mirrorserver"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/inference"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/parquet"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/builder/store/s3"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/component"
	"opencsg.com/csghub-server/_mocks/opencsg.com/csghub-server/mirror/queue"
	"opencsg.com/csghub-server/common/tests"
)

// Injectors from wire.go:

func initializeTestRepoComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRepoWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockClient := s3.NewMockClient(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	componentRepoComponentImpl := NewTestRepoComponent(config, mockStores, mockUserSvcClient, mockGitServer, mockTagComponent, mockClient, mockDeployer, mockAccountingComponent, mockPriorityQueue, mockMirrorServer)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestRepoWithMocks := &testRepoWithMocks{
		repoComponentImpl: componentRepoComponentImpl,
		mocks:             mocks,
	}
	return componentTestRepoWithMocks
}

func initializeTestPromptComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testPromptWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	componentPromptComponentImpl := NewTestPromptComponent(config, mockStores, mockRepoComponent, mockUserSvcClient, mockGitServer)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestPromptWithMocks := &testPromptWithMocks{
		promptComponentImpl: componentPromptComponentImpl,
		mocks:               mocks,
	}
	return componentTestPromptWithMocks
}

func initializeTestUserComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testUserWithMocks {
	mockStores := tests.NewMockStores(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	componentUserComponentImpl := NewTestUserComponent(mockStores, mockGitServer, mockSpaceComponent, mockRepoComponent, mockDeployer, mockAccountingComponent)
	mockTagComponent := component.NewMockTagComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestUserWithMocks := &testUserWithMocks{
		userComponentImpl: componentUserComponentImpl,
		mocks:             mocks,
	}
	return componentTestUserWithMocks
}

func initializeTestSpaceComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testSpaceWithMocks {
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	config := ProvideTestConfig()
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	componentSpaceComponentImpl := NewTestSpaceComponent(mockStores, mockRepoComponent, mockGitServer, mockDeployer, mockAccountingComponent, config, mockUserSvcClient)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestSpaceWithMocks := &testSpaceWithMocks{
		spaceComponentImpl: componentSpaceComponentImpl,
		mocks:              mocks,
	}
	return componentTestSpaceWithMocks
}

func initializeTestModelComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testModelWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockClient := inference.NewMockClient(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	componentModelComponentImpl := NewTestModelComponent(config, mockStores, mockRepoComponent, mockSpaceComponent, mockClient, mockDeployer, mockAccountingComponent, mockRuntimeArchitectureComponent, mockGitServer, mockUserSvcClient)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	s3MockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         s3MockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  mockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestModelWithMocks := &testModelWithMocks{
		modelComponentImpl: componentModelComponentImpl,
		mocks:              mocks,
	}
	return componentTestModelWithMocks
}

func initializeTestAccountingComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testAccountingWithMocks {
	mockStores := tests.NewMockStores(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	componentAccountingComponentImpl := NewTestAccountingComponent(mockStores, mockAccountingClient)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockGitServer := gitserver.NewMockGitServer(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestAccountingWithMocks := &testAccountingWithMocks{
		accountingComponentImpl: componentAccountingComponentImpl,
		mocks:                   mocks,
	}
	return componentTestAccountingWithMocks
}

func initializeTestDatasetViewerComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDatasetViewerWithMocks {
	mockStores := tests.NewMockStores(t)
	config := ProvideTestConfig()
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockReader := parquet.NewMockReader(t)
	componentDatasetViewerComponentImpl := NewTestDatasetViewerComponent(mockStores, config, mockRepoComponent, mockGitServer, mockReader)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestDatasetViewerWithMocks := &testDatasetViewerWithMocks{
		datasetViewerComponentImpl: componentDatasetViewerComponentImpl,
		mocks:                      mocks,
	}
	return componentTestDatasetViewerWithMocks
}

func initializeTestGitHTTPComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testGitHTTPWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockClient := s3.NewMockClient(t)
	componentGitHTTPComponentImpl := NewTestGitHTTPComponent(config, mockStores, mockRepoComponent, mockGitServer, mockClient)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestGitHTTPWithMocks := &testGitHTTPWithMocks{
		gitHTTPComponentImpl: componentGitHTTPComponentImpl,
		mocks:                mocks,
	}
	return componentTestGitHTTPWithMocks
}

func initializeTestDiscussionComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDiscussionWithMocks {
	mockStores := tests.NewMockStores(t)
	componentDiscussionComponentImpl := NewTestDiscussionComponent(mockStores)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockGitServer := gitserver.NewMockGitServer(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestDiscussionWithMocks := &testDiscussionWithMocks{
		discussionComponentImpl: componentDiscussionComponentImpl,
		mocks:                   mocks,
	}
	return componentTestDiscussionWithMocks
}

func initializeTestRuntimeArchComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testRuntimeArchWithMocks {
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	componentRuntimeArchitectureComponentImpl := NewTestRuntimeArchitectureComponent(mockStores, mockRepoComponent, mockGitServer)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestRuntimeArchWithMocks := &testRuntimeArchWithMocks{
		runtimeArchitectureComponentImpl: componentRuntimeArchitectureComponentImpl,
		mocks:                            mocks,
	}
	return componentTestRuntimeArchWithMocks
}

func initializeTestMirrorComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMirrorWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	mockClient := s3.NewMockClient(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	componentMirrorComponentImpl := NewTestMirrorComponent(config, mockStores, mockMirrorServer, mockRepoComponent, mockGitServer, mockClient, mockPriorityQueue)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestMirrorWithMocks := &testMirrorWithMocks{
		mirrorComponentImpl: componentMirrorComponentImpl,
		mocks:               mocks,
	}
	return componentTestMirrorWithMocks
}

func initializeTestCollectionComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testCollectionWithMocks {
	mockStores := tests.NewMockStores(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	componentCollectionComponentImpl := NewTestCollectionComponent(mockStores, mockUserSvcClient, mockSpaceComponent)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockGitServer := gitserver.NewMockGitServer(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestCollectionWithMocks := &testCollectionWithMocks{
		collectionComponentImpl: componentCollectionComponentImpl,
		mocks:                   mocks,
	}
	return componentTestCollectionWithMocks
}

func initializeTestDatasetComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDatasetWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	componentDatasetComponentImpl := NewTestDatasetComponent(config, mockStores, mockRepoComponent, mockUserSvcClient, mockSensitiveComponent, mockGitServer)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestDatasetWithMocks := &testDatasetWithMocks{
		datasetComponentImpl: componentDatasetComponentImpl,
		mocks:                mocks,
	}
	return componentTestDatasetWithMocks
}

func initializeTestCodeComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testCodeWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	componentCodeComponentImpl := NewTestCodeComponent(config, mockStores, mockRepoComponent, mockUserSvcClient, mockGitServer)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockCache := cache.NewMockCache(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		cache:            mockCache,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestCodeWithMocks := &testCodeWithMocks{
		codeComponentImpl: componentCodeComponentImpl,
		mocks:             mocks,
	}
	return componentTestCodeWithMocks
}

func initializeTestMultiSyncComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testMultiSyncWithMocks {
	config := ProvideTestConfig()
	mockStores := tests.NewMockStores(t)
	mockGitServer := gitserver.NewMockGitServer(t)
	componentMultiSyncComponentImpl := NewTestMultiSyncComponent(config, mockStores, mockGitServer)
	mockAccountingComponent := component.NewMockAccountingComponent(t)
	mockRepoComponent := component.NewMockRepoComponent(t)
	mockTagComponent := component.NewMockTagComponent(t)
	mockSpaceComponent := component.NewMockSpaceComponent(t)
	mockRuntimeArchitectureComponent := component.NewMockRuntimeArchitectureComponent(t)
	mockSensitiveComponent := component.NewMockSensitiveComponent(t)
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
		sensitive:           mockSensitiveComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	mockCache := cache.NewMockCache(t)
	inferenceMockClient := inference.NewMockClient(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
	mockReader := parquet.NewMockReader(t)
	mocks := &Mocks{
		stores:           mockStores,
		components:       componentMockedComponents,
		gitServer:        mockGitServer,
		userSvcClient:    mockUserSvcClient,
		s3Client:         mockClient,
		mirrorServer:     mockMirrorServer,
		mirrorQueue:      mockPriorityQueue,
		deployer:         mockDeployer,
		cache:            mockCache,
		inferenceClient:  inferenceMockClient,
		accountingClient: mockAccountingClient,
		preader:          mockReader,
	}
	componentTestMultiSyncWithMocks := &testMultiSyncWithMocks{
		multiSyncComponentImpl: componentMultiSyncComponentImpl,
		mocks:                  mocks,
	}
	return componentTestMultiSyncWithMocks
}

// wire.go:

type testRepoWithMocks struct {
	*repoComponentImpl
	mocks *Mocks
}

type testPromptWithMocks struct {
	*promptComponentImpl
	mocks *Mocks
}

type testUserWithMocks struct {
	*userComponentImpl
	mocks *Mocks
}

type testSpaceWithMocks struct {
	*spaceComponentImpl
	mocks *Mocks
}

type testModelWithMocks struct {
	*modelComponentImpl
	mocks *Mocks
}

type testAccountingWithMocks struct {
	*accountingComponentImpl
	mocks *Mocks
}

type testDatasetViewerWithMocks struct {
	*datasetViewerComponentImpl
	mocks *Mocks
}

type testGitHTTPWithMocks struct {
	*gitHTTPComponentImpl
	mocks *Mocks
}

type testDiscussionWithMocks struct {
	*discussionComponentImpl
	mocks *Mocks
}

type testRuntimeArchWithMocks struct {
	*runtimeArchitectureComponentImpl
	mocks *Mocks
}

type testMirrorWithMocks struct {
	*mirrorComponentImpl
	mocks *Mocks
}

type testCollectionWithMocks struct {
	*collectionComponentImpl
	mocks *Mocks
}

type testDatasetWithMocks struct {
	*datasetComponentImpl
	mocks *Mocks
}

type testCodeWithMocks struct {
	*codeComponentImpl
	mocks *Mocks
}

type testMultiSyncWithMocks struct {
	*multiSyncComponentImpl
	mocks *Mocks
}
