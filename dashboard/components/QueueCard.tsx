import type { QueueData } from '@/lib/api';
import { WorkerBadge } from './WorkerBadge';
import { ItemList } from './ItemList';

function formatQueueName(name: string): string {
  return name
    .split('-')
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1))
    .join(' ');
}

interface QueueCardProps {
  data: QueueData;
  // Shown when multiple containers are configured so the user knows which
  // container this queue belongs to.
  targetName?: string;
}

export function QueueCard({ data, targetName }: QueueCardProps) {
  const updatedAt = new Date(data.worker.updated_at);

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5 flex flex-col gap-4">
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          {targetName && (
            <p className="text-[10px] font-medium text-slate-500 uppercase tracking-wider mb-1 truncate">
              {targetName}
            </p>
          )}
          <h2 className="text-base font-semibold text-white leading-tight">
            {formatQueueName(data.name)}
          </h2>
        </div>
        <span className="text-[10px] text-slate-600 shrink-0 mt-0.5">
          {updatedAt.toLocaleTimeString()}
        </span>
      </div>

      {/* Worker state */}
      <WorkerBadge state={data.worker.state} claim={data.worker.current_claim} />

      {/* Queue counts */}
      <div className="grid grid-cols-2 gap-4 pt-3 border-t border-slate-700/60">
        <ItemList
          label="Pending"
          items={data.pending.items}
          count={data.pending.count}
        />
        <ItemList
          label="In Progress"
          items={data.inProgress.items}
          count={data.inProgress.count}
        />
      </div>
    </div>
  );
}
