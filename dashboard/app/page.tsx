'use client';

import { useCallback, useEffect, useState } from 'react';
import { fetchTargets, fetchAllQueueData, type Target, type QueueData } from '@/lib/api';
import { QueueCard } from '@/components/QueueCard';

const POLL_INTERVAL_MS = 5_000;

// Per-target state — queues and error are mutually exclusive.
interface TargetState {
  target: Target;
  queues: QueueData[];
  error: string | null;
}

function SkeletonCard() {
  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5 h-44 animate-pulse">
      <div className="h-3 bg-slate-700 rounded w-1/4 mb-3" />
      <div className="h-4 bg-slate-700 rounded w-1/3 mb-4" />
      <div className="h-3 bg-slate-700 rounded w-1/4 mb-6" />
      <div className="grid grid-cols-2 gap-4 pt-3 border-t border-slate-700/60">
        <div className="h-8 bg-slate-700 rounded" />
        <div className="h-8 bg-slate-700 rounded" />
      </div>
    </div>
  );
}

function OfflineCard({ target, error }: { target: Target; error: string }) {
  return (
    <div className="bg-slate-800 rounded-xl border border-red-900/50 p-5 flex flex-col gap-3">
      <div className="flex items-start justify-between gap-2">
        <span className="text-[10px] font-medium text-slate-500 uppercase tracking-wider">
          {target.name}
        </span>
      </div>
      <div className="flex items-center gap-2">
        <span className="h-2.5 w-2.5 rounded-full bg-red-500 shrink-0" />
        <span className="text-red-400 text-sm font-medium">Offline</span>
      </div>
      <p className="text-xs text-slate-500 truncate" title={error}>{error}</p>
    </div>
  );
}

export default function DashboardPage() {
  const [targets, setTargets] = useState<Target[]>([]);
  const [states, setStates] = useState<TargetState[]>([]);
  const [lastUpdated, setLastUpdated] = useState<Date | null>(null);
  const [targetsError, setTargetsError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  // Load target list once on mount.
  useEffect(() => {
    fetchTargets()
      .then(setTargets)
      .catch((e) => setTargetsError(e instanceof Error ? e.message : 'Failed to load targets'));
  }, []);

  // Poll all targets in parallel on every tick.
  const refresh = useCallback(async () => {
    if (targets.length === 0) return;

    const results = await Promise.all(
      targets.map(async (t): Promise<TargetState> => {
        try {
          const queues = await fetchAllQueueData(t.id);
          return { target: t, queues, error: null };
        } catch (e) {
          return {
            target: t,
            queues: [],
            error: e instanceof Error ? e.message : 'Unknown error',
          };
        }
      }),
    );

    setStates(results);
    setLastUpdated(new Date());
    setLoading(false);
  }, [targets]);

  useEffect(() => {
    if (targets.length === 0) return;
    refresh();
    const id = setInterval(refresh, POLL_INTERVAL_MS);
    return () => clearInterval(id);
  }, [targets, refresh]);

  // Aggregate stats across all targets.
  const allQueues = states.flatMap((s) => s.queues);
  const totalPending = allQueues.reduce((n, q) => n + q.pending.count, 0);
  const totalProcessing = allQueues.filter((q) => q.worker.state === 'processing').length;
  const offlineCount = states.filter((s) => s.error !== null).length;

  return (
    <div className="min-h-screen bg-slate-900 text-slate-100">
      <div className="max-w-5xl mx-auto px-4 py-8 space-y-6">

        {/* Header */}
        <div className="flex items-start justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-white tracking-tight">Automation</h1>
            <p className="text-sm text-slate-400 mt-0.5">
              {targets.length > 0
                ? `${targets.length} container${targets.length !== 1 ? 's' : ''}`
                : 'Queue dashboard'}
            </p>
          </div>
          <div className="text-right space-y-1 shrink-0">
            {targetsError ? (
              <p className="text-xs text-red-400">{targetsError}</p>
            ) : lastUpdated ? (
              <p className="text-xs text-slate-500">
                Updated {lastUpdated.toLocaleTimeString()}
              </p>
            ) : null}
            <p className="text-xs text-slate-600">
              Refreshes every {POLL_INTERVAL_MS / 1000}s
            </p>
          </div>
        </div>

        {/* Summary bar — only shown once data has loaded */}
        {!loading && states.length > 0 && (
          <div className="grid grid-cols-4 gap-3">
            {[
              { label: 'Containers', value: targets.length },
              { label: 'Pending', value: totalPending },
              { label: 'Processing', value: totalProcessing },
              {
                label: 'Offline',
                value: offlineCount,
                highlight: offlineCount > 0,
              },
            ].map(({ label, value, highlight }) => (
              <div
                key={label}
                className="bg-slate-800/60 border border-slate-700/60 rounded-lg px-4 py-3 flex flex-col items-center"
              >
                <span
                  className={`text-2xl font-bold tabular-nums ${
                    highlight ? 'text-red-400' : 'text-white'
                  }`}
                >
                  {value}
                </span>
                <span className="text-xs text-slate-400 mt-0.5">{label}</span>
              </div>
            ))}
          </div>
        )}

        {/* Cards */}
        {loading ? (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {Array.from({ length: 4 }).map((_, i) => (
              <SkeletonCard key={i} />
            ))}
          </div>
        ) : states.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-24 text-slate-500 space-y-2">
            <svg className="w-8 h-8 text-slate-600" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5}
                d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            <p className="text-sm">No containers configured</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            {states.flatMap(({ target, queues, error }) => {
              if (error !== null) {
                return [<OfflineCard key={target.id} target={target} error={error} />];
              }
              return queues.map((q) => (
                <QueueCard
                  key={`${target.id}/${q.name}`}
                  data={q}
                  targetName={targets.length > 1 ? target.name : undefined}
                />
              ));
            })}
          </div>
        )}
      </div>
    </div>
  );
}
