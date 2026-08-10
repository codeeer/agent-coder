# Spec: Betikler — Agent'ın Elindeki Standart Prosedürler

- **Spec no:** 012
- **Kapsam:** Merkezî betik kütüphanesi ve betiklerin agent container'ına konması
- **Durum:** Uygulandı
- **Sürüm:** 2026-08-11

---

## Problem

Bir agent adımı **her seferinde yeniden karar veriyor**. Keşif gerektiren işlerde
(bir hatayı bulmak, bir özelliği yazmak) doğru davranış budur.

Ama bazı işler keşif değil **prosedür**: bağımlılığı yükselt, geçişi uygula,
üretim öncesi kontrol listesini koştur. Bunlarda doğaçlama kazanç değil **risk** —
aynı akış iki kez çalıştığında iki farklı komut dizisi üretebilir. Model bugün
`npm update` yazar, yarın `npm install paket@latest`; ikisi de "yükseltme"dir ama
sonuçları farklıdır.

Betik zaten bunun için var: yazılmış, gözden geçirilmiş, her çalıştığında aynı
şeyi yapıyor. Eksik olan, onu agent'ın eline **kalıcı ve merkezî** biçimde
vermenin yolu.

Bugün iki kötü seçenek var: talimatın içine komutları uzun uzun yazmak (model
yine de kendi yorumunu katar), ya da her depoya betiği elle koymak (on depoda on
kopya, biri güncellenir dokuzu unutulur).

## Amaç

Bir kez yazılan betiği **birden fazla agent'a** atayabilmek ve agent'ın onu
çağırabilmesi.

> Model **ne zaman** çağıracağına karar verir; **ne yapacağına** betik karar
> verir.

Karar hâlâ modelde — çünkü hangi projede yükseltmenin gerektiğini, hangi hatanın
düzeltilmesi gerektiğini bilen o. Ama karar verdiği anda çalışan şey, her
seferinde aynı metin.

## Kullanıcı hikâyeleri

1. **Ekip olarak** yükseltme prosedürümüzü bir kez yazıp tüm projelerde aynı
   şekilde çalıştırmak istiyorum.
2. **Ekip olarak** betiği tek yerden güncellediğimde tüm agent'ların yeni sürümü
   kullanmasını istiyorum.
3. **Kullanıcı olarak** hangi agent'ın hangi betiklere eriştiğini görmek
   istiyorum.
4. **Kullanıcı olarak** bir agent'ın bilmediği betiği çağıramayacağından emin
   olmak istiyorum.
5. **Kullanıcı olarak** betiğin bir güvenlik açığı açmadığından emin olmak
   istiyorum.

## Kabul kriterleri

### Çalışma

- [x] Betik Ayarlar'dan yazılır, birden fazla agent'a atanır.
- [x] Atanmış betik container içinde `/home/agent/scripts/<ad>.sh` yolunda,
      **çalıştırılabilir** (`0o755`) olarak bulunur.
- [x] Agent talimat dosyasında betiklerin **tam yolu ve açıklaması** yazar;
      bilmediği bir dosyayı çağırması beklenemez.
- [x] Betik güncellendiğinde **sonraki çalıştırma** yeni içeriği kullanır; imaj
      yeniden derlenmez.
- [x] Betik dosyaları klonlanan deponun **dışında** durur; kullanıcının diff'inde
      görünmezler.

### Güvenlik

- [x] Betikler yalnızca **bash yetkisi açık** agent'lara kopyalanır. Yetkisi
      kapalı bir agent'ın container'ında betik dosyası **bulunmaz**.
- [x] Yetki kuralları (`BuildPermissions`) **değişmez**; bu özellik hiçbir yeni
      yetenek açmaz.
- [x] Betik içeriği gizli değer taşımaz; arayüz bunu açıkça yazar.

### Arayüz

- [x] Betik listesi sayfalanır.
- [x] Aynı ada ikinci betik kaydedilemez.
- [x] Bir agent'a atanmış betik silinirse atama da düşer, çalıştırma bozulmaz.

## Kapsam dışı

- **`script.run` akış düğümü.** Ertelendi — gerekçe K4.
- **"Bash kapalı ama şu betiği çalıştırabilir" modu.** Düşürüldü — gerekçe K2.
- **Betiklerin sürüm geçmişi.** Bugün son hâli saklanır.
- **Betik başına ayrı zaman sınırı.** Çalıştırmanın genel süre sınırı zaten sarıyor.
- **Yerel dosyadan veya depodan betik okuma.** Kaynak tek: kütüphane.

## Kararlar

**K1 — Betik agent'a bağlıdır, adıma değil.** MCP erişimindeki (spec 011 K1) ve
`allowBash`/`allowEdit` yetkilerindeki kararın aynısı: "bu agent neler yapabilir"
sorusunun parçası. Adıma bağlansaydı "reviewer kod değiştiremez" güvencesi her
adımda ayrı denetlenmek zorunda kalırdı.

**K2 — "Kısıtlı bash" fikri yanlış olduğu için düşürüldü.** Cazip görünüyordu:
bash'i kapalı tutup yalnızca atanmış betiklere izin vermek. Motorun yetki
sözlüğü bunu destekliyor gibi duruyor (`bash` için desen haritası kabul ediyor).

Ama eşleşme **ham komut metnine** yapılıyor; bash ayrıştırması yok. Yani
`/home/agent/scripts/upgrade.sh; env` deseni geçer — ve `env` çıktısında
`GIT_TOKEN` ile `AGENT_CODER_PROVIDER_KEY` var
(`internal/runner/opencode/runner.go:250-262`). Bugün tamamen kapalı olan bir
kapıyı açmış olurduk.

**Yetki desenleri güvenlik sınırı olarak kullanılmaz.** Bu kural bu spec'in
dışına da geçerli.

**K3 — Betik yalnızca bash yetkisi olan agent'a gider; kazanç determinizm,
güvenlik deltası sıfır.** `AllowBash` **açık** bir agent bugün zaten
`cat > x.sh && bash x.sh` ile istediği betiği yazıp çalıştırabiliyor. Ona hazır
betik vermek yeni bir yetenek açmıyor; yalnızca çalıştırdığı metnin **bizim
gözden geçirdiğimiz metin** olmasını sağlıyor.

Bu yüzden bu özellik "yeni bir kapı" değil, var olan kapıdan **tahmin
edilebilir** bir şeyin geçmesi.

**K4 — `script.run` düğümü ertelendi.** Yeni bir güven sınırı açtığı için değil:
betiği yazan kişi zaten agent tanımlayabilen kişi ve sistemde ikinci bir yetki
seviyesi yok. Ertelenme sebebi kapsam — asıl amaç "agent'ın eline betik vermek"
ve düğüm bunu karşılamıyor; ayrıca entrypoint modu, container'dan dosya okuma,
`Pusher` ayrıştırması ve şablonlu `Branch` olmak üzere dört ayrı mekanizma
gerektiriyor. Önce dar olan çalışsın.

**K5 — Betikler gizli değer değildir.** Şifrelenmezler, arayüzde tam metin
görünürler. Gizli değer betiğin içine değil, ortam değişkenine konur. Aksini
yapmak yanlış bir güvenlik hissi verirdi: betik zaten container içinde düz metin
olarak duruyor ve agent onu okuyabiliyor.

**K6 — Betikler `/work` altına konamaz.** Orası klonlama hedefi ve boş olmak
zorunda (`internal/runner/config_test.go` bunu kilitliyor). Ayrıca bizim
dosyalarımız kullanıcının diff'inde görünürdü — spec 003'ün davranış kuralı.

## Açık uçlar

- Klonlanan deponun içindeki bir `.opencode/` yapılandırması kendi komutlarını
  tanımlayabilir. Kullanıcının kendi deposu olduğu için bugün kabul edilebilir;
  çok kullanıcılı kuruluma geçilirken kapatılmalı (spec 011 ile aynı açık uç).
