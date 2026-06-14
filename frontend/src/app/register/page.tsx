'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth-store';
import { useToast } from '@/components/ui/Toast';

export default function RegisterPage() {
  const [form, setForm] = useState({ username: '', email: '', password: '', first_name: '', last_name: '', role: 'bidder' });
  const [loading, setLoading] = useState(false);
  const register = useAuthStore(s => s.register);
  const toast = useToast();
  const router = useRouter();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await register(form);
      toast.add('Account created! Welcome!', 'success');
      router.push('/dashboard');
    } catch (err: any) {
      toast.add(err.response?.data?.error || 'Registration failed', 'error');
    } finally {
      setLoading(false);
    }
  };

  const update = (field: string, value: string) => setForm(f => ({ ...f, [field]: value }));

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center px-4 py-12">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <Link href="/" className="text-3xl font-bold text-emerald-400">🔨 AuctionHub</Link>
          <p className="mt-2 text-zinc-500">Create your account</p>
        </div>

        <form onSubmit={handleSubmit} className="bg-zinc-900 border border-zinc-800 rounded-xl p-8 space-y-4">
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-xs text-zinc-400 mb-1">First Name</label>
              <input type="text" value={form.first_name} onChange={e => update('first_name', e.target.value)}
                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
            </div>
            <div>
              <label className="block text-xs text-zinc-400 mb-1">Last Name</label>
              <input type="text" value={form.last_name} onChange={e => update('last_name', e.target.value)}
                className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
            </div>
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Username</label>
            <input type="text" value={form.username} onChange={e => update('username', e.target.value)}
              className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Email</label>
            <input type="email" value={form.email} onChange={e => update('email', e.target.value)}
              className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" required />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">Password (min 8 chars)</label>
            <input type="password" value={form.password} onChange={e => update('password', e.target.value)}
              className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" minLength={8} required />
          </div>

          <div>
            <label className="block text-xs text-zinc-400 mb-1">I want to</label>
            <div className="grid grid-cols-2 gap-2">
              <button type="button" onClick={() => update('role', 'bidder')}
                className={`py-2.5 rounded-lg text-sm font-medium border transition-all ${
                  form.role === 'bidder' ? 'bg-emerald-600 border-emerald-500 text-white' : 'bg-zinc-800 border-zinc-700 text-zinc-400'
                }`}>
                🔨 Buy (Bidder)
              </button>
              <button type="button" onClick={() => update('role', 'seller')}
                className={`py-2.5 rounded-lg text-sm font-medium border transition-all ${
                  form.role === 'seller' ? 'bg-emerald-600 border-emerald-500 text-white' : 'bg-zinc-800 border-zinc-700 text-zinc-400'
                }`}>
                🏷️ Sell (Seller)
              </button>
            </div>
          </div>

          <button type="submit" disabled={loading}
            className="w-full py-2.5 bg-emerald-600 text-white rounded-lg font-medium hover:bg-emerald-500 disabled:opacity-50 transition-colors">
            {loading ? 'Creating...' : 'Create Account'}
          </button>

          <p className="text-center text-sm text-zinc-500">
            Already have an account? <Link href="/login" className="text-emerald-400 hover:text-emerald-300 font-medium">Sign in</Link>
          </p>
        </form>
      </div>
    </div>
  );
}
