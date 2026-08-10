---
name: spec-driven
description: Bu projede yeni bir özelliğe başlarken, spec/plan/tasks dosyalarını yazarken veya "şunu ekleyelim / şu özelliği yapalım" denildiğinde kullanılır. specs/NNN-ozellik/ altındaki spec.md → plan.md → tasks.md sırasını ve her dosyanın kurallarını tanımlar.
---

# Spec-Driven Geliştirme

Bu projede kod, üç aşamalı bir kapıdan geçmeden yazılmaz:

```
spec.md   NE + NEDEN     → onay → 
plan.md   NASIL          → onay → 
tasks.md  hangi sırayla  → onay → kod
```

Şablonlar: [specs/000-template/](../../../specs/000-template/)

## Yeni özelliğe başlama

1. Sıradaki numarayı bul: `ls specs/` → en büyük NNN + 1
2. `specs/NNN-kisa-kebab-ad/` klasörünü aç
3. `specs/000-template/spec.md`'yi kopyala ve doldur
4. **Dur ve kullanıcıya onaylat.** `plan.md`'e kendi kendine geçme.

Klasör adı kısa ve kebab-case: `001-workflow-engine`, `002-jira-trigger`.

## spec.md — NE ve NEDEN

Yazılır: problem, amaç, kullanıcı hikâyeleri, kabul kriterleri, hata durumları, kapsam dışı.

**Yazılmaz:** teknoloji adı, kütüphane, dosya yolu, tablo adı, fonksiyon imzası.

Test: Teknik olmayan biri okuyup neyin yapılacağını anlıyor mu? Hayır ise fazla teknik.

Kabul kriterleri gözlenebilir olmalı:

- ✅ "Kullanıcı bir node'a tıkladığında sağ panelde o node'un modeli görünür"
- ❌ "Node inspector düzgün çalışır"

Belirsizlik varsa `## Belirsizlikler` altına soru olarak yaz ve **kullanıcıya sor**.
Cevaplanmamış soru varken `plan.md`'e geçme.

## plan.md — NASIL

Yazılır: seçilen yaklaşım, elenen alternatifler ve gerekçeleri, veri modeli + migration,
Go/TS arayüzleri, HTTP endpoint'leri, değişecek dosyalar, riskler, test stratejisi,
uygulama sırası.

İki kural:

- Her başlık `spec.md`'deki bir hikâyeye dayanmalı. Dayanmıyorsa ya spec eksik ya kapsam kayması var.
- **Yeniden kullanılacak mevcut kod** bölümünü doldurmadan geçme. Yazmadan önce ara:
  benzer bir şey `backend/internal/` altında zaten var mı?

Riskli parçalar uygulama sırasında **başa** alınır. (Örnek: Faz 2'de runner sandbox'ının
workflow motorundan önce doğrulanması.)

## tasks.md — Sıra

Her görev iki şartı sağlar:

1. Tek oturumda bitirilebilir
2. Gözlenebilir bir sonucu var

Format: `- [ ] T20 <ne yapılacak> → <nasıl doğrulanır>`

- ✅ `T20 POST /api/workflows endpoint'i eklenir → curl ile 201 ve gövdede id döner`
- ❌ `T20 Workflow API'si yazılır`

Paralel çalışılabilenler `[P]` ile işaretlenir.

## Uygulama sırasında

- Görev bitince `[x]` yap, satır sonuna kısa not düş.
- **Plandan sapıldıysa** `tasks.md` içindeki `## Notlar` bölümüne **neden** sapıldığını yaz.
  Sessizce sapma — plan yanlışsa plan güncellenir.
- Yeni komut veya konvansiyon ortaya çıktıysa [AGENTS.md](../../../AGENTS.md)'i güncelle.

## Kapanış

`spec.md` kabul kriterlerinin tamamı elle doğrulandıktan sonra `spec.md` durumunu
"Uygulandı" yap. Kriterlerden biri karşılanmadıysa özellik bitmemiştir — kısmen
bitirip "tamam" deme, neyin eksik kaldığını açıkça söyle.

## Mimari kararlar

Bir özelliğin ötesinde, sistem geneline etki eden kararlar `specs/` değil
`plans/NN-konu-YYYY-AA-GG.md` altına yazılır. Numara artan sırada, tarih dosya adında.
