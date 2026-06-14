import api from '@/lib/api';

export interface Auction {
  id: string;
  seller_id: string;
  title: string;
  description: string;
  starting_price: number;
  current_price: number;
  state: string;
  start_time: string;
  end_time: string;
  winner_id?: string;
  total_bids: number;
  created_at: string;
}

export const auctionService = {
  list: (params?: { state?: string; page?: number; page_size?: number }) =>
    api.get('/api/v1/auctions', { params }).then(r => r.data),

  getById: (id: string) =>
    api.get(`/api/v1/auctions/${id}`).then(r => r.data),

  create: (data: { title: string; description: string; starting_price: number; reserve_price?: number; start_time: string; end_time: string }) =>
    api.post('/api/v1/auctions', data).then(r => r.data),

  update: (id: string, data: Partial<Auction>) =>
    api.put(`/api/v1/auctions/${id}`, data).then(r => r.data),

  delete: (id: string) =>
    api.delete(`/api/v1/auctions/${id}`).then(r => r.data),

  getMyAuctions: () =>
    api.get('/api/v1/seller/auctions').then(r => r.data),

  getCategories: () =>
    api.get('/api/v1/categories').then(r => r.data),

  getBids: (auctionId: string) =>
    api.get(`/api/v1/auctions/${auctionId}/bids`).then(r => r.data),

  getMinBid: (auctionId: string) =>
    api.get(`/api/v1/auctions/${auctionId}/min-bid`).then(r => r.data),
};
