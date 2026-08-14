# Sandbox veri sızıntısı analizi

**agent-coder'ın runner sandbox'ında çalışan opencode, kodu, sırları veya
telemetriyi dışarı sızdırıyor mu?**

Bu belge o soruyu ölçerek cevaplıyor. İki gerçek koşunun tüm ağ trafiği
açılarak kaydedildi, içine yerleştirilen izlenebilir dizgeler (kanaryalar)
mekanik olarak arandı ve her hedef adres sınıflandırıldı.

Ölçüm tarihi: 2026-08-14 · opencode 1.18.15 · mitmproxy 12.2.3

---

## Kısa cevap

**Kullanıcı verisi yalnızca yapılandırılmış model sağlayıcısına gitti.**
Kaynak kod, görev metni ve dosya adları OpenRouter'a ulaştı — ürünün işi
zaten bu. Bunun dışında hiçbir hedefe kullanıcı verisi gönderilmedi.

**Depo erişim token'ı yalnızca ait olduğu yere gitti.** Model sağlayıcısına
veya başka bir üçüncü tarafa hiçbir biçimde ulaşmadı.

**Depoya karışmış sır hiçbir yere gitmedi** — çünkü agent o dosyayı hiç
okumadı. Bu bir engelleme değil, bir tesadüf: okusaydı sağlayıcıya giderdi.

Buna karşılık üç bulgu dikkat gerektiriyor:

1. opencode her koşuda **kendi sunucusundan model kataloğu çekiyor**
   (`models.opencode.ai`) ve **GitHub'dan bir ikili dosya indirip
   çalıştırıyor** (`ripgrep`). İkisi de kullanıcı verisi göndermiyor ama
   kapalı ağlarda ürünü çalışmaz hale getirir.
2. **Sandbox'ın dışarı çıkışı tamamen serbest.** Koşu B'de agent, imajda
   zaten kurulu olan JDK ve Maven yerine internetten kendi JDK'sını indirip
   çalıştırdı. Aynı serbestlik, kötü niyetli bir görev metni veya depoya
   gömülü bir talimat için de geçerlidir.
3. **Maven trafiğinin bir kısmı ölçüm vekilini atladı.** O bağlantıların
   içeriği açılamadı; hedefleri bilindiği için risk düşük görünüyor ama
   içerikleri hakkında bir şey söylenemez.

---

## "Sızıntı" neyi kapsıyor

Ayrım raporun tamamını belirliyor:

| | Değerlendirme |
| --- | --- |
| Kodun **yapılandırılmış model sağlayıcısına** gitmesi | Sızıntı değil — ürünün çalışma biçimi |
| Herhangi bir verinin **üçüncü bir tarafa** gitmesi | Sızıntı |
| **Sırların** (token, parola, anahtar) **herhangi bir yere** gitmesi | Sızıntı — sağlayıcı dahil |
| Kullanım/tanılama verisinin arka planda gönderilmesi | Telemetri — ayrıca raporlanır |

---

## Yöntem

### İki bağımsız katman

**Katman 1 — vekil (mitmproxy).** Runner'ın çıkış trafiği bir vekile
yönlendirildi ve TLS açıldı; tam URL, başlıklar ve gövdeler kaydedildi.
Vekilin CA'sı, ürünün kurumsal sertifika mekanizmasından (spec 017)
beslendi — yani sertifika enjeksiyonu için ayrı bir yol açılmadı.

**Katman 2 — köprü üzerinde paket yakalama (tcpdump).** Docker ağının
köprüsünde ham paketler kaydedildi.

İkinci katman şart, çünkü **vekil trafiği yönlendirir, mecbur etmez.**
Vekili yok sayan bir istemci doğrudan çıkar ve vekil dökümünde *hiç
görünmez*. Yalnız vekile bakılsaydı böyle bir kaçış "sızıntı yok" diye
okunurdu. Nitekim Koşu B'de tam olarak bu oldu (aşağıya bakın).

Köprü yakalaması runner container'ı doğmadan önce başlatıldı; ilk paketler
kaçmadı. Container geçici olduğu için IP'si koşu **sırasında** kaydedildi —
bitişten sonra sorulamıyor.

### Kanaryalar

Bir mitmproxy dökümüne bakıp "şüpheli bir şey görmedim" demek kanıt değil.
Onun yerine sızıntının çıkabileceği her yere benzersiz, yüksek entropili bir
dizge kondu ve dökümde mekanik olarak arandı.

| Kanarya | Nereye kondu | Neyi ölçer |
| --- | --- | --- |
| Kaynak kodu | depodaki Java dosyasında bir sabit | kaynak kod nereye gidiyor |
| Depo sırrı | depodaki `.env.ornek` içinde | depoya karışmış sırlar |
| Git token'ı | depo erişim parolası — **gerçekten kullanıldı** | kimlik bilgisi sızması |
| Görev metni | task içinde izleme kimliği | prompt nereye gidiyor |
| Dosya adı | Java dosyasının adı | dosya adları / dizin listesi |

### Aramanın kodlamalara karşı sağlamlaştırılması

İlk kuru koşuda arama, **gerçekten gönderilmiş** bir kimlik bilgisini
bulamadı: token `Authorization: Basic` başlığında base64 içindeydi ve düz
metin araması onu göremiyordu. Bu haliyle rapor yanlış bir "isabet yok"
üretecekti.

Arama üç biçimi birden tarayacak şekilde düzeltildi:

- düz metin ve URL kodlaması
- **base64 — üç hizalama kaymasının üçü de**, çünkü aranan dizge daha uzun
  bir metnin ortasında kodlanmış olabiliyor
- **sadeleştirilmiş metin** (harf/rakam dışı her şey atılmış), çünkü JSON
  kaçışı ve akış hâlinde gelen (SSE) yanıtlar bir dizgeyi ikiye bölebiliyor

Düzeltmeden sonra aynı token 12 akışta bulundu. **Bu, dedektörün pozitif
kontrolüdür:** gerçek bir kimlik bilgisini gerçek bir kodlama içinde
bulabildiği gösterilmiş oldu. Aksi halde "isabet yok" sonuçları hiçbir şey
ifade etmezdi.

### Ölçüm ortamı

| | |
| --- | --- |
| Runner imajı | `agent-coder/opencode-runner:latest` (Node 24 tabanı) |
| opencode | 1.18.15 (`opencode serve`, headless) |
| Model sağlayıcı | OpenRouter · `qwen/qwen3.6-27b` |
| Depo | yerel kanarya deposu, smart HTTP + basic auth |
| Agent | özel tanım — düzenleme ve kabuk **açık**, web erişimi **kapalı** |

Web erişimi bilerek kapatıldı: açık olsaydı agent'ın kendi tercihiyle
yaptığı bir istek ile opencode'un kendi trafiği birbirine karışır ve atıf
imkânsızlaşırdı.

---

## Koşular

**Koşu A — taban çizgisi.** Tek bir dosyayı açıp bir Javadoc yorumu ekleme.
Amaç, opencode'un *kendi* trafiğini gürültüsüz görmek.
11 akış · 6 162 / 118 jeton.

**Koşu B — gerçekçi build.** `mvn compile`, kod ekleme, tekrar `mvn compile`.
Amaç, build araçlarının egress'ini de kapsamak.
88 akış · 36 916 / 161 jeton.

---

## Bulgular

### 1. Hedef envanteri ve atıf

Her istek, istemci kimliğine (`User-Agent`) göre kaynağına bağlandı.

**Koşu A**

| Akış | Hedef | İstemci |
| ---: | --- | --- |
| 4 | `openrouter.ai` | opencode |
| 4 | yerel depo | git |
| 1 | `models.opencode.ai` | opencode |
| 1 | `github.com` | opencode |
| 1 | `release-assets.githubusercontent.com` | opencode |

**Koşu B**

| Akış | Hedef | İstemci |
| ---: | --- | --- |
| 66 | `openrouter.ai` | opencode |
| 5 | `archive.apache.org` | curl (agent) |
| 4 | yerel depo | git |
| 2 | `registry.npmjs.org` | npm (agent) |
| 2 | `github.com` | curl (agent) |
| 2 | `release-assets.githubusercontent.com` | curl (agent) |
| 1 | `repo1.maven.org` | curl (agent) |
| 1 | `google.com` | curl (agent) |
| 1 | `models.opencode.ai` | opencode |
| 1 | `github.com` | opencode |
| 1 | `release-assets.githubusercontent.com` | opencode |

**opencode'un kendiliğinden ulaştığı üçüncü taraf adresler yalnızca üç
tanedir** ve her iki koşuda da aynıdır: `models.opencode.ai`, `github.com`,
`release-assets.githubusercontent.com`. Geri kalan her şey ya sağlayıcı, ya
depo, ya da agent'ın kabuk komutlarıdır.

### 2. Kanarya sonuçları

| Kanarya | Koşu A | Koşu B | Değerlendirme |
| --- | --- | --- | --- |
| Kaynak kodu | `openrouter.ai` — 2 akış, istek gövdesi | `openrouter.ai` — 65 akış | **Beklenen.** Sağlayıcı dışında hiçbir yerde yok |
| Görev metni | `openrouter.ai` — 4 akış | `openrouter.ai` — 66 akış | **Beklenen.** |
| Dosya adı | `openrouter.ai` | `openrouter.ai` | **Beklenen.** |
| **Git token'ı** | yerel depo — 3 akış, istek başlığı | yerel depo — 3 akış | **Temiz.** Yalnızca kimlik doğrulaması yaptığı sunucuya gitti |
| **Depo sırrı** | isabet yok | isabet yok | Hiçbir yere gitmedi |

Depo sırrının çıkmaması bir **koruma değil**, agent'ın o dosyayı hiç
okumamasıdır. Okusaydı — ki normal bir görevde okuyabilir — içeriği diğer
dosya içerikleri gibi sağlayıcıya giderdi. Depodaki sırlar için sandbox bir
sınır oluşturmuyor.

### 3. Model sağlayıcısına giden istek

`POST https://openrouter.ai/api/v1/chat/completions` gövdesi yalnızca şunları
taşıyor: `model`, `messages` (system + user), `tools`, `temperature`,
`top_p`, `max_tokens`, `stream`, `usage`.

Başlıklarda `Authorization` (sağlayıcı anahtarı — gitmesi gereken yer) ve
opencode'un kendi oturum kimliği (`x-session-id`) var. Ortam değişkenleri,
git kimlik bilgileri veya makineye ait bir tanımlayıcı **yok**.

### 4. opencode'un kendi çağrıları

**`GET https://models.opencode.ai/api.json`** — her koşuda bir kez, 3,6 MB
model kataloğu. Gövdesiz `GET`; kullanıcı verisi taşımıyor. Başlıklarında
sürüm bilgisi (`opencode/latest/1.18.15/cli`) ve birer izleme kimliği
(`traceparent`, `b3`) var. **Bu kimlikler iki koşuda farklıydı** — yani
kalıcı bir cihaz/kullanıcı parmak izi değil, istek başına üretiliyor.

**`GET https://github.com/BurntSushi/ripgrep/releases/download/15.1.0/…`**
→ `release-assets.githubusercontent.com`, 1,87 MB. opencode, **çalışma
anında GitHub'dan bir ikili dosya indirip sandbox içinde kullanıyor.** Veri
göndermiyor; ama:

- kapalı veya SSL denetimli bir ağda bu indirme düşer,
- ürünün güvendiği yüzey, imajda pinlenmiş paketlerin ötesine geçer:
  çalışma anında indirilen bir ikili, imaj derlenirken doğrulanmamıştır.

### 5. Vekil atlatma — ölçümün sınırı

Runner container'ının kurduğu her TCP bağlantısı sayıldı (SYN paketleri):

| | Runner'ın kurduğu dış bağlantı | Vekilden geçen | **Doğrudan çıkan** |
| --- | ---: | ---: | ---: |
| Koşu A | 5 | 5 | **0** |
| Koşu B | 26 | 21 | **5** |

Koşu A'da runner, vekil dışında hiçbir adrese bağlanmadı — o koşunun tüm
trafiği ölçüldü.

Koşu B'de **`repo.maven.apache.org` (104.18.19.12) adresine beş TLS
bağlantısı doğrudan kuruldu.** Bu bağlantılar vekile uğramadığı için
içerikleri açılamadı; hakkında söylenebilecek tek şey hedefleri ve
zamanlarıdır. Kanarya araması bu beş bağlantının içini **görmedi**.

Muhtemel sebep — kanıtlanmadı: JVM hiçbir `*_PROXY` ortam değişkenini
okumaz, vekil ancak sistem özellikleriyle verilebilir; agent ise imajdaki
Maven yerine internetten indirdiği Maven ve JDK'yı çalıştırdı. Aynı koşuda
`repo1.maven.org` isteğinin (curl ile yapılan) vekilden geçmiş olması bu
yorumu destekliyor ama kanıtlamıyor.

**Bu bir bulgu olduğu kadar bir uyarıdır:** kurumsal bir ağda "tüm trafiği
vekilden geçiriyoruz" varsayımı, Java tarafı için ayrıca doğrulanmadıkça
geçerli değildir.

### 6. Sandbox'ın çıkışı serbest

Koşu B'de agent, imajda **zaten kurulu olan** JDK 17/25 ve Maven 3.9.16
yerine `github.com/adoptium`'dan JDK 17'yi ve `archive.apache.org`'dan Maven
3.9.9'u indirdi, açtı ve çalıştırdı. Ayrıca `google.com`'a bir bağlantı
denemesi ve npm kayıt defterine iki sorgu yaptı.

Görev bunların hiçbirini istemiyordu. Bu, opencode'un bir kusuru değil,
**sandbox'ın ağ politikasının sonucudur:** container her adrese
çıkabiliyor. Sonuç olarak:

- kaynak kodu dışarı taşımak isteyen bir talimat (kötü niyetli görev metni,
  depoya gömülmüş bir yönerge, ele geçirilmiş bir bağımlılık) bunu
  yapabilir; bu ölçüm böyle bir girişimin *engellenmediğini* gösteriyor,
- kurumsal ağ senaryosunda agent'ın hangi adreslere çıkabileceği ürün
  tarafından sınırlanmıyor.

### 7. Telemetri ve oturum paylaşımı

- İki koşuda da **hiçbir analitik/telemetri ucuna istek gidilmedi.**
- opencode ikilisinde Sentry/PostHog/Amplitude benzeri bir analitik uç
  adresi bulunmadı. (`telemetry` ve `segment` dizgeleri metin bölütleme ve
  arayüz bileşeni adlarından geliyor.)
- opencode'un bir **oturum paylaşma** özelliği var (`--share` bayrağı ve bir
  `POST share` ucu). Ürün bu ucu hiç çağırmıyor; runner `opencode serve` ile
  başlatılıyor ve backend'in opencode istemcisinde paylaşımla ilgili tek bir
  çağrı yok. İki koşuda da paylaşım trafiği gözlenmedi.
- İkilide var olup **bu koşularda hiç kullanılmayan** uçlar mevcut:
  `console.opencode.ai`, `opencode.ai/zen`, `opencode.ai/theme.json`. Bu
  ölçüm onların hangi koşullarda çağrıldığını söylemiyor.

---

## Değerlendirme

| Bulgu | Risk | Neden |
| --- | --- | --- |
| Kod ve prompt sağlayıcıya gidiyor | — | Ürünün çalışma biçimi; sağlayıcı seçimi kullanıcının |
| Git token'ı sağlayıcıya gitmiyor | — | Ölçüldü, temiz |
| Depodaki sırlar korunmuyor | **Orta** | Agent okursa sağlayıcıya gider; engel yok |
| Çalışma anında ikili indirme (ripgrep) | **Orta** | Kapalı ağda kırılır; imaj dışı güven yüzeyi |
| `models.opencode.ai` çağrısı | Düşük | Veri göndermiyor; kapalı ağda kırılır |
| Sandbox'tan serbest çıkış | **Yüksek** | Sızdırma girişimi engellenmiyor |
| Java trafiği vekili atlıyor | Orta | Kurumsal vekil/denetim varsayımını bozar |

---

## Bakılmayan yerler

Bu ölçümün kapsamadıkları — sonuçları okurken bunlar geçerlidir:

- **İki koşu, tek model, tek sağlayıcı.** Başka sağlayıcılarda (litellm,
  openai_compatible) veya başka modellerde davranış ölçülmedi.
- **MCP sunucusu tanımlanmadı.** MCP, tanımı gereği veriyi üçüncü bir
  sunucuya taşır ve ayrı bir sızıntı yüzeyidir; ölçülmedi.
- **Agent betikleri (spec 012) kullanılmadı.**
- **Web erişimi (webfetch) kapalıydı.** Açık olsaydı agent'ın kendi istekleri
  ayrı bir yüzey oluştururdu.
- **Push / PR yolu çalıştırılmadı.**
- Vekili atlayan beş Maven bağlantısının **içeriği ölçülmedi**.
- Çözümlemede **1 MB'ı aşan yanıt gövdeleri kırpıldı** (kayıtta
  işaretlenerek). İstek gövdeleri kırpılmadı — sızıntı dışarı giden veridir.
- Ölçümün kendisi koşulları değiştiriyor: vekil ve ek bir CA devredeydi.
  Bunların yokluğunda opencode'un farklı davrandığına dair bir belirti
  görülmedi, ama bu da ölçülmedi.
- Container içi DNS (127.0.0.11) köprüde görünmez; ad çözümlemesi yerine
  TLS SNI ve hedef IP ölçüldü.

---

## Öneriler

1. **Sandbox çıkışını kısıtlayın.** Runner'ın ulaşabileceği adresler bir
   listeye indirgenebilirse (sağlayıcı, depo, paket depoları), hem serbest
   çıkış riski hem de beklenmedik indirmeler ortadan kalkar.
2. **Çalışma anındaki indirmeleri imaja alın.** `ripgrep` imajda pinlenmiş
   olsaydı hem kapalı ağ sorunu hem de doğrulanmamış ikili sorunu çözülürdü.
3. **Java tarafının vekil ayarını ayrıca doğrulayın.** Kurumsal ağ
   kurulumunda `MAVEN_OPTS` ile verilen vekil özelliklerinin gerçekten
   uygulandığı ölçülmeli — env değişkenleri JVM için yeterli değil.
4. **Depodaki sırlar için bir uyarı düşünün.** Agent'ın okuduğu her dosya
   sağlayıcıya gidiyor; bu, kullanıcının bilmesi gereken bir gerçek.

---

## Tekrarlanabilirlik

Ölçüm düzeneği ve tüm betikler: [`scripts/sizinti-analizi/`](../scripts/sizinti-analizi/)

```sh
cd scripts/sizinti-analizi
./kur.sh                  # kanaryalar, kanarya deposu, mitmproxy, CA
# .env'e basılan iki satırı ekleyin, sonra: make down && make up
./yeni-kosu.sh kosu-a     # doğrulamalı temiz yakalama
# … koşuyu başlatın …
./analiz.sh kosu-a        # ham sayılar
```

Ham trafik dökümleri depoya **girmez**: içlerinde gerçek sağlayıcı anahtarı
geçiyor.
