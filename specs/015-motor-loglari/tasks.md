# Görevler: Motor logları

- **Spec no:** 015 — [spec.md](spec.md) · [plan.md](plan.md)
- **Durum:** Tamamlandı

---

## Yapılanlar

- [x] T10 Migration `000015` — `run_engine_logs`, cascade, tekillik
- [x] T20 `Redact` + `SecretsOf` (base64 `_auth` biçimi dahil)
- [x] T21 `collectEngineLogs` — üç kaynak, `defer` sırası
- [x] T22 `Container.ReadDir` (tar) ve `Logs` (stdcopy)
- [x] T23 `sessionTranscript` + alan bazlı kırpma
- [x] T24 Saklama: gzip, sondan kırpma, `ON CONFLICT`, purge zamanlayıcı
- [x] T25 Üç ayar + `GET /api/runs/{id}/engine-logs`
- [x] T30 Koşu detayında sekme, kaynak seçici, arama, indirme
- [x] T31 Oturum geçmişi için konuşma görünümü
- [x] T40 Testler: gidiş-dönüş, kırpma, cascade, retention, maskeleme
- [x] T90 Gerçek koşularla doğrulama
- [x] T92 `nasil-calisir` sayfası ve `AGENTS.md` güncellendi

---

## Ölçüm notları

### Ölçüm 1 — Özellik, kendi teşhis edeceği bir hatayı ilk gün buldu

Maskeleme testi için agent'a "`~/.npmrc` dosyasını `cat` ile yazdır"
dendi. Koşu asıldı. Kullanıcı sordu: *"7 dk dır çalışıyor normal mi"*.

Cevap, **tam da yeni yazılan özellikle** bulundu — hâlâ ayakta olan
container'ın motor log dosyaları okundu:

```
message=evaluated permission=external_directory pattern=/home/agent/*
        action.action=ask
message=asking id=per_ff84… permission=external_directory
```

Koşular başsız; `ask` iznini cevaplayacak kimse yok. Çalıştırma 30 dakikalık
zaman aşımına kadar **sıfır token'da** asılı kalıyordu. Ölçüldü: düzeltmeden
sonra aynı iş **20 saniyede** bitti.

Bu bir "yeni özelliğin çıkardığı hata" değil; önceden de vardı ve bir başka
koşu 22 dakika aynı şekilde asılmıştı — kimse sebebini bilmiyordu.

**Ders:** teşhis aracı, teşhis edilecek şeyden önce gelir.

### Ölçüm 2 — Görünmeyen bozukluk, gösterilince ortaya çıktı

`docker logs` TTY'siz container'da 8 baytlık başlıklarla çerçeveleniyor.
Kod bunu biliyordu; yorumda "hata ayıklama için ham okumak yeterli,
çerçeveler gözle ayıklanabilir" yazıyordu ve doğruydu — log yalnızca
`slog`'a gittiği sürece.

Aynı içerik ekranda gösterilince satır başlarında `\x02\x00\x00…` çıktı:

```
'\x02\x00\x00\x00\x00\x00\x00Y[runner] repo klonlanıyor: …'
```

`stdcopy.StdCopy` ile çözüldü ve test bağlandı.

**Ders:** bir verinin "yeterince iyi" olması, onu kimin okuduğuna bağlıdır.
Tüketici değişince kabul kriteri de değişir.

### Ölçüm 3 — Kullanıcının tarifi ile gerçek uyuşmadı

İstek şöyleydi: *"`storage/` altındaki message/part JSON'ları toplansın"*.
Ölçüm başka söyledi — o dizin yok; oturum deposu **SQLite**:

```
/home/agent/.local/share/opencode/opencode.db  (+ -wal, -shm)
/home/agent/.local/share/opencode/log/opencode.log
/home/agent/.local/share/opencode/snapshot/<hash>/
```

Veritabanı dosyasını ham kopyalamak metin üretmez, maskeleme uygulanamaz ve
arayüzde gösterilemez. Aynı veri motorun HTTP API'sinden JSON olarak alındı;
istenen sonuç (SSE kopsa da geçmiş kaybolmasın) korundu.

**Ders:** tarif edilen mekanizma ile istenen sonuç ayrı şeylerdir. Mekanizma
ölçümle çelişirse sonuç korunur, mekanizma değişir — ve bu **söylenir**.

### Ölçüm 4 — Doğru kırpma, yanlış biçim

Saklama katmanı boyut sınırını aşan içeriğin **sonunu** koruyor. Düz metin
için doğru. Oturum geçmişi JSON olunca sonuç şu oldu:

```
ham 4.293.545 B → saklanan 2.033.220 B (kırpıldı)
ilk 80 karakter: '"Solorio\",\n+  "Solorzano\",…'   ← ortasından kesilmiş
json.loads → Extra data: line 1 column 1615137
```

Yani okunur görünüm **tam da en çok gerektiği koşuda** kayboluyordu; kullanıcı
ham JSON görüyordu.

Ölçüm sebebi gösterdi: içeriğin **%96'sı iki alandı** (1,6 MB npm çıktısı,
320 KB üretilmiş dosya). Kırpma alan bazına alındı — yapı korunuyor, uzun
metinlerin ortası çıkıyor. Aynı görev yeniden koştu: **4.293.545 B → 87.942 B**,
kırpma sınırına hiç yaklaşmıyor, 12 mesajın hepsi duruyor.

**Ders:** genel bir kısıt (bayt sınırı) yapılandırılmış veriye uygulanırken
veriyi bozabilir. Sınır, verinin **birimine** göre uygulanmalı.

### Ölçüm 5 — Testin gerçekten bekçilik ettiği kanıtlanmalı

"Sırlar maskeleniyor" iddiası için birim testi yetmiyor: `Redact` saf bir
fonksiyon ve kendi testi var, ama asıl soru **toplama yolunun maskelemeden
geçip geçmediği**.

Gerçek container üzerinde bir test yazıldı, sonra `Redact` çağrısı geçici
olarak düşürüldü:

```
--- FAIL: TestCollectEngineLogs_SirlarMaskelenir
    "…_auth=Y2kta3VsbGFuaWNpOm5wbS10ZXN0…" should not contain …
```

**Ders:** bir güvenlik testi, ancak korumayı kaldırınca kırmızıya dönüyorsa
kanıttır. Yeşil olması tek başına bir şey söylemez.

### Ölçüm 6 — Uçtan uca sızıntı testi ilk denemede boşa çıktı

Agent'a sağlayıcı anahtarını `echo` ettiren bir koşu yapıldı. Saklanan logda
anahtar yoktu — ama **maskeleme sayacı da sıfırdı**. Yani anahtar loga hiç
düşmemişti; boru hattı sınanmamıştı.

Sonuç "geçti" değil, "ölçüm geçersiz"di. Kanıt ancak sırların bilerek
yerleştirildiği kontrollü bir container testiyle elde edildi (Ölçüm 5).

**Ders:** olumsuz bir gözlem (`sır görünmüyor`) ancak sırrın **oraya
ulaştığı** gösterilmişse kanıttır.

### Ölçüm 7 — `rawSize` bayt, içerik uzunluğu karakter

`stdout` için `rawSize=314` ama JSON'dan okunan metnin uzunluğu `309`.
Tutarsızlık değil: Go `len()` **bayt** sayar, Python/JS string uzunluğu
**karakter**. Türkçe harfler UTF-8'de çok baytlı.

Arayüz `rawSize`'ı gösteriyor — doğru olan bu, çünkü saklama sınırı da bayt
cinsinden.

### Ölçüm 8 — Kullanıcının istediği "sekme"ydi, ben panel yapmıştım

İstek açıkça *"ayrı bir sekmede görüntülensin"* diyordu; ilk uygulama logu
koşu detayının en altına bir panel olarak koydu ve bu bir yorum satırıyla
("teşhis katmanı, en altta") gerekçelendirildi.

Gerekçe makuldü ama istenen bu değildi. Kullanıcı fark etti. Sekmeye
çevrildi.

**Ders:** makul bir alternatif, açıkça istenen şeyin yerine geçmez. Sapma
yapılacaksa önce **söylenir**, sessizce uygulanmaz.
