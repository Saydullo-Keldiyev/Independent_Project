'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { Navbar } from '@/components/layout/Navbar';
import { AuthGuard } from '@/components/layout/AuthGuard';
import api from '@/lib/api';
import { formatPrice } from '@/lib/utils';

export default function AdminPage() {
  const { data: analytics } = useQuery({
    queryKey: ['admin-analytics'],
    queryFn: () => api.get('/api/v1/analytics/admin/dashboard').then(r => r.data),
  });

  const { data: trending } = useQuery({
    queryKey: ['admin-trending'],
    queryFn: () => api.get('/api/v1/analytics/trending').then(r => r.data),
  });

  const stats = analytics?.data || {};
  const trendingData = trending?.data || {};

  return (
    <AuthGuard requiredRole={['admin']}>
      <div className="min-h-screen bg-zinc-950">
        <Navbar />
        <div className="max-w-7xl mx-auto px-4 py-8">
          <div className="flex items-center justify-between mb-8">
            <div>
              <h1 className="text-3xl font-bold text-zinc-100">🛡️ Admin Panel</h1>
              <p className="text-zinc-500 mt-1">Platform management & analytics</p>
            </div>
            <span className="px-3 py-1 bg-red-500/20 text-red-400 border border-red-500/30 rounded-full text-xs font-semibold">
              ADMIN
            </span>
          </div>

          {/* KPI Cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
            <KPICard title="Total Revenue" value={formatPrice(stats.total_revenue || 0)} icon="💰" trend="+12%" />
            <KPICard title="Today Revenue" value={formatPrice(stats.today_revenue || 0)} icon="📈" trend="+5%" />
            <KPICard title="Today Bids" value={(stats.today_bids || 0).toString()} icon="🔨" trend="+23%" />
            <KPICard title="Active Users" value={(stats.concurrent_users || 0).toString()} icon="👥" trend="live" />
          </div>

          {/* Sections */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            {/* Top Sellers */}
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <h2 className="text-lg font-bold text-zinc-100 mb-4">🏆 Top Sellers</h2>
              {(trendingData.top_sellers || []).length === 0 ? (
                <p className="text-zinc-500 text-center py-4">No data yet</p>
              ) : (
                <div className="space-y-2">
                  {(trendingData.top_sellers || []).slice(0, 5).map((s: any, i: number) => (
                    <div key={i} className="flex items-center justify-between p-3 bg-zinc-800/50 rounded-lg">
                      <span className="text-sm text-zinc-300">#{i + 1} {s.Member?.slice(0, 12) || 'Seller'}...</span>
                      <span className="font-mono text-emerald-400">{formatPrice(s.Score || 0)}</span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Top Categories */}
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <h2 className="text-lg font-bold text-zinc-100 mb-4">📊 Top Categories</h2>
              {(trendingData.top_categories || []).length === 0 ? (
                <p className="text-zinc-500 text-center py-4">No data yet</p>
              ) : (
                <div className="space-y-2">
                  {(trendingData.top_categories || []).slice(0, 5).map((c: any, i: number) => (
                    <div key={i} className="flex items-center justify-between p-3 bg-zinc-800/50 rounded-lg">
                      <span className="text-sm text-zinc-300">{c.Member || 'Category'}</span>
                      <span className="text-zinc-400">{c.Score || 0} auctions</span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>

          {/* Admin actions */}
          <div className="mt-8 bg-zinc-900 border border-zinc-800 rounded-xl p-6">
            <h2 className="text-lg font-bold text-zinc-100 mb-4">⚡ Quick Actions</h2>
            <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
              <Link href="/admin/users">
                <ActionButton icon="👥" label="Manage Users" />
              </Link>
              <Link href="/auctions">
                <ActionButton icon="🏷️" label="All Auctions" />
              </Link>
              <ActionButton icon="💳" label="Payments" />
              <ActionButton icon="📊" label="Reports" />
            </div>
          </div>
        </div>
      </div>
    </AuthGuard>
  );
}

function KPICard({ title, value, icon, trend }: { title: string; value: string; icon: string; trend: string }) {
  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-5">
      <div className="flex items-center justify-between">
        <span className="text-2xl">{icon}</span>
        <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${
          trend === 'live' ? 'bg-emerald-500/20 text-emerald-400' : 'bg-blue-500/20 text-blue-400'
        }`}>{trend}</span>
      </div>
      <p className="text-2xl font-bold text-zinc-100 mt-3">{value}</p>
      <p className="text-xs text-zinc-500 mt-1">{title}</p>
    </div>
  );
}

function ActionButton({ icon, label }: { icon: string; label: string }) {
  return (
    <button className="flex flex-col items-center gap-2 p-4 bg-zinc-800/50 border border-zinc-800 rounded-lg hover:border-zinc-700 transition-colors">
      <span className="text-2xl">{icon}</span>
      <span className="text-xs text-zinc-400">{label}</span>
    </button>
  );
}
