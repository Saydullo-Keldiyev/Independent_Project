package kafka

const (
	EventUserRegistered = "USER_REGISTERED"
	EventUserLoggedIn   = "USER_LOGGED_IN"
	EventWalletCreated  = "WALLET_CREATED"
)

type UserEvent struct {
	Type      string `json:"type"`
	UserID    string `json:"user_id"`
	Email     string `json:"email,omitempty"`
	Role      string `json:"role,omitempty"`
	WalletID  string `json:"wallet_id,omitempty"`
	Timestamp int64  `json:"timestamp"`
}
