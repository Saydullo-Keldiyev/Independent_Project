'use client';

import { useEffect, useState, useRef } from 'react';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { auctionService, Auction } from '@/services/auction.service';
import { wsService } from '@/services/websocket.service';
import { formatPrice, formatDate } from '@/lib/utils';
import { CountdownTimer } from '@/components/auction/CountdownTimer';
import { BidPanel } from '@/components/auction/BidPanel';
import { BidHistory } from '@/components/auction/BidHistory';

interface Bid {
  id: string;
  user_id: string;
  amount: number;
  created_at: string;
}

export default function AuctionDetailPage() {
  const params = useParams();
  const id = params.id as string;

  const [bids, setBids] = useState<Bid[]>([]);
  const [livePrice, setLivePrice] = useState<number | null>(null);
  const [liveBidCount, setLiveBidCount] = useState<number | null>(null);
  const [flash, setFlash] = useState(false);

  // Use ref to track latest bids without causing re-renders in the WS handler
  const bidsRef = useRef<Bid[]>([]);
  bidsRef.current = bids;

  // Fetch auction data
  const { data: auctionData, isLoading } = useQuery({
    queryKey: ['auction', id],
    queryFn: () => auctionService.getById(id),
    enabled: !!id,
  });

  // Fetch bids
  const { data: bidsData } = useQuery({
    queryKey: ['auction-bids', id],
    queryFn: () => auctionService.getBids(id),
    enabled: !!id,
    refetchInterval: 10000, // refetch every 10s as fallback
  });

  const auction: Auction | null = auctionData?.data || auctionData || null;
  const auctionRef = useRef<Auction | null>(null);
  auctionRef.current = auction;

  useEffect(() => {
    if (bidsData) {
      const bidList = bidsData.data?.bids || bidsData.data || [];
      setBids(bidList);
    }
  }, [bidsData]);

  // WebSocket — real-time bid updates
  // Stable handler using refs to avoid dependency on bids/auction state
  useEffect(() => {
    if (!id) return;

    const handleNewBid = (data: any) => {
      const newBid: Bid = {
        id: data.bid_id || data.id || Math.random().toString(),
        user_id: data.user_id,
        amount: data.amount,
        created_at: data.timestamp || new Date().toISOString(),
      };

      setBids(prev => [newBid, ...prev]);
      setLivePrice(data.amount);
      setLiveBidCount(prev => (prev || auctionRef.current?.total_bids || 0) + 1);

      // Flash animation
      setFlash(true);
      setTimeout(() => setFlash(false), 1000);

      // Outbid notification — if someone else bid higher
      const myUserId = typeof window !== 'undefined' ? localStorage.getItem('user_id') : null;
      const currentBids = bidsRef.current;
      if (myUserId && data.user_id !== myUserId && currentBids.length > 0 && currentBids[0]?.user_id === myUserId) {
        if (typeof window !== 'undefined') {
          import('@/components/ui/Toast').then(({ useToast }) => {
            useToast.getState().add('⚠️ You have been outbid!', 'warning');
          });
        }
      }
    };

    wsService.connect(id);
    wsService.on('new_bid', handleNewBid);

    return () => {
      wsService.off('new_bid', handleNewBid);
      wsService.disconnect();
    };
  }, [id]);

  if (isLoading) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!auction) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center text-zinc-500">
        Auction not found
      </div>
    );
  }

  const currentPrice = livePrice || auction.current_price;
  const totalBids = liveBidCount || auction.total_bids;
  const isActive = auction.state === 'active';

  return (
    <div className="min-h-screen bg-zinc-950">
      {/* Header */}
      <header className="border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm sticky top-0 z-50">
        <div className="max-w-7xl mx-auto px-4 py-3 flex items-center gap-3">
          <Link href="/" className="text-lg font-bold text-emerald-400">🔨 AuctionHub</Link>
          <span className="text-zinc-700">/</span>
          <Link href="/auctions" className="text-zinc-500 hover:text-zinc-300 text-sm">Auctions</Link>
          <span className="text-zinc-700">/</span>
          <span className="text-zinc-400 text-sm truncate max-w-[200px]">{auction.title}</span>
        </div>
      </header>

      <div className="max-w-7xl mx-auto px-4 py-8">
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left — Main content */}
          <div className="lg:col-span-2 space-y-6">
            {/* Auction header */}
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <div className="flex items-center justify-between mb-4">
                <span className={`px-3 py-1 rounded-full text-xs font-semibold flex items-center gap-1.5 ${
                  isActive ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
                  'bg-zinc-700/50 text-zinc-400 border border-zinc-700'
                }`}>
                  {isActive && <span className="w-1.5 h-1.5 bg-emerald-400 rounded-full animate-pulse" />}
                  {auction.state.toUpperCase()}
                </span>

                {isActive && <CountdownTimer endTime={auction.end_time} />}
              </div>

              <h1 className="text-3xl font-bold text-zinc-100 mb-2">{auction.title}</h1>
              <p className="text-zinc-400 leading-relaxed">{auction.description || 'No description provided.'}</p>

              {/* Winner banner when auction ended */}
              {!isActive && auction.winner_id && (
                <div className="mt-6 p-4 bg-yellow-500/10 border border-yellow-500/30 rounded-xl flex items-center gap-3">
                  <span className="text-3xl">🏆</span>
                  <div>
                    <p className="text-yellow-400 font-bold text-lg">Auction Won!</p>
                    <p className="text-zinc-300 text-sm">
                      Winner: <span className="font-mono text-yellow-300">{auction.winner_id.slice(0, 12)}...</span>
                    </p>
                    <p className="text-zinc-500 text-xs mt-0.5">
                      Final Price: <span className="text-emerald-400 font-semibold">{formatPrice(currentPrice)}</span>
                    </p>
                  </div>
                </div>
              )}

              {!isActive && !auction.winner_id && (
                <div className="mt-6 p-4 bg-zinc-800/50 border border-zinc-700 rounded-xl flex items-center gap-3">
                  <span className="text-3xl">🏁</span>
                  <div>
                    <p className="text-zinc-400 font-bold text-lg">Auction Ended</p>
                    <p className="text-zinc-500 text-sm">No winner — no bids were placed.</p>
                  </div>
                </div>
              )}

              {/* Stats grid */}
              <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mt-6">
                <div className="bg-zinc-800/50 border border-zinc-800 rounded-lg p-3">
                  <p className="text-[10px] text-zinc-500 uppercase tracking-wider">Starting Price</p>
                  <p className="text-lg font-semibold text-zinc-300">{formatPrice(auction.starting_price)}</p>
                </div>
                <div className="bg-zinc-800/50 border border-zinc-800 rounded-lg p-3">
                  <p className="text-[10px] text-zinc-500 uppercase tracking-wider">Total Bids</p>
                  <p className="text-lg font-semibold text-zinc-300">{totalBids}</p>
                </div>
                <div className="bg-zinc-800/50 border border-zinc-800 rounded-lg p-3">
                  <p className="text-[10px] text-zinc-500 uppercase tracking-wider">Ends At</p>
                  <p className="text-sm font-medium text-zinc-300">{formatDate(auction.end_time)}</p>
                </div>
              </div>
            </div>

            {/* Bid History */}
            <BidHistory bids={bids} isLive={isActive} />
          </div>

          {/* Right — Bid Panel */}
          <div className="space-y-6">
            <div className="sticky top-20">
              <BidPanel
                auctionId={id}
                currentPrice={currentPrice}
                isActive={isActive}
                winnerId={auction.winner_id || undefined}
                sellerId={auction.seller_id}
              />

              {/* Auction info */}
              <div className="mt-4 bg-zinc-900 border border-zinc-800 rounded-xl p-5">
                <h4 className="text-sm font-semibold text-zinc-400 mb-3">Auction Info</h4>
                <div className="space-y-2.5 text-sm">
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Seller</span>
                    <span className="text-zinc-300 font-mono text-xs">{auction.seller_id.slice(0, 12)}...</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Created</span>
                    <span className="text-zinc-300">{formatDate(auction.created_at)}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-zinc-500">Auction ID</span>
                    <span className="text-zinc-500 font-mono text-xs">{auction.id.slice(0, 8)}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
