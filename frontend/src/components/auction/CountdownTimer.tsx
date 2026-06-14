'use client';

import { useEffect, useState } from 'react';

interface Props {
  endTime: string;
  compact?: boolean;
  onEnd?: () => void;
}

export function CountdownTimer({ endTime, compact, onEnd }: Props) {
  const [timeLeft, setTimeLeft] = useState(calcTimeLeft(endTime));

  useEffect(() => {
    const interval = setInterval(() => {
      const left = calcTimeLeft(endTime);
      setTimeLeft(left);
      if (left.total <= 0) {
        clearInterval(interval);
        onEnd?.();
      }
    }, 1000);
    return () => clearInterval(interval);
  }, [endTime, onEnd]);

  if (timeLeft.total <= 0) {
    return <span className="text-zinc-500 font-medium">Ended</span>;
  }

  const isUrgent = timeLeft.total < 3600000; // less than 1 hour

  if (compact) {
    return (
      <span className={`text-sm font-mono font-bold ${isUrgent ? 'text-red-400' : 'text-orange-400'}`}>
        {timeLeft.days > 0 && `${timeLeft.days}d `}
        {pad(timeLeft.hours)}:{pad(timeLeft.minutes)}:{pad(timeLeft.seconds)}
      </span>
    );
  }

  return (
    <div className={`flex gap-2 ${isUrgent ? 'text-red-400' : 'text-zinc-100'}`}>
      {timeLeft.days > 0 && (
        <TimeBlock value={timeLeft.days} label="Days" />
      )}
      <TimeBlock value={timeLeft.hours} label="Hrs" />
      <TimeBlock value={timeLeft.minutes} label="Min" />
      <TimeBlock value={timeLeft.seconds} label="Sec" urgent={isUrgent} />
    </div>
  );
}

function TimeBlock({ value, label, urgent }: { value: number; label: string; urgent?: boolean }) {
  return (
    <div className={`text-center px-3 py-2 rounded-lg ${urgent ? 'bg-red-500/10 border border-red-500/30' : 'bg-zinc-800 border border-zinc-700'}`}>
      <div className="text-xl font-mono font-bold">{pad(value)}</div>
      <div className="text-[10px] uppercase tracking-wider text-zinc-500">{label}</div>
    </div>
  );
}

function calcTimeLeft(endTime: string) {
  const diff = new Date(endTime).getTime() - Date.now();
  if (diff <= 0) return { total: 0, days: 0, hours: 0, minutes: 0, seconds: 0 };
  return {
    total: diff,
    days: Math.floor(diff / 86400000),
    hours: Math.floor((diff % 86400000) / 3600000),
    minutes: Math.floor((diff % 3600000) / 60000),
    seconds: Math.floor((diff % 60000) / 1000),
  };
}

function pad(n: number) { return n.toString().padStart(2, '0'); }
