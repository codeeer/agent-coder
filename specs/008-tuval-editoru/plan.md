# Plan: Tuval Editörü

- **Spec no:** 008 — [spec.md](spec.md)
- **Durum:** Uygulandı

---

## Backend değişikliği: YOK

Graf modeli Faz 3'te tuval düşünülerek kuruldu. Düğüm `position` alanı zaten
saklanıyor, tetikleyici zaten düğüm olarak modelleniyor, doğrulama zaten
dallanmayı ve birleşmeyi kabul ediyor, motor zaten paralel çalıştırıyor.

Bu faz **yalnızca arayüz**. Backend'e dokunulmayacak olması, Faz 3'teki
"pozisyonları şimdiden sakla" kararının karşılığı.

## Bağımlılık

`@xyflow/react` (React Flow) — K1. Tek yeni paket.

Docker imajı `npm ci` ile kuruyor; `package-lock.json` güncellenecek. Kütüphane
runtime bağımlılığı olduğu için üretim imajına da giriyor (~50 KB gzip).

## Dosya düzeni

```
components/flow/
  FlowCanvas.tsx      tuvalin kendisi (düzenleme ve izleme modları)
  AgentNode.tsx       agent düğümü — ad, agent, model, durum
  TriggerNode.tsx     tetikleyici düğümü
  NodeInspector.tsx   sağ panel: agent + model + talimat
  layout.ts           konumu olmayan düğümleri yerleştirir (saf fonksiyon)
  theme.ts            React Flow değişkenlerini projenin jetonlarına bağlar
lib/workflow-graph.ts genel graf ↔ tuval dönüşümü (doğrusal varsayım kalkıyor)
```

`lib/workflow-graph.ts` içindeki `stepsToGraph`/`graphToSteps` **doğrusal zincir
varsayıyordu** — K2 ile o editör kalktığı için yerini genel dönüşüm alıyor.
`makeStepId` kalıyor: düğüm kimliği hâlâ addan türetiliyor ve sabit kalıyor,
çünkü şablon referansları ona bakıyor.

## Otomatik yerleşim — `layout.ts`

Konumu olmayan (veya hepsi 0,0 olan) düğümler yerleştirilir:

- **Sütun = seviye.** Motorun `Levels()` hesabıyla aynı mantık: aynı seviyedeki
  düğümler aynı sütunda, birbirinden bağımsız oldukları için.
- **Satır = seviye içindeki sıra.**
- Soldan sağa akış: tetikleyici en solda.

Saf fonksiyon, React'tan bağımsız, test edilebilir. Faz 3'te öğrenilen kural:
mantık bileşenin içine gömülmez.

Yerleşim **yalnızca konum yoksa** çalışır — kullanıcının taşıdığı düğüm bir daha
yerinden oynamaz.

## Doğrulama geri bildirimi

Backend zaten düğüm bazında kusur döndürüyor (`problems[].nodeId`). Tuval bunu
düğümün kendisine bağlar: kusurlu düğüm kırmızı çerçeve alır, üzerine gelince
mesaj görünür, sağ panelde de listelenir.

Döngü çizilmesi React Flow'un `isValidConnection` kancasıyla **çizim anında**
engellenir — kaydetmeye kadar beklemek, kullanıcıyı yaptığı işi geri almaya
zorlamak olurdu. Yine de backend doğrulaması son söz: arayüz bir şeyi kaçırırsa
kayıt reddedilir.

## İzleme modu

Aynı `FlowCanvas`, `mode="run"` ile:

- Düğümler **salt okunur** (sürüklenmez, bağ çekilmez)
- Her düğüm adım durumunu taşır: bekleyen soluk, çalışan nabızlı çerçeve, biten
  yeşil onay, hatalı kırmızı, atlanan çizgili/soluk
- Durum **yalnızca renkle** anlatılmaz: her düğümde durum etiketi de var
  (spec 008 kabul kriteri; projenin genel kuralı)
- Canlı akış Faz 3'teki gibi: SSE "değişti" der, kayıt yeniden çekilir, tuval
  yeni duruma boyanır

## Tema

React Flow kendi CSS değişkenlerini kullanıyor; bunlar projenin jetonlarına
bağlanacak (`theme.ts` + bir CSS bloğu). Arka plan noktaları, bağ rengi, seçim
halkası, minimap — hepsi `--color-*` jetonlarından beslenecek ki tema anahtarı
tuvali de çevirsin.

Bu, spec 006'da öğrenilenin doğrudan uygulaması: bir renk sabiti CSS'e gömülmez.

## Riskler

| Risk | Önlem |
|---|---|
| Kütüphane teması bizim jetonlarımızla çakışır | Değişkenler tek dosyada eşlenir; iki temada da ekran görüntüsüyle bakılır |
| Eski akışlar üst üste yığılır | `layout.ts` otomatik yerleşim; testi var |
| Tuval klavyeyle kullanılamaz | React Flow'un klavye desteği açık bırakılır; düğüm bileşenleri odaklanabilir olur |
| Kaydedilmemiş değişiklik kaybolur | "Kaydedilmedi" uyarısı + sayfadan ayrılırken onay |
| Büyük akışta başarım düşer | Düğüm bileşenleri `memo`; bu fazda 20-30 düğüm hedef |

## Doğrulama

1. Dallanan-birleşen akış tuvalden kurulur, kaydedilir, **paralel çalışır**
   (zaman damgaları örtüşür)
2. Konumlar kaydedilir; sayfa yenilenince düğümler aynı yerde
3. Döngü çizilmeye çalışılınca engellenir
4. Geçersiz akış kaydedilince kusur ilgili düğümde görünür
5. Çalışan akış tuvalde canlı izlenir; yenilemede durum doğru
6. Konumsuz eski akış açılınca düzgün yerleşir
7. Açık ve koyu temada ekran görüntüsü alınır
8. Klavyeyle düğüm seçilip silinebilir
