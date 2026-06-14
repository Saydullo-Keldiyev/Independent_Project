'use client';

import { useQuery } from '@tanstack/react-query';
import { Navbar } from '@/components/layout/Navbar';
import { AuthGuard } from '@/components/layout/AuthGuard';
import { AuctionCard } from '@/components/auction/AuctionCard';
import api from '@/lib/api';

export default function WatchlistPage() {
  const { data, isLoading } = useQuery({
    queryKey: ['watchlist'],
    queryFn: () => api.get('/api/v1/watchlist').then(r => r.data),
  });

  const auctions = data?.data || [];

  return (
    <AuthGuard>
      <div className="min-h-screen bg-zinc-950">
        <Navbar />
        <div className="max-w-7xl mx-auto px-4 py-8">
          <h1 className="text-3xl font-bold text-zinc-100 mb-2">👁️ Watchlist</h1>
          <p className="text-zinc-500 mb-8">Auctions you&apos;re watching</p>

          {isLoading ? (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {[...Array(3)].map((_, i) => (
                <div key={i} className="bg-zinc-900 border border-zinc-800 rounded-xl p-4 animate-pulse">
                  <div className="h-44 bg-zinc-800 rounded-lg mb-4" />
                  <div className="h-4 bg-zinc-800 rounded w-3/4" />
                </div>
              ))}
            </div>
          ) : auctions.length === 0 ? (
            <div className="text-center py-20 bg-zinc-900/50 border border-zinc-800 rounded-xl">
              <div className="text-5xl mb-4">👁️</div>
              <p className="text-zinc-400 text-lg">Your watchlist is empty</p>
              <p className="text-zinc-600 text-sm mt-2">Add auctions to watch their progress</p>
            </div>
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
              {auctions.map((auction: any) => (
                <AuctionCard key={auction.id} auction={auction} />
              ))}
            </div>
          )}
        </div>
      </div>
    </AuthGuard>
  );
}
