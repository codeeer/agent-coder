# Görevler: <Özellik Adı>

- **Spec no:** NNN — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Başlamadı | Devam ediyor | Tamamlandı

> **Bu dosyanın kuralı:** `plan.md` onaylanmadan yazılmaz. Her görev
> **tek oturumda bitirilebilir** ve **gözlenebilir bir sonuç** üretir.
> "Backend'i yaz" görev değildir; "`POST /api/x` endpoint'i 201 dönsün" görevdir.
> Tamamlananları `[x]` yapın, tamamlanma notunu satırın sonuna ekleyin.

Görev formatı: `- [ ] T<no> <ne yapılacak> → <nasıl doğrulanır>`
Paralel çalışılabilecek görevler `[P]` ile işaretlenir.

---

## Hazırlık

- [ ] T01 ... → ...

## Veri katmanı

- [ ] T10 Migration `NNNNNN_ad.sql` yazılır → `make migrate` hatasız, tablo `\dt` ile görülür
- [ ] T11 sqlc sorguları eklenir → `make sqlc` üretim hatasız derlenir

## Backend

- [ ] T20 ... → ...

## Frontend

- [ ] T30 ... → ...

## Testler

- [ ] T40 ... → `make test` yeşil

## Doğrulama ve kapanış

- [ ] T90 `spec.md` kabul kriterlerinin tamamı elle doğrulanır
- [ ] T91 `spec.md` durumu "Uygulandı" yapılır
- [ ] T92 `AGENTS.md` gerekiyorsa güncellenir (yeni komut, yeni konvansiyon)

---

## Notlar

Uygulama sırasında ortaya çıkan sapmalar, kararlar ve nedenleri.
Plandan sapıldıysa **neden** sapıldığı buraya yazılır.
