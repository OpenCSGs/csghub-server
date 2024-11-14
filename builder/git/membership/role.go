package membership

type Role string

const (
	RoleUnknown Role = ""
	RoleRead    Role = "read"
	RoleWrite   Role = "write"
	RoleAdmin   Role = "admin"
)

func (r Role) CanRead() bool {
	return r != RoleUnknown
}

func (r Role) CanWrite() bool {
	return r == RoleWrite || r == RoleAdmin
}

func (r Role) CanAdmin() bool {
	return r == RoleAdmin
}
