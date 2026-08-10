import { EmptyState, PageHeader } from "@/components/ui/primitives";

/**
 * Henüz uygulanmamış sayfalar için yer tutucu.
 *
 * Hangi fazda geleceğini açıkça yazar — boş bir sayfa "bozuk" gibi görünür,
 * bu bileşen "henüz sırası gelmedi" der.
 */
export function PagePlaceholder({
  title,
  phase,
  description,
}: {
  title: string;
  phase: string;
  description: string;
}) {
  return (
    <div>
      <PageHeader title={title} />
      <EmptyState title={`${phase} kapsamında gelecek`} description={description} />
    </div>
  );
}
