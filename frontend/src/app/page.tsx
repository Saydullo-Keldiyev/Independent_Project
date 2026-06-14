'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { auctionService } from '@/services/auction.service';
import { AuctionCard } from '@/components/auction/AuctionCard';
import { Navbar } from '@/components/layout/Navbar';

export default function HomePage() {
  const { data, isLoading } = useQuery({
    queryKey: ['home-auctions'],
    queryFn: () => auctionService.list({ page_size: 6 }),
  });

  const auctions = data?.data?.auctions || data?.data || [];

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />

      {/* Hero */}
      <section className="relative overflow-hidden">
        <div className="absolute inset-0 bg-gradient-to-b from-emerald-500/5 to-transparent" />
        <div className="max-w-7xl mx-auto px-4 py-24 text-center relative">
          <div className="inline-flex items-center gap-2 px-3 py-1 bg-emerald-500/10 border border-emerald-500/20 rounded-full text-emerald-400 text-xs font-medium mb-6">
            <span className="w-1.5 h-1.5 bg-emerald-400 rounded-full animate-pulse" />
            Live Auctions Running Now
          </div>
          <h1 className="text-5xl md:text-7xl font-bold text-zinc-100 mb-4 tracking-tight">
            Bid Live.<br />
            <span className="text-emerald-400">Win Instantly.</span>
          </h1>
          <p className="text-xl text-zinc-400 max-w-2xl mx-auto mb-8">
            WebSocket-powered real-time auction platform. Place bids, track prices, and win — all in milliseconds.
          </p>
          <div className="flex gap-4 justify-center">
            <Link href="/auctions" className="px-8 py-3 bg-emerald-600 text-white rounded-lg font-semibold hover:bg-emerald-500 transition-all hover:shadow-lg hover:shadow-emerald-500/20">
              Browse Auctions
            </Link>
            <Link href="/register" className="px-8 py-3 border border-zinc-700 text-zinc-300 rounded-lg font-semibold hover:border-zinc-500 transition-colors">
              Start Selling
            </Link>
          </div>
        </div>
      </section>

      {/* Active Auctions */}
      <section className="max-w-7xl mx-auto px-4 py-16">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h2 className="text-2xl font-bold text-zinc-100">🔥 Active Auctions</h2>
            <p className="text-zinc-500 text-sm mt-1">Real-time bidding happening now</p>
          </div>
          <Link href="/auctions" className="text-emerald-400 hover:text-emerald-300 text-sm font-medium">
            View all →
          </Link>
        </div>

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
          <div className="text-center py-16 bg-zinc-900/50 border border-zinc-800 rounded-xl">
            <div className="text-5xl mb-4">🏷️</div>
            <p className="text-zinc-400 text-lg">No active auctions yet</p>
            <p className="text-zinc-600 mt-2">Create one or check back later!</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {auctions.map((auction: any) => (
              <AuctionCard key={auction.id} auction={auction} />
            ))}
          </div>
        )}
      </section>

      {/* Features */}
      <section className="border-t border-zinc-800 py-20">
        <div className="max-w-7xl mx-auto px-4">
          <h2 className="text-2xl font-bold text-zinc-100 text-center mb-12">Why AuctionHub?</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
            {[
              { icon: '⚡', title: 'Real-Time Bidding', desc: 'WebSocket-powered. See bids as they happen — zero delay.' },
              { icon: '🔒', title: 'Secure Payments', desc: 'ACID transactions, hold/release, automatic settlements.' },
              { icon: '📊', title: 'Live Analytics', desc: 'Track auctions, revenue, and bidding patterns in real-time.' },
            ].map(f => (
              <div key={f.title} className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 hover:border-zinc-700 transition-colors">
                <div className="text-3xl mb-3">{f.icon}</div>
                <h3 className="font-semibold text-zinc-100 mb-2">{f.title}</h3>
                <p className="text-zinc-500 text-sm leading-relaxed">{f.desc}</p>
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="border-t border-zinc-800 py-8">
        <div className="max-w-7xl mx-auto px-4 text-center text-zinc-600 text-sm">
          © 2026 AuctionHub — Production-grade microservices platform
        </div>
      </footer>
    </div>
  );
}
