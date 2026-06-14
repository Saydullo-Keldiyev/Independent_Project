'use client';

import { formatPrice, formatDate } from '@/lib/utils';

interface Bid {
  id: string;
  user_id: string;
  amount: number;
  created_at: string;
}

interface Props {
  bids: Bid[];
  isLive?: boolean;
}

export function BidHistory({ bids, isLive }: Props) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-bold text-zinc-100">📊 Bid History</h3>
        {isLive && (
          <span className="flex items-center gap-1.5 text-xs text-emerald-400">
            <span className="w-1.5 h-1.5 bg-emerald-400 rounded-full animate-pulse" />
            LIVE
          </span>
        )}
      </div>

      {bids.length === 0 ? (
        <div className="text-center py-8 text-zinc-600">
          <p className="text-lg">No bids yet</p>
          <p className="text-sm mt-1">Be the first to bid!</p>
        </div>
      ) : (
        <div className="space-y-2 max-h-96 overflow-y-auto pr-1">
          {bids.map((bid, i) => (
            <div
              key={bid.id || i}
              className={`flex items-center justify-between p-3 rounded-lg transition-all ${
                i === 0
                  ? 'bg-emerald-500/10 border border-emerald-500/20 animate-slide-up'
                  : 'bg-zinc-800/50 border border-zinc-800'
              }`}
            >
              <div className="flex items-center gap-3">
                <span className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold ${
                  i === 0 ? 'bg-emerald-500 text-white' : 'bg-zinc-700 text-zinc-400'
                }`}>
                  {i === 0 ? '👑' : i + 1}
                </span>
                <div>
                  <p className="text-sm text-zinc-300 font-medium">
                    {bid.user_id.slice(0, 8)}...
                  </p>
                  <p className="text-[11px] text-zinc-600">{formatDate(bid.created_at)}</p>
                </div>
              </div>
              <p className={`font-bold font-mono ${i === 0 ? 'text-emerald-400 text-lg' : 'text-zinc-300'}`}>
                {formatPrice(bid.amount)}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
