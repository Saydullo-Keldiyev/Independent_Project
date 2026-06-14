'use client';

export default function Error({ error, reset }: { error: Error; reset: () => void }) {
  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
      <div className="text-center max-w-md px-4">
        <div className="text-6xl mb-4">⚠️</div>
        <h1 className="text-3xl font-bold text-zinc-100 mb-2">Something went wrong</h1>
        <p className="text-zinc-500 mb-6 text-sm">{error.message || 'An unexpected error occurred'}</p>
        <div className="flex gap-3 justify-center">
          <button onClick={reset} className="px-6 py-2.5 bg-emerald-600 text-white rounded-lg hover:bg-emerald-500 transition-colors">
            Try Again
          </button>
          <a href="/" className="px-6 py-2.5 bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg hover:border-zinc-600 transition-colors">
            Go Home
          </a>
        </div>
      </div>
    </div>
  );
}
