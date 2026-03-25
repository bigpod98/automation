import { NextRequest, NextResponse } from 'next/server';
import { resolveTargetUrl } from '@/lib/targets';

type Params = Promise<{ target: string; path: string[] }>;

export async function GET(_req: NextRequest, { params }: { params: Params }) {
  const { target, path } = await params;

  const baseUrl = resolveTargetUrl(target);
  if (!baseUrl) {
    return NextResponse.json(
      { error: `Unknown target "${target}"` },
      { status: 404 },
    );
  }

  const upstreamUrl = `${baseUrl}/${path.join('/')}`;
  try {
    const res = await fetch(upstreamUrl, {
      headers: { Accept: 'application/json' },
      cache: 'no-store',
    });
    const body: unknown = await res.json();
    return NextResponse.json(body, { status: res.status });
  } catch (e) {
    return NextResponse.json(
      { error: `Upstream unreachable: ${e instanceof Error ? e.message : String(e)}` },
      { status: 502 },
    );
  }
}
