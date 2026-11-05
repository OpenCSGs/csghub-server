package types

import "context"

type RouteCallTarget struct {
	Model      string
	UpstreamID int64
	Protocol   string
}
type routeCallTargetKey struct{}

func WithRouteCallTarget(ctx context.Context, target RouteCallTarget) context.Context {
	return context.WithValue(ctx, routeCallTargetKey{}, target)
}
func RouteCallTargetFromContext(ctx context.Context) RouteCallTarget {
	value, _ := ctx.Value(routeCallTargetKey{}).(RouteCallTarget)
	return value
}
