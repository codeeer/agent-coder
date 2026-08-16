"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useState } from "react";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { RunBatchDetail, RunBatchItem } from "@/lib/types";
import { IconExternal, IconFolder, IconTrash } from "@/components/ui/icons";
import {
  Button,
  Card,
  ConfirmStrip,
  formatDate,
  List,
  Notice,
  PageHeader,
  Section,
  Skeleton,
} from "@/components/ui/primitives";
import {
  RunBatchBadge,
  RunBatchItemBadge,
  isBatchActive,
} from "@/components/workflows/RunBatchBadges";
import { CountStrip } from "@/components/workflows/CountStrip";
import {
  devamEtiketi,
  iptalSonucu,
  ogeCalismaYolu,
  silmeSonucu,
} from "@/components/workflows/batch-selection";

/**
 * Toplu işin ilerleme ekranı (spec 023 H3).
 *
 * TEK EKRAN: otuz işin durumunu tek tek aramak, elle tetiklemekten iyi olmaz.
 * Üstte sayılar, altta öğe listesi; her satır kendi akış çalışmasına bağlanır.
 *
 * İş bitince ekran BOŞALMAZ: sonuç özeti kalır. Kullanıcı kampanyanın sonucunu
 * ancak burada görüyor.
 */
export default function RunBatchDetailPage() {
  const { id } = useParams<{ id: string }>();
  const qc = useQueryClient();
  const router = useRouter();

  const [iptalOnayi, setIptalOnayi] = useState(false);
  const [silmeOnayi, setSilmeOnayi] = useState(false);

  const batch = useQuery({
    queryKey: ["run-batch", id],
    queryFn: () => api.runBatches.get(id),
    // Süren işte kendiliğinden tazelenir (mevcut refetchInterval kalıbı).
    refetchInterval: (q) => (q.state.data && isBatchActive(q.state.data.status) ? 3000 : false),
  });

  const tazele = () => {
    void qc.invalidateQueries({ queryKey: ["run-batch", id] });
    void qc.invalidateQueries({ queryKey: ["run-batches"] });
  };

  const iptal = useMutation({
    mutationFn: () => api.runBatches.cancel(id),
    onSuccess: () => {
      setIptalOnayi(false);
      tazele();
    },
  });

  const devam = useMutation({
    mutationFn: () => api.runBatches.resume(id),
    onSuccess: tazele,
  });

  // Silinen işin detay sayfası artık yok: kullanıcı listeye döner. Burada
  // kalsaydı sonraki tazeleme "bulunamadı" hatasıyla düşerdi.
  const sil = useMutation({
    mutationFn: () => api.runBatches.remove(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["run-batches"] });
      router.push("/run-batches");
    },
  });

  if (batch.isPending) return <Skeleton rows={4} />;
  if (batch.isError) {
    return <Notice tone="error">{describeError(batch.error).message}</Notice>;
  }

  const b = batch.data;
  const items = b.items ?? [];
  const devamMetni = devamEtiketi(b.counts.interrupted);
  const iptalEdilebilir = b.counts.pending > 0;

  /*
    SİLME YALNIZCA GERÇEKTEN SİLİNEBİLİYORKEN ÇIKAR.

    Sunucudaki koruma ile birebir aynı koşul: kuyruk durmuş VE ortada canlı öğe
    kalmamış olmalı. (İptal edilmiş bir işin o sırada çalışan öğesi bitene kadar
    sürer — durum 'cancelled' iken bile canlı çalışma kalabiliyor.)

    Tıklanmayan bir çöp kutusu göstermek, bu işin baştaki hatasıydı.
  */
  const silinebilir =
    !isBatchActive(b.status) && b.counts.pending === 0 && b.counts.running === 0;

  return (
    <div className="space-y-6">
      <PageHeader
        title={b.workflowName}
        description={b.task || "Görev metni verilmedi."}
        actions={
          <div className="flex items-center gap-2">
            <RunBatchBadge status={b.status} />
            <Link href={`/workflows/${b.workflowId}`}>
              <Button size="sm" variant="ghost" icon={<IconExternal />}>
                Akışı aç
              </Button>
            </Link>
          </div>
        }
      />

      <Card>
        <div className="mb-3 flex flex-wrap items-center gap-x-4 gap-y-1 text-2xs text-ink-3">
          <span>Başladı: {formatDate(b.createdAt)}</span>
          {/* Son hareket YALNIZCA iş bittiğinde yazılır: sürerken her tazelemede
              değişen bir zaman damgası bilgi değil, kıpırtı olurdu. */}
          {!isBatchActive(b.status) && <span>Son hareket: {formatDate(b.updatedAt)}</span>}
        </div>

        <CountStrip counts={b.counts} active={isBatchActive(b.status)} />

        <div className="mt-4 flex flex-wrap items-center gap-2 border-t border-line pt-4">
          {/*
            KALDIĞI YERDEN DEVAM — yalnızca kesilmiş öğe varken çıkar ve kaç
            işin sıraya alınacağını ÜZERİNDE yazar. Tamamlananlar tekrar
            koşturulmaz; gerçekten başarısız olanlar da kendiliğinden sıraya
            alınmaz — onlar çalıştı ve bir sonuç üretti.
          */}
          {devamMetni && (
            <Button
              variant="primary"
              disabled={devam.isPending}
              onClick={() => devam.mutate()}
            >
              {devam.isPending ? "Sıraya alınıyor…" : devamMetni}
            </Button>
          )}

          {iptalEdilebilir && !iptalOnayi && (
            <Button onClick={() => setIptalOnayi(true)}>Toplu işi iptal et</Button>
          )}

          {!devamMetni && !iptalEdilebilir && (
            <p className="text-xs text-ink-3">
              {b.counts.running > 0
                ? "Süren işler var; bekleyen iş kalmadı."
                : "Bu toplu işte yapılacak bir şey kalmadı."}
            </p>
          )}

          {/*
            SİLME SAĞA AYRILIR ve KENARLIKLIDIR.

            Sağda: soldaki eylemler işi ilerletir ("devam et", "iptal et"),
            silme ise işi ortadan kaldırır — aynı öbekte durursa yanlışlıkla
            tıklanacak sıradaki düğme olur.

            `danger`: durgunken sessiz, hover'da kırmızı — projenin yıkıcı
            eylem kalıbı. ghost olarak da denendi ve satırdaki düz metnin
            yanında düğme olduğu anlaşılmıyordu; `danger` ölçülmüş
            `control-line` sınırını taşıyor.
          */}
          {silinebilir && !silmeOnayi && (
            <Button
              className="ml-auto"
              variant="danger"
              icon={<IconTrash />}
              onClick={() => setSilmeOnayi(true)}
            >
              Sil
            </Button>
          )}
        </div>

        {iptalOnayi && (
          <div className="mt-3 overflow-hidden rounded-lg border border-danger/30">
            {/*
              Onay şeridi SONUCU yazar: "emin misiniz?" hiçbir şey söylemez.
              Çalışan işlerin süreceği de burada yazılı — kullanıcı "iptal"
              deyince her şeyin duracağını sanmamalı.
            */}
            <ConfirmStrip
              question="Toplu iş iptal edilsin mi?"
              consequence={iptalSonucu(b.counts.pending, b.counts.running)}
              confirmLabel="Evet, iptal et"
              busyLabel="İptal ediliyor…"
              busy={iptal.isPending}
              error={iptal.isError ? describeError(iptal.error).message : undefined}
              onConfirm={() => iptal.mutate()}
              onCancel={() => setIptalOnayi(false)}
            />
          </div>
        )}

        {silmeOnayi && (
          <div className="mt-3 overflow-hidden rounded-lg border border-danger/30">
            {/*
              Sonuç YAZILIR ve sayı verilir: silinen şey yalnızca bu kayıt değil,
              altındaki bütün geçmiş. Kullanıcı neyi kaybettiğini tıklamadan
              önce bilmeli.
            */}
            <ConfirmStrip
              question="Toplu iş silinsin mi?"
              consequence={silmeSonucu(b.counts.total)}
              busy={sil.isPending}
              error={sil.isError ? describeError(sil.error).message : undefined}
              onConfirm={() => sil.mutate()}
              onCancel={() => setSilmeOnayi(false)}
            />
          </div>
        )}

        {devam.isError && (
          <div className="mt-3">
            <Notice tone="error">{describeError(devam.error).message}</Notice>
          </div>
        )}
      </Card>

      <Section
        title="İşler"
        description="Sıra eklenme sırasıdır; her satır kendi akış çalışmasına bağlanır."
      >
        <List>
          {items.map((it) => (
            <ItemRow key={it.id} workflowId={b.workflowId} item={it} />
          ))}
        </List>
        {items.length === 0 && (
          <Card>
            <p className="text-sm text-ink-3">
              Bu toplu işin öğesi kalmadı — seçilen projeler silinmiş olabilir.
            </p>
          </Card>
        )}
      </Section>
    </div>
  );
}

function ItemRow({ workflowId, item }: { workflowId: string; item: RunBatchItem }) {
  const yol = ogeCalismaYolu(workflowId, item);

  const govde = (
    <>
      <span className="w-6 shrink-0 text-right text-2xs text-ink-3 tabular-nums">
        {item.position + 1}
      </span>
      <div className="w-24 shrink-0">
        <RunBatchItemBadge status={item.status} />
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex items-center gap-1.5 truncate text-sm">
          <IconFolder className="size-3.5 shrink-0 text-ink-3" />
          {item.projectName}
        </div>
        {/*
          Sebep satırın kendi içinde: başarısızlığı görmek için ikinci bir
          ekrana gitmek gerekmemeli.

          RENK DURUMA GÖRE: "Backend yeniden başladığında kesildi" bir hata
          değil, bir açıklama. Kırmızı yazmak onu derleme hatasıyla aynı
          ağırlığa koyar ve gerçekten başarısız olan satırı görünmez yapardı.
        */}
        {item.error && (
          <div
            className={`mt-0.5 truncate text-2xs ${
              item.status === "failed" ? "text-danger" : "text-ink-3"
            }`}
            title={item.error}
          >
            {item.error}
          </div>
        )}
      </div>
      {yol && <IconExternal className="shrink-0 text-ink-3" />}
    </>
  );

  // Başlatılmamış öğenin çalışması yok: satır bağlantı değil düz metin olur.
  // Boş bir adrese bağlamak, tıklayınca hiçbir şey bulunmayan bir sayfaya
  // götürürdü.
  return yol ? (
    <Link
      href={yol}
      className="flex items-center gap-3 px-4 py-2.5 transition-colors hover:bg-raised"
    >
      {govde}
    </Link>
  ) : (
    <div className="flex items-center gap-3 px-4 py-2.5">{govde}</div>
  );
}

export type { RunBatchDetail };
