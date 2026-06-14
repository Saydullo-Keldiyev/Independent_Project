package email

import "fmt"

// ── Template functions — return Message structs ready to send ──────────────────

func OutbidTemplate(auctionTitle string, newAmount float64) Message {
	return Message{
		Subject: fmt.Sprintf("You've been outbid on %s", auctionTitle),
		Body: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #e74c3c;">You've Been Outbid!</h2>
  <p>Someone placed a higher bid of <strong>$%.2f</strong> on <strong>%s</strong>.</p>
  <p>Don't miss out — place a new bid now to stay in the running!</p>
  <a href="https://auction.com" style="display: inline-block; padding: 12px 24px; background: #3498db; color: white; text-decoration: none; border-radius: 4px;">Place New Bid</a>
  <p style="color: #999; font-size: 12px; margin-top: 30px;">If you no longer wish to receive these emails, update your notification preferences.</p>
</body>
</html>`, newAmount, auctionTitle),
		IsHTML: true,
	}
}

func AuctionEndingSoonTemplate(auctionTitle string, minutesLeft int) Message {
	return Message{
		Subject: fmt.Sprintf("%s ends in %d minutes!", auctionTitle, minutesLeft),
		Body: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #f39c12;">⏰ Auction Ending Soon!</h2>
  <p><strong>%s</strong> ends in <strong>%d minutes</strong>.</p>
  <p>Make sure your bid is the highest before time runs out!</p>
  <a href="https://auction.com" style="display: inline-block; padding: 12px 24px; background: #e67e22; color: white; text-decoration: none; border-radius: 4px;">View Auction</a>
</body>
</html>`, auctionTitle, minutesLeft),
		IsHTML: true,
	}
}

func PaymentSuccessTemplate(auctionTitle string, amount float64) Message {
	return Message{
		Subject: fmt.Sprintf("Payment confirmed for %s", auctionTitle),
		Body: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #27ae60;">✅ Payment Confirmed</h2>
  <p>Your payment of <strong>$%.2f</strong> for <strong>%s</strong> has been processed successfully.</p>
  <p>The seller will be notified and shipping details will follow shortly.</p>
  <a href="https://auction.com/orders" style="display: inline-block; padding: 12px 24px; background: #27ae60; color: white; text-decoration: none; border-radius: 4px;">View Order</a>
</body>
</html>`, amount, auctionTitle),
		IsHTML: true,
	}
}

func WelcomeTemplate(username string) Message {
	return Message{
		Subject: "Welcome to Auction System!",
		Body: fmt.Sprintf(`
<!DOCTYPE html>
<html>
<body style="font-family: Arial, sans-serif; max-width: 600px; margin: 0 auto;">
  <h2 style="color: #3498db;">Welcome, %s! 🎉</h2>
  <p>Your account has been created successfully. You can now:</p>
  <ul>
    <li>Browse active auctions</li>
    <li>Place bids in real-time</li>
    <li>Create your own auctions</li>
  </ul>
  <a href="https://auction.com" style="display: inline-block; padding: 12px 24px; background: #3498db; color: white; text-decoration: none; border-radius: 4px;">Start Exploring</a>
</body>
</html>`, username),
		IsHTML: true,
	}
}
