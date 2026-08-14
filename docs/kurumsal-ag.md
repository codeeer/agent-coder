# Kurumsal ağ: paket deposu ve kök sertifika

Bu belge, agent'ın **kapalı bir kurumsal ağda** çalışması için gereken iki
ayarı, kurulumu **canlıya çıkmadan önce provasını yapmayı** ve bugün neyin
kapsandığını anlatır.

README'deki [SSL inspection yapan kurumsal ağlar](../README.md#ssl-inspection-yapan-kurumsal-ağlar)
ve [Kurumsal paket deposu (Nexus)](../README.md#kurumsal-paket-deposu-nexus)
bölümleri *ne yapılacağını* söyler. Burası *nasıl doğrulanacağını* söyler —
çünkü bu iki ayarın yanlış olduğu, ancak bir çalıştırma yarıda düştüğünde
anlaşılıyor ve hata mesajı çoğu zaman asıl sebebi göstermiyor.

---

## İki ayar

| Ne | Nerede | Ne zaman gerekir |
| --- | --- | --- |
| npm kayıt defteri adresi + kullanıcı adı | Arayüz → **Ayarlar → Paket deposu** | Genel npm deposuna çıkılamıyorsa |
| Maven deposu adresi | Aynı ekran | Maven Central'a çıkılamıyorsa |
| Paket deposu süre sınırı | Aynı ekran | Ulaşılamayan depoda çalıştırma dakikalarca beklemesin diye |
| Parola / token | Aynı ekran → **Kimlik doğrulama** kartı | Depo anonim okumaya kapalıysa |
| Kurumsal kök sertifika | Arayüz → **Ayarlar → Kurumsal ağ** | Ağ HTTPS'i kendi sertifikasıyla açıp yeniden imzalıyorsa |

Kimlik bilgisi **adrese gömülmez** (`https://kullanici:token@…` reddedilir) ve
ortam değişkenine konmaz; container'a `~/.npmrc` olarak 0600 izinle yazılır.

Sertifika **imaja gömülmez**: imaj herkese dağıtılıyor. Arayüzden verilen
sertifika veritabanında saklanır ve her çalıştırmada container'a kopyalanır.

**Dosya seçebilirsiniz.** Kurumsal ekipler sertifikayı farklı biçimlerde
dağıtıyor ve elinizdekinin hangisi olduğu çoğu zaman dosya adından anlaşılmıyor:

| Biçim | Nasıl görünür |
| --- | --- |
| PEM | `-----BEGIN CERTIFICATE-----` ile başlayan metin |
| DER | İkili — metin editöründe anlamsız |
| PKCS#7 (`.p7b`) | Kök ve ara sertifikayı birlikte taşır |

Üçü de kabul edilir; hangisi verilirse verilsin içeride PEM'e çevrilir.
Dönüştürme komutu çalıştırmanız gerekmez. Dosya seçmek **tek başına
kaydetmez**: içerik alana düşer, görürsünüz, sonra "Kaydet"e basarsınız.

**`.env` yolu yedek olarak duruyor.** `RUNNER_EXTRA_CA_CERT` ile kurulmuş
mevcut sistemler çalışmaya devam eder; arayüzden bir sertifika girilirse **o**
geçerli olur. Hangisinin geçerli olduğu ekranda yazar.

---

## Prova: kendi makinenizde kurumsal Nexus

Aşağıdaki adımlar gerçek bir Nexus 3'ü, kendi kök sertifikanızla TLS arkasında
ve **kimlik doğrulama zorunlu** olarak ayağa kaldırır. Amaç kurulumun canlıya
benzemesi: kapalı ağda karşılaşacağınız üç hatanın da burada, kontrollü biçimde
çıkması.

> Bu bir **provadır.** Kendi imzaladığınız kök sertifika ve `admin:kurum123`
> gibi bir parola üretimde kullanılmaz.

### 1. Kök sertifika ve sunucu sertifikası üretin

```bash
mkdir -p /tmp/kurum-ca && cd /tmp/kurum-ca

openssl genrsa -out kurum-ca.key 4096
openssl req -x509 -new -nodes -key kurum-ca.key -sha256 -days 3650 \
  -subj "/C=TR/O=Ornek Kurum/CN=Ornek Kurum Kok CA" -out kurum-ca.pem

openssl genrsa -out nexus.key 2048
openssl req -new -key nexus.key -subj "/C=TR/O=Ornek Kurum/CN=nexus.local" -out nexus.csr
printf "subjectAltName=DNS:nexus.local,DNS:nexus\nextendedKeyUsage=serverAuth\n" > ext.cnf
openssl x509 -req -in nexus.csr -CA kurum-ca.pem -CAkey kurum-ca.key -CAcreateserial \
  -out nexus.crt -days 3650 -sha256 -extfile ext.cnf
```

### 2. Nexus'u başlatın

Runner container'ları `agent-coder_internal` ağına bağlanıyor; Nexus da oraya
bağlanmalı ki container içinden adıyla çözülebilsin.

```bash
docker run -d --name kurum-nexus \
  --network agent-coder_internal --network-alias nexus-backend \
  -e INSTALL4J_ADD_VM_PARAMS="-Xms1200m -Xmx1200m -XX:MaxDirectMemorySize=1500m" \
  sonatype/nexus3:latest

# Açılışı 1-3 dakika sürer.
until docker exec kurum-nexus curl -sf -o /dev/null \
      http://localhost:8081/service/rest/v1/status; do sleep 5; done
```

Başlangıç parolasını alıp bilinen bir parolayla değiştirin:

```bash
PW=$(docker exec kurum-nexus cat /nexus-data/admin.password)
docker exec kurum-nexus curl -s -u "admin:$PW" \
  -X PUT "http://localhost:8081/service/rest/v1/security/users/admin/change-password" \
  -H "Content-Type: text/plain" -d "kurum123"
```

**EULA'yı kabul edin.** Nexus 3.95 Community, kabul edilmeden içerik sunmaz;
atlanırsa üstveri gelir ama tarball indirmede `403 Forbidden` alırsınız:

```bash
D=$(docker exec kurum-nexus curl -s -u admin:kurum123 \
      "http://localhost:8081/service/rest/v1/system/eula" \
      | python3 -c "import sys,json;print(json.load(sys.stdin)['disclaimer'])")
docker exec kurum-nexus curl -s -u admin:kurum123 \
  -X POST "http://localhost:8081/service/rest/v1/system/eula" \
  -H "Content-Type: application/json" -d "{\"accepted\":true,\"disclaimer\":\"$D\"}"
```

### 3. npm proxy deposu ve kapalı anonim erişim

```bash
docker exec kurum-nexus curl -s -u admin:kurum123 \
  -X POST "http://localhost:8081/service/rest/v1/repositories/npm/proxy" \
  -H "Content-Type: application/json" -d '{
    "name":"npm-group","online":true,
    "storage":{"blobStoreName":"default","strictContentTypeValidation":true},
    "proxy":{"remoteUrl":"https://registry.npmjs.org","contentMaxAge":1440,"metadataMaxAge":1440},
    "negativeCache":{"enabled":true,"timeToLive":1440},
    "httpClient":{"blocked":false,"autoBlock":true},
    "npm":{"removeNonCataloged":false}}'

# Kimlik doğrulama yolunun da sınanması için anonim erişimi kapatın.
docker exec kurum-nexus curl -s -u admin:kurum123 \
  -X PUT "http://localhost:8081/service/rest/v1/security/anonymous" \
  -H "Content-Type: application/json" -d '{"enabled":false}'
```

### 4. Önüne TLS koyun

```bash
cat > /tmp/kurum-ca/nginx.conf <<'EOF'
events {}
http {
  server {
    listen 443 ssl;
    server_name nexus.local;
    ssl_certificate     /etc/nginx/certs/nexus.crt;
    ssl_certificate_key /etc/nginx/certs/nexus.key;
    client_max_body_size 0;
    location / {
      proxy_pass http://nexus-backend:8081;
      proxy_set_header Host              $host;
      proxy_set_header X-Forwarded-For   $proxy_add_x_forwarded_for;
      proxy_set_header X-Forwarded-Proto https;
      proxy_read_timeout 300s;
    }
  }
}
EOF

docker run -d --name kurum-nginx \
  --network agent-coder_internal --network-alias nexus.local \
  -v /tmp/kurum-ca/nginx.conf:/etc/nginx/nginx.conf:ro \
  -v /tmp/kurum-ca/nexus.crt:/etc/nginx/certs/nexus.crt:ro \
  -v /tmp/kurum-ca/nexus.key:/etc/nginx/certs/nexus.key:ro \
  nginx:alpine
```

`X-Forwarded-Proto` **atlanmamalı**: Nexus, npm'e döndürdüğü tarball
adreslerini bu başlıktan türetiyor. Yoksa metadata `http://localhost:8081/…`
der ve indirme container içinden düşer.

### 5. Üç hâli sırayla görün

```bash
CA=/tmp/kurum-ca/kurum-ca.pem

# 1) Sertifika tanıtılmamış  -> TLS düşer
docker run --rm --network agent-coder_internal curlimages/curl:latest \
  -s -o /dev/null -w '%{http_code} %{errormsg}\n' https://nexus.local/repository/npm-group/lodash

# 2) Sertifika var, kimlik yok -> 401
docker run --rm --network agent-coder_internal -v $CA:/ca.pem:ro curlimages/curl:latest \
  -s -o /dev/null -w '%{http_code}\n' --cacert /ca.pem https://nexus.local/repository/npm-group/lodash

# 3) Sertifika + kimlik -> 200
docker run --rm --network agent-coder_internal -v $CA:/ca.pem:ro curlimages/curl:latest \
  -s -o /dev/null -w '%{http_code}\n' --cacert /ca.pem -u admin:kurum123 \
  https://nexus.local/repository/npm-group/lodash
```

Beklenen çıktı sırasıyla:

```
000 SSL certificate ... unable to get local issuer certificate
401
200
```

### 6. Agent Coder'ı bu depoya bağlayın

Arayüzden **Ayarlar → Paket deposu**:

- npm kayıt defteri adresi: `https://nexus.local/repository/npm-group/`
- Kullanıcı adı: `admin`
- **Kimlik doğrulama** kartı → parola: `kurum123`

Sonra **Ayarlar → Kurumsal ağ**: "Dosya seç" ile `/tmp/kurum-ca/kurum-ca.pem`
dosyasını verin (veya içeriğini yapıştırın) ve "Kaydet"e basın. Sertifikanın
sahibi, imzalayanı ve bitiş tarihi hemen üstünde görünecek.

**Yeniden başlatmaya gerek yok.**

### 7. Runner imajıyla doğrulayın

Bu adım, bir çalıştırma başlatmadan tam olarak container içinde olacak şeyi
sınar:

```bash
CA=/tmp/kurum-ca/kurum-ca.pem
printf 'registry=https://nexus.local/repository/npm-group/\n//nexus.local/repository/npm-group/:_auth=%s\n' \
  "$(printf 'admin:kurum123' | base64)" > /tmp/kurum-ca/npmrc

docker run --rm --network agent-coder_internal --entrypoint sh \
  -v /tmp/kurum-ca/npmrc:/home/agent/.npmrc:ro \
  -v $CA:/etc/ssl/certs/kurumsal-ca.pem:ro \
  -e NODE_EXTRA_CA_CERTS=/etc/ssl/certs/kurumsal-ca.pem \
  --user 10001 agent-coder/opencode-runner:node-24.13.0 -c '
    mkdir -p /tmp/p && cd /tmp/p && echo "{\"name\":\"t\",\"version\":\"1.0.0\"}" > package.json
    npm install express --no-audit --no-fund >/dev/null 2>&1
    node -e "const l=require(\"/tmp/p/package-lock.json\");
      Object.entries(l.packages).filter(([k])=>k).slice(0,3)
        .forEach(([k,v])=>console.log(k,\"->\",v.resolved))"'
```

Her satır `https://nexus.local/...` ile başlıyorsa kurulum doğru. Nexus
tarafından da bakılabilir:

```bash
docker exec kurum-nexus curl -s -u admin:kurum123 \
  "http://localhost:8081/service/rest/v1/components?repository=npm-group" \
  | grep -c '"name"'

docker exec kurum-nexus sh -c \
  "grep 'npm-group.*\.tgz' /nexus-data/log/request.log | tail -3"
```

Erişim logundaki satırlarda kullanıcı alanı `-` değil **`admin`** olmalı:
tarball istekleri de kimlikli gidiyor demektir.

### Temizlik

```bash
docker rm -f kurum-nexus kurum-nginx
rm -rf /tmp/kurum-ca
```

**Ayarlar → Kurumsal ağ** bölümündeki sertifikayı da "Sıfırla" ile kaldırın.

---

## Kök sertifika bugün neyi kapsıyor

`RUNNER_EXTRA_CA_CERT` dolu olduğunda dosya container'a
`/etc/ssl/certs/kurumsal-ca.pem` yoluna salt okunur bağlanır ve iki ortam
değişkeniyle gösterilir. Runner imajında ölçülen sonuç:

| Araç | Durum | Nereden geliyor |
| --- | --- | --- |
| Node / npm | ✅ tanıyor | `NODE_EXTRA_CA_CERTS` |
| git | ✅ tanıyor | `GIT_SSL_CAINFO` |
| curl | ✅ tanıyor | `CURL_CA_BUNDLE` |
| Backend'in kendi çağrıları | ✅ tanıyor | Go güven havuzuna eklenir |
| Maven | ✅ tanıyor | JDK güven listesinin kopyasına eklenir, `MAVEN_OPTS` ile gösterilir |
| Doğrudan `java -jar` | ❌ tanımıyor | Aşağıya bakın |

Sertifika güvenilen kök listesine **eklenir**, yerine geçmez: genel
sertifikalar geçerli kalmaya devam eder.

**Değiştirmek yeniden başlatma gerektirmez.** Arayüzden yeni bir sertifika
kaydedildiğinde hem sonraki çalıştırmalar hem de ürünün kendi giden çağrıları
yeni sertifikayı kullanır.

Kalan boşluk — **doğrudan `java` çağrıları**:

Sertifika Maven'a `MAVEN_OPTS` ile gösteriliyor, yani `mvn` ve `mvnw`
kapsanıyor. Agent doğrudan `java -jar` çalıştırırsa kurumsal sertifika
tanınmaz.

Bu bilinçli bir seçim: alternatif olan `JAVA_TOOL_OPTIONS` her JVM'i kapsardı
ama her başlangıçta stderr'e bir satır basıyor ve o satır **agent'ın okuduğu
her araç çıktısına** düşerdi. Aynı gerekçeyle npm'in `always-auth` uyarısı da
kaldırılmıştı (spec 014).

---

## Hata → sebep

| Gördüğünüz | Sebep |
| --- | --- |
| `unable to get local issuer certificate` / `UNABLE_TO_VERIFY_LEAF_SIGNATURE` | Kök sertifika tanıtılmamış. **Ayarlar → Kurumsal ağ**'dan girin. |
| Sertifikayı kaydederken "geçerli bir sertifika değil" | İçerik sertifika içermiyor. Dosyayı "Dosya seç" ile verin — ikili biçimler yapıştırılamaz. |
| Ekranda "süresi dolmuş" yazıyor | Sertifika kabul edildi ama geçerlilik tarihi geçmiş. Kurum yenilemişse yeni dosyayı verin. |
| npm `401 Unauthorized` | Kullanıcı adı ayarda ama parola/token **Kimlik doğrulama** kartında yok (ya da tersi). İkisi birden dolu olmalı. |
| Üstveri geliyor, `.tgz` indirmede `403 Forbidden` | Nexus tarafı. Community sürümde EULA kabul edilmemiş olabilir; depo izinlerini de kontrol edin. |
| npm istekleri `localhost:8081`'e gidiyor | TLS sonlandıran vekilde `X-Forwarded-Proto` / `Host` başlıkları eksik. |
| `çalışma ortamı hazırlanamadı` | Genelde depo klonlaması; önce sertifikayı, sonra Git erişimini kontrol edin. |
| Agent `--registry` verip genel depoya kaçıyor | Adres ayarda tanımlı değil. Tanımlıysa agent'ın talimatına "adresi değiştirme" notu ekleniyor. |

---

## Değiştirilmeyecek olan

**TLS doğrulamasını kapatan bir ayar yoktur ve eklenmeyecektir.**
`NODE_TLS_REJECT_UNAUTHORIZED=0`, `npm config set strict-ssl false` veya
`git config http.sslVerify false` sorunu görünmez yapar, ortadan kaldırmaz.
Kurumsal ağın çözümü kök sertifikayı **tanıtmaktır**.

---

## Java ve Maven

Runner imajında **iki JDK** var:

| Yol | Sürüm |
| --- | --- |
| `/opt/java/25` | **varsayılan** — `java` ve `mvn` bunu kullanır |
| `/opt/java/17` | |

Yollar mimariden bağımsız sabit bağlardır; Temurin'in gerçek dizini
`…-amd64`/`…-arm64` ile biter ve doğrudan kullanılmamalıdır.

**Sürüm değiştirmek `java` ve `mvn` için AYNI DEĞİL** — bu ayrım agent'ın
talimatında da yazılıdır:

```bash
JAVA_HOME=/opt/java/17 mvn -B test     # Maven için çalışır
/opt/java/17/bin/java -version         # doğrudan java için tam yol
JAVA_HOME=/opt/java/17 java -version   # ÇALIŞMAZ — java'yı PATH bulur
```

### Maven deposu

**Ayarlar → Paket deposu → Maven deposu adresi**, örneğin:

```
https://nexus.sirket.local/repository/maven-public/
```

Kimlik bilgisi npm ile **ortaktır**; ikinci bir kullanıcı adı veya parola
istenmez. Adres tanımlıyken container'a `~/.m2/settings.xml` yazılır ve
`<mirrorOf>*</mirrorOf>` ile **projenin kendi `pom.xml`'inde ilan ettiği
depolar dahil** her şey kuruma yönlendirilir. Parola dosyanın içindedir ve
0600 izinle yazılır; ortam değişkenine konmaz.

### Süre sınırı

**Ayarlar → Paket deposu → Paket deposu süre sınırı** (varsayılan 60 saniye).
npm ve Maven için aynı değer geçerlidir.

Neden var: paket yöneticilerinin varsayılanı tek istek için beş dakika ve
üstüne birkaç kez yeniden deniyorlar. Ulaşılamayan bir depoya karşı ölçüldü:

| | npm | Maven |
| --- | --- | --- |
| Ayar öncesi davranış | **295 sn** | **98 sn** |
| Varsayılan ayarla (60 sn) | **96 sn** | — |
| Kısa ayarla | 104 sn (10 sn) | 31 sn (3 sn) |

Yani ulaşılamayan bir depoda çalıştırma, beş dakika yerine bir buçuk dakikada
sonuca varıyor.

Yavaş bir bağlantıda büyük paketler kesiliyorsa ayarı yükseltin (600 saniyeye
kadar).
