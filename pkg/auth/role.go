package auth

type Resource int

const (
	QUERY Resource = iota
	STREAM
	START
	STOP
)

// Default Roles
const (
	ADMIN    = "admin"
	OBSERVER = "observer"
)

type Role struct {
	name      string
	resources map[Resource]bool
}

func (r *Role) CanAccess(rsrc Resource) bool {
	return r.resources[rsrc]
}
