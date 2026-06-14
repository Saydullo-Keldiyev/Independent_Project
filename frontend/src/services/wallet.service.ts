import api from '@/lib/api';

export const walletService = {
  getWallet: () => api.get('/api/v1/wallet').then(r => r.data),
  getHistory: () => api.get('/api/v1/wallet/history').then(r => r.data),
  deposit: (amount: number) => api.post('/api/v1/wallet/deposit', { amount }).then(r => r.data),
  withdraw: (amount: number) => api.post('/api/v1/wallet/withdraw', { amount }).then(r => r.data),
};
