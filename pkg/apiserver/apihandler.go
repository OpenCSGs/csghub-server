package apiserver

import (
	"git-devops.opencsg.com/product/community/starhub-server/pkg/cache"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/httpbase"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/log"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/model"
	"git-devops.opencsg.com/product/community/starhub-server/pkg/serverhost"
)

// R is the response envelope
type R = httpbase.R

// OK responds the client with standard JSON.
//
// Example:
// * OK(c, something)
// * OK(c, nil)
var OK = httpbase.OK

type APIHandler struct {
	db         *model.DB
	cache      *cache.Cache
	logger     log.Logger
	serverHost *serverhost.ServerHost
}

type APIHandlerOpt struct {
	DB         *model.DB
	Cache      *cache.Cache
	ServerHost *serverhost.ServerHost
}

func NewAPIHandler(opt *APIHandlerOpt) (apihandler *APIHandler) {
	return &APIHandler{
		db:         opt.DB,
		cache:      opt.Cache,
		logger:     log.Clone(log.Namespace("workflow/apiHandler")),
		serverHost: opt.ServerHost,
	}
}
