import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const auctionListDuration = new Trend('auction_list_duration');
const auctionDetailDuration = new Trend('auction_detail_duration');
const createAuctionDuration = new Trend('create_auction_duration');

export const options = {
  stages: [
    { duration: '30s', target: 50 },   // ramp up
    { duration: '2m',  target: 200 },  // sustained load
    { duration: '1m',  target: 500 },  // peak load
    { duration: '30s', target: 0 },    // ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.05'],
    auction_list_duration: ['p(95)<300'],
    auction_detail_duration: ['p(95)<200'],
  },
};

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export function setup() {
  // Login to get token
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    email: 'loadtest@auction.com',
    password: 'loadtest123',
  }), { headers: { 'Content-Type': 'application/json' } });

  const token = loginRes.json('data.access_token') || loginRes.json('access_token');
  return { token };
}

export default function(data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Scenario 1: List auctions (most common operation)
  const listRes = http.get(`${BASE_URL}/api/v1/auctions?page=1&page_size=12`, { headers });
  auctionListDuration.add(listRes.timings.duration);
  check(listRes, { 'list 200': (r) => r.status === 200 }) || errorRate.add(1);

  sleep(0.5);

  // Scenario 2: Get auction detail
  const auctions = listRes.json('data.auctions') || listRes.json('data') || [];
  if (auctions.length > 0) {
    const randomAuction = auctions[Math.floor(Math.random() * auctions.length)];
    const detailRes = http.get(`${BASE_URL}/api/v1/auctions/${randomAuction.id}`, { headers });
    auctionDetailDuration.add(detailRes.timings.duration);
    check(detailRes, { 'detail 200': (r) => r.status === 200 }) || errorRate.add(1);
  }

  sleep(0.5);

  // Scenario 3: Create auction (10% of users — sellers)
  if (Math.random() < 0.1) {
    const createRes = http.post(`${BASE_URL}/api/v1/auctions`, JSON.stringify({
      title: `Load Test Auction ${Date.now()}`,
      description: 'Created by k6 load test',
      starting_price: Math.floor(Math.random() * 1000) + 10,
      start_time: new Date().toISOString(),
      end_time: new Date(Date.now() + 3600000).toISOString(),
    }), { headers });
    createAuctionDuration.add(createRes.timings.duration);
    check(createRes, { 'create 2xx': (r) => r.status >= 200 && r.status < 300 }) || errorRate.add(1);
  }

  sleep(1);
}
