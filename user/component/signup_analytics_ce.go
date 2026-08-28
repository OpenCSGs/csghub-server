//go:build !ee && !saas

package component

import "opencsg.com/csghub-server/builder/store/database"

func (*userComponentImpl) captureSignupSuccess(*database.User) {}
