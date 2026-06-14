import http from 'k6/http';
import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// ── Custom metrics ────────────────────────────────────────────────────────────
const bidLatency = new Trend('bid_latency_ms');
const bidFailRate = new Rate('bid_fail_rate');

// ── Test scenarios ────────────────────────────────────────────────────────────
export const options = {
  scenarios: {
    // Normal load: 100 users placing bids
    normal_load: {
      executor: 'constant-vus',
      vus: 100,
      duration: '5m',
      startTime: '0s',
    },
    // Spike test: sudden burst to 5000 users
    spike_test: {
      executor: 'ramping-vus',
      startVUs: 100,
      stages: [
        { duration: '30s', target: 100 },
        { duration: '10s', target: 5000 },  // spike!
        { duration: '2m', target: 5000 },
        { duration: '30s', target: 100 },   // recover
      ],
      startTime: '6m',
    },
    // Stress test: ramp until failure
    stress_test: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 1000 },
        { duration: '2m', target: 5000 },
        { duration: '2m', target: 10000 },
        { duration: '2m', target: 15000 },  // find breaking point
        { duration: '1m', target: 0 },
      ],
      startTime: '15m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<200', 'p(99)<500'],  // 95th < 200ms, 99th < 500ms
    bid_latency_ms: ['p(95)<100'],                   // bid placement < 100ms p95
    bid_fail_rate: ['rate<0.01'],                     // < 1% failure rate
    http_req_failed: ['rate<0.01'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const TOKEN = __ENV.AUTH_TOKEN || 'test-token';

// ── Main test function ────────────────────────────────────────────────────────
export default function () {
  const auctionId = `auction-${Math.floor(Math.random() * 100)}`;
  const amount = Math.floor(Math.random() * 10000) + 100;

  const payload = JSON.stringify({
    auction_id: auctionId,
    amount: amount,
  });

  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${TOKEN}`,
      'Idempotency-Key': `${__VU}-${__ITER}-${Date.now()}`,
    },
  };

  const start = Date.now();
  const res = http.post(`${BASE_URL}/api/v1/bids`, payload, params);
  const duration = Date.now() - start;

  bidLatency.add(duration);

  const success = check(res, {
    'status is 201 or 200': (r) => r.status === 201 || r.status === 200,
    'response has success': (r) => {
      try { return JSON.parse(r.body).success === true; }
      catch { return false; }
    },
    'latency < 200ms': () => duration < 200,
  });

  bidFailRate.add(!success);

  sleep(Math.random() * 2); // 0-2s think time
}

// ── WebSocket stress test ─────────────────────────────────────────────────────
export function websocketTest() {
  const url = `ws://${BASE_URL.replace('http://', '')}/api/v1/ws/auction-1`;

  const res = ws.connect(url, { headers: { 'Authorization': `Bearer ${TOKEN}` } }, function (socket) {
    socket.on('open', () => {
      console.log('WS connected');
    });

    socket.on('message', (data) => {
      check(data, {
        'ws message received': (d) => d.length > 0,
      });
    });

    socket.on('error', (e) => {
      console.error('WS error:', e);
    });

    // Keep connection alive for 30s
    sleep(30);
    socket.close();
  });

  check(res, { 'ws status is 101': (r) => r && r.status === 101 });
}
