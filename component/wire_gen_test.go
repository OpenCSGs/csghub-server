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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
	s3MockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockAccountingClient := accounting.NewMockAccountingClient(t)
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
	componentMockedComponents := &mockedComponents{
		accounting:          mockAccountingComponent,
		repo:                mockRepoComponent,
		tag:                 mockTagComponent,
		space:               mockSpaceComponent,
		runtimeArchitecture: mockRuntimeArchitectureComponent,
	}
	mockGitServer := gitserver.NewMockGitServer(t)
	mockUserSvcClient := rpc.NewMockUserSvcClient(t)
	mockClient := s3.NewMockClient(t)
	mockMirrorServer := mirrorserver.NewMockMirrorServer(t)
	mockPriorityQueue := queue.NewMockPriorityQueue(t)
	mockDeployer := deploy.NewMockDeployer(t)
	inferenceMockClient := inference.NewMockClient(t)
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
	}
	componentTestAccountingWithMocks := &testAccountingWithMocks{
		accountingComponentImpl: componentAccountingComponentImpl,
		mocks:                   mocks,
	}
	return componentTestAccountingWithMocks
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