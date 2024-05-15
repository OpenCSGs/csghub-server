package httpbase

import "errors"

var ErrorNeedLogin = errors.New("please login first")
var ErrorNotEnoughPermission = errors.New("not enough permission")
