'use client';

import { useEffect } from 'react';
import { useRouter } from 'next/navigation';
import { useAuthStore } from '@/store/auth-store';

interface Props {
  children: React.ReactNode;
  requiredRole?: string[];
  fallback?: React.ReactNode;
}

export function AuthGuard({ children, requiredRole, fallback }: Props) {
  const { isAuthenticated, isLoading, user, loadUser } = useAuthStore();
  const router = useRouter();

  useEffect(() => { loadUser(); }, [loadUser]);

  useEffect(() => {
    if (!isLoading && !isAuthenticated) {
      router.push('/login');
    }
  }, [isLoading, isAuthenticated, router]);

  if (isLoading) {
    return fallback || (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  if (!isAuthenticated) return null;

  if (requiredRole && user && !requiredRole.includes(user.role)) {
    return (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
        <div className="text-center">
          <div className="text-5xl mb-4">🚫</div>
          <h1 className="text-2xl font-bold text-zinc-100">Access Denied</h1>
          <p className="text-zinc-500 mt-2">You don&apos;t have permission to view this page</p>
          <button onClick={() => router.back()} className="mt-4 px-4 py-2 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-300 hover:text-zinc-100">
            Go Back
          </button>
        </div>
      </div>
    );
  }

  return <>{children}</>;
}
