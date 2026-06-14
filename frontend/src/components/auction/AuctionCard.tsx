'use client';

import Link from 'next/link';
import { Auction } from '@/services/auction.service';
import { formatPrice, timeLeft } from '@/lib/utils';
import { CountdownTimer } from './CountdownTimer';

interface Props {
  auction: Auction;
}

export function AuctionCard({ auction }: Props) {
  const isActive = auction.state === 'active';
  const isEnded = auction.state === 'ended';

  return (
    <Link href={`/auctions/${auction.id}`}>
      <div className="group bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden hover:border-emerald-500/50 transition-all duration-300 hover:shadow-lg hover:shadow-emerald-500/5">
        {/* Image */}
        <div className="h-44 bg-gradient-to-br from-zinc-800 to-zinc-900 flex items-center justify-center relative overflow-hidden">
          <span className="text-5xl group-hover:scale-110 transition-transform duration-300">🏷️</span>
          {/* Status badge */}
          <div className={`absolute top-3 right-3 px-2.5 py-1 rounded-full text-xs font-semibold ${
            isActive ? 'bg-emerald-500/20 text-emerald-400 border border-emerald-500/30' :
            isEnded ? 'bg-zinc-700/50 text-zinc-400' :
            'bg-orange-500/20 text-orange-400 border border-orange-500/30'
          }`}>
            {isActive && <span className="inline-block w-1.5 h-1.5 bg-emerald-400 rounded-full mr-1.5 animate-pulse" />}
            {auction.state.toUpperCase()}
          </div>
        </div>

        {/* Content */}
        <div className="p-4 space-y-3">
          <h3 className="font-semibold text-zinc-100 truncate group-hover:text-emerald-400 transition-colors">
            {auction.title}
          </h3>

          {/* Price + Time */}
          <div className="flex items-center justify-between">
            <div>
              <p className="text-xs text-zinc-500 uppercase tracking-wide">Current Bid</p>
              <p className="text-lg font-bold text-emerald-400">{formatPrice(auction.current_price)}</p>
            </div>
            <div className="text-right">
              <p className="text-xs text-zinc-500 uppercase tracking-wide">
                {isEnded ? 'Ended' : 'Time Left'}
              </p>
              {isActive ? (
                <CountdownTimer endTime={auction.end_time} compact />
              ) : (
                <p className="text-sm text-zinc-400">{timeLeft(auction.end_time)}</p>
              )}
            </div>
          </div>

          {/* Footer */}
          <div className="flex items-center justify-between pt-2 border-t border-zinc-800">
            <span className="text-xs text-zinc-500">
              🔥 {auction.total_bids} bid{auction.total_bids !== 1 ? 's' : ''}
            </span>
            <span className="text-xs text-zinc-500">
              Starting: {formatPrice(auction.starting_price)}
            </span>
          </div>
        </div>
      </div>
    </Link>
  );
}
