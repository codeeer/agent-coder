"""Vekil dökümünü çözümler: hedef envanteri + kanarya araması.

Bu betik raporun SAYILARINI üretir. İki soruyu ayrı ayrı cevaplıyor:

  1. Trafik NEREYE gitti?  — host envanteri
  2. İÇİNDE NE vardı?      — kanarya araması

İkisi ayrı tutuluyor çünkü ayrı şeyler kanıtlıyorlar: bilinmeyen bir hedef,
gövdesi boş olsa bile bir bulgudur; tanıdık bir hedefe giden kanarya ise
hedefin tanıdıklığına rağmen bulgudur.

Kullanım: python3 analiz.py cikti/akislar.jsonl cikti/kanaryalar.env
"""

import base64
import collections
import json
import re
import sys
import urllib.parse


def base64_bicimleri(deger):
    """Bir dizgenin base64 çıktısı içinde aranabilecek biçimleri.

    KURU KOŞUDA ÖLÇÜLEREK EKLENDİ: kanarya token'ı `Authorization: Basic`
    başlığında base64 içinde taşınıyordu ve düz arama onu BULAMADI. Yalnız
    düz aramaya güvenilseydi rapor, gerçekten gönderilmiş bir kimlik bilgisi
    için "isabet yok" derdi.

    Aranan dizge daha uzun bir metnin ORTASINDA kodlanmış olabilir; base64
    üç baytta bir hizalandığı için üç kaydırmanın üçü de denenir ve
    hizalamadan etkilenen kenar karakterleri atılır.
    """
    ham = deger.encode()
    bicimler = set()
    for kaydirma in range(3):
        kodlu = base64.b64encode(b"\x00" * kaydirma + ham).decode()
        bas = (kaydirma * 8 + 5) // 6
        parca = kodlu[bas:].rstrip("=")
        # Sondaki karakter de eksik baytlardan etkilenir.
        if len(parca) > 4:
            bicimler.add(parca[:-1])
    return bicimler


def aranacak_bicimler(deger):
    """Bir kanaryanın telde görünebileceği tüm biçimleri."""
    bicimler = {deger, urllib.parse.quote(deger)}
    bicimler |= base64_bicimleri(deger)
    return {b for b in bicimler if len(b) >= 8}


def sadelestir(metin):
    """Harf ve rakam dışındaki her şeyi atar.

    NEDEN: JSON kaçışı ve SSE parçalanması bir dizgeyi ikiye bölebiliyor
    (`"KANARY","AKAYNAK…"`). Model yanıtı akış halinde geldiğinde bu gerçek
    bir olasılık; sadeleştirilmiş metinde arama bu bölünmeleri de yakalar.
    """
    return re.sub(r"[^A-Za-z0-9]", "", metin)


def kanaryalari_oku(yol):
    kanaryalar = {}
    with open(yol, encoding="utf-8") as f:
        for satir in f:
            satir = satir.strip()
            if not satir or satir.startswith("#") or "=" not in satir:
                continue
            ad, deger = satir.split("=", 1)
            if ad.startswith("KANARYA_") and ad != "KANARYA_ID":
                kanaryalar[ad] = deger
    return kanaryalar


def main():
    akis_yolu, kanarya_yolu = sys.argv[1], sys.argv[2]
    kanaryalar = kanaryalari_oku(kanarya_yolu)

    hostlar = collections.Counter()
    host_yollari = collections.defaultdict(collections.Counter)
    isabetler = collections.defaultdict(lambda: collections.defaultdict(collections.Counter))
    hatalar = collections.Counter()
    biciml_onbellek = {}
    toplam = 0

    with open(akis_yolu, encoding="utf-8") as f:
        for satir in f:
            satir = satir.strip()
            if not satir:
                continue
            kayit = json.loads(satir)
            toplam += 1

            istek = kayit.get("istek") or {}
            host = istek.get("host") or (kayit.get("sunucu") or {}).get("sni") or "<bilinmiyor>"
            hostlar[host] += 1
            yol = (istek.get("yol") or "").split("?")[0]
            host_yollari[host][f"{istek.get('yontem','?')} {yol}"] += 1

            if kayit.get("hata"):
                hatalar[f"{host}: {kayit['hata']}"] += 1

            # Her bölüm ayrı aranıyor: bulgunun İSTEKTE mi YANITTA mı olduğu
            # sızıntı sorusunun cevabını değiştirir. İstekte olması "biz
            # gönderdik", yanıtta olması "karşı taraf geri yolladı" demektir.
            bolumler = {
                "istek-başlığı": json.dumps(
                    (kayit.get("istek") or {}).get("basliklar") or {}, ensure_ascii=False),
                "istek-gövdesi": (kayit.get("istek") or {}).get("govde") or "",
                "yanıt-başlığı": json.dumps(
                    (kayit.get("yanit") or {}).get("basliklar") or {}, ensure_ascii=False),
                "yanıt-gövdesi": (kayit.get("yanit") or {}).get("govde") or "",
            }

            for ad, deger in kanaryalar.items():
                if not deger:
                    continue
                bicimler = biciml_onbellek.setdefault(ad, aranacak_bicimler(deger))
                sade_deger = sadelestir(deger)
                for etiket, icerik in bolumler.items():
                    if not icerik:
                        continue
                    if any(b in icerik for b in bicimler):
                        isabetler[ad][host][etiket] += 1
                    elif sade_deger and sade_deger in sadelestir(icerik):
                        isabetler[ad][host][etiket + " (parçalanmış)"] += 1

    yaz = print
    yaz(f"# Vekil dökümü çözümlemesi\n")
    yaz(f"Toplam akış: {toplam}\n")

    yaz("## Hedef envanteri\n")
    yaz(f"{'akış':>6}  host")
    for host, adet in hostlar.most_common():
        yaz(f"{adet:>6}  {host}")
    yaz("")

    yaz("## Host başına uç noktalar\n")
    for host, _ in hostlar.most_common():
        yaz(f"### {host}")
        for yol, adet in host_yollari[host].most_common(15):
            yaz(f"  {adet:>5}  {yol}")
        yaz("")

    yaz("## Kanarya araması\n")
    for ad, deger in kanaryalar.items():
        if ad not in isabetler:
            yaz(f"- {ad}: **isabet yok**  (aranan: {deger})")
            continue
        yaz(f"- {ad}: **İSABET**  (aranan: {deger})")
        for host, yonler in isabetler[ad].items():
            for yon, adet in yonler.items():
                yaz(f"    - {host} — {adet} akış — {yon}")
    yaz("")

    if hatalar:
        yaz("## Akış hataları\n")
        for h, adet in hatalar.most_common(20):
            yaz(f"  {adet:>5}  {h}")


if __name__ == "__main__":
    main()
