//go:build !ee && !saas

package component

import "context"

func expectUpdateModelArchType(_ context.Context, _ *testRuntimeArchWithMocks) {}
