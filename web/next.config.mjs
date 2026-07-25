/** @type {import('next').NextConfig} */
// Static export (ADR-013: the dashboard is a CDN-served static build, no Node
// runtime). `output: 'export'` emits a fully static site to `out/`. Dev mode
// omits it so dynamic detail routes ([id] with the "_" placeholder param)
// resolve without pre-generation; production CDN rewrites handle those paths.
const nextConfig = {
  ...(process.env.NODE_ENV === 'production' ? { output: 'export' } : {}),
  reactStrictMode: true,
  trailingSlash: true,
  // The Next image optimizer needs a server; static export requires unoptimized.
  images: { unoptimized: true },
};

export default nextConfig;
