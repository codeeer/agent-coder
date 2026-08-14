"""mitmproxy akış dosyasını satır-başına-JSON'a çevirir.

ÇEVRİMDIŞI çalışır: canlı yakalama sırasında bir eklenti koşsaydı, ölçümün
ortasında düşmesi koşuyu (ve harcanan parayı) çöpe atardı.

Gövdeler `get_text` ile alınıyor — bu, içerik kodlamasını (gzip, br) AÇAR.
Ham baytlar üzerinde arama yapılsaydı sıkıştırılmış bir gövdedeki kanarya
bulunamaz ve sızıntı gözden kaçardı.

Kullanım (mitmproxy container'ı içinde):
    python3 /cikti/ihrac.py /cikti/akislar.mitm /cikti/akislar.jsonl
"""

import json
import sys

from mitmproxy import io as mio
from mitmproxy.exceptions import FlowReadException


# YANIT gövdeleri için üst sınır. İSTEK gövdeleri ASLA kırpılmaz — sızıntı
# tanımı gereği dışarı GİDEN veridir ve orada durur.
#
# Sınır ölçülerek kondu: Koşu B'de agent internetten bir JDK (190 MB) ve
# Maven indirdi; döküm 455 MB'a çıktı. Kırpma olmadan çözümleme belleğe
# sığmıyor. Kırpılan her gövde kayıtta İŞARETLENİR — sessiz kesme, "arandı
# ama bulunamadı" ile "hiç aranmadı"yı birbirine karıştırırdı.
YANIT_SINIRI = 1_000_000


def metin(mesaj, sinir=None):
    """Gövdeyi metne çevirir; ikili içerik için uzunluk bilgisi döner."""
    if mesaj is None:
        return None
    try:
        govde = mesaj.get_text(strict=False)
    except Exception:  # noqa: BLE001 — hangi hata olursa olsun ölçüm sürmeli
        icerik = mesaj.raw_content or b""
        return f"<ikili: {len(icerik)} bayt>"
    if govde is None:
        return None
    if sinir is not None and len(govde) > sinir:
        return govde[:sinir] + f"\n<KIRPILDI: toplam {len(govde)} karakter>"
    return govde


def cevir(flow):
    kayit = {"tur": flow.type}

    istek = getattr(flow, "request", None)
    if istek is not None:
        kayit["istek"] = {
            "zaman": istek.timestamp_start,
            "yontem": istek.method,
            "sema": istek.scheme,
            "host": istek.pretty_host,
            "port": istek.port,
            "yol": istek.path,
            "basliklar": dict(istek.headers),
            # Sınır YOK: giden veri ölçümün konusu.
            "govde": metin(istek),
        }

    yanit = getattr(flow, "response", None)
    if yanit is not None:
        kayit["yanit"] = {
            "durum": yanit.status_code,
            "basliklar": dict(yanit.headers),
            "govde": metin(yanit, YANIT_SINIRI),
        }

    hata = getattr(flow, "error", None)
    if hata is not None:
        kayit["hata"] = str(hata)

    # TLS açılamadıysa (sertifika sabitleme, vekil reddi) tek ipucu SNI olur.
    sunucu = getattr(flow, "server_conn", None)
    if sunucu is not None:
        kayit["sunucu"] = {
            "sni": getattr(sunucu, "sni", None),
            "adres": list(sunucu.address) if sunucu.address else None,
        }

    return kayit


def main():
    kaynak, hedef = sys.argv[1], sys.argv[2]
    sayac = 0
    with open(kaynak, "rb") as girdi, open(hedef, "w", encoding="utf-8") as cikti:
        okuyucu = mio.FlowReader(girdi)
        try:
            for flow in okuyucu.stream():
                cikti.write(json.dumps(cevir(flow), ensure_ascii=False) + "\n")
                sayac += 1
        except FlowReadException as e:
            # Kesik dosya beklenen bir durum: yakalama koşu bitmeden
            # durdurulmuş olabilir. O ana kadarki akışlar geçerlidir.
            print(f"uyarı: akış dosyası kesik ({e}) — {sayac} akış okundu",
                  file=sys.stderr)
    print(sayac)


if __name__ == "__main__":
    main()
