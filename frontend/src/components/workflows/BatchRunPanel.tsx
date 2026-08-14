"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { api } from "@/lib/api";
import { describeError } from "@/lib/errors";
import type { Project } from "@/lib/types";
import {
  Badge,
  Button,
  Card,
  Checkbox,
  Input,
  Notice,
  SearchField,
} from "@/components/ui/primitives";
import {
  baslatEtiketi,
  gorunenleriBirak,
  projeAra,
  secimAcKapa,
  tumunuSec,
} from "./batch-selection";

/**
 * Bir akışı çok projede sıraya koyma (spec 023 H1).
 *
 * TEK ÇALIŞTIRMANIN YANINDA DURUR, onun yerine geçmez: bir projede denemek ile
 * otuz projede kampanya yürütmek aynı karar değil. Kullanıcı önce birinde
 * dener, sonra buraya gelir.
 *
 * Seçim mantığı bileşenin içinde değil `batch-selection.ts` içinde: otuz
 * projelik bir listede "tümünü seç gerçekten ne seçti" sorusu gözle
 * doğrulanamaz.
 */
export function BatchRunPanel({
  workflowId,
  projects,
  disabled,
  disabledReason,
}: {
  workflowId: string;
  projects: Project[];
  /** Akışın kayıtlı tanımı yoksa ya da kaydedilmemiş değişiklik varsa. */
  disabled: boolean;
  disabledReason?: string;
}) {
  const router = useRouter();
  const qc = useQueryClient();

  const [task, setTask] = useState("");
  const [sorgu, setSorgu] = useState("");
  const [secili, setSecili] = useState<Set<string>>(new Set());

  const gorunen = projeAra(projects, sorgu);
  const secilenSayisi = secili.size;

  const baslat = useMutation({
    mutationFn: () =>
      api.runBatches.create({
        workflowId,
        task: task.trim(),
        projectIds: [...secili],
      }),
    onSuccess: (batch) => {
      void qc.invalidateQueries({ queryKey: ["run-batches"] });
      // Kullanıcı doğrudan ilerleme ekranına geçer: otuz işin durumu ancak
      // orada tek ekranda görünür.
      router.push(`/run-batches/${batch.id}`);
    },
  });

  return (
    <Card>
      <div className="flex flex-wrap items-end gap-3">
        <label className="block min-w-64 flex-1">
          <span className="text-2xs tracking-wide text-ink-2 uppercase">Görev</span>
          <Input
            className="mt-1"
            value={task}
            placeholder="Örn: Node 18'den 24'e yükselt"
            onChange={(e) => setTask(e.target.value)}
          />
        </label>
      </div>

      {/*
        Görev metni toplu işin KENDİSİNDE: otuz öğe aynı işi yapıyor. Proje
        başına ayrı metin, otuz kutu doldurmak demek olurdu.
      */}
      <p className="mt-1.5 text-2xs text-ink-3">
        Aynı görev metni seçilen bütün projelerde çalışır.
      </p>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <div className="min-w-56 flex-1">
          <SearchField
            value={sorgu}
            placeholder="Proje ara — ad veya adres"
            aria-label="Proje ara"
            onChange={(e) => setSorgu(e.target.value)}
          />
        </div>

        {/*
          İki ikincil eylem: kenarlıklı, dolu değil. Ekranın tek birincil
          eylemi başlatma düğmesi (ui.md → eylem hiyerarşisi).
        */}
        <Button size="sm" onClick={() => setSecili(tumunuSec(secili, gorunen))}>
          {sorgu.trim() === "" ? "Tümünü seç" : `Görünen ${gorunen.length} projeyi seç`}
        </Button>
        <Button
          size="sm"
          variant="ghost"
          disabled={secilenSayisi === 0}
          onClick={() => setSecili(gorunenleriBirak(secili, gorunen))}
        >
          Seçimi temizle
        </Button>

        <Badge tone={secilenSayisi > 0 ? "accent" : "neutral"}>
          {secilenSayisi} / {projects.length} seçili
        </Badge>
      </div>

      {projects.length === 0 ? (
        <div className="mt-3">
          <Notice tone="warning">Tanımlı proje yok. Projeler ekranından ekleyin.</Notice>
        </div>
      ) : (
        <div className="mt-3 max-h-72 overflow-y-auto rounded-card border border-line">
          {gorunen.length === 0 ? (
            <p className="px-3 py-6 text-center text-sm text-ink-3">
              Aramaya uyan proje yok.
            </p>
          ) : (
            <ul className="divide-y divide-line">
              {gorunen.map((p) => (
                <li key={p.id}>
                  <div className="flex items-center gap-3 px-3 py-2 transition-colors hover:bg-raised">
                    <Checkbox
                      label={p.name}
                      labelHidden
                      checked={secili.has(p.id)}
                      onChange={() => setSecili(secimAcKapa(secili, p.id))}
                    />
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm">{p.name}</div>
                      <div className="truncate text-2xs text-ink-3">{p.repoUrl}</div>
                    </div>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </div>
      )}

      {disabled && disabledReason && (
        <div className="mt-3">
          <Notice tone="warning">{disabledReason}</Notice>
        </div>
      )}

      <div className="mt-4 flex items-center gap-3">
        <Button
          variant="primary"
          disabled={disabled || secilenSayisi === 0 || baslat.isPending}
          onClick={() => baslat.mutate()}
        >
          {baslat.isPending ? "Sıraya alınıyor…" : baslatEtiketi(secilenSayisi)}
        </Button>
        {/*
          Kuyruğun ne yapacağı ÖNCEDEN yazılır: kullanıcı otuz işin aynı anda
          başlamayacağını, sırayla koşacağını bilerek başlatmalı.
        */}
        <p className="text-xs text-ink-3">
          İşler eşzamanlılık sınırı kadar paralel çalışır, kalanı sırada bekler.
        </p>
      </div>

      {baslat.isError && (
        <div className="mt-3">
          <Notice tone="error" title={describeError(baslat.error).message}>
            {describeError(baslat.error).hint}
          </Notice>
        </div>
      )}
    </Card>
  );
}
