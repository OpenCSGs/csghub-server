//go:build wireinject && !saas
// +build wireinject,!saas

package component

import (
	"context"

	"github.com/google/wire"
	"github.com/stretchr/testify/mock"
)

func initializeTestDatasetComponent(ctx context.Context, t interface {
	Cleanup(func())
	mock.TestingT
}) *testDatasetWithMocks {
	wire.Build(
		MockSuperSet, DatasetComponentSet,
		wire.Struct(new(testDatasetWithMocks), "*"),
	)
	return &testDatasetWithMocks{}
}
