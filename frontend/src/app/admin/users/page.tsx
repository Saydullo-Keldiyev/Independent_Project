'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import Link from 'next/link';
import { Navbar } from '@/components/layout/Navbar';
import { AuthGuard } from '@/components/layout/AuthGuard';
import { adminService, AdminUser } from '@/services/admin.service';
import { useToast } from '@/components/ui/Toast';
import { formatDate } from '@/lib/utils';

export default function ManageUsersPage() {
  const queryClient = useQueryClient();
  const toast = useToast();
  const [page, setPage] = useState(1);
  const [roleFilter, setRoleFilter] = useState('');
  const [search, setSearch] = useState('');
  const [searchInput, setSearchInput] = useState('');
  const [editingUser, setEditingUser] = useState<AdminUser | null>(null);
  const [confirmAction, setConfirmAction] = useState<{ type: string; user: AdminUser } | null>(null);

  const { data, isLoading } = useQuery({
    queryKey: ['admin-users', page, roleFilter, search],
    queryFn: () => adminService.listUsers({ page, limit: 20, role: roleFilter || undefined, search: search || undefined }),
  });

  const users: AdminUser[] = data?.data?.users || data?.users || [];
  const total = data?.data?.total || data?.total || 0;
  const totalPages = Math.ceil(total / 20);

  const updateRoleMutation = useMutation({
    mutationFn: ({ userId, role }: { userId: string; role: string }) => adminService.updateRole(userId, role),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      toast.add('Role updated successfully', 'success');
      setEditingUser(null);
    },
    onError: () => toast.add('Failed to update role', 'error'),
  });

  const banMutation = useMutation({
    mutationFn: (userId: string) => adminService.banUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      toast.add('User banned', 'success');
      setConfirmAction(null);
    },
    onError: () => toast.add('Failed to ban user', 'error'),
  });

  const unbanMutation = useMutation({
    mutationFn: (userId: string) => adminService.unbanUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      toast.add('User unbanned', 'success');
    },
    onError: () => toast.add('Failed to unban user', 'error'),
  });

  const deleteMutation = useMutation({
    mutationFn: (userId: string) => adminService.deleteUser(userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['admin-users'] });
      toast.add('User deleted', 'success');
      setConfirmAction(null);
    },
    onError: () => toast.add('Failed to delete user', 'error'),
  });

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    setSearch(searchInput);
    setPage(1);
  };

  return (
    <AuthGuard requiredRole={['admin']}>
      <div className="min-h-screen bg-zinc-950">
        <Navbar />
        <div className="max-w-7xl mx-auto px-4 py-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-6">
            <div>
              <div className="flex items-center gap-3">
                <Link href="/admin" className="text-zinc-500 hover:text-zinc-300 text-sm">← Admin</Link>
                <span className="text-zinc-700">/</span>
                <h1 className="text-2xl font-bold text-zinc-100">👥 Manage Users</h1>
              </div>
              <p className="text-zinc-500 mt-1 text-sm">{total} total users</p>
            </div>
          </div>

          {/* Filters */}
          <div className="flex flex-col md:flex-row gap-3 mb-6">
            <form onSubmit={handleSearch} className="flex-1">
              <div className="relative">
                <input
                  type="text"
                  value={searchInput}
                  onChange={e => setSearchInput(e.target.value)}
                  placeholder="Search by username, email, or name..."
                  className="w-full px-4 py-2.5 pl-10 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-200 placeholder-zinc-600 focus:ring-1 focus:ring-emerald-500 outline-none"
                />
                <span className="absolute left-3 top-3 text-zinc-600 text-sm">🔍</span>
              </div>
            </form>
            <div className="flex gap-2">
              {['', 'admin', 'seller', 'bidder'].map(r => (
                <button
                  key={r}
                  onClick={() => { setRoleFilter(r); setPage(1); }}
                  className={`px-3 py-2 rounded-lg text-xs font-medium transition-all ${
                    roleFilter === r
                      ? 'bg-emerald-600 text-white'
                      : 'bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-zinc-200'
                  }`}
                >
                  {r === '' ? 'All' : r.charAt(0).toUpperCase() + r.slice(1)}
                </button>
              ))}
            </div>
          </div>

          {/* Users table */}
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl overflow-hidden">
            {isLoading ? (
              <div className="p-8 text-center">
                <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin mx-auto" />
              </div>
            ) : users.length === 0 ? (
              <div className="p-8 text-center text-zinc-500">No users found</div>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b border-zinc-800">
                      <th className="text-left text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">User</th>
                      <th className="text-left text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">Email</th>
                      <th className="text-left text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">Role</th>
                      <th className="text-left text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">Status</th>
                      <th className="text-left text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">Joined</th>
                      <th className="text-right text-xs text-zinc-500 uppercase tracking-wide px-4 py-3">Actions</th>
                    </tr>
                  </thead>
                  <tbody>
                    {users.map(user => (
                      <tr key={user.id} className="border-b border-zinc-800/50 hover:bg-zinc-800/30 transition-colors">
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-3">
                            <div className={`w-8 h-8 rounded-full flex items-center justify-center text-xs font-bold ${
                              user.role === 'admin' ? 'bg-red-500/20 text-red-400' :
                              user.role === 'seller' ? 'bg-purple-500/20 text-purple-400' :
                              'bg-blue-500/20 text-blue-400'
                            }`}>
                              {user.first_name?.[0] || user.username?.[0] || '?'}
                            </div>
                            <div>
                              <p className="text-sm font-medium text-zinc-200">{user.first_name} {user.last_name}</p>
                              <p className="text-xs text-zinc-500">@{user.username}</p>
                            </div>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-sm text-zinc-400">{user.email}</td>
                        <td className="px-4 py-3">
                          <span className={`px-2 py-0.5 rounded text-xs font-medium ${
                            user.role === 'admin' ? 'bg-red-500/20 text-red-400' :
                            user.role === 'seller' ? 'bg-purple-500/20 text-purple-400' :
                            'bg-blue-500/20 text-blue-400'
                          }`}>
                            {user.role}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          {user.is_active ? (
                            <span className="flex items-center gap-1.5 text-xs text-emerald-400">
                              <span className="w-1.5 h-1.5 bg-emerald-400 rounded-full" /> Active
                            </span>
                          ) : (
                            <span className="flex items-center gap-1.5 text-xs text-red-400">
                              <span className="w-1.5 h-1.5 bg-red-400 rounded-full" /> Banned
                            </span>
                          )}
                        </td>
                        <td className="px-4 py-3 text-xs text-zinc-500">{formatDate(user.created_at)}</td>
                        <td className="px-4 py-3">
                          <div className="flex items-center justify-end gap-1">
                            <button
                              onClick={() => setEditingUser(user)}
                              className="px-2 py-1 text-xs bg-zinc-800 border border-zinc-700 rounded text-zinc-400 hover:text-zinc-200 hover:border-zinc-600"
                            >
                              Role
                            </button>
                            {user.is_active ? (
                              <button
                                onClick={() => setConfirmAction({ type: 'ban', user })}
                                className="px-2 py-1 text-xs bg-orange-500/10 border border-orange-500/30 rounded text-orange-400 hover:bg-orange-500/20"
                              >
                                Ban
                              </button>
                            ) : (
                              <button
                                onClick={() => unbanMutation.mutate(user.id)}
                                className="px-2 py-1 text-xs bg-emerald-500/10 border border-emerald-500/30 rounded text-emerald-400 hover:bg-emerald-500/20"
                              >
                                Unban
                              </button>
                            )}
                            <button
                              onClick={() => setConfirmAction({ type: 'delete', user })}
                              className="px-2 py-1 text-xs bg-red-500/10 border border-red-500/30 rounded text-red-400 hover:bg-red-500/20"
                            >
                              Delete
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex justify-center items-center gap-3 mt-6">
              <button onClick={() => setPage(p => Math.max(1, p - 1))} disabled={page === 1}
                className="px-4 py-2 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 disabled:opacity-30">
                ← Previous
              </button>
              <span className="text-zinc-500 text-sm">Page {page} of {totalPages}</span>
              <button onClick={() => setPage(p => p + 1)} disabled={page >= totalPages}
                className="px-4 py-2 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 disabled:opacity-30">
                Next →
              </button>
            </div>
          )}
        </div>

        {/* Edit Role Modal */}
        {editingUser && (
          <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setEditingUser(null)} />
            <div className="relative bg-zinc-900 border border-zinc-800 rounded-xl p-6 max-w-sm w-full mx-4 animate-scale-in">
              <h3 className="text-lg font-bold text-zinc-100 mb-2">Change Role</h3>
              <p className="text-sm text-zinc-400 mb-4">
                User: <span className="text-zinc-200">{editingUser.username}</span>
              </p>
              <div className="space-y-2">
                {['bidder', 'seller', 'admin'].map(role => (
                  <button
                    key={role}
                    onClick={() => updateRoleMutation.mutate({ userId: editingUser.id, role })}
                    disabled={editingUser.role === role}
                    className={`w-full py-2.5 rounded-lg text-sm font-medium transition-all ${
                      editingUser.role === role
                        ? 'bg-emerald-600/20 text-emerald-400 border border-emerald-500/30 cursor-default'
                        : 'bg-zinc-800 border border-zinc-700 text-zinc-300 hover:border-emerald-500/50 hover:text-emerald-400'
                    }`}
                  >
                    {role.charAt(0).toUpperCase() + role.slice(1)}
                    {editingUser.role === role && ' (current)'}
                  </button>
                ))}
              </div>
              <button onClick={() => setEditingUser(null)} className="w-full mt-4 py-2 text-sm text-zinc-500 hover:text-zinc-300">
                Cancel
              </button>
            </div>
          </div>
        )}

        {/* Confirm Action Modal */}
        {confirmAction && (
          <div className="fixed inset-0 z-50 flex items-center justify-center">
            <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setConfirmAction(null)} />
            <div className="relative bg-zinc-900 border border-zinc-800 rounded-xl p-6 max-w-sm w-full mx-4 animate-scale-in">
              <h3 className="text-lg font-bold text-zinc-100 mb-2">
                {confirmAction.type === 'ban' ? '⚠️ Ban User' : '🗑️ Delete User'}
              </h3>
              <p className="text-sm text-zinc-400 mb-4">
                {confirmAction.type === 'ban'
                  ? `Are you sure you want to ban ${confirmAction.user.username}? They won't be able to log in.`
                  : `Are you sure you want to permanently delete ${confirmAction.user.username}? This cannot be undone.`
                }
              </p>
              <div className="flex gap-3">
                <button onClick={() => setConfirmAction(null)}
                  className="flex-1 py-2.5 bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg font-medium">
                  Cancel
                </button>
                <button
                  onClick={() => {
                    if (confirmAction.type === 'ban') banMutation.mutate(confirmAction.user.id);
                    else deleteMutation.mutate(confirmAction.user.id);
                  }}
                  className={`flex-1 py-2.5 rounded-lg font-medium text-white ${
                    confirmAction.type === 'ban' ? 'bg-orange-600 hover:bg-orange-500' : 'bg-red-600 hover:bg-red-500'
                  }`}
                >
                  {confirmAction.type === 'ban' ? 'Ban User' : 'Delete'}
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </AuthGuard>
  );
}
