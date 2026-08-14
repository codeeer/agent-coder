# Sahte kurumsal Bitbucket

Spec 021'in (gruptan toplu proje ekleme) doğrulama düzeneği.

## Neden var

Toplu ekleme yalnızca **kendi sunucusunda çalışan** kurumsal Bitbucket için
geçerli. Bitbucket Data Center kurulumu bir lisans anahtarı istiyor ve o
anahtar bir Atlassian hesabıyla üretiliyor; bu depoda öyle bir örnek yok.

Bu sunucu, o boşluğu **kısmen** dolduruyor.

## Neyi kanıtlar, neyi kanıtlamaz

**Kanıtlar** — hepsi bizim kodumuz:

- grup adresinin çözülmesi (context path, kişisel alan, fazlalık yollar)
- sayfalama döngüsü (`isLastPage` / `nextPageStart`) — liste bilerek iki
  sayfaya bölünüyor
- `links.clone` içinden HTTP adresinin seçilmesi
- adrese gömülü kullanıcı adının ayıklanması (adresler bilerek `ahmet@` ile
  veriliyor)
- varsayılan branch'in **depodan** okunması — her deponun branch'i farklı
  (`develop`, `main`, `release/2026`)
- mükerrer denetimi, arşivli kaydın seçili gelmemesi, arayüzün tüm durumları

**Kanıtlamaz:** Atlassian'ın gerçek yanıtını. Varsayımımız yanlışsa bu sunucu
aynı yanlışı tekrarlar. Buradan geçen bir doğrulama *"gerçek sunucuda
çalışıyor"* diye sunulamaz.

## Çalıştırma

```bash
python3 scripts/sahte-bitbucket/sunucu.py
```

Depolar geçici bir dizinde üretilir; sunucu kapanınca iz kalmaz.

Ardından üründe bir Bitbucket erişimi tanımlayın (adres:
`http://host.docker.internal:7990`, kullanıcı adı ve anahtar herhangi bir şey
olabilir — bu sunucu kimlik doğrulamıyor) ve **Projeler → Gruptan içe
aktar**'da şu adresi verin:

```
http://host.docker.internal:7990/projects/ODEME
```

> `host.docker.internal` backend container'ının makineye ulaşma yolu. Sunucuyu
> makinede çalıştırıp ürünü container'da bıraktığımız için gereken bu.
