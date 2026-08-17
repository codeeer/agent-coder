"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import {
  REPO_SUBDIR_KEY,
  projectDirLabel,
  scriptPath,
  truthy,
  type Script,
} from "@/lib/types";
import { IconPlus, IconTrash, IconEdit } from "@/components/ui/icons";
import { Pagination } from "@/components/ui/Pagination";
import { ScriptFolders } from "./ScriptFolders";
import {
  Badge,
  Button,
  PanelCard,
  Input,
  List,
  Mono,
  Notice,
  Panel,
  RowAction,
  SearchField,
  Select,
  Skeleton,
  Textarea,
  Well,
  formatDate,
  ConfirmInline,
} from "@/components/ui/primitives";

/**
 * Sayfa boyutu.
 *
 * 10'du, 20 oldu: yoğun satırla (~44px) yirmi kayıt yaklaşık iki ekran tutuyor,
 * eskiden on kayıt üç ekran tutuyordu. Sayfalama listeyi değil VERİTABANINI
 * koruyor; asıl bulma aracı artık arama.
 */
const PAGE_SIZE = 20;

/** Klasör süzgecinin "klasörsüz" değeri — boş bırakmak "hepsi" demek. */
const KLASORSUZ = "none";

/**
 * Betik kütüphanesi — agent'ların çalıştırabileceği hazır prosedürler.
 *
 * NEDEN VAR: model bir işi her seferinde yeniden yorumlar. Keşifte doğru olan
 * bu davranış, prosedürde (yükseltme, geçiş, kontrol listesi) risktir. Betik bir
 * kez yazılır ve her çalıştığında aynı şeyi yapar.
 *
 * EKRANIN ANA GÖREVİ ARAMAKTIR, eklemek değil: kütüphane büyüdükçe kullanıcı
 * buraya çoğunlukla "şu betiği düzenleyeceğim" diye geliyor. Bu yüzden üstte
 * araç çubuğu (arama + kampanya süzgeci), altında yoğun satırlar var; arama
 * SUNUCUDA yapılıyor, yoksa yalnızca açık sayfayı arar ve var olan bir betiğe
 * "yok" derdi.
 */
export function ScriptSection() {
  const [adding, setAdding] = useState(false);
  const [offset, setOffset] = useState(0);
  const [sorgu, setSorgu] = useState("");
  const [klasor, setKlasor] = useState("");

  const folders = useQuery({
    queryKey: ["script-folders"],
    queryFn: api.scriptFolders.list,
  });

  const scripts = useQuery({
    queryKey: ["scripts", offset, sorgu, klasor],
    queryFn: () =>
      api.scripts.list({
        limit: PAGE_SIZE,
        offset,
        q: sorgu.trim() || undefined,
        folder: klasor || undefined,
      }),
  });

  // Süzgeç değişince sayfa başa döner: üçüncü sayfadayken süzmek, sonuç varken
  // boş bir sayfa göstermek olurdu.
  const suz = (yeni: () => void) => {
    yeni();
    setOffset(0);
  };

  const suzuluyor = sorgu.trim() !== "" || klasor !== "";

  return (
    <Panel
      title="Script'ler"
      description="Agent'ların çalıştırabileceği hazır kabuk betikleri. Hangi agent'ın hangi betiği kullanabileceğini Agent'lar ekranından seçersiniz."
      action={
        !adding && (
          <Button
            variant="primary"
            icon={<IconPlus className="size-4" />}
            onClick={() => setAdding(true)}
          >
            Betik ekle
          </Button>
        )
      }
    >
      <div className="space-y-3">
        {/*
          Kampanyalar betiklerin ÜSTÜNDE değil, yanında: klasör satırına
          basmak listeyi o kampanyaya süzüyor. İki liste artık birbirine
          bağlı — eskiden aynı kampanyanın adımları iki ayrı yerde duruyordu.
        */}
        <ScriptFolders
          seciliKlasor={klasor}
          onSelect={(id) => suz(() => setKlasor(id === klasor ? "" : id))}
        />

        <div className="flex flex-wrap items-center gap-2 border-t border-line pt-3">
          <div className="min-w-52 flex-1">
            <SearchField
              value={sorgu}
              placeholder="Betik ara — ad veya açıklama"
              aria-label="Betik ara"
              onChange={(e) => suz(() => setSorgu(e.target.value))}
            />
          </div>

          <Select
            className="w-52"
            aria-label="Kampanya süzgeci"
            value={klasor}
            onChange={(e) => suz(() => setKlasor(e.target.value))}
          >
            <option value="">Tüm betikler</option>
            <option value={KLASORSUZ}>Klasörsüz (ortak)</option>
            {folders.data?.items.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name}
              </option>
            ))}
          </Select>

          {suzuluyor && (
            <Button size="sm" variant="ghost" onClick={() => suz(() => { setSorgu(""); setKlasor(""); })}>
              Süzgeci temizle
            </Button>
          )}
        </div>

        {adding && <ScriptForm onDone={() => setAdding(false)} />}

        {scripts.isPending && <Skeleton rows={2} />}
        {scripts.isError && (
          <Notice tone="error">{describeError(scripts.error).message}</Notice>
        )}

        {/*
          İki ayrı boşluk, iki ayrı cümle: hiç betik olmaması ile aramanın
          sonuç vermemesi aynı şey değil. İkisine de "henüz betik yok" demek,
          süzgeci açık unutmuş kullanıcıya kütüphanesinin silindiğini
          düşündürürdü.
        */}
        {scripts.data?.total === 0 && !adding && (
          <PanelCard>
            {suzuluyor ? (
              <p className="text-sm text-ink-2">
                Bu süzgece uyan betik yok.{" "}
                <button
                  className="underline underline-offset-2 hover:text-ink"
                  onClick={() => suz(() => { setSorgu(""); setKlasor(""); })}
                >
                  Süzgeci temizle
                </button>
              </p>
            ) : (
              <p className="text-sm text-ink-2">
                Henüz betik yok. Bir agent standart bir işi &mdash; bağımlılık
                yükseltme, geçiş uygulama, kontrol listesi &mdash; her seferinde
                biraz farklı yapabilir. Betik bunu sabitler:{" "}
                <strong>model ne zaman çağıracağına karar verir, ne yapacağına
                betik karar verir.</strong>
              </p>
            )}
          </PanelCard>
        )}

        {scripts.data && scripts.data.items.length > 0 && (
          <List>
            {scripts.data.items.map((s) => (
              <ScriptRow key={s.id} script={s} />
            ))}
          </List>
        )}

        {scripts.data && (
          <Pagination
            total={scripts.data.total}
            limit={scripts.data.limit}
            offset={scripts.data.offset}
            onChange={setOffset}
            unit="betik"
          />
        )}

        {/* Sınırın kendisi bir bilgi: kullanıcı betiğini yazıp neden hiç
            çalışmadığını aramasın. */}
        <p className="text-xs text-ink-3">
          Betikler yalnızca <strong>komut çalıştırma yetkisi açık</strong>{" "}
          agent&apos;lara verilir. Yetkisi kapalı bir agent&apos;ın ortamına
          kopyalanmazlar.
        </p>
      </div>
    </Panel>
  );
}

/**
 * Tek satır — eski kartın üçte biri kadar yer tutar.
 *
 * Ne gitti: "Güncellendi" satırı üstveriye indi (yol ile aynı satırda), iki
 * düğme hover'a çekildi. Her satırda duran bir "Sil", listenin asıl işinin
 * (bulmak) önüne geçiyordu.
 */
function ScriptRow({ script }: { script: Script }) {
  const qc = useQueryClient();
  const [editing, setEditing] = useState(false);
  const [confirming, setConfirming] = useState(false);

  const remove = useMutation({
    mutationFn: () => api.scripts.remove(script.id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      void qc.invalidateQueries({ queryKey: ["script-folders"] });
      void qc.invalidateQueries({ queryKey: ["agents"] });
    },
  });

  if (editing) {
    return (
      <div className="p-3">
        <ScriptForm script={script} onDone={() => setEditing(false)} />
      </div>
    );
  }

  return (
    <div className="group px-4 py-2.5">
      <div className="flex items-start gap-3">
        {/*
          İKİ SATIR, üç değil: ad + kampanya üstte, açıklama ve üstveri altta.
          Üçüncü satır (ayrı duran yol) satır yüksekliğini 76px'e çıkarıyordu;
          aynı bilgi burada 52px'e sığıyor ve ekranda iki katı satır görünüyor.

          Yol GİZLENMİYOR, sağa alınıyor: kullanıcı agent talimatında ona atıfta
          bulunuyor. Dar ekranda kırpılır, tamamı `title`da durur.
        */}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-medium">{script.name}</span>
            {script.folderName && <Badge tone="neutral">{script.folderName}</Badge>}
          </div>

          {/*
            ÖNCELİK AÇIKLAMADA: yol `shrink-0` iken uzun bir yol açıklamayı
            sıfıra sıkıştırıyordu — üstveri, ikincil bilgiyi ekrandan siliyordu.
            İkisi de kırpılabilir; açıklama `flex-1` ile önce yer alır.
          */}
          <div className="mt-0.5 flex items-center gap-2 text-2xs">
            <span className="min-w-0 flex-1 truncate text-ink-2" title={script.description}>
              {script.description || "açıklama yok"}
            </span>
            <span
              /*
                ÜST SINIR ŞART: `flex-1`in temeli 0 olduğu için, sınırsız bir
                yol yanındaki açıklamayı sıfıra indiriyordu — ölçüldü, beş
                satırda açıklama genişliği 0px'ti. Yol satırın en fazla
                %45'ini alır, gerisi açıklamanın.
              */
              className="hidden max-w-[45%] min-w-0 shrink truncate font-mono text-ink-3 md:inline"
              title={scriptPath(script)}
            >
              {scriptPath(script)}
            </span>
            <span className="shrink-0 text-ink-3">{formatDate(script.updatedAt)}</span>
          </div>
        </div>

        {!confirming && (
          <RowAction className="gap-1.5">
            <Button size="sm" icon={<IconEdit />} onClick={() => setEditing(true)}>
              Düzenle
            </Button>
            <Button
              size="sm"
              variant="danger"
              icon={<IconTrash />}
              onClick={() => setConfirming(true)}
            >
              Sil
            </Button>
          </RowAction>
        )}
      </div>

      {confirming && (
        <div className="mt-2">
          <ConfirmInline
            question={
              <>
                <strong>{script.name}</strong> silinsin mi?
              </>
            }
            consequence="Agent'lardan da kaldırılacak."
            busy={remove.isPending}
            onConfirm={() => remove.mutate()}
            onCancel={() => setConfirming(false)}
          />
        </div>
      )}

      {remove.isError && (
        <div className="mt-2">
          <Notice tone="error">{describeError(remove.error).message}</Notice>
        </div>
      )}
    </div>
  );
}

function ScriptForm({ script, onDone }: { script?: Script; onDone: () => void }) {
  const qc = useQueryClient();
  const editing = script !== undefined;

  const [name, setName] = useState(script?.name ?? "");
  const [description, setDescription] = useState(script?.description ?? "");
  const [folderId, setFolderId] = useState(script?.folderId ?? "");
  const [content, setContent] = useState(
    script?.content ?? `#!/usr/bin/env bash\nset -euo pipefail\n\ncd "$PROJECT_DIR"\n`,
  );

  const folders = useQuery({
    queryKey: ["script-folders"],
    queryFn: api.scriptFolders.list,
  });

  /*
   * Proje kökü ARTIK SABİT DEĞİL (spec 025). Betik yazarına gösterilen yol
   * yerleşim ayarına bağlı; sabit yazılsaydı ayar açıkken kullanıcıya var
   * olmayan bir dizin söylenirdi.
   *
   * BEKLEMEDEN GÖSTERİLMEZ: bu sekmeye doğrudan girildiğinde sorgu soğuk
   * başlıyor (ayar bölümü ayrı sekmede). Yükleme sırasında varsayılana
   * düşseydi ekran önce `/work` der, sonra sessizce değişirdi — ve o ilk
   * hali okuyup betiğine yazan kullanıcı yanlış yolu sabitlerdi.
   */
  const settings = useQuery({
    queryKey: ["settings"],
    queryFn: api.settings.list,
  });
  const repoAltKlasoru = truthy(
    settings.data?.items.find((s) => s.key === REPO_SUBDIR_KEY)?.value ?? "",
  );

  const secilenKlasor = folders.data?.items.find((f) => f.id === folderId);

  const save = useMutation({
    mutationFn: () =>
      editing
        ? api.scripts.update(script.id, {
            name: name.trim(),
            description: description.trim(),
            content,
            folderId: folderId || undefined,
            // Klasörden çıkarmak ile "dokunma" JSON'da aynı görünüyor;
            // ayrı bir bayrak olmadan ayırt edilemez.
            clearFolder: folderId === "",
          })
        : api.scripts.create({
            name: name.trim(),
            description: description.trim(),
            content,
            folderId: folderId || undefined,
          }),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ["scripts"] });
      void qc.invalidateQueries({ queryKey: ["script-folders"] });
      onDone();
    },
  });

  return (
    <PanelCard>
      <p className="text-sm font-medium">
        {editing ? "Betiği düzenle" : "Yeni betik"}
      </p>

      <div className="mt-3 grid gap-3 sm:grid-cols-2">
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Ad</span>
          <Input
            className="mt-1"
            value={name}
            placeholder="upgrade-deps"
            onChange={(e) => setName(e.target.value)}
          />
          {/* Adın dosya adına dönüştüğünü söylemek gerekiyor: kullanıcı neden
              boşluk ve büyük harf kabul edilmediğini yoksa anlamaz. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Dosya adı olur:{" "}
            <span className="font-mono break-all">
              {scriptPath({
                name: name.trim() || "upgrade-deps",
                folderName: secilenKlasor?.name,
              })}
            </span>
            . Küçük harf, rakam ve - kullanılabilir.
          </span>
        </label>

        {/* Klasör seçimi: kampanyaya ait betikler bir arada dursun.
            Klasörsüz bırakmak MEŞRU — birden fazla kampanyada işe yarayan
            ortak betikler oraya kopyalanmamalı. */}
        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">
            Klasör
          </span>
          <Select
            className="mt-1"
            value={folderId}
            onChange={(e) => setFolderId(e.target.value)}
          >
            <option value="">Klasörsüz (ortak betik)</option>
            {folders.data?.items.map((f) => (
              <option key={f.id} value={f.id}>
                {f.name}
              </option>
            ))}
          </Select>
          <span className="mt-1 block text-2xs text-ink-3">
            Bir kampanyanın adımları klasörde toplanır ve agent&apos;a tek
            seçimle bağlanır.
          </span>
        </label>

        <label className="block">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">
            Ne işe yarar
          </span>
          <Input
            className="mt-1"
            value={description}
            placeholder="Bağımlılıkları güvenli sürümlere yükseltir"
            onChange={(e) => setDescription(e.target.value)}
          />
          {/* Açıklama süs değil: agent'ın talimatına yazılıyor ve betiğin ne
              zaman çağrılacağını modele anlatan tek ipucu bu. */}
          <span className="mt-1 block text-2xs text-ink-3">
            Agent&apos;ın talimatına yazılır — betiği <strong>ne zaman</strong>{" "}
            çağıracağını buradan anlar.
          </span>
        </label>
      </div>

      <label className="mt-3 block">
        <span className="text-2xs tracking-wide text-ink-2 uppercase">İçerik</span>
        <Textarea
          className="mt-1 min-h-56 font-mono text-xs leading-relaxed"
          value={content}
          spellCheck={false}
          onChange={(e) => setContent(e.target.value)}
        />
      </label>

      <Well className="mt-3 p-3">
        <p className="text-xs">
          <strong>Betiğe gizli değer yazmayın.</strong> Betikler şifrelenmez ve
          agent onları okuyabilir. Token gerekiyorsa ortam değişkeninden okuyun:{" "}
          <Mono>&quot;$GIT_TOKEN&quot;</Mono>
        </p>
        <p className="mt-2 text-2xs text-ink-2">
          Hata durumunda durması için <Mono>set -euo pipefail</Mono> önerilir.
        </p>
      </Well>

      {/* PROJE DİZİNİ — betik yazarının bilmediği tek şey bu.
          Kullanıcı projesinin İÇİNDEKİ yolu biliyor; kökün nereye açıldığını
          kaynağı okumadan öğrenemezdi.

          Yol ayardan geldiği için ayar OKUNANA KADAR hiç yazılmıyor: yanlış
          bir yol göstermek, hiç göstermemekten kötü. */}
      <Well className="mt-3 p-3">
        <p className="text-xs">
          {settings.isSuccess ? (
            <>
              <strong>
                Proje <Mono>{projectDirLabel(repoAltKlasoru)}</Mono> altına klonlanır
              </strong>{" "}
              ve bu yol betiğe <Mono>$PROJECT_DIR</Mono> olarak geçer.
            </>
          ) : (
            <>
              <strong>Projenin klonlandığı yol</strong> betiğe{" "}
              <Mono>$PROJECT_DIR</Mono> olarak geçer.
            </>
          )}{" "}
          Çalışma dizinine güvenmeyin, bu değişkeni kullanın:{" "}
          <Mono>&quot;$PROJECT_DIR/config/webpack.config.js&quot;</Mono>
        </p>
        <p className="mt-2 text-2xs text-ink-2">
          Kalıcı olması gereken her değişiklik bu dizinin altında olmalı —
          başka bir yere yazılan dosya değişiklik kaydına girmez ve
          çalıştırma bitince kaybolur.
        </p>
      </Well>

      {save.isError && <Notice tone="error">{describeError(save.error).message}</Notice>}

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          variant="primary"
          onClick={() => save.mutate()}
          disabled={save.isPending || !name.trim() || !content.trim()}
        >
          {save.isPending ? "Kaydediliyor…" : "Kaydet"}
        </Button>
        <Button onClick={onDone} disabled={save.isPending}>
          Vazgeç
        </Button>
        <span className="text-xs text-ink-3">
          Değişiklik bir sonraki çalıştırmada geçerli olur.
        </span>
      </div>
    </PanelCard>
  );
}
