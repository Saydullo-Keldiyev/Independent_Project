import Link from 'next/link';

export default function NotFound() {
  return (
    <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
      <div className="text-center">
        <div className="text-8xl mb-4">🔍</div>
        <h1 className="text-4xl font-bold text-zinc-100 mb-2">404</h1>
        <p className="text-zinc-500 mb-6">Page not found</p>
        <Link href="/" className="px-6 py-2.5 bg-emerald-600 text-white rounded-lg hover:bg-emerald-500 transition-colors">
          Go Home
        </Link>
      </div>
    </div>
  );
}
