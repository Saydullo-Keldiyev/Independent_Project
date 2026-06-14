'use client';

import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Navbar } from '@/components/layout/Navbar';
import api from '@/lib/api';
import { formatDate } from '@/lib/utils';

export default function NotificationsPage() {
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ['notifications'],
    queryFn: () => api.get('/api/v1/notifications').then(r => r.data),
  });

  const markAllRead = useMutation({
    mutationFn: () => api.post('/api/v1/notifications/read-all'),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['notifications'] }),
  });

  const notifications = data?.notifications || data?.data?.notifications || [];
  const unreadCount = data?.unread_count || data?.data?.unread_count || 0;

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />
      <div className="max-w-3xl mx-auto px-4 py-8">
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-3xl font-bold text-zinc-100">🔔 Notifications</h1>
            {unreadCount > 0 && (
              <p className="text-emerald-400 text-sm mt-1">{unreadCount} unread</p>
            )}
          </div>
          {unreadCount > 0 && (
            <button onClick={() => markAllRead.mutate()}
              className="px-4 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-sm text-zinc-400 hover:text-zinc-200">
              Mark all read
            </button>
          )}
        </div>

        {isLoading ? (
          <div className="space-y-3">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="bg-zinc-900 border border-zinc-800 rounded-xl p-4 animate-pulse">
                <div className="h-4 bg-zinc-800 rounded w-3/4 mb-2" />
                <div className="h-3 bg-zinc-800 rounded w-1/2" />
              </div>
            ))}
          </div>
        ) : notifications.length === 0 ? (
          <div className="text-center py-16 bg-zinc-900/50 border border-zinc-800 rounded-xl">
            <div className="text-5xl mb-4">🔕</div>
            <p className="text-zinc-400 text-lg">No notifications yet</p>
            <p className="text-zinc-600 text-sm mt-1">You&apos;ll see bid updates and auction alerts here</p>
          </div>
        ) : (
          <div className="space-y-2">
            {notifications.map((notif: any, i: number) => (
              <div key={notif.id || i} className={`bg-zinc-900 border rounded-xl p-4 transition-colors ${
                notif.is_read ? 'border-zinc-800' : 'border-emerald-500/30 bg-emerald-500/5'
              }`}>
                <div className="flex items-start justify-between">
                  <div>
                    <p className="font-medium text-zinc-200">{notif.title}</p>
                    <p className="text-sm text-zinc-500 mt-1">{notif.message}</p>
                  </div>
                  {!notif.is_read && (
                    <span className="w-2 h-2 bg-emerald-400 rounded-full mt-2 flex-shrink-0" />
                  )}
                </div>
                <p className="text-xs text-zinc-600 mt-2">{formatDate(notif.created_at)}</p>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
