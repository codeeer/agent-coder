"use client";

import { useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { GitProvider, ImportLine, ImportRepo } from "@/lib/types";
import {
  Badge,
  Button,
  Checkbox,
  Input,
  Label,
  Notice,
  Panel,
  Select,
} from "@/components/ui/primitives";
import { IconCheck, IconAlert, IconFolder } from "@/components/ui/icons";
import { repoLabel } from "./repo-url";
import {
  etkinProvider,
  secilebilir,
  secimAcKapa,
  tumunuSec,
  varsayilanSecim,
} from "./import-selection";

/**
 * Kurumsal Bitbucket grubundan toplu proje ekleme (spec 021).
 *
 * İKİ FAZ, TEK PANEL: önce grup adresi verilir ve repository'ler listelenir,
 * sonra seçilenler eklenir. Ayrı iki ekrana bölünmedi — kullanıcının kararı
 * (hangileri) ancak listeyi görünce oluşuyor ve liste kaybolduğunda karar da
 * kaybolurdu.
 *
 * BULUT DEĞİL. Bu yol yalnızca kendi sunucusunda çalışan kurumsal Bitbucket
 * içindir; bulut adresi backend tarafından ayrı bir mesajla reddedilir.
 */
export function ImportGroupPanel({
  gitProviders,
  onDone,
}: {
  gitProviders: GitProvider[];
  onDone: () => void;
}) {
  const queryClient = useQueryClient();

  const bitbucketler = gitProviders.filter((p) => p.type === "bitbucket");

  const [groupUrl, setGroupUrl] = useState("");

  // Durum yalnızca ELLE yapılan seçimi tutar; görünen değer türetilir.
  // Gerekçe ve sınır durumları: import-selection.ts → etkinProvider.
  const [providerSecimi, setProviderSecimi] = useState("");
  const providerId = etkinProvider(providerSecimi, bitbucketler);
  const [repos, setRepos] = useState<ImportRepo[] | null>(null);
  const [secili, setSecili] = useState<Set<string>>(new Set());

  /** slug → sonuç satırı. İçe aktarma sırasında dolar. */
  const [sonuclar, setSonuclar] = useState<Map<string, ImportLine>>(new Map());
  const [ozet, setOzet] = useState<ImportLine["summary"] | null>(null);

  const onizle = useMutation({
    mutationFn: () => api.projects.importPreview({ groupUrl, gitProviderId: providerId }),
    onSuccess: (d) => {
      setRepos(d.repos);
      setSecili(new Set(varsayilanSecim(d.repos)));
      setSonuclar(new Map());
      setOzet(null);
    },
  });

  const aktar = useMutation({
    mutationFn: async () => {
      const secilenler = (repos ?? []).filter((r) => secili.has(r.slug));
      await api.projects.importRun(
        { gitProviderId: providerId, repos: secilenler },
        (line) => {
          if (line.summary) {
            setOzet(line.summary);
            return;
          }
          // Satırlar geldikçe yazılır: kullanıcı hangisinin bittiğini anında
          // görür, sonuçların tamamını beklemez.
          setSonuclar((onceki) => new Map(onceki).set(line.slug ?? "", line));
        },
      );
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ["projects"] });
    },
  });

  const secilenSayisi = secili.size;
  const bitenSayisi = sonuclar.size;

  return (
    <Panel
      title="Gruptan içe aktar"
      description="Kurumsal Bitbucket'ta bir grubun adresini verin; altındaki repository'ler listelensin ve seçtikleriniz proje olarak eklensin."
      action={
        <Button variant="ghost" onClick={onDone}>
          Kapat
        </Button>
      }
    >
      {bitbucketler.length === 0 ? (
        <Notice tone="warning">
          Tanımlı bir Bitbucket erişimi yok. Ayarlar → Git repository&apos;ler
          bölümünden ekleyin.
        </Notice>
      ) : (
        <>
          <div className="flex flex-wrap items-end gap-3">
            <div className="min-w-0 flex-1">
              <Label>Grup adresi</Label>
              <Input
                value={groupUrl}
                onChange={(e) => setGroupUrl(e.target.value)}
                placeholder="https://bitbucket.sirket.com/projects/ODEME"
                spellCheck={false}
              />
            </div>

            <div className="w-56 shrink-0">
              <Label>Git erişimi</Label>
              <Select
                value={providerId}
                onChange={(e) => setProviderSecimi(e.target.value)}
              >
                {bitbucketler.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.name}
                  </option>
                ))}
              </Select>
            </div>

            <Button
              variant="secondary"
              onClick={() => onizle.mutate()}
              disabled={onizle.isPending || groupUrl.trim() === ""}
            >
              {onizle.isPending ? "Listeleniyor…" : "Listele"}
            </Button>
          </div>

          {onizle.isError && (
            <div className="mt-3">
              <Notice tone="error">{describeError(onizle.error).message}</Notice>
            </div>
          )}

          {repos && (
            <RepoListesi
              repos={repos}
              secili={secili}
              sonuclar={sonuclar}
              onToggle={(slug) => setSecili((s) => secimAcKapa(s, slug))}
              onTumu={() => setSecili(new Set(tumunuSec(repos)))}
              onTemizle={() => setSecili(new Set())}
              aktariliyor={aktar.isPending}
              bitenSayisi={bitenSayisi}
              secilenSayisi={secilenSayisi}
              onAktar={() => aktar.mutate()}
            />
          )}

          {aktar.isError && (
            <div className="mt-3">
              <Notice tone="error">{describeError(aktar.error).message}</Notice>
            </div>
          )}

          {ozet && <Ozet ozet={ozet} sonuclar={sonuclar} />}
        </>
      )}
    </Panel>
  );
}

function RepoListesi({
  repos,
  secili,
  sonuclar,
  onToggle,
  onTumu,
  onTemizle,
  onAktar,
  aktariliyor,
  bitenSayisi,
  secilenSayisi,
}: {
  repos: ImportRepo[];
  secili: Set<string>;
  sonuclar: Map<string, ImportLine>;
  onToggle: (slug: string) => void;
  onTumu: () => void;
  onTemizle: () => void;
  onAktar: () => void;
  aktariliyor: boolean;
  bitenSayisi: number;
  secilenSayisi: number;
}) {
  if (repos.length === 0) {
    return (
      <p className="mt-4 text-sm text-ink-2">
        Bu grupta repository yok. {/* Hata değil: sakin bir cümle (spec 021). */}
      </p>
    );
  }

  const yeniSayisi = repos.filter(secilebilir).length;

  return (
    <>
      {/* Araç çubuğu: sayaç solda, toplu eylemler ve asıl eylem sağda.
          Tek dolu düğme var — asıl eylem o (ui.md → Eylem hiyerarşisi). */}
      <div className="mt-4 flex flex-wrap items-center justify-between gap-3 border-b border-line pb-2">
        <p className="text-xs text-ink-2">
          <span className="font-medium text-ink">{repos.length}</span> repository ·{" "}
          <span className="font-medium text-ink">{yeniSayisi}</span> eklenebilir ·{" "}
          <span className="font-medium text-ink">{secilenSayisi}</span> seçili
        </p>

        <div className="flex items-center gap-2">
          <Button variant="ghost" onClick={onTumu} disabled={aktariliyor}>
            Tümünü seç
          </Button>
          <Button variant="ghost" onClick={onTemizle} disabled={aktariliyor}>
            Temizle
          </Button>
          <Button
            variant="primary"
            onClick={onAktar}
            disabled={aktariliyor || secilenSayisi === 0}
          >
            {aktariliyor
              ? `Ekleniyor… ${bitenSayisi} / ${secilenSayisi}`
              : `${secilenSayisi} projeyi ekle`}
          </Button>
        </div>
      </div>

      <ul className="divide-y divide-line">
        {repos.map((r) => (
          <RepoSatiri
            key={r.slug}
            repo={r}
            secili={secili.has(r.slug)}
            sonuc={sonuclar.get(r.slug)}
            disabled={aktariliyor}
            onToggle={() => onToggle(r.slug)}
          />
        ))}
      </ul>
    </>
  );
}

function RepoSatiri({
  repo,
  secili,
  sonuc,
  disabled,
  onToggle,
}: {
  repo: ImportRepo;
  secili: boolean;
  sonuc?: ImportLine;
  disabled: boolean;
  onToggle: () => void;
}) {
  const etiket = repoLabel(repo.cloneUrl);
  const secilebilirMi = secilebilir(repo);

  return (
    <li className="flex items-center gap-3 py-2">
      <Checkbox
        label={repo.name}
        labelHidden
        checked={secili}
        disabled={disabled || !secilebilirMi}
        onChange={onToggle}
      />

      <IconFolder className="size-4 shrink-0 text-ink-3" />

      <div className="min-w-0 flex-1">
        <p className="truncate text-sm font-medium">{repo.name}</p>
        <p className="truncate font-mono text-xs text-ink-3">{etiket.path}</p>
      </div>

      {/* Durum tek kanal değil: rozetin metni de var (ui.md → Anlamlı renk). */}
      {repo.archived && <Badge tone="neutral">arşivli</Badge>}
      {!secilebilirMi && <Badge tone="info">zaten kayıtlı</Badge>}
      {sonuc && <SonucRozeti sonuc={sonuc} />}
    </li>
  );
}

function SonucRozeti({ sonuc }: { sonuc: ImportLine }) {
  if (sonuc.result === "created") {
    return (
      <Badge tone="success">
        <IconCheck className="mr-1 size-3" />
        eklendi
      </Badge>
    );
  }
  if (sonuc.result === "failed") {
    return (
      <Badge tone="danger" title={sonuc.reason}>
        <IconAlert className="mr-1 size-3" />
        {sonuc.reason ?? "başarısız"}
      </Badge>
    );
  }
  return <Badge tone="neutral">atlandı</Badge>;
}

/**
 * Sonuç özeti.
 *
 * Üç sayı AYRI AYRI yazılır: "51 işlendi" demek, kaçının gerçekten eklendiğini
 * gizlerdi. Başarısızlar ayrıca adıyla ve sebebiyle listelenir — sayı tek
 * başına ne yapılacağını söylemiyor.
 */
function Ozet({
  ozet,
  sonuclar,
}: {
  ozet: NonNullable<ImportLine["summary"]>;
  sonuclar: Map<string, ImportLine>;
}) {
  const basarisizlar = [...sonuclar.values()].filter((s) => s.result === "failed");
  const hicbiriYeniDegil = ozet.created === 0 && ozet.failed === 0 && ozet.skipped > 0;

  return (
    <div className="mt-4 rounded-lg border border-line bg-raised p-3">
      <p className="text-sm">
        <span className="font-medium text-ok">{ozet.created} eklendi</span>
        {" · "}
        <span className="text-ink-2">{ozet.skipped} zaten kayıtlıydı</span>
        {ozet.failed > 0 && (
          <>
            {" · "}
            <span className="font-medium text-danger">{ozet.failed} eklenemedi</span>
          </>
        )}
      </p>

      {hicbiriYeniDegil && (
        <p className="mt-1 text-xs text-ink-2">
          Bu grupta eklenecek yeni repository yok — hepsi zaten kayıtlı.
        </p>
      )}

      {basarisizlar.length > 0 && (
        <ul className="mt-2 space-y-1">
          {basarisizlar.map((s) => (
            <li key={s.slug} className="text-xs text-ink-2">
              <span className="font-mono text-ink">{s.name ?? s.slug}</span> — {s.reason}
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
