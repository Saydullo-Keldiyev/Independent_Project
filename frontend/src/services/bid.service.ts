import api from '@/lib/api';

export const bidService = {
  placeBid: (auctionId: string, amount: number) =>
    api.post('/api/v1/bids', { auction_id: auctionId, amount }).then(r => r.data),

  getMyBids: () =>
    api.get('/api/v1/bids/me').then(r => r.data),

  getAuctionBids: (auctionId: string) =>
    api.get(`/api/v1/auctions/${auctionId}/bids`).then(r => r.data),
};
