import { LocationDetailView } from "./view";

// Static export requires generateStaticParams. Return one placeholder so the
// route's JS bundle is emitted; in production, Cloudflare Pages SPA fallback
// rewrites serve this bundle for any /locations/{uuid} path, and the client
// component reads the real id from the URL at runtime.
export function generateStaticParams() {
  return [{ id: "_" }];
}

// Next 15: params is a Promise in server components.
export default async function LocationDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <LocationDetailView locationId={id} />;
}
