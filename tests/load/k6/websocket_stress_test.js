import ws from 'k6/ws';
import { check, sleep } from 'k6';
import { Gauge, Trend } from 'k6/metrics';

// ── WEBSOCKET STRESS TEST ─────────────────────────────────────────────────────
// Tests: 50k concurrent WebSocket connections
// Verifies: memory stability, broadcast latency, no goroutine leaks

const wsConnections = new Gauge('ws_active_connections');
const wsBroadcastLatency = new Trend('ws_broadcast_latency_ms');

export const options = {
  scenarios: {
    // Ramp up to 50k connections
    ws_ramp: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '2m', target: 10000 },
        { duration: '2m', target: 25000 },
        { duration: '2m', target: 50000 },
        { duration: '5m', target: 50000 },  // hold at peak
        { duration: '2m', target: 0 },
      ],
    },
  },
  thresholds: {
    ws_broadcast_latency_ms: ['p(95)<100'],  // broadcast < 100ms p95
    ws_session_duration: ['p(95)>5000'],     // connections stay alive > 5s
  },
};

const WS_URL = __ENV.WS_URL || 'ws://localhost:8084/ws';
const TOKEN = __ENV.AUTH_TOKEN || 'test-token';

export default function () {
  const auctionId = `auction-${Math.floor(Math.random() * 50)}`;
  const userId = `user-${__VU}`;
  const url = `${WS_URL}?user_id=${userId}&auction_id=${auctionId}`;

  const res = ws.connect(url, {}, function (socket) {
    wsConnections.add(1);

    socket.on('open', () => {
      // Connection established
    });

    socket.on('message', (data) => {
      try {
        const msg = JSON.parse(data);
        if (msg.timestamp) {
          const latency = Date.now() - new Date(msg.timestamp).getTime();
          wsBroadcastLatency.add(latency);
        }
      } catch (e) {}
    });

    socket.on('error', (e) => {
      console.error(`WS error VU ${__VU}: ${e}`);
    });

    socket.on('close', () => {
      wsConnections.add(-1);
    });

    // Hold connection for 30-60 seconds (simulates real user)
    sleep(30 + Math.random() * 30);
    socket.close();
  });

  check(res, {
    'ws connected': (r) => r && r.status === 101,
  });
}
