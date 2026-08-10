# Görevler: Betikler

- **Spec no:** 012 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Uygulandı

---

## Kütüphane

- [x] T01 `internal/scripts/script.go` — `Script` tipi, ad doğrulama
      (`[a-z0-9-]`), `FileName()`, `Path()`, sentinel hatalar
- [x] T02 Migration `000011_betikler.sql` — `scripts` + `agent_scripts`
- [x] T03 `internal/scripts/store.go` — `List/Get/Create/Update/Delete` +
      `ForAgent` + `SetAgentScripts`
- [x] T04 Testler: ad doğrulama, aynı ad ikinci kez, atama, satır sonu temizliği

## Çalıştırmaya bağlama

- [x] T10 `runner.AgentSpec.Scripts`
- [x] T11 `BuildConfigFiles` → `/home/agent/scripts/<ad>.sh`, `Mode 0o755`
- [x] T12 `buildAgentFile` → "Kullanabileceğin betikler" bloğu (yalnızca betik
      varken)
- [x] T13 `runner/Dockerfile` → `mkdir -p /home/agent/scripts`
- [x] T14 `runbuild.Builder` agent'ın betiklerini çözüp aktarır
- [x] T15 **Testler:** `AllowBash` kapalıyken dosya üretilmez · yol `/work`
      dışında · mod `0o755` · `BuildPermissions` değişmemiş

## Arayüz

- [x] T20 `httpapi/scripts.go` + router + wiring (sayfalama `internal/paging`)
- [x] T21 Ayarlar → "Betikler" sekmesi; içerik düzenleyici + iki uyarı
- [x] T22 Agent formunda betik seçimi
- [x] T23 Bash yetkisi kapalı agent'ta seçim yapılırsa arayüz **uyarır**

## Doğrulama

- [x] T40 [plan.md](plan.md) doğrulama listesi 1–7 · tema eşliği 54 kontrol,
      0 kalan
- [x] T41 **Gerçek çalıştırmayla uçtan uca** — iki koşu:
      yetkili agent betiği çağırdı ve `BETIK_CALISTI=evet` çıktısını verdi;
      yetkisiz agent aynı betik atanmışken `HIC BETIK YOK` dedi

## Belgeler

- [x] T50 `AGENTS.md` → betikler `/work` dışında, `0o755`, **yetki desenleri
      güvenlik sınırı değildir**
- [x] T51 `specs/README.md` → 012 satırı
- [x] T52 "Nasıl çalışır" sayfası → 7. adım + `ScriptDeterminism` diyagramı +
      güvenlik kartı

---

## Ölçüm notları

### Ölçüm 1 — Cazip olan çözüm, gerçek bir açıktı

İlk plan "bash kapalı ama şu betiğe izinli" modunu **özelliğin en değerli
parçası** olarak sunuyordu: motorun yetki sözlüğü `bash` için desen kabul ediyor
ve son eşleşen kural kazanıyor, yani teknik olarak yazılabilirdi.

Kullanıcı sordu: *"eğer bu arka kapı açılmasına sebep olacaksa erteleyelim."*
Kod okunduğunda cevap netti — **eşleşme ham komut metnine yapılıyor, bash
ayrıştırması yok**. `betik.sh; env` deseni geçer ve `env` çıktısında `GIT_TOKEN`
ile sağlayıcı anahtarı var (`opencode/runner.go:250-262`).

Ders: bir mekanizmanın **desteklediği** şey, o şeyin **güvenli** olduğu anlamına
gelmiyor. Fikir ertelenmedi, düşürüldü — ve neden düşürüldüğü [spec K2](spec.md)
olarak yazıldı ki bir sonraki oturumda "bu neden yapılmamış ki, kolay görünüyor"
diye geri gelmesin.

### Ölçüm 2 — "Yeni yetenek açmıyor" iddiası test edilebilir olmalı

`BuildPermissions`'ın hiç değişmemesi bu özelliğin güvenlik iddiasının
tamamıdır. Ama bir yorum satırı bunu koruyamaz; bir sonraki geliştirici oraya
tek satır ekler ve iddia sessizce yalan olur.

Bu yüzden testin kendisi iddiayı kilitliyor:

```go
withScripts := BuildPermissions(AgentSpec{AllowBash: true, Scripts: [...]})
without     := BuildPermissions(AgentSpec{AllowBash: true})
require.Equal(t, without, withScripts)
```

Bir güvenlik kararı, ancak **kırıldığında bir testi düşürüyorsa** karardır.

### Ölçüm 3 — Hatırlanan şema, okunan şema değildir (tekrar)

Store testinde agent kaydını elle `INSERT` ile yazdım ve `source` sütununu
atladım — `NOT NULL` ihlaliyle düştü. Rapor testinde `workflows.hook_token` ile
yapılan hatanın **aynısı**, aynı sebeple: şemayı okumak yerine hatırladım.

Düzeltme yalnızca sütunu eklemek değildi; fixture gerçek `agentreg.Store`
üzerinden yazıldı. Elle SQL, şema değişince sessizce kırılır; üretim yolundan
geçen fixture kırılmaz.

### Ölçüm 4 — Görünmeyen karakterin bedeli

`normalizeContent` süs değil. Windows'ta yazılıp yapıştırılan bir betikte `\r`
kalırsa shebang satırı `#!/bin/bash\r` olarak okunuyor ve kabuk
`command not found` diyor — kullanıcının **ekranda göremediği** bir karakter
yüzünden. Kayıt anında temizlenmesi, hata ayıklaması en pahalı sorunlardan
birini baştan siliyor. Gerçek çalıştırmada CRLF ile kaydedilen betik sorunsuz
koştu.

### Ölçüm 5 — Dosyayı koymak, agent'ın bilmesi demek değil

MCP araçlarından farkı burada: onlar modele **araç** olarak sunuluyor, betikler
ise sadece dosya. Talimat dosyasına liste yazılmasaydı betik container'da durur
ve hiç çağrılmazdı — hatasız, sessiz bir işe yaramazlık.

Aynı sebeple yetkisi kapalı agent'a **liste de yazılmıyor**: çalıştıramayacağı
bir betiği anlatmak, onu var olmayan bir yolu denemeye iterdi. Canlı koşuda bu
doğrulandı — yetkisiz agent "HIC BETIK YOK" dedi.

### Ölçüm 6 — Test agent'ı silinemedi

Doğrulama için oluşturulan `betik-kapi-testi` agent'ı, çalıştırma geçmişi
olduğu için silinemiyor (`agent_in_use`). Bu kural bilinçli: geçmiş kayıt hangi
agent'la koştuğunu göstermeli.

Ama sonuç şu ki **doğrulama, ekranda kalıcı çöp bırakabiliyor**. Bugün kabul
edilebilir; agent'lara "arşivle" durumu geldiğinde bu da çözülür.
