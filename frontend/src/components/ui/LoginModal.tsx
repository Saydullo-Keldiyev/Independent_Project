'use client';

import Link from 'next/link';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  message?: string;
}

export function LoginModal({ isOpen, onClose, message }: Props) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={onClose} />
      <div className="relative bg-zinc-900 border border-zinc-800 rounded-xl p-8 max-w-sm w-full mx-4 animate-slide-up">
        <div className="text-center">
          <div className="text-4xl mb-4">🔒</div>
          <h2 className="text-xl font-bold text-zinc-100 mb-2">Login Required</h2>
          <p className="text-zinc-400 text-sm mb-6">
            {message || 'Please login to continue'}
          </p>
          <div className="flex gap-3">
            <Link href="/login" className="flex-1 py-2.5 bg-emerald-600 text-white rounded-lg font-medium text-center hover:bg-emerald-500">
              Login
            </Link>
            <Link href="/register" className="flex-1 py-2.5 bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg font-medium text-center hover:border-zinc-600">
              Register
            </Link>
          </div>
          <button onClick={onClose} className="mt-4 text-xs text-zinc-600 hover:text-zinc-400">
            Cancel
          </button>
        </div>
      </div>
    </div>
  );
}
