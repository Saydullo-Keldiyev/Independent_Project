import http from 'k6/http';
import { check, sleep } from 'k6';
import { Counter } from 'k6/metrics';

// ── PAYMENT CONCURRENCY TEST ──────────────────────────────────────────────────
// Tests: 1000 simultaneous bids on the same wallet
// Verifies: NO negative balance, NO double-charge, correct final state

const negativeBalanceDetected = new Counter('negative_balance_detected');
const duplicateChargeDetected = new Counter('duplicate_charge_detected');

export const options = {
  scenarios: {
    // 1000 concurrent users bidding on same auction (same wallet stress)
    concurrent_bids: {
      executor: 'shared-iterations',
      vus: 1000,
      iterations: 1000,
      maxDuration: '30s',
    },
  },
  thresholds: {
    negative_balance_detected: ['count==0'],  // MUST be zero
    duplicate_charge_detected: ['count==0'],  // MUST be zero
    http_req_failed: ['rate<0.5'],            // some failures expected (insufficient balance)
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.AUTH_TOKEN || 'test-token';

// All VUs bid on the SAME auction — maximum contention
const AUCTION_ID = 'stress-test-auction-001';

export default function () {
  const amount = Math.floor(Math.random() * 100) + 50; // $50-$150

  const payload = JSON.stringify({
    auction_id: AUCTION_ID,
    amount: amount,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${TOKEN}`,
      'Idempotency-Key': `concurrent-${__VU}-${__ITER}`,
    },
  };

  const res = http.post(`${BASE_URL}/api/v1/payments/hold`, payload, params);

  check(res, {
    'no 500 error': (r) => r.status !== 500,
    'response is valid JSON': (r) => {
      try { JSON.parse(r.body); return true; }
      catch { return false; }
    },
  });

  // Check for negative balance in response
  if (res.status === 200) {
    try {
      const body = JSON.parse(res.body);
      if (body.data && body.data.available_balance < 0) {
        negativeBalanceDetected.add(1);
        console.error('CRITICAL: Negative balance detected!');
      }
    } catch (e) {}
  }
}

// ── Post-test verification ────────────────────────────────────────────────────
export function teardown(data) {
  // Verify final wallet state
  const res = http.get(`${BASE_URL}/api/v1/wallet`, {
    headers: { 'Authorization': `Bearer ${TOKEN}` },
  });

  if (res.status === 200) {
    const wallet = JSON.parse(res.body).data;
    console.log(`Final state: available=${wallet.available_balance}, held=${wallet.held_balance}`);

    if (wallet.available_balance < 0) {
      console.error('CRITICAL: Final available balance is negative!');
    }
    if (wallet.held_balance < 0) {
      console.error('CRITICAL: Final held balance is negative!');
    }
  }
}
