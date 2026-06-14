import type { Metadata } from 'next';
import { Providers } from './providers';
import { ToastProvider } from '@/components/ui/Toast';
import './globals.css';

export const metadata: Metadata = {
  title: 'AuctionHub — Real-Time Auction Platform',
  description: 'Bid live, win instantly. WebSocket-powered real-time auctions.',
  openGraph: {
    title: 'AuctionHub — Real-Time Auction Platform',
    description: 'Bid live, win instantly.',
    type: 'website',
  },
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" className="dark">
      <body className="min-h-screen bg-zinc-950 text-zinc-100 antialiased">
        <Providers>
          {children}
          <ToastProvider />
        </Providers>
      </body>
    </html>
  );
}
