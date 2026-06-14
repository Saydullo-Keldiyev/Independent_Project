'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import { Navbar } from '@/components/layout/Navbar';
import { auctionService } from '@/services/auction.service';

export default function CreateAuctionPage() {
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [form, setForm] = useState({
    title: '', description: '', starting_price: '', reserve_price: '',
    start_time: '', end_time: '', category_id: '',
  });

  const update = (field: string, value: string) => setForm(f => ({ ...f, [field]: value }));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      const data = {
        title: form.title,
        description: form.description,
        starting_price: parseFloat(form.starting_price),
        reserve_price: form.reserve_price ? parseFloat(form.reserve_price) : 0,
        start_time: new Date(form.start_time).toISOString(),
        end_time: new Date(form.end_time).toISOString(),
      };
      const result = await auctionService.create(data);
      const auctionId = result.data?.id || result.id;
      router.push(auctionId ? `/auctions/${auctionId}` : '/auctions');
    } catch (err: any) {
      setError(err.response?.data?.error || 'Failed to create auction');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />
      <div className="max-w-3xl mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Create Auction</h1>
        <p className="text-zinc-500 mb-8">List your item for bidding</p>

        <form onSubmit={handleSubmit} className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 space-y-5">
          {error && <div className="p-3 bg-red-500/10 border border-red-500/30 rounded-lg text-red-400 text-sm">{error}</div>}

          <div>
            <label className="block text-sm text-zinc-400 mb-1.5">Title *</label>
            <input type="text" value={form.title} onChange={e => update('title', e.target.value)}
              className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 focus:border-emerald-500 outline-none"
              placeholder="e.g. iPhone 16 Pro Max 256GB" required />
          </div>

          <div>
            <label className="block text-sm text-zinc-400 mb-1.5">Description *</label>
            <textarea value={form.description} onChange={e => update('description', e.target.value)}
              className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none min-h-[120px] resize-y"
              placeholder="Describe your item in detail..." required />
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-zinc-400 mb-1.5">Starting Price ($) *</label>
              <input type="number" step="0.01" min="0.01" value={form.starting_price} onChange={e => update('starting_price', e.target.value)}
                className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none"
                placeholder="100.00" required />
            </div>
            <div>
              <label className="block text-sm text-zinc-400 mb-1.5">Reserve Price ($)</label>
              <input type="number" step="0.01" min="0" value={form.reserve_price} onChange={e => update('reserve_price', e.target.value)}
                className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none"
                placeholder="Optional minimum" />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm text-zinc-400 mb-1.5">Start Time *</label>
              <input type="datetime-local" value={form.start_time} onChange={e => update('start_time', e.target.value)}
                className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
            </div>
            <div>
              <label className="block text-sm text-zinc-400 mb-1.5">End Time *</label>
              <input type="datetime-local" value={form.end_time} onChange={e => update('end_time', e.target.value)}
                className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
            </div>
          </div>

          <button type="submit" disabled={loading}
            className="w-full py-3 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg font-bold text-lg transition-all disabled:opacity-50 active:scale-[0.98]">
            {loading ? 'Creating...' : '🏷️ Create Auction'}
          </button>
        </form>
      </div>
    </div>
  );
}
