import { matchesAny, needle } from "../../lib/search.ts";
import type { Project, RunBatchItem } from "@/lib/types";

/**
 * Toplu çalıştırmada proje seçimi ve toplu iş özetinin kuralları (spec 023).
 *
 * Bileşenin içine gömülmüyor: "otuz projeden hangileri seçili" ve "devam
 * düğmesi çıkmalı mı" soruları gözle doğrulanamaz; ancak kendi modülünde test
 * edilebilir (AGENTS.md → TypeScript konvansiyonları).
 */

/**
 * Arama süzgeci — ad ve repository adresi üzerinde.
 *
 * Adres de aranır çünkü aynı adı taşıyan iki proje farklı depolara bakabilir
 * ve kullanıcı hangisini seçtiğini ancak adresten ayırt eder.
 */
export function projeAra(projects: Project[], sorgu: string): Project[] {
  const n = needle(sorgu);
  if (n === "") return projects;
  return projects.filter((p) => matchesAny([p.name, p.repoUrl], n));
}

/** Tek bir projenin seçimini açar ya da kapatır. */
export function secimAcKapa(secili: Set<string>, id: string): Set<string> {
  const yeni = new Set(secili);
  if (!yeni.delete(id)) yeni.add(id);
  return yeni;
}

/**
 * "Tümünü seç" — YALNIZCA görünen (süzülmüş) projeleri alır.
 *
 * Süzgecin dışındakileri de alsaydı kullanıcı gördüğü altı satırı seçtiğini
 * sanır, otuz proje sıraya girerdi. Zaten seçili olanlar korunur: arama
 * değiştirip tekrar basmak önceki seçimi silmemeli.
 */
export function tumunuSec(secili: Set<string>, gorunen: Project[]): Set<string> {
  const yeni = new Set(secili);
  for (const p of gorunen) yeni.add(p.id);
  return yeni;
}

/** "Seçimi temizle" — yalnızca görünenleri bırakır, gizlileri korur. */
export function gorunenleriBirak(secili: Set<string>, gorunen: Project[]): Set<string> {
  const yeni = new Set(secili);
  for (const p of gorunen) yeni.delete(p.id);
  return yeni;
}

/**
 * Başlatma düğmesinin metni — KAÇ İŞİN sıraya alınacağını söyler.
 *
 * "Başlat" tek başına kaç projenin koşacağını söylemez; otuz projelik bir
 * kampanyada bu, kullanıcının bilmeden başlattığı bir maliyet olurdu.
 */
export function baslatEtiketi(secilenSayisi: number): string {
  if (secilenSayisi === 0) return "Proje seçin";
  return `${secilenSayisi} projede çalıştır`;
}

/**
 * "Kaldığı yerden devam et" düğmesi çıkmalı mı, çıkacaksa üzerinde ne yazacak.
 *
 * Yalnızca KESİLMİŞ öğe varken çıkar (spec 023 H5a): başarısız olanlar çalıştı
 * ve bir sonuç üretti, onları kendiliğinden tekrarlamak yan etkilerini
 * habersizce tekrarlamak olurdu.
 *
 * Sayı düğmenin ÜZERİNDE yazar — kullanıcı basmadan önce ne olacağını bilmeli.
 */
export function devamEtiketi(kesilen: number): string | null {
  if (kesilen <= 0) return null;
  return `Kaldığı yerden devam et (${kesilen} iş)`;
}

/**
 * İptalin SONUCU — onaydan önce yazılır.
 *
 * "Emin misiniz?" hiçbir şey söylemez. Ne olacağı sayıyla yazılır ve çalışan
 * işlerin SÜRECEĞİ açıkça söylenir; kullanıcı "iptal" deyince her şeyin
 * duracağını sanmamalı.
 */
export function iptalSonucu(bekleyen: number, calisan: number): string {
  const parcalar: string[] = [];
  parcalar.push(
    bekleyen > 0 ? `${bekleyen} bekleyen iş düşer` : "Düşecek bekleyen iş yok",
  );
  if (calisan > 0) {
    parcalar.push(
      `${calisan} çalışan iş kendi hâlinde sürer ve sonucu kaydedilir`,
    );
  }
  return `${parcalar.join("; ")}.`;
}

/**
 * Öğenin kendi akış çalışmasının adresi — yoksa null.
 *
 * Başlatılmamış bir öğenin çalışması yoktur; satır o zaman bağlantı DEĞİL düz
 * metin olur. Boş bir adrese bağlanan satır, tıklayınca hiçbir şey bulunmayan
 * bir sayfaya götürürdü.
 */
export function ogeCalismaYolu(
  workflowId: string,
  item: RunBatchItem,
  // Dönüş tipi düz `string` DEĞİL şablon: Next'in tipli yolları (`typedRoutes`)
  // `href` için gerçek yol biçimini istiyor ve düz metni kabul etmiyor.
): `/workflows/${string}/runs/${string}` | null {
  if (!item.workflowRunId) return null;
  return `/workflows/${workflowId}/runs/${item.workflowRunId}`;
}
