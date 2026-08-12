# Agent Coder

**Kod yazan AI agent'larını görsel bir tuval üzerinde birbirine bağlayıp çalıştıran platform.**

Jira'dan bir task gelir → bir agent kodu yazar → başka bir agent inceler → değişiklik
branch'e gönderilir → PR açılır → Jira'ya sonuç yorumu düşer. Hepsi kendiliğinden,
her adım farklı bir LLM modeliyle.

```text
  ┌─────────────┐    ┌──────────┐    ┌───────────┐    ┌────────┐    ┌─────────────┐
  │ Jira task'ı │───▶│  Analiz  │───▶│ Kod yazma │───▶│ PR aç  │───▶│ Jira yorumu │
  └─────────────┘    │ (Haiku)  │    │  (Opus)   │    └────────┘    └─────────────┘
                     └──────────┘    └───────────┘
```

Her kutu tuvalde sürüklenir, birbirine bağlanır ve kendi modelini seçer.

---

## Neden?

Bugün kod yazan agent'lar tek tek, elle, terminalden çalıştırılıyor. Bir adımın
çıktısını diğerine taşımak kopyala-yapıştır; hangi adımın ne kadara mal olduğu
belirsiz; aynı işi ikinci kez yapmak baştan başlamak demek.

Agent Coder bunu bir **akışa** dönüştürüyor: adımlar birbirine bağlı, her adım kendi
modeliyle, her çalıştırma kayıtlı ve maliyeti ölçülü.

## Öne çıkanlar

| Özellik | Ne demek |
|---|---|
| 🎨 **Görsel tuval** | Sürükle-bırak akış editörü; dallanma ve birleşme desteği |
| 🧠 **Adım başına model** | Analiz için ucuz bir model, kod yazımı için güçlü bir model |
| ⚡ **Paralel çalıştırma** | Birbirine bağlı olmayan adımlar aynı anda koşar |
| 🔒 **İzole sandbox** | Her adım kendi geçici Docker container'ında, kendi repo klonuyla |
| 📡 **Canlı izleme** | Adım adım log akışı (SSE), diff görüntüleme, iptal |
| 💰 **Maliyet takibi** | Adım başına token ve USD; yönetici raporu |
| 📜 **Hazır betikler** | Prosedür işlerinde LLM doğaçlaması yerine sabit `.sh` çalıştırma |
| 🔗 **Entegrasyonlar** | Jira (tetikleyici + yorum), GitHub / Bitbucket / genel Git (push + PR), MCP (iki yönlü) |
| 🌗 **Açık / koyu tema** | İkisi de ölçülerek WCAG AA'ya getirildi |

## Teknoloji

| Katman | Seçim |
|--------|-------|
| Arayüz | Next.js 15 · TypeScript · Tailwind v4 · [`@xyflow/react`](https://reactflow.dev) |
| Sunucu | Go — kendi DAG akış motoru, ek altyapı servisi yok |
| Veritabanı | PostgreSQL 16 |
| Agent motoru | [opencode](https://opencode.ai) (headless), `Runner` arayüzü arkasında değiştirilebilir |
| Modeller | [OpenRouter](https://openrouter.ai), [LiteLLM](https://litellm.ai) veya OpenAI-uyumlu herhangi bir servis |

---

# Kurulum

Aşağıdaki adımlar sıfırdan, hiç bilmeyen biri için yazıldı. **Toplam süre: ~10 dakika**
(çoğu imaj indirmekle geçer).

## 1. Gerekenler

Yalnızca iki şey:

- **Docker Desktop** (veya Docker Engine + Compose v2) — [indir](https://www.docker.com/products/docker-desktop/)
- **Git**

> Go, Node.js veya PostgreSQL kurmanız **gerekmez**. Her şey container içinde çalışır.

Docker'ın çalıştığını doğrulayın:

```bash
docker --version          # Docker version 24+ 
docker compose version    # Docker Compose version v2+
```

Docker Desktop'a en az **4 GB bellek** ayırın (Settings → Resources). Agent
container'ları varsayılan olarak iş başına 4 GB'a kadar kullanabilir.

## 2. Projeyi indirin

```bash
git clone https://github.com/codeeer/agent-coder.git
cd agent-coder
```

## 3. Ayar dosyasını oluşturun

```bash
make env
```

Bu komut `.env` dosyasını oluşturur **ve gereken gizli değerleri kendisi üretir**:

```
.env oluşturuldu.
  ✓ SECRET_ENCRYPTION_KEY üretildi
  ✓ OPENCODE_SERVER_PASSWORD üretildi
```

**Elle düzenlemeniz gereken hiçbir zorunlu alan yok.** Model API anahtarınızı da
buraya yazmayacaksınız — arayüzden gireceksiniz.

> **`SECRET_ENCRYPTION_KEY`'i yedekleyin.** Kaydedeceğiniz tüm API anahtarları
> onunla şifrelenir; kaybederseniz veya değiştirirseniz eskiler çözülemez ve
> hepsini yeniden girmeniz gerekir.

İki durumda `.env`'e dokunmanız gerekir — ikisi de aşağıda anlatıldı:
**[portlar doluysa](#portlar-doluysa)** ve
**[sunucuya kuruyorsanız](#sunucuya-kurulum)**.

### Portlar doluysa

Varsayılanlar: arayüz **3002**, API **8080**, PostgreSQL **5434**. Makinenizde
biri kullanılıyorsa `.env` içinden değiştirin:

```env
FRONTEND_PORT=3005
BACKEND_PORT=8085
POSTGRES_PORT=5440
```

Sonra `make restart`. Başka hiçbir yeri elle düzeltmeniz gerekmez: arayüzün
API adresi ve sunucunun CORS ayarı bu değerlerden **kendiliğinden** üretilir.

> Portu kullanan şeyi merak ederseniz: `lsof -i :3002` (macOS/Linux),
> `netstat -ano | findstr :3002` (Windows).

### Sunucuya kurulum

Kendi bilgisayarınızda çalıştıracaksanız **bu bölümü atlayın.**

Bir sunucuya kurup **başka bir makineden** tarayıcıyla açacaksanız, sunucunun
adresini yazmanız gerekir:

```env
PUBLIC_HOST=192.168.1.40        # ya da agent.sirket.local
```

Sonra `make restart`.

Sebebi şu: tarayıcı sunucuya **doğrudan** bağlanır. `localhost` kalırsa, siz
uzaktan açtığınızda tarayıcı **kendi bilgisayarınıza** bağlanmaya çalışır ve
ekran boş gelir. `PUBLIC_HOST` hem arayüzün API adresini hem de sunucunun CORS
ayarını birlikte düzeltir.

> Bu adres arayüz imajına **gömülmez** — sunucu her istekte ortamdan okuyup
> sayfaya yazar. Bu yüzden aynı hazır imaj her kuruluma uyar ve değişiklik
> yeniden derleme gerektirmez.

Ters vekil (nginx, Traefik) arkasındaysanız iki adresi de tam yazın:

```env
NEXT_PUBLIC_API_URL=https://api.sirket.com
CORS_ORIGINS=https://agent.sirket.com
```

> ⚠️ **Kimlik doğrulama yok.** Sunucuya kuruyorsanız yalnızca özel ağa açın —
> internete açık bırakmayın. Ayrıntı: [Güvenlik notları](#güvenlik-notları).

### SSL inspection yapan kurumsal ağlar

**Ev/ofis ağındaysanız bu bölümü atlayın.**

Bazı kurumsal ağlar giden HTTPS trafiğini kendi sertifikalarıyla açıp yeniden
imzalar (SSL inspection / TLS interception). Böyle bir ağda runner
container'ının yaptığı HTTPS istekleri şu hatayla düşer:

```
unable to get local issuer certificate
```

En görünür sonucu **depo klonlamanın** başarısız olmasıdır: container hazır
olamaz ve çalıştırma "çalışma ortamı hazırlanamadı" ile biter.

**Çözüm: kurumun kök sertifikasını tanıtın.**

1. Kök sertifikayı PEM olarak edinin. Genelde BT'den istenir; tarayıcıdan da
   çıkarılabilir. Sunucuda çoğu zaman zaten kuruludur:

   ```bash
   # Debian/Ubuntu
   ls /usr/local/share/ca-certificates/
   # RHEL/Rocky
   ls /etc/pki/ca-trust/source/anchors/
   ```

2. Yolu `.env` dosyasına yazın:

   ```env
   RUNNER_EXTRA_CA_CERT=/etc/pki/ca-trust/source/anchors/kurum-kok.pem
   ```

3. `make restart`.

Bundan sonra dosya her runner container'ına **salt okunur** bağlanır ve
`NODE_EXTRA_CA_CERTS` ile `GIT_SSL_CAINFO` üzerinden gösterilir. Bu, güvenilen
kök listesine **ekleme** yapar; genel sertifikalar geçerli kalır.

> **TLS doğrulamasını kapatan bir ayar yoktur ve eklenmeyecektir.**
> `NODE_TLS_REJECT_UNAUTHORIZED=0`, `npm config set strict-ssl false` veya
> `git config http.sslVerify false` sorunu görünmez yapar, ortadan kaldırmaz —
> ve bu imaj başka kurumlarda da çalışıyor. Kurumun kök sertifikası da imaja
> **gömülmez**: imaj herkese dağıtılıyor.

> **Sağlayıcı sürücüleri için artık internet gerekmiyor.** Runner imajı,
> OpenAI-uyumlu sağlayıcıların ihtiyaç duyduğu sürücü paketlerini derleme
> anında içine alır (sürümler `runner/package-lock.json` ile sabit). Bir
> çalıştırma sırasında paket deposuna **hiç istek çıkmaz**; air-gapped
> ortamlarda da koşar.

## 4. Başlatın

İki yol var. **Yalnızca denemek istiyorsanız hızlı olanı seçin.**

### ⚡ Hızlı yol — hazır imajlarla

```bash
make quickstart
```

Üç imaj da yayınlanmış hallerinden **çekilir — hiçbir şey derlenmez.**
`make runner` adımına da gerek kalmaz.

> İmajlar `ghcr.io/codeeer/...` altında, **amd64 ve arm64** için yayınlanır.
> Intel/AMD sunucu da Apple Silicon da doğru imajı kendiliğinden çeker.

### Hangi etiketi kullanmalı

`latest`, **main'e her commit'te yenilenir.** Denemek için en doğrusu budur:
`make quickstart` her çalıştığında güncel imajı çeker, yani bir düzeltme
yapıldığında siz de alırsınız.

Ama "bugün çalışan kurulum yarın da aynı kalsın" istiyorsanız sabit bir etikete
geçin:

```bash
IMAGE_TAG=0.1.1        make quickstart   # sürüm etiketi
IMAGE_TAG=sha-ebcc8a4  make quickstart   # tek bir commit
```

| Etiket | Ne zaman değişir | Kimin için |
|---|---|---|
| `latest` | main'e her commit'te | denemek, güncel kalmak |
| `0.1.1` · `0.1` | yalnızca yeni sürüm etiketinde | üretim benzeri kurulum |
| `sha-<kısa>` | hiç — tek bir commit'e çakılı | hata ayıklama, tekrarlanabilirlik |

Elinizdeki imajın hangi commit'ten geldiğini her zaman sorabilirsiniz:

```bash
docker inspect ghcr.io/codeeer/agent-coder-backend:latest \
  --format '{{ index .Config.Labels "org.opencontainers.image.revision" }}'
```

> Bu etiketleme, yayınlanan imajların bir süre main'in gerisinde kalmasından
> sonra kuruldu: kod düzeltilmiş ama imaj yayınlanmamıştı ve `quickstart`
> kullanan herkes eski backend ile koşuyordu. Artık **her main commit'i imaj
> üretir** ve imaj hangi commit'ten geldiğini üzerinde taşır.

### Güncelleme

Ayrı bir komut yok — **`make quickstart` aynı zamanda güncelleme komutudur.**
Üç imajı yeniden çeker, değişen servisi yeniden yaratır. `make restart` de
hazır imajlı bir kurulumu tanır ve kaynaktan derlemeye geçmez.

Aynı işi elle yapacaksanız **`.env` içindeki `RUNNER_IMAGE` satırını da
güncelleyin.** Runner bir compose servisi değil; adını backend `.env`'den
okuyor. Yalnızca komut satırında verirseniz bir sonraki başlatmada sessizce
eski yerel imaja dönersiniz:

```bash
docker pull ghcr.io/codeeer/agent-coder-runner:latest
sed -i.bak 's|^RUNNER_IMAGE=.*|RUNNER_IMAGE=ghcr.io/codeeer/agent-coder-runner:latest|' .env && rm -f .env.bak
```

### 🔨 Kaynaktan derleme yolu

Kodda değişiklik yapacaksanız veya hazır imaja güvenmek istemiyorsanız:

```bash
make runner     # agent çalıştırma imajı — bir kez, birkaç dakika
make up
```

### İkisinde de sonuç aynı

Üç servis ayağa kalkar ve veritabanı şeması kendiliğinden uygulanır:

```
  Arayüz : http://localhost:3002
  API    : http://localhost:8080/health
```

(Portları değiştirdiyseniz komut sizin adreslerinizi yazdırır.)

Doğrulayın:

```bash
curl localhost:8080/health     # {"status":"ok",...}
make ps                        # üç servis de "healthy" olmalı
```

Tarayıcıda **<http://localhost:3002>** adresini açın.

> 💡 Sistemin nasıl çalıştığını mimari diyagramlarıyla anlatan bir sayfa
> uygulamanın içinde var: sol menüde **Nasıl çalışır**. Birine anlatırken
> doğrudan onu kullanabilirsiniz.

---

# İlk akışınız

Açılış ekranı size üç adımlık bir kontrol listesi gösterir. Sırayla:

## 1️⃣ Model sağlayıcı tanımlayın

**Ayarlar → Modeller → Sağlayıcı ekle**

**OpenRouter zorunlu değil.** Üç tür de eşit desteklenir; hangisi elinizdeyse
onu seçin:

| Sağlayıcı | Kimin için | Gereken |
|---|---|---|
| **LiteLLM proxy** | Kurum içi proxy'si olanlar | Proxy adresi + anahtar |
| **OpenAI-uyumlu servis** | vLLM, Azure OpenAI, Ollama — `/v1/models` sunan her şey | Adres + anahtar |
| **OpenRouter** | Dışarıdan tek anahtarla yüzlerce modele erişmek isteyenler | [API anahtarı](https://openrouter.ai/keys) |

Kendi sunucunuzda model çalıştırıyorsanız (Ollama, vLLM) **dışarıya hiç
çıkmadan** kullanabilirsiniz — kod deponuz da model çağrılarınız da ağınızda
kalır.

Hangisini seçerseniz seçin sistem **kaydetmeden önce bağlanıp doğrular** ve model
kataloğunu çeker. Fiyat ve bağlam bilgisi sağlayıcı bildiriyorsa gelir;
bildirmiyorsa katalogda yalnızca model adları görünür.

> Anahtarınız veritabanında AES-256-GCM ile şifrelenir. Arayüzde bir daha tam
> haliyle gösterilmez — yalnızca son 4 karakteri görünür.

**Birden fazla sağlayıcı aynı anda tanımlı olabilir** ve her agent adımı
hangisini kullanacağını ayrı seçer: analizi kurum içi ucuz bir modele, kod
yazımını güçlü bir modele verebilirsiniz.

> Hiç sağlayıcı tanımlamazsanız uygulama yine açılır; açılış ekranı bu adımı
> eksik gösterir ve bir agent çalıştırmayı denediğinizde ne yapmanız gerektiğini
> söyleyen bir uyarı alırsınız.

## 2️⃣ Proje ekleyin

**Projeler → Proje ekle**

Agent'ların üzerinde çalışacağı kod deposu:

| Alan | Örnek |
|------|-------|
| Ad | `Benim Projem` |
| Depo adresi | `https://github.com/kullanici/depo.git` |
| Varsayılan branch | `main` |

Sistem kaydetmeden önce depoya **gerçekten erişebildiğini** doğrular.

Özel (private) bir depo veya PR açma isteğiniz varsa önce
**Ayarlar → Kod depoları**'ndan bir erişim tanımlayın:

- **GitHub:** [Personal access token](https://github.com/settings/tokens) — `repo` yetkisi yeterli
- **Bitbucket:** kullanıcı adı + app password
- **Genel Git:** kullanıcı adı + parola/token

## 3️⃣ Akışı kurun

**Akışlar → Akış oluştur** → ad verin ve projeyi seçin.

Tuval açılır. Üstteki düğmelerle adım ekleyin:

- **Agent** — bir agent'ı bir modelle çalıştırır
- **PR aç** — değişiklikten pull request açar
- **Jira yorumu** — bir issue'ya sonuç yazar

Bir adımın **sağ ucundan** diğerinin **sol ucuna** sürükleyerek bağlayın.
Adıma tıklayınca sağ panelde ayarları açılır: agent, model, talimat.

Talimat içinde şu referanslar kullanılır:

| Referans | Anlamı |
|---|---|
| `{{ input }}` | Akışa verdiğiniz görev metni |
| `{{ steps.<adım>.output }}` | Önceki bir adımın çıktısı |
| `{{ steps.<adım>.diff }}` | Önceki adımın ürettiği değişiklik |
| `{{ trigger.summary }}` | Jira task'ının özeti (Jira tetiklemeli akışlarda) |

> Yanlış bir referans yazarsanız **kaydetme anında** reddedilir ve hangi adımda
> olduğu söylenir — çalışma sırasında sessizce boş kalmaz.

Kod değiştiren bir adımda **"Değişikliği branch'e gönder"** kutusunu işaretleyin;
PR düğümü o branch'i kullanır.

**Kaydet** → görev metnini yazın → **Akışı çalıştır**.

Adımlar tuvalde canlı renklenir. Bitince her adımın çıktısını, diff'ini, kaç token
harcadığını ve kaç dolar tuttuğunu görürsünüz.

---

# Jira'dan otomatik tetikleme

Bir Jira task'ı açıldığında akışın kendiliğinden çalışmasını isterseniz:

**1.** [Jira Cloud](https://www.atlassian.com/software/jira/free) ücretsiz planı yeterli
(10 kullanıcıya kadar, REST API dahil).

**2.** Bir [API token](https://id.atlassian.com/manage-profile/security/api-tokens) üretin.

**3.** **Ayarlar → Jira**: site adresi (`https://siteniz.atlassian.net`), e-postanız ve token.

**4.** Akış ekranında başlangıç düğümüne tıklayın → **Nasıl başlar: Jira task'ı** →
bir JQL sorgusu yazın:

```jql
project = SCRUM AND status = "Yapılacaklar" AND labels = agent
```

Eşleşen her task akışı **bir kez** başlatır. Task Jira'da güncellenirse yeniden
çalışır; akışın kendi yazdığı yorum tetikleyici sayılmaz.

Varsayılan olarak 5 dakikada bir taranır (**Ayarlar → Jira → Tetikleyici**).
Anında işlenmesini isterseniz aynı panelde görünen **webhook adresini** Jira'ya
tanımlayın — tarama yedek yol olarak kalır, ikisi aynı korumadan geçer.

Akışı geçici olarak durdurmak için akış ekranındaki **Duraklat** düğmesi:
otomatik tetikleme durur, elle çalıştırma açık kalır.

---

# Dış araçlar (MCP)

Agent Coder, [Model Context Protocol](https://modelcontextprotocol.io) ile **iki
yönde** de konuşur.

## Agent'larınız dış araçlara erişsin

**Ayarlar → Dış araçlar → Sunucu ekle.** Uzak (HTTP/SSE) bir MCP sunucusu
tanımlayın — hata takip sistemi, dokümantasyon, veritabanı şeması…

Kaydetmeden önce sunucuya bağlanılır ve sunduğu araçlar listelenir; böylece bir
agent'a **neye erişim verdiğinizi** görürsünüz. Erişim anahtarı şifreli saklanır
ve agent'ın okuyabileceği hiçbir dosyaya yazılmaz.

Sonra **Agent'lar** ekranından hangi agent'ın hangi sunucuyu kullanacağını
seçin. Seçilmeyen sunucuların araçları o agent'a **hiç sunulmaz**.

> Bir sunucuya bağlanılamazsa çalışma sessizce devam etmez — canlı olay akışında
> uyarı görürsünüz. Aksi halde agent'ın neden araçsız kaldığı görünmezdi.

## Akışın kendisi bir aracı çağırsın

Tuvale **MCP aracı** düğümü ekleyin. Agent'ın kararına bırakmadan, belirli bir
aracı belirli argümanlarla çağırır:

```json
{
  "repoName": "modelcontextprotocol/go-sdk",
  "question": "{{ input }}"
}
```

Şablonlar JSON'un içinde, tırnak arasında yazılır. Aracın çıktısı sonraki adıma
`{{ steps.<adım>.output }}` ile geçer.

## Agent Coder'ı başka araçlardan kullanın

**Ayarlar → Dış araçlar** bölümünün altındaki *Agent Coder'ı dışarıya aç* adresini Claude Desktop veya
Cursor yapılandırmanıza ekleyin. Üç araç sunulur: `akislari_listele`,
`akis_calistir`, `calisma_durumu`.

> Adres bir anahtardır — bilen herkes akışlarınızı başlatabilir. Sızdıysa aynı
> bölümden yenileyin.

---

# Betikler — standart işte standart sonuç

Bir agent her seferinde yeniden karar verir. Keşifte (hatayı bul, özelliği yaz)
doğru olan bu davranış, **prosedürde** risktir: aynı iş bugün `npm update`,
yarın `npm install paket@latest` olarak koşabilir.

**Ayarlar → Betikler → Betik ekle** ile hazır bir kabuk betiği tanımlayın:

| Alan | Örnek |
|---|---|
| Ad | `upgrade-deps` → `/home/agent/scripts/upgrade-deps.sh` |
| Ne işe yarar | `Bağımlılıkları güvenli sürümlere yükseltir` |
| İçerik | `.sh` dosyanızın metni |

```bash
#!/bin/bash
set -euo pipefail

npm ci
npm update --save
npm test
```

Sonra **Agent'lar** ekranından hangi agent'ın kullanacağını seçin. Betik o
agent'ın ortamına konur ve talimatına yol + açıklamayla yazılır.

> Sınır şu: **model ne zaman çağıracağına karar verir, betik ne yapılacağına.**
> "Ne işe yarar" alanı bu yüzden önemli — agent'ın betiği ne zaman çağıracağını
> anladığı tek ipucu odur.

Betiği tek yerden güncellersiniz; **bir sonraki çalıştırma** yeni sürümü kullanır,
imaj yeniden derlenmez.

**Betikler yalnızca "komut çalıştırabilir" yetkisi açık agent'lara verilir.**
Yetkisi kapalı bir agent'ın ortamına hiç kopyalanmazlar. Yeni bir yetki
açılmıyor: o agent betiği bugün de kendisi yazıp çalıştırabiliyordu — değişen
tek şey, çalıştırdığı metnin sizin gözden geçirdiğiniz metin olması.

> ⚠️ **Betiğe token yazmayın.** Betikler şifrelenmez ve agent onları okuyabilir.
> Gizli değer gerekiyorsa ortam değişkeninden okuyun: `"$GIT_TOKEN"`.

---

# Hazır agent'lar

Beş agent kurulu gelir ve talimatları **Agent'lar** ekranından düzenlenebilir:

| Agent | Ne yapar | Yetkileri |
|-------|----------|-----------|
| `analyst` | Task'ı analiz eder, etkilenen dosyaları ve planı çıkarır | yalnızca okur |
| `coder` | Planı uygular, kod yazar ve değiştirir | dosya + komut + ağ |
| `reviewer` | Diff'i inceler, bulgu listesi döner | yalnızca okur |
| `tester` | Değişen kod için test yazar ve çalıştırır | dosya + komut |
| `upgrader` | Bağımlılık/framework yükseltir, breaking change'leri düzeltir | dosya + komut + ağ |

Kendi agent'ınızı da oluşturabilirsiniz. Değiştirdiğiniz hazır bir agent'ı tek
tıkla özgün haline döndürebilirsiniz.

---

# Komutlar

```bash
make help              # tüm komutları listeler

make quickstart        # hazır imajlarla başlat (runner derlenmez)
make up                # kaynaktan derleyip başlat
make down              # durdur (veri korunur)
make clean             # durdur ve TÜM veriyi sil
make restart           # yeniden başlat
make ps                # servis durumları
make logs              # canlı loglar

make dev               # hot reload ile geliştirme modu
make test              # birim testler
make test-integration  # gerçek Postgres'e karşı (stack ayakta olmalı)
make lint              # linter'lar

make psql              # veritabanı kabuğu
make migrate-status    # şema durumu
```

---

# Sorun giderme

<details>
<summary><b>"port already in use" hatası</b></summary>

Varsayılan portlar 3002 / 8080 / 5434. Sizde doluysa `.env` içinden değiştirin:

```env
FRONTEND_PORT=3005
BACKEND_PORT=8085
POSTGRES_PORT=5440
```

Sonra `make restart`. Arayüzün API adresi ve CORS ayarı bu değerlerden
kendiliğinden üretilir — başka bir yeri elle düzeltmeniz gerekmez.

Portu kimin tuttuğunu bulmak için: `lsof -i :3002` (macOS/Linux) veya
`netstat -ano | findstr :3002` (Windows).
</details>

<details>
<summary><b>Arayüz açılıyor ama hiç veri gelmiyor / "sunucuya ulaşılamıyor"</b></summary>

Arayüz ayakta ama API'ye ulaşamıyor. Sırayla:

**1. API gerçekten ayakta mı?**

```bash
curl localhost:8080/health     # {"status":"ok",...}
make ps                        # üçü de "healthy" olmalı
```

**2. Sunucuya kurup uzaktan mı açıyorsunuz?**

En sık neden bu. `.env` içindeki `PUBLIC_HOST` hâlâ `localhost` ise tarayıcınız
**kendi bilgisayarınıza** bağlanmaya çalışıyor. Sunucunun adresini yazın ve
`make restart` yapın:

```env
PUBLIC_HOST=192.168.1.40
```

**3. `.env`'de eski bir `NEXT_PUBLIC_API_URL` satırı var mı?**

Varsa `PUBLIC_HOST`'u **sessizce ezer**. Ters vekil kullanmıyorsanız o satırı
silin. (`make up` bu durumu fark ederse zaten uyarır.)

**4. Tarayıcı konsolunda CORS hatası mı yazıyor?**

`CORS_ORIGINS`, arayüzü açtığınız adresle birebir aynı olmalı — protokol ve port
dahil. Elle yazdıysanız kontrol edin; yazmadıysanız `PUBLIC_HOST` +
`FRONTEND_PORT` doğru olmalı.

Değişiklikten sonra **`make restart` şart**: arayüzün API adresi derleme anında
gömülür, yalnızca yeniden başlatmak yetmez.
</details>

<details>
<summary><b>"SECRET_ENCRYPTION_KEY ayarlanmamış" hatası</b></summary>

`.env` içindeki anahtarı üretmeyi atlamışsınız:

```bash
openssl rand -base64 32
```

Çıktıyı `SECRET_ENCRYPTION_KEY=` satırına yapıştırıp `make restart` yapın.
</details>

<details>
<summary><b>Agent çalıştırınca "runner imajı bulunamadı"</b></summary>

Bu imaj agent'ların içinde koştuğu ortamdır ve elinizde yok. İkisinden biri:

```bash
make quickstart        # hazır imajı çeker
make runner            # ya da kaynaktan derler
```

`make quickstart` kullandıysanız `.env` içindeki `RUNNER_IMAGE` ile çekilen
imajın adı aynı olmalı — elle değiştirdiyseniz kontrol edin.
</details>

<details>
<summary><b>"no matching manifest" / "exec format error"</b></summary>

Çektiğiniz imaj makinenizin mimarisine uymuyor. Yayınlanan imajlar hem `amd64`
hem `arm64` içerir; bu hatayı alıyorsanız muhtemelen **kendi derlediğiniz** bir
imajı başka mimarideki bir makineye taşımışsınız.

Ne olduğunu görmek için:

```bash
docker buildx imagetools inspect ghcr.io/codeeer/agent-coder-runner:latest
```

Çözüm: o makinede `make quickstart` çalıştırın (doğru mimariyi kendisi çeker)
veya `make runner` ile yerinde derleyin.
</details>

<details>
<summary><b>Model listesi boş</b></summary>

Ayarlar → Modeller bölümünde bir sağlayıcı tanımlı olmalı. Tanımlıysa
**Kataloğu yenile** düğmesine basın; hata varsa satırın altında yazar.
</details>

<details>
<summary><b>Jira taraması akış başlatmıyor</b></summary>

Akış ekranında başlangıç düğümüne tıklayın; **Son tarama** satırı kaç task
eşleştiğini ve varsa hatayı gösterir. Sık nedenler:

- Akış **duraklatılmış** (üstte "Etkinleştir" yazıyorsa)
- JQL hiçbir şeyle eşleşmiyor — Jira'da aynı sorguyu deneyin
- Task zaten işlenmiş: aynı task ikinci kez başlatılmaz, ancak Jira'da
  güncellenirse yeniden çalışır
</details>

<details>
<summary><b>Docker container'ları çalışma sonrası kalıyor mu?</b></summary>

Hayır. Her çalıştırma bittiğinde container ve volume silinir. Doğrulamak için:

```bash
docker ps -a | grep opencode-runner    # boş olmalı
docker volume ls | grep run-           # boş olmalı
```
</details>

---

# Güvenlik notları

- **API anahtarları** veritabanında AES-256-GCM ile şifreli saklanır; API
  yanıtlarında ve loglarda asla görünmez (testlerle doğrulanır).
- **Agent container'ları** izole bir Docker ağında çalışır, dışarıya port açmaz,
  CPU/bellek sınırlıdır ve iş bitince silinir.
- **Docker soketi** backend'e mount edilir — bu host üzerinde root eşdeğeri
  yetkidir. Üretimde uzak bir Docker host'a veya kısıtlı bir sokete taşıyın.
- **Kimlik doğrulama v1'de yoktur.** Tek kullanıcılıdır ve internete açık bir
  sunucuda çalıştırılmamalıdır. Şema `user_id` taşır; auth sonradan eklenecek.
- **Dış tetikleme adresleri** anahtar niteliğindedir — paylaşırken dikkat edin,
  gerekirse arayüzden yenileyin.
- **Yayınlanan imajlarda gizli değer yoktur.** `.env` üç derleme bağlamında da
  hariç tutulur; imajlar yayından önce hem katman katman hem de ortam
  değişkenleri üzerinden taranır. Kendiniz doğrulamak isterseniz:

  ```bash
  # İmajın hangi iş akışından çıktığını kanıtlar
  gh attestation verify oci://ghcr.io/codeeer/agent-coder-runner:latest \
     --repo codeeer/agent-coder
  ```

  Hazır imaja güvenmek istemiyorsanız [kaynaktan derleme
  yolu](#-kaynaktan-derleme-yolu) her zaman açık.

---

# Proje yapısı

```
backend/    Go servisi — DAG motoru, sandbox orkestrasyonu, entegrasyonlar
frontend/   Next.js arayüzü — tuval editörü, çalışma izleme, rapor
runner/     Agent'ların içinde koştuğu opencode imajı
deploy/     docker-compose dosyaları
specs/      Özellik başına spec / plan / tasks (spec-driven geliştirme)
plans/      Numaralı, tarihli mimari plan dokümanları
scripts/    Görsel doğrulama ve tema denetimi araçları
.opencode/  Ürünün sunduğu agent tanımları
```

## Nasıl geliştiriliyor?

Bu proje **spec-driven** yürütülüyor: her özellik kod yazılmadan önce
`specs/NNN-ozellik/` altında üç dosyayla tanımlanıyor — *ne* ve *neden*
(`spec.md`), *nasıl* (`plan.md`), *sırayla ne yapılacak* (`tasks.md`).

`tasks.md` dosyalarının sonunda **"Ölçüm"** başlıklı notlar var: geliştirme
sırasında yapılan yanlışlar, nasıl bulundukları ve kök nedenleri. Örneğin akışın
Jira'ya yazdığı yorumun kendisini yeniden tetiklemesi, ya da bir renk token'ının
bir temada geçip diğerinde kalması. Kodun *neden* öyle olduğunu merak ederseniz
cevap çoğu zaman oradadır.

Mimari, konvansiyonlar ve tüm kurallar: **[AGENTS.md](AGENTS.md)**

## Durum

Çalışır durumda. Jira task'ından PR'a kadar tüm zincir gerçek bir Jira sitesi ve
gerçek bir GitHub deposu üzerinde uçtan uca doğrulandı.

Sırada: kimlik doğrulama, `condition` ve `http.request` düğümleri, betikleri
deterministik bir akış düğümü olarak çalıştırma, Bitbucket PR.

## Lisans

MIT
