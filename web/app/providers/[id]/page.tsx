import { ProviderDetailView } from "./view";

export function generateStaticParams() {
  return [{ id: "_" }];
}

export default async function ProviderDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  return <ProviderDetailView providerId={id} />;
}
