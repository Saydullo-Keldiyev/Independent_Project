# Bugfix Requirements Document

## Introduction

The auction detail page's "Current Bid" price only updates after a manual page refresh, despite the WebSocket connection being established. When a user places a bid, the bid history shows the new entry (with a "LIVE" indicator), but the main Current Bid stat remains stale until the browser is refreshed. This undermines the real-time auction experience and can lead to users bidding on outdated price information.

## Bug Analysis

### Current Behavior (Defect)

1.1 WHEN a new bid is placed by any user THEN the system reconnects the WebSocket (disconnect + reconnect) due to `handleNewBid` callback reference changing on every bid arrival, causing missed broadcast messages during the reconnection window

1.2 WHEN a WebSocket "new_bid" message arrives THEN the system processes the message twice (once via the `'new_bid'` handler and once via the `'*'` wildcard handler), potentially corrupting bid state with duplicate entries

1.3 WHEN the current user places a bid via BidPanel THEN the system relies entirely on the WebSocket broadcast to update the UI, but the broadcast is lost during the reconnection cycle triggered by the stale callback dependency

1.4 WHEN `handleNewBid` fires and updates `bids` state THEN the system triggers a cascading re-render → new `handleNewBid` reference → useEffect cleanup → `wsService.disconnect()` → `wsService.connect()` → handler re-registration, creating an infinite reconnection loop on every incoming bid

### Expected Behavior (Correct)

2.1 WHEN a new bid is placed by any user THEN the system SHALL update the "Current Bid" price display immediately for all viewers without requiring a page refresh, and without disconnecting/reconnecting the WebSocket

2.2 WHEN a WebSocket "new_bid" message arrives THEN the system SHALL process the message exactly once, updating bid state, live price, and bid count atomically

2.3 WHEN the current user places a bid via BidPanel THEN the system SHALL optimistically update the local cache (React Query invalidation) to ensure the new bid is reflected immediately, independent of WebSocket delivery

2.4 WHEN the WebSocket handler processes a new bid THEN the system SHALL maintain a stable WebSocket connection without triggering useEffect cleanup/reconnection cycles

### Unchanged Behavior (Regression Prevention)

3.1 WHEN a user navigates to an auction detail page THEN the system SHALL CONTINUE TO establish a WebSocket connection and display the initial auction data (price, bid count, history) correctly

3.2 WHEN the auction ends or the user navigates away THEN the system SHALL CONTINUE TO properly disconnect the WebSocket and clean up event handlers

3.3 WHEN the WebSocket connection drops unexpectedly THEN the system SHALL CONTINUE TO attempt automatic reconnection with exponential backoff (up to 5 attempts)

3.4 WHEN a user is outbid THEN the system SHALL CONTINUE TO display the "You have been outbid!" toast notification

3.5 WHEN a bid is placed successfully via BidPanel THEN the system SHALL CONTINUE TO display the success toast and clear the input field

3.6 WHEN the WebSocket is temporarily unavailable THEN the system SHALL CONTINUE TO fall back to the 10-second React Query refetch interval for data freshness
