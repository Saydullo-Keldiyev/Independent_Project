'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { useAuthStore } from '@/store/auth-store';
import { auctionService } from '@/services/auction.service';
import { walletService } from '@/services/wallet.service';
import { bidService } from '@/services/bid.service';
import { Navbar } from '@/components/layout/Navbar';
import { formatPrice } from '@/lib/utils';

export default function DashboardPage() {
  const { user, isAuthenticated, isLoading, loadUser } = useAuthStore();
  const router = useRouter();

  useEffect(() => { loadUser(); }, [loadUser]);
  useEffect(() => {
    if (!isLoading && !isAuthenticated) router.push('/login');
  }, [isLoading, isAuthenticated, router]);

  const { data: walletData } = useQuery({
    queryKey: ['wallet'],
    queryFn: walletService.getWallet,
    enabled: isAuthenticated,
  });

  const { data: myBids } = useQuery({
    queryKey: ['my-bids'],
    queryFn: bidService.getMyBids,
    enabled: isAuthenticated,
  });

  const { data: myAuctions } = useQuery({
    queryKey: ['my-auctions'],
    queryFn: auctionService.getMyAuctions,
    enabled: isAuthenticated && user?.role === 'seller',
  });

  const wallet = walletData?.data || walletData;
  const bids = myBids?.data?.bids || myBids?.data || [];
  const auctions = myAuctions?.data || [];

  if (isLoading) return <div className="min-h-screen bg-zinc-950 flex items-center justify-center"><div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" /></div>;

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />

      <div className="max-w-7xl mx-auto px-4 py-8">
        {/* Welcome */}
        <div className="mb-8">
          <h1 className="text-3xl font-bold text-zinc-100">Welcome back, {user?.first_name} 👋</h1>
          <p className="text-zinc-500 mt-1">Here&apos;s your auction activity overview</p>
        </div>

        {/* Stats cards */}
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
          <StatCard title="Wallet Balance" value={formatPrice(wallet?.balance || wallet?.available_balance || 0)} icon="💰" color="emerald" />
          <StatCard title="Active Bids" value={bids.length.toString()} icon="🔨" color="blue" />
          <StatCard title="My Auctions" value={Array.isArray(auctions) ? auctions.length.toString() : '0'} icon="🏷️" color="purple" />
          <StatCard title="Role" value={user?.role || 'bidder'} icon="👤" color="orange" />
        </div>

        {/* Quick actions */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
          <Link href="/auctions" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
            <div className="text-2xl mb-2">🔍</div>
            <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Browse Auctions</h3>
            <p className="text-sm text-zinc-500 mt-1">Find items to bid on</p>
          </Link>
          {(user?.role === 'seller' || user?.role === 'admin') && (
            <Link href="/auctions/create" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
              <div className="text-2xl mb-2">➕</div>
              <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Create Auction</h3>
              <p className="text-sm text-zinc-500 mt-1">List an item for sale</p>
            </Link>
          )}
          <Link href="/wallet" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
            <div className="text-2xl mb-2">💳</div>
            <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Wallet</h3>
            <p className="text-sm text-zinc-500 mt-1">Manage your funds</p>
          </Link>
          {(user?.role === 'seller' || user?.role === 'admin') && (
            <Link href="/analytics" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
              <div className="text-2xl mb-2">📊</div>
              <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Analytics</h3>
              <p className="text-sm text-zinc-500 mt-1">View your performance</p>
            </Link>
          )}
          <Link href="/watchlist" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
            <div className="text-2xl mb-2">👁️</div>
            <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Watchlist</h3>
            <p className="text-sm text-zinc-500 mt-1">Saved auctions</p>
          </Link>
          <Link href="/notifications" className="bg-zinc-900 border border-zinc-800 rounded-xl p-5 hover:border-emerald-500/50 transition-colors group">
            <div className="text-2xl mb-2">🔔</div>
            <h3 className="font-semibold text-zinc-200 group-hover:text-emerald-400">Notifications</h3>
            <p className="text-sm text-zinc-500 mt-1">Alerts & updates</p>
          </Link>
        </div>

        {/* Recent bids */}
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
          <h2 className="text-lg font-bold text-zinc-100 mb-4">Recent Bids</h2>
          {bids.length === 0 ? (
            <p className="text-zinc-500 text-center py-6">No bids yet. Start bidding!</p>
          ) : (
            <div className="space-y-2">
              {bids.slice(0, 5).map((bid: any, i: number) => (
                <div key={bid.id || i} className="flex items-center justify-between p-3 bg-zinc-800/50 rounded-lg">
                  <span className="text-sm text-zinc-300">Auction: {bid.auction_id?.slice(0, 8)}...</span>
                  <span className="font-bold text-emerald-400">{formatPrice(bid.amount)}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function StatCard({ title, value, icon, color }: { title: string; value: string; icon: string; color: string }) {
  const colors: Record<string, string> = {
    emerald: 'border-emerald-500/20 bg-emerald-500/5',
    blue: 'border-blue-500/20 bg-blue-500/5',
    purple: 'border-purple-500/20 bg-purple-500/5',
    orange: 'border-orange-500/20 bg-orange-500/5',
  };
  return (
    <div className={`border rounded-xl p-5 ${colors[color] || colors.emerald}`}>
      <div className="flex items-center justify-between">
        <div>
          <p className="text-xs text-zinc-500 uppercase tracking-wide">{title}</p>
          <p className="text-2xl font-bold text-zinc-100 mt-1">{value}</p>
        </div>
        <span className="text-3xl">{icon}</span>
      </div>
    </div>
  );
}
