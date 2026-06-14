'use client';

import { useQuery } from '@tanstack/react-query';
import { Navbar } from '@/components/layout/Navbar';
import { AuthGuard } from '@/components/layout/AuthGuard';
import { useAuthStore } from '@/store/auth-store';
import api from '@/lib/api';
import { formatPrice } from '@/lib/utils';

export default function AnalyticsPage() {
  const { user } = useAuthStore();

  const { data } = useQuery({
    queryKey: ['seller-analytics', user?.id],
    queryFn: () => api.get(`/api/v1/analytics/seller/${user?.id}`).then(r => r.data),
    enabled: !!user?.id,
  });

  const stats = data?.data || {};

  return (
    <AuthGuard requiredRole={['seller', 'admin']}>
      <div className="min-h-screen bg-zinc-950">
        <Navbar />
        <div className="max-w-5xl mx-auto px-4 py-8">
          <h1 className="text-3xl font-bold text-zinc-100 mb-2">📊 Analytics</h1>
          <p className="text-zinc-500 mb-8">Your seller performance</p>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <p className="text-xs text-zinc-500 uppercase">Total Revenue</p>
              <p className="text-3xl font-bold text-emerald-400 mt-2">{formatPrice(stats.total_revenue || 0)}</p>
            </div>
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <p className="text-xs text-zinc-500 uppercase">Total Auctions</p>
              <p className="text-3xl font-bold text-zinc-100 mt-2">{stats.total_auctions || 0}</p>
            </div>
            <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
              <p className="text-xs text-zinc-500 uppercase">Success Rate</p>
              <p className="text-3xl font-bold text-zinc-100 mt-2">{stats.success_rate || 0}%</p>
            </div>
          </div>

          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
            <h2 className="text-lg font-bold text-zinc-100 mb-4">Performance Overview</h2>
            <div className="h-64 flex items-center justify-center text-zinc-600">
              <p>📈 Charts will appear when you have auction data</p>
            </div>
          </div>
        </div>
      </div>
    </AuthGuard>
  );
}
