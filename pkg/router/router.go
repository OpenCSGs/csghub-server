package router

import (
	"net/http"
	"strings"
)

const (
	APIMount = "/api"
	GitMount = "/git"
)

type Router struct {
	api APIHandler
	git GitHandler
}

func NewRouter(api APIHandler, git GitHandler) *Router {
	return &Router{
		api: api,
		git: git,
	}
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	if r.isGitRequest(req) {
		r.git.ServeHTTP(w, req)
		return
	}

	if r.isAPIRequest(req) {
		r.api.ServeHTTP(w, req)
		return
	}
}

func (r *Router) isGitRequest(req *http.Request) bool {
	p := req.URL.Path
	return strings.HasPrefix(p, GitMount)
}

func (r *Router) isAPIRequest(req *http.Request) bool {
	p := req.URL.Path
	return strings.HasPrefix(p, APIMount)
}
