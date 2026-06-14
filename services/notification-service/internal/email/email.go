package email

import (
	"fmt"
	"net/smtp"
	"strings"
)

// Sender sends emails via SMTP
type Sender struct {
	host     string
	port     int
	username string
	password string
	from     string
	fromName string
}

// Config holds SMTP configuration
type Config struct {
	Host        string
	Port        int
	Username    string
	Password    string
	FromAddress string
	FromName    string
}

// NewSender creates a new email sender
func NewSender(cfg Config) *Sender {
	return &Sender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.FromAddress,
		fromName: cfg.FromName,
	}
}

// Message represents an email to send
type Message struct {
	To      []string
	Subject string
	Body    string
	IsHTML  bool
}

// Send sends an email via SMTP
func (s *Sender) Send(msg Message) error {
	auth := smtp.PlainAuth("", s.username, s.password, s.host)

	contentType := "text/plain"
	if msg.IsHTML {
		contentType = "text/html"
	}

	headers := fmt.Sprintf(
		"From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: %s; charset=UTF-8\r\n\r\n",
		s.fromName,
		s.from,
		strings.Join(msg.To, ", "),
		msg.Subject,
		contentType,
	)

	body := headers + msg.Body
	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	return smtp.SendMail(addr, auth, s.from, msg.To, []byte(body))
}

// ── Email templates ───────────────────────────────────────────────────────────

// BidPlacedEmail returns the email body for a new bid notification
func BidPlacedEmail(auctionTitle string, amount float64, bidderName string) Message {
	return Message{
		Subject: fmt.Sprintf("New bid on %s", auctionTitle),
		Body: fmt.Sprintf(`
<h2>New Bid Placed</h2>
<p>A new bid of <strong>$%.2f</strong> has been placed on <strong>%s</strong> by %s.</p>
<p>Log in to place a higher bid before the auction ends.</p>
`, amount, auctionTitle, bidderName),
		IsHTML: true,
	}
}

// AuctionWonEmail returns the email body for auction winner notification
func AuctionWonEmail(auctionTitle string, amount float64) Message {
	return Message{
		Subject: fmt.Sprintf("Congratulations! You won %s", auctionTitle),
		Body: fmt.Sprintf(`
<h2>🎉 You Won the Auction!</h2>
<p>Congratulations! You won <strong>%s</strong> with a bid of <strong>$%.2f</strong>.</p>
<p>Our team will contact you shortly with payment and delivery details.</p>
`, auctionTitle, amount),
		IsHTML: true,
	}
}

// AuctionOutbidEmail notifies a bidder they've been outbid
func AuctionOutbidEmail(auctionTitle string, newAmount float64) Message {
	return Message{
		Subject: fmt.Sprintf("You've been outbid on %s", auctionTitle),
		Body: fmt.Sprintf(`
<h2>You've Been Outbid</h2>
<p>Someone placed a higher bid of <strong>$%.2f</strong> on <strong>%s</strong>.</p>
<p>Place a new bid now to stay in the running!</p>
`, newAmount, auctionTitle),
		IsHTML: true,
	}
}
