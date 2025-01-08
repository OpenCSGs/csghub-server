//go:build wireinject
// +build wireinject

package callback

import (
	"context"

	"github.com/google/wire"
	"github.com/stretchr/testify/mock"
)

type testGitCallbackWithMocks struct {
	*gitCallbackComponentImpl
	mocks *Mocks
}

func initializeTestGitCallbackComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testGitCallbackWithMocks {
	wire.Build(
		MockCallbackSuperSet, GitCallbackComponentSet,
		wire.Struct(new(testGitCallbackWithMocks), "*"),
	)
	return &testGitCallbackWithMocks{}
}
