package auth

// Role constants used across all services for RBAC
const (
	RoleAdmin  = "admin"
	RoleSeller = "seller"
	RoleBidder = "bidder"
)

// AllRoles returns all valid roles
func AllRoles() []string {
	return []string{RoleAdmin, RoleSeller, RoleBidder}
}

// IsValidRole checks if a given role string is valid
func IsValidRole(role string) bool {
	for _, r := range AllRoles() {
		if r == role {
			return true
		}
	}
	return false
}
