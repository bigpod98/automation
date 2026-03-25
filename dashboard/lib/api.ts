// Public target descriptor — only id and name, no upstream URL.
export interface Target {
  id: string;
  name: string;
}

export interface WorkerStatus {
  name: string;
  state: 'idle' | 'processing';
  current_claim?: string;
  updated_at: string;
}

export interface QueueSummary {
  name: string;
  worker: WorkerStatus;
}

export interface ItemsResponse {
  queue: string;
  items: string[];
  count: number;
}

export interface QueueData extends QueueSummary {
  pending: ItemsResponse;
  inProgress: ItemsResponse;
}

async function get<T>(targetId: string, path: string): Promise<T> {
  const res = await fetch(`/api/${targetId}${path}`, { cache: 'no-store' });
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<T>;
}

export async function fetchTargets(): Promise<Target[]> {
  const res = await fetch('/api/targets');
  if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
  return res.json() as Promise<Target[]>;
}

export async function fetchAllQueueData(targetId: string): Promise<QueueData[]> {
  const summaries = await get<QueueSummary[]>(targetId, '/queues');

  const results = await Promise.allSettled(
    summaries.map(async (s) => {
      const [pending, inProgress] = await Promise.all([
        get<ItemsResponse>(targetId, `/queues/${s.name}/pending`),
        get<ItemsResponse>(targetId, `/queues/${s.name}/in-progress`),
      ]);
      return { ...s, pending, inProgress } satisfies QueueData;
    }),
  );

  return results
    .filter((r): r is PromiseFulfilledResult<QueueData> => r.status === 'fulfilled')
    .map((r) => r.value);
}
