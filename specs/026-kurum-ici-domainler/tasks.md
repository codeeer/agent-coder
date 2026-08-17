# Görevler: Kurum içi domain'ler

- **Spec no:** 026 — [spec.md](spec.md) · [plan.md](plan.md)
- **Tarih:** 2026-08-17

Her görev tek oturumda biter ve gözlenebilir bir sonucu vardır.
`[P]` işaretli görevler paralel yürütülebilir.

---

## Commit (a) — Eşleştirme ve yönlendirme

- [ ] **T1** `hostlist.Listed` yazılır: boş listede `false`, dolu listede
      `Match` ile aynı desen kuralı → `go build ./...` geçer
- [ ] **T2** `Match`'in doc yorumuna boş-liste farkı ve neden iki fonksiyon
      olduğu yazılır → iki fonksiyonun ayrımı kaynaktan okunabilir
- [ ] **T3** `hostlist` testleri: `Listed` boş listede `false`, `Match` boş
      listede `true`; aynı desen kümesiyle ikisinin farklı cevap verdiği bir
      vaka → `go test ./internal/hostlist/` yeşil
- [ ] **T4** `netgate.Run`'a `Direct []hostlist.Pattern` alanı eklenir →
      `go test ./...` yeşil (alan henüz kullanılmıyor, davranış değişmez)
- [ ] **T5** Yönlendirme kararı tek bir yardımcıya alınır ve `tunel` onu
      kullanır: eşleşirse hedefe doğrudan bağlanır, `CONNECT` gönderilmez →
      mevcut netgate testleri yeşil kalır
- [ ] **T6** `duzHTTP` aynı kararı kullanır: eşleşirse taşıyıcı proxy'siz
      kurulur → mevcut testler yeşil kalır
- [ ] **T7** Test düzeneği: sahte upstream proxy + sahte hedef sunucu, hangi
      tarafın bağlantı aldığı ölçülebilir → düzenek kendi kendini doğrulayan
      bir vakayla sınanır
- [ ] **T8** Yönlendirme testleri (CONNECT): `Direct` boş → upstream;
      eşleşiyor → hedef, upstream **hiç aranmaz**; eşleşmiyor → upstream →
      `go test ./internal/netgate/` yeşil
- [ ] **T9** Aynı üç vaka düz HTTP için → yeşil
- [ ] **T10** Sıra testi: `Allow` reddediyor + hedef `Direct`'te → 403 döner,
      ne upstream ne hedef aranır → yeşil
- [ ] **T11** Geri düşme testi: doğrudan bağlantı kurulamıyor → 502 döner ve
      upstream **hiç aranmaz** → yeşil
- [ ] **T12** `go vet ./...`, `go test ./...`, `gofmt` temiz → commit (a)

## Commit (b) — Ayar ve görünürlük

- [ ] **T13** `runner.EgressSpec`'e `InternalHosts string` eklenir →
      `go build ./...` geçer
- [ ] **T14** `opencode/egress.go` ham metni desenlere çevirip
      `netgate.Run.Direct`'e taşır; ayrıştırma hatası **yutulur, loglanır** ve
      liste boş sayılır → bozuk metinle çalıştırma düşmez (test)
- [ ] **T15** `settings/registry.go`'ya yeni `Key` + `KindHostList` tanımı;
      açıklama hem "izin vermez" hem "geniş tutmak trafiği kurumsal denetimin
      dışına çıkarır" der → `go test ./internal/settings/` yeşil
- [ ] **T16** `main.go`'da ayar `Egress` closure'ına bağlanır → ayar
      `GET /api/settings` yanıtında görünür
- [ ] **T17** [P] `httpapi/egress.go` durum yanıtına çözülmüş liste eklenir →
      `GET /api/network/egress` listeyi döner
- [ ] **T18** [P] `types.ts` yanıt tipine alan eklenir → `npx tsc --noEmit`
      temiz
- [ ] **T19** [P] `EgressStatus.tsx` listeyi gösterir → çıkış durumu ekranında
      proxy'siz gidilen domain'ler görünür
- [ ] **T20** Ayarlar ekranı elle doğrulanır: liste alanı mevcut kart
      üslubunda görünür, açıklama riski söyler → ekran görüntüsü
- [ ] **T21** `npx tsc --noEmit` ve `npx eslint .` temiz → commit (b)

## Commit (c) — Belge

- [ ] **T22** README/docs'ta çıkış denetimi anlatılan yere yeni ayar ve iki
      listenin farkı ("biri izin, diğeri yol") eklenir → belge okununca
      hangisinin ne yaptığı anlaşılır
- [ ] **T23** Spec 020'nin karar geçmişine tarihli madde eklenir → kararın
      izi 020'den 026'ya sürülebilir
- [ ] **T24** commit (c)

## Doğrulama (commit'lerden sonra)

- [ ] **T25** Ayar boşken bir çalıştırma → trafik bugünkü gibi proxy'den geçer
- [ ] **T26** Listeye depo domain'i yazılır → klonlama proxy'ye uğramaz
- [ ] **T27** Aynı çalıştırmada dış bir adres (`github.com`) → proxy'den
      geçmeye devam eder
- [ ] **T28** İzin listesi doluyken, listede olmayan bir iç domain →
      reddedilir (yönlendirme izin vermiyor)

---

## Notlar

Plandan sapılırsa **neden** sapıldığı buraya yazılır.

- **T7 düzeneği bu spec'in en kritik parçası.** "Hangi tarafın bağlantı
  aldığı" ölçülemezse yönlendirme testlerinin hepsi boş iddiaya dönüşür —
  ikisi de 200 dönerse test geçer ama özellik çalışmıyor olabilir. Düzenek
  kendi kendini doğrulayan bir vakayla sınanmadan diğer testler yazılmaz.
- **T11 negatif iddia taşıyor** ("upstream hiç aranmaz"). Bunu ölçmek için
  sahte upstream'in çağrı sayacı tutulmalı; yalnızca dönen hata koduna
  bakmak yetmez, çünkü geri düşme olsaydı da 502 dönebilirdi.
- **T25-T28 gerçek bir kurumsal proxy gerektirmiyor**: sahte bir upstream
  yeterli. Gerçek kurumsal ortam provası yapılamıyorsa bu açıkça bildirilir,
  görev atlanmış sayılmaz.
- Spec'te açık kalan tek soru (doğrudan gidilen hedeflerin olay akışına
  yazılıp yazılmayacağı) **hayır** varsayımıyla planlandı. Aksi
  kararlaştırılırsa T8-T11'e bir bildirim iddiası eklenir ve spec H5'e kabul
  kriteri girer.
