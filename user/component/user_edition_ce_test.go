//go:build !saas

package component

import "testing"

func expectProfileUpdate(*testing.T, *userComponentImpl) func() { return func() {} }
