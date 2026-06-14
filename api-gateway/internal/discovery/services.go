package discovery

// Service names for routing and circuit breakers.
const (
	UserService         = "user"
	AuctionService      = "auction"
	BidService          = "bid"
	NotificationService = "notification"
)

// Registry holds upstream base URLs (Kubernetes DNS in production).
type Registry struct {
	User         string
	Auction      string
	Bid          string
	Notification string
}

func NewRegistry(user, auction, bid, notification string) *Registry {
	return &Registry{
		User: user, Auction: auction, Bid: bid, Notification: notification,
	}
}

func (r *Registry) BaseURL(name string) string {
	switch name {
	case UserService:
		return r.User
	case AuctionService:
		return r.Auction
	case BidService:
		return r.Bid
	case NotificationService:
		return r.Notification
	default:
		return ""
	}
}

// UpstreamURLs returns all backends for load balancing (extend with multiple pods via env).
func (r *Registry) UpstreamURLs(name string) []string {
	if u := r.BaseURL(name); u != "" {
		return []string{u}
	}
	return nil
}
