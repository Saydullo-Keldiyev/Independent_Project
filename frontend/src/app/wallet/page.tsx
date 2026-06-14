'use client';

import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Navbar } from '@/components/layout/Navbar';
import { walletService } from '@/services/wallet.service';
import { formatPrice, formatDate } from '@/lib/utils';

export default function WalletPage() {
  const queryClient = useQueryClient();
  const [depositAmount, setDepositAmount] = useState('');
  const [showDeposit, setShowDeposit] = useState(false);

  const { data: walletData, isLoading } = useQuery({
    queryKey: ['wallet'],
    queryFn: walletService.getWallet,
  });

  const { data: historyData } = useQuery({
    queryKey: ['wallet-history'],
    queryFn: walletService.getHistory,
  });

  const depositMutation = useMutation({
    mutationFn: (amount: number) => walletService.deposit(amount),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['wallet'] });
      queryClient.invalidateQueries({ queryKey: ['wallet-history'] });
      setDepositAmount('');
      setShowDeposit(false);
    },
  });

  const wallet = walletData?.data || walletData;
  const transactions = historyData?.data?.transactions || historyData?.data || [];

  return (
    <div className="min-h-screen bg-zinc-950">
      <Navbar />
      <div className="max-w-4xl mx-auto px-4 py-8">
        <h1 className="text-3xl font-bold text-zinc-100 mb-8">💳 Wallet</h1>

        {/* Balance card */}
        <div className="bg-gradient-to-br from-emerald-600/20 to-zinc-900 border border-emerald-500/30 rounded-xl p-8 mb-8">
          <p className="text-sm text-emerald-300/70 uppercase tracking-wide">Available Balance</p>
          <p className="text-4xl font-bold text-zinc-100 mt-2 font-mono">
            {isLoading ? '...' : formatPrice(wallet?.balance || wallet?.available_balance || 0)}
          </p>
          <div className="flex gap-3 mt-6">
            <button onClick={() => setShowDeposit(!showDeposit)}
              className="px-5 py-2 bg-emerald-600 text-white rounded-lg font-medium hover:bg-emerald-500 transition-colors">
              + Deposit
            </button>
            <button className="px-5 py-2 bg-zinc-800 border border-zinc-700 text-zinc-300 rounded-lg font-medium hover:border-zinc-600 transition-colors">
              Withdraw
            </button>
          </div>
        </div>

        {/* Deposit form */}
        {showDeposit && (
          <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6 mb-8 animate-slide-up">
            <h3 className="font-semibold text-zinc-200 mb-4">Deposit Funds</h3>
            <div className="flex gap-3">
              <input type="number" step="0.01" min="1" value={depositAmount}
                onChange={e => setDepositAmount(e.target.value)}
                className="flex-1 px-4 py-2.5 bg-zinc-800 border border-zinc-700 rounded-lg text-zinc-100 focus:ring-2 focus:ring-emerald-500 outline-none"
                placeholder="Amount ($)" />
              <button
                onClick={() => depositMutation.mutate(parseFloat(depositAmount))}
                disabled={!depositAmount || depositMutation.isPending}
                className="px-6 py-2.5 bg-emerald-600 text-white rounded-lg font-medium hover:bg-emerald-500 disabled:opacity-50">
                {depositMutation.isPending ? 'Processing...' : 'Deposit'}
              </button>
            </div>
            {/* Quick amounts */}
            <div className="flex gap-2 mt-3">
              {[50, 100, 500, 1000].map(amt => (
                <button key={amt} onClick={() => setDepositAmount(amt.toString())}
                  className="px-3 py-1.5 bg-zinc-800 border border-zinc-700 rounded text-xs text-zinc-400 hover:border-emerald-500/50 hover:text-emerald-400">
                  ${amt}
                </button>
              ))}
            </div>
          </div>
        )}

        {/* Transaction history */}
        <div className="bg-zinc-900 border border-zinc-800 rounded-xl p-6">
          <h2 className="text-lg font-bold text-zinc-100 mb-4">Transaction History</h2>
          {transactions.length === 0 ? (
            <p className="text-zinc-500 text-center py-8">No transactions yet</p>
          ) : (
            <div className="space-y-2">
              {transactions.map((tx: any, i: number) => (
                <div key={tx.id || i} className="flex items-center justify-between p-3 bg-zinc-800/50 rounded-lg">
                  <div className="flex items-center gap-3">
                    <span className={`w-8 h-8 rounded-full flex items-center justify-center text-sm ${
                      tx.type === 'deposit' ? 'bg-emerald-500/20 text-emerald-400' :
                      tx.type === 'hold' ? 'bg-orange-500/20 text-orange-400' :
                      'bg-zinc-700 text-zinc-400'
                    }`}>
                      {tx.type === 'deposit' ? '↓' : tx.type === 'hold' ? '⏸' : '↑'}
                    </span>
                    <div>
                      <p className="text-sm text-zinc-300 capitalize">{tx.type}</p>
                      <p className="text-xs text-zinc-600">{tx.description || formatDate(tx.created_at)}</p>
                    </div>
                  </div>
                  <span className={`font-mono font-bold ${tx.type === 'deposit' || tx.type === 'release' ? 'text-emerald-400' : 'text-red-400'}`}>
                    {tx.type === 'deposit' || tx.type === 'release' ? '+' : '-'}{formatPrice(Math.abs(tx.amount))}
                  </span>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
