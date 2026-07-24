/** @type {import('next').NextConfig} */
// Static export (ADR-013: the dashboard is a CDN-served static build, no Node
// runtime). `output: 'export'` emits a fully static site to `out/`.
const nextConfig = {
  output: 'export',
  reactStrictMode: true,
  trailingSlash: true,
  // The Next image optimizer needs a server; static export requires unoptimized.
  images: { unoptimized: true },
};

export default nextConfig;
