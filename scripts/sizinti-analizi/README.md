# Sızıntı analizi düzeneği

opencode'un runner sandbox'ında **nereye ne gönderdiğini** ölçen geçici
düzenek. Ürünün parçası değildir; `veri-sizintisi-analizi` branch'inde yaşar.

Bulgular: [`docs/veri-sizintisi-analizi.md`](../../docs/veri-sizintisi-analizi.md)

## Neden iki katman

**Vekil (mitmproxy)** ne gönderildiğini söyler — tam URL, başlıklar, gövde.
Ama trafiği yönlendirir, mecbur etmez: vekili yok sayan bir istemci doğrudan
çıkar ve dökümde hiç görünmez.

**Köprü yakalaması (tcpdump)** ne kadarının vekilden geçtiğini söyler.
İçerik yok, ama kaçan bağlantı varsa yalnız burada görünür.

Tek katmanla çalışılsaydı "vekilde bir şey yok" sonucu iki farklı şeyi
aynı anda ifade ederdi: *göndermedi* ve *vekilden geçmedi*. Bunlar aynı şey
değil.

## Kanarya yöntemi

Sızıntı sorusu göz kararıyla cevaplanamaz — bir koşu on binlerce satır
üretiyor. Bunun yerine sızıntının çıkabileceği her yere benzersiz, yüksek
entropili bir dizge konur ve dökümde **mekanik olarak** aranır.

| Kanarya | Nerede | Neyi ölçer |
|---|---|---|
| `KANARYA_KAYNAK_KODU` | depodaki Java dosyasında | kaynak kodun nereye gittiği |
| `KANARYA_DEPO_SIRRI` | depodaki `.env.ornek` içinde | depoya karışmış sırlar |
| `KANARYA_GIT_TOKEN` | depo erişim parolası (gerçekten kullanılır) | kimlik bilgisi sızması |
| `KANARYA_PROMPT` | görev metninde | prompt'un nereye gittiği |
| `KANARYA_DOSYA_ADI` | dosya adında | dosya adlarının / dizin listesinin sızması |

## Sıra

```sh
./kur.sh                      # kanaryalar, kanarya deposu, mitmproxy, CA
# .env'e basılan iki satırı ekleyin, sonra:  make down && make up

./yakala.sh başlat kosu-a     # köprü yakalaması (koşudan ÖNCE)
./izle.sh runner-ip-a &       # runner container IP'lerini kaydet
# … koşuyu başlatın, bitmesini bekleyin …
./yakala.sh durdur

./analiz.sh kosu-a            # ham sayılar → cikti/cozumleme-*.md
```

## Temizlik

```sh
docker compose -f docker-compose.yml down
docker rm -f sizinti-tcpdump
# .env'den RUNNER_HTTP_PROXY ve RUNNER_EXTRA_CA_CERT satırlarını silin
```

`cikti/` git'e girmez: içinde gerçek API anahtarları geçen ham trafik var.
