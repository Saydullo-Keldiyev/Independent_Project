package auth

import "github.com/auction-system/user-service/internal/model"

// Role hierarchy for RBAC checks
var roleLevel = map[model.Role]int{
	model.RoleBidder: 1,
	model.RoleSeller: 2,
	model.RoleAdmin:  3,
}

func HasRole(userRole, required model.Role) bool {
	return roleLevel[userRole] >= roleLevel[required]
}

func CanAccessWallet(userRole model.Role) bool {
	return HasRole(userRole, model.RoleBidder)
}

func CanManageUsers(userRole model.Role) bool {
	return userRole == model.RoleAdmin
}
