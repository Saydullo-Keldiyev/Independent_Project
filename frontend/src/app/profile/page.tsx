'use client';

import { useState } from 'react';
import { Navbar } from '@/components/layout/Navbar';
import { AuthGuard } from '@/components/layout/AuthGuard';
import { useAuthStore } from '@/store/auth-store';
import { useToast } from '@/components/ui/Toast';
import api from '@/lib/api';

export default function ProfilePage() {
  const { user, loadUser } = useAuthStore();
  const toast = useToast();
  const [editing, setEditing] = useState(false);
  const [form, setForm] = useState({ first_name: user?.first_name || '', last_name: user?.last_name || '' });
  const [saving, setSaving] = useState(false);

  const handleSave = async () => {
    setSaving(true);
    try {
      await api.put('/api/v1/users/me', form);
      toast.add('Profile updated!', 'success');
      setEditing(false);
      loadUser();
    } catch {
      toast.add('Failed to update profile', 'error');
    } finally {
      setSaving(false);
    }
  };

  return (
    <AuthGuard>
      <div className="min-h-screen bg-zinc-950">
        <Navbar />
        <div className="max-w-2xl mx-auto px-4 py-8">
          <h1 className="text-3xl font-bold text-zinc-100 mb-8">👤 Profile</h1>

          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 space-y-6">
            {/* Avatar */}
            <div className="flex items-center gap-4">
              <div className="w-16 h-16 bg-emerald-600 rounded-full flex items-center justify-center text-2xl font-bold text-white">
                {user?.first_name?.[0] || 'U'}
              </div>
              <div>
                <p className="text-lg font-semibold text-zinc-100">{user?.first_name} {user?.last_name}</p>
                <p className="text-sm text-zinc-500">{user?.email}</p>
                <span className="inline-block mt-1 px-2 py-0.5 bg-emerald-500/20 text-emerald-400 text-xs rounded-full border border-emerald-500/30">
                  {user?.role}
                </span>
              </div>
            </div>

            <hr className="border-zinc-800" />

            {/* Info */}
            {editing ? (
              <div className="space-y-4">
                <div className="grid grid-cols-2 gap-4">
                  <div>
                    <label className="block text-xs text-zinc-500 mb-1">First Name</label>
                    <input value={form.first_name} onChange={e => setForm(f => ({ ...f, first_name: e.target.value }))}
                      className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" />
                  </div>
                  <div>
                    <label className="block text-xs text-zinc-500 mb-1">Last Name</label>
                    <input value={form.last_name} onChange={e => setForm(f => ({ ...f, last_name: e.target.value }))}
                      className="w-full px-3 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none" />
                  </div>
                </div>
                <div className="flex gap-2">
                  <button onClick={handleSave} disabled={saving}
                    className="px-4 py-2 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-500 disabled:opacity-50">
                    {saving ? 'Saving...' : 'Save'}
                  </button>
                  <button onClick={() => setEditing(false)} className="px-4 py-2 bg-zinc-800 border border-zinc-700 text-zinc-400 rounded-lg text-sm">
                    Cancel
                  </button>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex justify-between items-center">
                  <div>
                    <p className="text-xs text-zinc-500">Username</p>
                    <p className="text-zinc-200">{user?.username}</p>
                  </div>
                  <button onClick={() => setEditing(true)} className="px-3 py-1.5 bg-zinc-800 border border-zinc-700 rounded-lg text-xs text-zinc-400 hover:text-zinc-200">
                    Edit
                  </button>
                </div>
                <div>
                  <p className="text-xs text-zinc-500">Email</p>
                  <p className="text-zinc-200">{user?.email}</p>
                </div>
                <div>
                  <p className="text-xs text-zinc-500">Member since</p>
                  <p className="text-zinc-200">2026</p>
                </div>
              </div>
            )}

            <hr className="border-zinc-800" />

            {/* Security */}
            <div>
              <h3 className="text-sm font-semibold text-zinc-300 mb-3">Security</h3>
              <div className="space-y-2">
                <button className="w-full text-left px-4 py-3 bg-zinc-800/50 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-700">
                  🔑 Change Password
                </button>
                <button className="w-full text-left px-4 py-3 bg-zinc-800/50 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 hover:border-zinc-700">
                  📱 Active Sessions
                </button>
                <button className="w-full text-left px-4 py-3 bg-red-500/5 border border-red-500/20 rounded-lg text-sm text-red-400 hover:bg-red-500/10">
                  🗑️ Delete Account
                </button>
              </div>
            </div>
          </div>
        </div>
      </div>
    </AuthGuard>
  );
}
