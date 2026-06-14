'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { auctionService } from '@/services/auction.service';
import { AuctionCard } from '@/components/auction/AuctionCard';
import { Navbar } from '@/components/layout/Navbar';

export default function AuctionsPage() {
  const [state, setState] = useState('');  // empty = all states
  const [page, setPage] = useState(1);

  const { data, isLoading } = useQuery({
    queryKey: ['auctions', state, page],
    queryFn: () => auctionService.list({ state: state || undefined, page, page_size: 12 }),
  });

  const auctions = data?.data?.auctions || data?.data || [];
  const total = data?.data?.total || 0;

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />

      <div className="max-w-7xl mx-auto px-4 py-8">
        {/* Title + Filters */}
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 mb-8">
          <div>
            <h1 className="text-3xl font-bold text-zinc-100">Auctions</h1>
            <p className="text-zinc-500 mt-1">{total} auction{total !== 1 ? 's' : ''} found</p>
          </div>

          {/* State filter */}
          <div className="flex gap-2 bg-zinc-900 border border-zinc-800 rounded-lg p-1">
            {[
              { value: '', label: 'All' },
              { value: 'active', label: 'Active' },
              { value: 'scheduled', label: 'Scheduled' },
              { value: 'ended', label: 'Ended' },
            ].map(s => (
              <button
                key={s.value}
                onClick={() => { setState(s.value); setPage(1); }}
                className={`px-4 py-2 rounded-md text-sm font-medium transition-all ${
                  state === s.value
                    ? 'bg-emerald-600 text-white shadow-lg shadow-emerald-500/20'
                    : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                {s.label}
              </button>
            ))}
          </div>
        </div>

        {/* Grid */}
        {isLoading ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[...Array(6)].map((_, i) => (
              <div key={i} className="bg-zinc-900 border border-zinc-800 rounded-xl p-4 animate-pulse">
                <div className="h-44 bg-zinc-800 rounded-lg mb-4" />
                <div className="h-4 bg-zinc-800 rounded w-3/4 mb-2" />
                <div className="h-4 bg-zinc-800 rounded w-1/2" />
              </div>
            ))}
          </div>
        ) : auctions.length === 0 ? (
          <div className="text-center py-20">
            <div className="text-5xl mb-4">🏷️</div>
            <p className="text-xl text-zinc-400">No {state || ''} auctions found</p>
            <p className="text-zinc-600 mt-2">Check back later or try a different filter</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {auctions.map((auction: any) => (
              <AuctionCard key={auction.id} auction={auction} />
            ))}
          </div>
        )}

        {/* Pagination */}
        {total > 12 && (
          <div className="flex justify-center items-center gap-3 mt-10">
            <button
              onClick={() => setPage(p => Math.max(1, p - 1))}
              disabled={page === 1}
              className="px-4 py-2 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 disabled:opacity-30"
            >
              ← Previous
            </button>
            <span className="text-zinc-500 text-sm">Page {page}</span>
            <button
              onClick={() => setPage(p => p + 1)}
              disabled={auctions.length < 12}
              className="px-4 py-2 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 disabled:opacity-30"
            >
              Next →
            </button>
          </div>
        )}
      </div>
    </div>
  );
}
