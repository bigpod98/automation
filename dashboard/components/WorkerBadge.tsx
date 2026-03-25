interface WorkerBadgeProps {
  state: 'idle' | 'processing';
  claim?: string;
}

export function WorkerBadge({ state, claim }: WorkerBadgeProps) {
  if (state === 'processing') {
    return (
      <div className="flex items-center gap-2 min-w-0">
        <span className="relative flex h-2.5 w-2.5 shrink-0">
          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
          <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-amber-500" />
        </span>
        <span className="text-amber-400 text-sm font-medium shrink-0">Processing</span>
        {claim && (
          <span
            className="text-slate-400 text-xs truncate"
            title={claim}
          >
            {claim}
          </span>
        )}
      </div>
    );
  }

  return (
    <div className="flex items-center gap-2">
      <span className="h-2.5 w-2.5 rounded-full bg-green-500 shrink-0" />
      <span className="text-green-400 text-sm font-medium">Idle</span>
    </div>
  );
}
