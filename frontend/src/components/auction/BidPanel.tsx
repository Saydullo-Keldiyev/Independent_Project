'use client';

import { useState } from 'react';
import { bidService } from '@/services/bid.service';
import { useAuthStore } from '@/store/auth-store';
import { useToast } from '@/components/ui/Toast';
import { LoginModal } from '@/components/ui/LoginModal';
import { formatPrice } from '@/lib/utils';

interface Props {
  auctionId: string;
  currentPrice: number;
  isActive: boolean;
  winnerId?: string;
  sellerId?: string;
}

export function BidPanel({ auctionId, currentPrice, isActive, winnerId, sellerId }: Props) {
  const [amount, setAmount] = useState('');
  const [loading, setLoading] = useState(false);
  const [showConfirm, setShowConfirm] = useState(false);
  const [showLogin, setShowLogin] = useState(false);
  const { user, isAuthenticated } = useAuthStore();
  const toast = useToast();

  const minBid = currentPrice + 1;
  const isSeller = user?.id && sellerId && user.id === sellerId;

  // Not active
  if (!isActive) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 text-center">
        <div className="text-3xl mb-3">🏁</div>
        <p className="text-zinc-400 text-lg">Auction Ended</p>
        {winnerId && <p className="mt-2 text-emerald-400 font-medium">Winner: {winnerId.slice(0, 8)}...</p>}
      </div>
    );
  }

  // Seller can't bid
  if (isSeller) {
    return (
      <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 text-center">
        <div className="text-3xl mb-3">🏷️</div>
        <p className="text-zinc-200 text-lg font-medium">This is your auction</p>
        <p className="text-zinc-500 text-sm mt-2">You cannot bid on your own auction</p>
        <div className="mt-4 p-3 bg-zinc-800/50 rounded-lg">
          <p className="text-xs text-zinc-500">Current highest bid</p>
          <p className="text-xl font-bold text-emerald-400">{formatPrice(currentPrice)}</p>
        </div>
      </div>
    );
  }

  const handleBidClick = () => {
    if (!isAuthenticated) {
      setShowLogin(true);
      return;
    }

    const bidAmount = parseFloat(amount);
    if (!bidAmount || bidAmount < minBid) {
      toast.add(`Minimum bid is ${formatPrice(minBid)}`, 'warning');
      return;
    }
    setShowConfirm(true);
  };

  const confirmBid = async () => {
    setShowConfirm(false);
    setLoading(true);
    try {
      await bidService.placeBid(auctionId, parseFloat(amount));
      toast.add('🎉 Bid placed successfully!', 'success');
      setAmount('');
    } catch (err: any) {
      const msg = err.response?.data?.error || 'Failed to place bid';
      if (msg.includes('insufficient') || msg.includes('balance')) {
        toast.add('💰 Insufficient wallet balance. Please deposit first.', 'error');
      } else {
        toast.add(msg, 'error');
      }
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
        <h3 className="text-lg font-bold text-zinc-100 mb-4 flex items-center gap-2">
          💰 Place Your Bid
          <span className="inline-block w-2 h-2 bg-emerald-400 rounded-full animate-pulse" />
        </h3>

        <form onSubmit={e => { e.preventDefault(); handleBidClick(); }} className="space-y-4">
          <div>
            <label className="block text-xs text-zinc-500 uppercase tracking-wide mb-2">
              Bid Amount (min: {formatPrice(minBid)})
            </label>
            <div className="relative">
              <span className="absolute left-4 top-3 text-zinc-500 font-medium">$</span>
              <input
                type="number"
                step="0.01"
                min={minBid}
                value={amount}
                onChange={e => setAmount(e.target.value)}
                placeholder={minBid.toFixed(2)}
                className="w-full pl-8 pr-4 py-3 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 font-mono text-lg focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 outline-none transition-all"
                required
              />
            </div>
          </div>

          {/* Quick bid buttons */}
          <div className="flex gap-2">
            {[1, 5, 10, 50].map(inc => (
              <button
                key={inc}
                type="button"
                onClick={() => setAmount((currentPrice + inc).toFixed(2))}
                className="flex-1 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-xs text-zinc-400 hover:border-emerald-500/50 hover:text-emerald-400 transition-colors"
              >
                +${inc}
              </button>
            ))}
          </div>

          <button
            type="submit"
            disabled={loading}
            className="w-full py-3.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg font-bold text-lg transition-all disabled:opacity-50 disabled:cursor-not-allowed active:scale-[0.98]"
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <span className="w-4 h-4 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                Placing bid...
              </span>
            ) : (
              '🔨 Place Bid'
            )}
          </button>

          <p className="text-[11px] text-zinc-600 text-center">
            By bidding, you agree to pay if you win this auction.
          </p>
        </form>
      </div>

      {/* Confirm dialog */}
      {showConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setShowConfirm(false)} />
          <div className="relative bg-zinc-900 border border-zinc-800 rounded-xl p-6 max-w-sm w-full mx-4 animate-slide-up">
            <h3 className="text-lg font-bold text-zinc-100 mb-2">Confirm Bid</h3>
            <p className="text-zinc-400 text-sm mb-4">
              You are about to bid <span className="text-emerald-400 font-bold">{formatPrice(parseFloat(amount))}</span> on this auction.
            </p>
            <p className="text-xs text-zinc-500 mb-6">This amount will be held from your wallet until the auction ends.</p>
            <div className="flex gap-3">
              <button onClick={() => setShowConfirm(false)} className="flex-1 py-2.5 bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg font-medium">
                Cancel
              </button>
              <button onClick={confirmBid} className="flex-1 py-2.5 bg-emerald-600 text-white rounded-lg font-medium hover:bg-emerald-500">
                Confirm Bid
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Login modal for guests */}
      <LoginModal isOpen={showLogin} onClose={() => setShowLogin(false)} message="Please login to place a bid" />
    </>
  );
}
