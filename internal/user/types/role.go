package types

import "time"

type Role struct {
	ID        string
	OrgID     string
	Name      string
	IsBuiltin bool
	CreatedAt time.Time
}

// Built-in role names, created for every organization by
// EnsureBuiltinRoles. Companies can add custom roles on top of these
// (DESIGN.md 7.1: "支持管理员自定义角色...不锁死在内置角色里").
const (
	RoleSuperAdmin     = "超管"
	RoleAdmin          = "管理员"
	RoleDepartmentLead = "部门负责人"
	RoleEmployee       = "员工"
)
