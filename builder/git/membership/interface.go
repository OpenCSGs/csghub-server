package membership

import "context"

type GitMemerShip interface {
	AddMember(ctx context.Context, org, user string, role Role) error
	RemoveMember(ctx context.Context, org, user string, role Role) error
	IsRole(ctx context.Context, org, user string, role Role) (bool, error)
	AddRoles(ctx context.Context, org string, roles []Role) error
}
