'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useState, useEffect } from 'react';
import { useAuthStore } from '@/store/auth-store';

export function Navbar() {
  const { user, isAuthenticated, isLoading, loadUser, logout } = useAuthStore();
  const [mobileOpen, setMobileOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const router = useRouter();

  useEffect(() => { loadUser(); }, [loadUser]);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (searchQuery.trim()) {
      router.push(`/auctions?q=${encodeURIComponent(searchQuery.trim())}`);
      setSearchQuery('');
    }
  };

  const isSeller = user?.role === 'seller';
  const isAdmin = user?.role === 'admin';

  return (
    <header className="border-b border-zinc-800 bg-zinc-950/80 backdrop-blur-sm sticky top-0 z-50">
      <div className="max-w-7xl mx-auto px-4 py-3 flex items-center justify-between gap-4">
        {/* Logo */}
        <Link href="/" className="text-lg font-bold text-emerald-400 flex-shrink-0">🔨 AuctionHub</Link>

        {/* Search bar — desktop */}
        <form onSubmit={handleSearch} className="hidden md:flex flex-1 max-w-md">
          <div className="relative w-full">
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search auctions..."
              className="w-full px-4 py-2 pl-9 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-200 placeholder-zinc-600 focus:ring-1 focus:ring-emerald-500 focus:border-emerald-500 outline-none"
            />
            <span className="absolute left-3 top-2.5 text-zinc-600 text-sm">🔍</span>
          </div>
        </form>

        {/* Desktop nav */}
        <nav className="hidden md:flex items-center gap-4">
          <Link href="/auctions" className="text-sm text-zinc-400 hover:text-zinc-200 transition-colors">Auctions</Link>
          {isAuthenticated && (
            <>
              <Link href="/dashboard" className="text-sm text-zinc-400 hover:text-zinc-200">Dashboard</Link>
              {isSeller && <Link href="/auctions/create" className="text-sm text-emerald-400 hover:text-emerald-300">+ Create</Link>}
              {isAdmin && <Link href="/admin" className="text-sm text-red-400 hover:text-red-300">Admin</Link>}
            </>
          )}
        </nav>

        {/* Right side */}
        <div className="hidden md:flex items-center gap-3">
          {isLoading ? (
            <div className="w-20 h-8 bg-zinc-800 rounded animate-pulse" />
          ) : isAuthenticated ? (
            <>
              <Link href="/notifications" className="relative p-2 text-zinc-400 hover:text-zinc-200 transition-colors">
                🔔
              </Link>
              <Link href="/wallet" className="text-sm text-zinc-400 hover:text-zinc-200">💳</Link>
              <Link href="/profile" className="flex items-center gap-2 group">
                <div className="w-8 h-8 bg-emerald-600 rounded-full flex items-center justify-center text-xs font-bold text-white group-hover:ring-2 ring-emerald-400 transition-all">
                  {user?.first_name?.[0] || 'U'}
                </div>
              </Link>
              <button onClick={() => { logout(); router.push('/'); }} className="text-xs text-zinc-600 hover:text-red-400 transition-colors">
                Logout
              </button>
            </>
          ) : (
            <>
              <Link href="/login" className="text-sm text-zinc-400 hover:text-zinc-200">Login</Link>
              <Link href="/register" className="px-4 py-1.5 bg-emerald-600 text-white rounded-lg text-sm hover:bg-emerald-500 transition-colors">
                Register
              </Link>
            </>
          )}
        </div>

        {/* Mobile hamburger */}
        <button onClick={() => setMobileOpen(!mobileOpen)} className="md:hidden p-2 text-zinc-400">
          {mobileOpen ? '✕' : '☰'}
        </button>
      </div>

      {/* Mobile menu */}
      {mobileOpen && (
        <div className="md:hidden border-t border-zinc-800 bg-zinc-950 px-4 py-4 space-y-3 animate-slide-up">
          {/* Mobile search */}
          <form onSubmit={handleSearch}>
            <input
              type="text"
              value={searchQuery}
              onChange={e => setSearchQuery(e.target.value)}
              placeholder="Search auctions..."
              className="w-full px-4 py-2.5 bg-zinc-900 border border-zinc-800 rounded-lg text-sm text-zinc-200 placeholder-zinc-600 outline-none"
            />
          </form>

          <Link href="/auctions" onClick={() => setMobileOpen(false)} className="block py-2 text-zinc-300">Auctions</Link>

          {isAuthenticated ? (
            <>
              <Link href="/dashboard" onClick={() => setMobileOpen(false)} className="block py-2 text-zinc-300">Dashboard</Link>
              <Link href="/wallet" onClick={() => setMobileOpen(false)} className="block py-2 text-zinc-300">Wallet</Link>
              <Link href="/notifications" onClick={() => setMobileOpen(false)} className="block py-2 text-zinc-300">Notifications</Link>
              <Link href="/profile" onClick={() => setMobileOpen(false)} className="block py-2 text-zinc-300">Profile</Link>
              {isSeller && <Link href="/auctions/create" onClick={() => setMobileOpen(false)} className="block py-2 text-emerald-400">+ Create Auction</Link>}
              {isAdmin && <Link href="/admin" onClick={() => setMobileOpen(false)} className="block py-2 text-red-400">Admin Panel</Link>}
              <button onClick={() => { logout(); setMobileOpen(false); router.push('/'); }} className="block py-2 text-red-400">Logout</button>
            </>
          ) : (
            <div className="flex gap-3 pt-2">
              <Link href="/login" onClick={() => setMobileOpen(false)} className="flex-1 py-2.5 text-center bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-300 text-sm">Login</Link>
              <Link href="/register" onClick={() => setMobileOpen(false)} className="flex-1 py-2.5 text-center bg-emerald-600 rounded-lg text-white text-sm">Register</Link>
            </div>
          )}
        </div>
      )}
    </header>
  );
}
