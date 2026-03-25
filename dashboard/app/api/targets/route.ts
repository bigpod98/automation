import { NextResponse } from 'next/server';
import { parseTargets } from '@/lib/targets';

// Returns { id, name } for each configured target — the upstream URL is never exposed.
export function GET() {
  const targets = parseTargets().map(({ id, name }) => ({ id, name }));
  return NextResponse.json(targets);
}
