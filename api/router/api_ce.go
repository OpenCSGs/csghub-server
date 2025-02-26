//go:build !ee && !saas

package router

type ServerImpl struct {
	*BaseServer
}

func NewServer(base *BaseServer) *ServerImpl {
	return &ServerImpl{
		BaseServer: base,
	}
}

func (s *ServerImpl) RegisterRoutes(enableSwagger bool) error {
	return s.BaseServer.RegisterRoutes(enableSwagger)
}
