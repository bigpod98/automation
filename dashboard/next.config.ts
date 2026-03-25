import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  // Required for the minimal Docker image (copies only what's needed to run).
  output: 'standalone',
};

export default nextConfig;
