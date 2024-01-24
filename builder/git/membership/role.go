package membership

type Role string

const (
	RoleUnkown Role = ""
	RoleRead   Role = "read"
	RoleWrite  Role = "write"
	RoleAdmin  Role = "admin"
)

func (r Role) CanRead() bool {
	return r != RoleUnkown
}

func (r Role) CanWrite() bool {
	return r == RoleWrite || r == RoleAdmin
}

func (r Role) CanAdmin() bool {
	return r == RoleAdmin
}
