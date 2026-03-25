// Server-side only — never import this from a client component or lib/api.ts.

export interface TargetConfig {
  id: string;
  name: string;
  url: string; // never sent to the browser
}

/**
 * Parses the API_TARGETS environment variable.
 *
 * Format: comma-separated id=url pairs.
 * Example:
 *   API_TARGETS=production=http://prod.example.com:8080,staging=http://staging:8080
 *
 * Falls back to a single "default" target at http://localhost:8080.
 */
export function parseTargets(): TargetConfig[] {
  const raw = process.env.API_TARGETS ?? 'default=http://localhost:8080';
  return raw
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean)
    .map((entry) => {
      const eq = entry.indexOf('=');
      if (eq === -1) {
        throw new Error(
          `Invalid API_TARGETS entry "${entry}" — expected format: id=http://host:port`,
        );
      }
      const id = entry.slice(0, eq).trim();
      const url = entry.slice(eq + 1).trim().replace(/\/$/, '');
      const name =
        id.charAt(0).toUpperCase() +
        id.slice(1).replace(/[-_]+/g, ' ');
      return { id, name, url };
    });
}

export function resolveTargetUrl(targetId: string): string | null {
  return parseTargets().find((t) => t.id === targetId)?.url ?? null;
}
