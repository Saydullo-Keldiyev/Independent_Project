package auth

func IsAdmin(role string) bool  { return role == RoleAdmin }
func IsSeller(role string) bool { return role == RoleSeller || role == RoleAdmin }

const (
	RoleAdmin  = "admin"
	RoleSeller = "seller"
	RoleBidder = "bidder"
)

func HasRole(role string, allowed ...string) bool {
	for _, a := range allowed {
		if role == a {
			return true
		}
	}
	return false
}
