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
| 🔗 **Entegrasyonlar** | Jira (tetikleyici + yorum), GitHub / Bitbucket / genel Git (push + PR) |
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
git clone https://github.com/<kullanici>/agent-coder.git
cd agent-coder
```

## 3. Ayar dosyasını oluşturun

```bash
make env        # .env.example dosyasını .env olarak kopyalar
```

Şimdi `.env` dosyasını bir editörle açıp **tek bir zorunlu değeri** üretin:

```bash
# Şifreleme anahtarı — API anahtarlarınız veritabanında bununla şifrelenir.
openssl rand -base64 32
```

Çıkan değeri `.env` içindeki `SECRET_ENCRYPTION_KEY=` satırına yapıştırın:

```env
SECRET_ENCRYPTION_KEY=çıktıyı+buraya+yapıştırın=
```

> **Bu anahtarı kaybetmeyin.** Kaydedilmiş tüm API anahtarları onunla şifrelenir;
> değiştirirseniz eskiler çözülemez ve yeniden girilmesi gerekir.

Geri kalan her şeyin makul bir varsayılanı var. Portlar sizde doluysa yine `.env`
üzerinden değiştirebilirsiniz (`FRONTEND_PORT`, `BACKEND_PORT`, `POSTGRES_PORT`).

**Model API anahtarınızı .env'e yazmanıza gerek yok** — arayüzden gireceksiniz.

## 4. Agent çalıştırma imajını derleyin

```bash
make runner
```

Bu, agent'ların içinde çalışacağı opencode imajını üretir. Bir kez yapılır;
ilk seferde birkaç dakika sürebilir.

## 5. Başlatın

```bash
make up
```

Üç servis ayağa kalkar ve veritabanı şeması kendiliğinden uygulanır:

```
  Arayüz : http://localhost:3002
  API    : http://localhost:8080/health
```

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

**Ayarlar → LLM sağlayıcılar → Sağlayıcı ekle**

En kolayı [OpenRouter](https://openrouter.ai/keys): ücretsiz hesap açıp bir API
anahtarı alın (`sk-or-v1-...`). Tek anahtarla Claude, GPT, Gemini, Llama ve
yüzlerce modele erişirsiniz.

Anahtarı yapıştırıp kaydedin. Sistem anahtarı **kaydetmeden önce doğrular** ve
model kataloğunu (300+ model, fiyatlarıyla) çeker.

> Anahtarınız veritabanında AES-256-GCM ile şifrelenir. Arayüzde bir daha tam
> haliyle gösterilmez — yalnızca son 4 karakteri görünür.

Kurumsal bir LiteLLM proxy'niz varsa onu da ekleyebilirsiniz; birden fazla
sağlayıcı aynı anda tanımlı olabilir ve her agent adımı hangisini kullanacağını
ayrı seçer.

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
**Ayarlar → Git erişimleri**'nden bir erişim tanımlayın:

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

Varsayılan olarak 5 dakikada bir taranır (**Ayarlar → Jira tetikleyici**).
Anında işlenmesini isterseniz aynı panelde görünen **webhook adresini** Jira'ya
tanımlayın — tarama yedek yol olarak kalır, ikisi aynı korumadan geçer.

Akışı geçici olarak durdurmak için akış ekranındaki **Duraklat** düğmesi:
otomatik tetikleme durur, elle çalıştırma açık kalır.

---

# Dış araçlar (MCP)

Agent Coder, [Model Context Protocol](https://modelcontextprotocol.io) ile **iki
yönde** de konuşur.

## Agent'larınız dış araçlara erişsin

**Ayarlar → Dış araçlar (MCP) → Sunucu ekle.** Uzak (HTTP/SSE) bir MCP sunucusu
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

**Ayarlar → Agent Coder'ı dışarıya aç** bölümündeki adresi Claude Desktop veya
Cursor yapılandırmanıza ekleyin. Üç araç sunulur: `akislari_listele`,
`akis_calistir`, `calisma_durumu`.

> Adres bir anahtardır — bilen herkes akışlarınızı başlatabilir. Sızdıysa aynı
> bölümden yenileyin.

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

make up                # başlat
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

Sonra `make down && make up`.
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

```bash
make runner
```

komutunu çalıştırmayı atlamışsınız. Bu imaj agent'ların içinde koştuğu ortamdır.
</details>

<details>
<summary><b>Model listesi boş</b></summary>

Ayarlar → LLM sağlayıcılar bölümünde bir sağlayıcı tanımlı olmalı. Tanımlıysa
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

Sırada: kimlik doğrulama, `condition` ve `http.request` düğümleri, Bitbucket PR.

## Lisans

MIT
