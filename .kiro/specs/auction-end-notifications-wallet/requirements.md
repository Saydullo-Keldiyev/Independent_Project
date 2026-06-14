# Requirements Document

## Introduction

This feature adds two tightly-coupled capabilities to the auction system:

1. **Auction End Notifications** — When an auction ends, the system sends targeted notifications to the seller (informing them of the outcome, winner, and final price) and to the winning bidder (confirming their win and final bid amount). Notifications are delivered via WebSocket (real-time) and email (persistent).

2. **Wallet Integration with Auction Lifecycle** — The wallet system is enhanced to fully support auction completion. Funds are held when a bid is placed, released when a bidder is outbid, charged (settled) from the winner's wallet on auction end, and credited to the seller's wallet.

The auction-service scheduler already publishes `auction.ended` Kafka events with winner and amount data. The notification-service already consumes these events but lacks seller/winner-specific notification logic. The user-service wallet already supports hold/release but lacks settle (charge winner) and credit (pay seller) operations.

## Glossary

- **Auction_Service**: The Go microservice responsible for auction lifecycle management and the scheduler that ends expired auctions.
- **Bid_Service**: The Go microservice responsible for bid placement, validation, and publishing bid events to Kafka.
- **Notification_Service**: The Go microservice that consumes Kafka events and dispatches notifications via WebSocket and email.
- **User_Service**: The Go microservice that manages user accounts and the wallet system (balance, holds, transactions).
- **Wallet**: A financial account associated with each user, storing available balance and recording all transactions.
- **Hold**: A reservation of funds from a bidder's available balance, reducing spendable balance while the bid is active.
- **Release**: The return of held funds to a bidder's available balance when they are outbid or the auction is cancelled.
- **Settle**: The finalization of a hold into a charge — converting the winner's held funds into a payment upon auction completion.
- **Credit**: The addition of funds to the seller's wallet balance representing the winning bid payment.
- **Kafka_Event**: An asynchronous message published to a Kafka topic, used for inter-service communication.
- **DLQ**: Dead Letter Queue — a Kafka topic where failed messages are sent for later inspection and retry.

## Requirements

### Requirement 1: Notify Seller on Auction End with Winner

**User Story:** As a seller, I want to receive a notification when my auction ends with a winner, so that I know who won and the final sale price.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE Notification_Service SHALL create a persistent notification for the seller containing the auction title, winner identifier, and winning bid amount.
2. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE Notification_Service SHALL deliver a real-time WebSocket notification to the seller with the auction outcome details.
3. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID AND the seller has a registered email address, THE Notification_Service SHALL send an email to the seller containing the auction title, winner identifier, and winning bid amount.
4. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE Notification_Service SHALL include the auction title in the event payload by reading it from the existing AuctionEvent.Title field.

### Requirement 2: Notify Seller on Auction End without Winner

**User Story:** As a seller, I want to be notified when my auction ends without any bids or without meeting the reserve price, so that I can decide to relist.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received with an empty WinnerID, THE Notification_Service SHALL create a persistent notification for the seller indicating the auction ended without a winner.
2. WHEN an `auction.ended` Kafka event is received with an empty WinnerID, THE Notification_Service SHALL deliver a real-time WebSocket notification to the seller indicating the auction ended without a winning bid.

### Requirement 3: Notify Winning Bidder on Auction End

**User Story:** As a bidder, I want to receive a notification when I win an auction, so that I know I secured the item and can confirm the final price.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE Notification_Service SHALL create a persistent notification for the winner congratulating them and confirming the winning bid amount.
2. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE Notification_Service SHALL deliver a real-time WebSocket notification to the winner with the auction title and confirmed winning amount.
3. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID AND the winner has a registered email address, THE Notification_Service SHALL send a congratulations email to the winner containing the auction title and winning bid amount.

### Requirement 4: Hold Funds on Bid Placement

**User Story:** As a bidder, I want my bid amount to be reserved from my wallet when I place a bid, so that I cannot spend the same funds elsewhere while the auction is active.

#### Acceptance Criteria

1. WHEN a bid is placed, THE Bid_Service SHALL call the User_Service wallet hold endpoint to reserve the bid amount from the bidder's available balance.
2. IF the bidder's available wallet balance is less than the bid amount, THEN THE Bid_Service SHALL reject the bid with an insufficient funds error before persisting the bid.
3. WHEN a hold is successfully created, THE User_Service SHALL record a wallet transaction of type "hold" with the bid reference and amount.
4. THE User_Service SHALL execute the hold operation within a database transaction using row-level locking (SELECT FOR UPDATE) to prevent race conditions.

### Requirement 5: Release Funds for Outbid Users

**User Story:** As a bidder, I want my held funds to be returned to my available balance when I am outbid, so that I can use those funds for other bids.

#### Acceptance Criteria

1. WHEN a new bid is placed that exceeds the current highest bid, THE Bid_Service SHALL publish a `bid.outbid` Kafka event containing the previous highest bidder's user ID and their held bid amount.
2. WHEN a `bid.outbid` Kafka event is received, THE User_Service SHALL release the held funds for the outbid user, restoring the amount to their available balance.
3. WHEN funds are released, THE User_Service SHALL record a wallet transaction of type "release" with the auction reference and released amount.
4. IF the release operation fails, THEN THE User_Service SHALL send the failed event to the DLQ for manual reconciliation.

### Requirement 6: Settle Winner's Wallet on Auction End

**User Story:** As a platform operator, I want the winner's held funds to be settled (converted to a charge) when an auction ends, so that the payment is finalized.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE User_Service SHALL settle the winner's held funds by converting the hold into a permanent debit transaction of type "settle".
2. THE User_Service SHALL execute the settle operation within a database transaction to maintain data consistency.
3. IF the settle operation fails, THEN THE User_Service SHALL send the failed event to the DLQ and log the error for manual intervention.
4. WHEN a settle transaction is recorded, THE User_Service SHALL store the auction ID and winning amount as transaction metadata.

### Requirement 7: Credit Seller's Wallet on Auction End

**User Story:** As a seller, I want to receive the winning bid amount in my wallet when my auction ends successfully, so that I am paid for the item sold.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received with a non-empty WinnerID, THE User_Service SHALL credit the seller's wallet with the winning bid amount.
2. WHEN the seller's wallet is credited, THE User_Service SHALL record a wallet transaction of type "credit" with the auction ID and amount.
3. THE User_Service SHALL execute the credit operation within the same logical flow as the settle operation to ensure both succeed or both are retried.
4. IF the credit operation fails, THEN THE User_Service SHALL send the failed event to the DLQ for manual reconciliation.

### Requirement 8: Release All Non-Winner Holds on Auction End

**User Story:** As a bidder who did not win, I want all my held funds for the ended auction to be returned to my available balance when the auction concludes.

#### Acceptance Criteria

1. WHEN an `auction.ended` Kafka event is received, THE User_Service SHALL release holds for all bidders on that auction except the winner.
2. WHEN holds are released for non-winners, THE User_Service SHALL record a wallet transaction of type "release" for each bidder with the auction reference.
3. IF any individual release operation fails, THEN THE User_Service SHALL continue processing remaining releases and log the failure for retry.

### Requirement 9: Idempotent Event Processing

**User Story:** As a platform operator, I want all wallet and notification operations triggered by Kafka events to be idempotent, so that duplicate event delivery does not cause double-charges or duplicate notifications.

#### Acceptance Criteria

1. THE Notification_Service SHALL check event idempotency using a unique event identifier in Redis before processing any auction end notification.
2. THE User_Service SHALL check event idempotency using a unique event identifier before processing any wallet settle, credit, or release operation triggered by an auction end event.
3. WHEN a duplicate event is detected, THE Notification_Service SHALL skip processing and log the duplicate at debug level.
4. WHEN a duplicate event is detected, THE User_Service SHALL skip processing and log the duplicate at debug level.

### Requirement 10: Enriched Auction Ended Event Payload

**User Story:** As a developer, I want the `auction.ended` Kafka event to contain all information needed by downstream consumers, so that services do not need to make synchronous calls back to the auction-service.

#### Acceptance Criteria

1. WHEN the Auction_Service scheduler ends an auction with a winner, THE Auction_Service SHALL publish an `auction.ended` event containing: auction ID, seller ID, winner ID, winning amount, auction title, and timestamp.
2. WHEN the Auction_Service scheduler ends an auction without a winner, THE Auction_Service SHALL publish an `auction.ended` event containing: auction ID, seller ID, auction title, and timestamp with an empty winner ID and zero amount.
3. THE Auction_Service SHALL use the existing `AuctionEvent` struct which already includes all required fields (AuctionID, SellerID, WinnerID, Amount, Title, Timestamp).

### Requirement 11: Wallet Transaction Types for Auction Settlement

**User Story:** As a developer, I want the wallet system to support "settle" and "credit" transaction types, so that auction payment flows are accurately recorded and auditable.

#### Acceptance Criteria

1. THE User_Service SHALL support a "settle" transaction type representing the finalization of a winner's held funds into a permanent charge.
2. THE User_Service SHALL support a "credit" transaction type representing the payment received by a seller from a completed auction.
3. THE User_Service SHALL expose settle and credit operations as internal service methods callable by the Kafka event consumer.
4. THE User_Service SHALL record all settle and credit transactions with the associated auction ID in the transaction description for audit traceability.
