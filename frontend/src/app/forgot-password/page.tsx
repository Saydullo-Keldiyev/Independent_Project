'use client';

import { useState } from 'react';
import Link from 'next/link';
import { useToast } from '@/components/ui/Toast';
import api from '@/lib/api';

export default function ForgotPasswordPage() {
  const [email, setEmail] = useState('');
  const [loading, setLoading] = useState(false);
  const [sent, setSent] = useState(false);
  const toast = useToast();

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    try {
      await api.post('/api/v1/auth/forgot-password', { email });
      setSent(true);
      toast.add('Reset link sent to your email', 'success');
    } catch {
      toast.add('If the email exists, a reset link has been sent', 'info');
      setSent(true);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center px-4">
      <div className="w-full max-w-md">
        <div className="text-center mb-8">
          <Link href="/" className="text-3xl font-bold text-emerald-400">🔨 AuctionHub</Link>
          <p className="mt-2 text-zinc-500">Reset your password</p>
        </div>

        {sent ? (
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-8 text-center">
            <div className="text-4xl mb-4">📧</div>
            <h2 className="text-xl font-bold text-zinc-100 mb-2">Check your email</h2>
            <p className="text-zinc-400 text-sm mb-6">If an account exists with that email, we sent a password reset link.</p>
            <Link href="/login" className="text-emerald-400 hover:text-emerald-300 text-sm font-medium">← Back to login</Link>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="bg-zinc-900 border border-zinc-800 rounded-xl p-8 space-y-5">
            <div>
              <label className="block text-sm text-zinc-400 mb-1.5">Email address</label>
              <input type="email" value={email} onChange={e => setEmail(e.target.value)}
                className="w-full px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none"
                placeholder="you@example.com" required />
            </div>
            <button type="submit" disabled={loading}
              className="w-full py-2.5 bg-emerald-600 text-white rounded-lg font-medium hover:bg-emerald-500 disabled:opacity-50">
              {loading ? 'Sending...' : 'Send Reset Link'}
            </button>
            <p className="text-center text-sm text-zinc-500">
              <Link href="/login" className="text-emerald-400 hover:text-emerald-300">← Back to login</Link>
            </p>
          </form>
        )}
      </div>
    </div>
  );
}
