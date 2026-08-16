#!/usr/bin/env python3
"""
Bekçi mutasyonu — "bu kural gerçekten korunuyor mu?" sorusunu makineye sordurur.

Üretim kodundaki her `if <koşul> { return ...Err... }` bloğunu TEK TEK kaldırır
ve o paketin testlerini koşar. Test hâlâ geçiyorsa o kural KORUNMUYOR demektir:
ileride biri o bekçiyi kaldırdığında hiçbir şey uyarmaz.

NEDEN GEREKLİ: bir testin geçmesi, bir şeyi koruduğunu kanıtlamaz. Testler kod
yazıldıktan sonra eklendiğinde ilk denemede geçerler ve bu "çalışıyor" gibi
görünür. Gerçek ölçüt, kodu bozunca kırmızıya dönmeleridir. Bu betik o ölçütü
tüm pakete uygular.

────────────────────────────────────────────────────────────────────────────
EMNİYET

Bu betik kaynak dosyaları DEĞİŞTİRİYOR. Üç kural onu tehlikesiz kılıyor ve
üçü de burada, kodda zorlanıyor — belgede değil:

 1. ÇALIŞMA AĞACINA ASLA YAZMAZ. Kaynağın tek kullanımlık bir kopyasını kendisi
    çıkarır ve yalnızca onun üzerinde çalışır. Elle verilen bir hedef kabul
    edilmez. Gerekçe: betik `kill -9`, OOM ya da elektrik kesintisiyle kötü
    anda ölürse geri koyma adımı çalışmaz ve ağaçta bir emniyet kuralı
    silinmiş hâlde kalır. Sonra `git commit -a` yapan biri onu farkında olmadan
    repodan düşürür.

 2. VERİTABANI ADI DENETLENİR. Testler tabloları TRUNCATE ediyor ve bu betik
    testleri yüzlerce kez koşuyor. `TEST_DATABASE_URL` bir test veritabanını
    göstermiyorsa hiç başlamaz.

 3. DERLEME ÜRETMEZ. Yalnızca `go test` çalıştırır. Dağıtım üreten bir işin
    içine KONMAMALI: mutasyona uğramış ağacın paketlenmesi teorik olarak
    mümkündür. `make mutasyon` bilerek elle tetiklenir.

Ağa çıkmaz, hiçbir şey indirmez, kimlik bilgisi okumaz, git geçmişine dokunmaz.
"""

import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import tempfile

# `if ... {` ile başlayıp gövdesi tek bir `return ...Err...` olan bloklar.
#
# Bilerek DAR: çok satırlı gövdeleri, `if/else` zincirlerini ve `switch`
# dallarını yakalamaz. Geniş bir desen derlenmeyen kod üretir ve her mutasyon
# "test başarısız" sayılıp yanlış bir güven verirdi — derleme hatası, korumanın
# kanıtı değildir.
GUARD = re.compile(
    r"\n(\t+)if [^\n]*\{\n\t+return (?:[^\n]*, )?(?:&?\w*\.?Err\w+|Err\w+)\n\1\}\n"
)

# Bekçinin hemen üstündeki yorumda bu işaret varsa mutasyon denenmez.
ATLA = "mutasyon:atla"


def atlanmis(kaynak: str, konum: int) -> bool:
    """Bekçinin üstündeki yorum bloğunda atlama işareti var mı?

    NEDEN VAR: gerçekten test EDİLEMEYEN bekçiler var — örneğin çağırdığı
    fonksiyonun sözleşmesi gereği hiç ulaşılamayan bir dal. Onlar her koşuda
    "korunmuyor" diye raporlanırsa araç sürekli kırmızı yanar ve insanlar
    bakmayı bırakır; o noktadan sonra GERÇEK bir boşluk da gürültünün içinde
    kaybolur.

    İşaret bilinçli olarak YORUMA yazılıyor: atlama gerekçesi kodun yanında
    durmak zorunda. Ayrı bir liste dosyası tutulsaydı, bekçi silindikten sonra
    bile listede kalır ve kimse fark etmezdi.
    """
    for satir in reversed(kaynak[:konum].splitlines()):
        kirpik = satir.strip()
        if not kirpik:
            continue
        if not kirpik.startswith("//") and not kirpik.startswith("*"):
            return False  # yorum bloğu bitti
        if ATLA in kirpik:
            return True
    return False


def emniyet_denetimi(kaynak: pathlib.Path) -> str:
    """Ön koşulları sınar; test veritabanı adresini döner."""
    if not (kaynak / "go.mod").is_file():
        cik(f"{kaynak} bir Go modülü değil.")

    url = os.environ.get("TEST_DATABASE_URL", "")
    if not url:
        cik("TEST_DATABASE_URL boş. Bu betik entegrasyon testlerini koşar.")

    # Adres bir TEST veritabanını göstermeli. Testler TRUNCATE ediyor ve bu
    # betik onları yüzlerce kez koşuyor; yanlış adres geri alınamaz.
    ad = url.rsplit("/", 1)[-1].split("?")[0]
    if "test" not in ad:
        cik(
            f"TEST_DATABASE_URL bir test veritabanı göstermiyor: {ad!r}\n"
            "Testler tabloları TRUNCATE ediyor; adında 'test' geçmeyen bir\n"
            "veritabanına karşı çalıştırılmaz."
        )
    return url


def cik(mesaj: str) -> None:
    print(f"HATA: {mesaj}", file=sys.stderr)
    sys.exit(2)


def testler_geciyor(calisma: pathlib.Path, paket: str) -> bool:
    r = subprocess.run(
        ["go", "test", f"./{paket}", "-count=1"],
        capture_output=True, text=True, cwd=calisma,
    )
    return r.returncode == 0


def tara(calisma: pathlib.Path, paketler: list[str]) -> dict:
    hayatta: list[dict] = []
    toplam = 0
    atlanan = 0

    for paket in paketler:
        dizin = calisma / paket
        if not dizin.is_dir():
            print(f"  ! {paket} yok, atlanıyor", file=sys.stderr)
            continue

        for dosya in sorted(dizin.glob("*.go")):
            if dosya.name.endswith("_test.go"):
                continue

            ozgun = dosya.read_text()
            for m in list(GUARD.finditer(ozgun)):
                if atlanmis(ozgun, m.start()):
                    atlanan += 1
                    continue
                toplam += 1
                dosya.write_text(ozgun[: m.start()] + "\n" + ozgun[m.end():])
                try:
                    if testler_geciyor(calisma, paket):
                        satir = ozgun[: m.start()].count("\n") + 2
                        hayatta.append({
                            "paket": paket,
                            "yer": f"{dosya.relative_to(calisma)}:{satir}",
                            "bekci": " ".join(m.group(0).split())[:110],
                        })
                finally:
                    # Kopya üzerinde çalışıldığı için buradaki bir arıza bile
                    # çalışma ağacını etkilemez (bkz. EMNİYET/1).
                    dosya.write_text(ozgun)

        print(f"  · {paket}", file=sys.stderr)

    return {"toplam": toplam, "atlanan": atlanan, "hayatta": hayatta}


def paketleri_bul(kaynak: pathlib.Path) -> list[str]:
    """`internal/` altındaki tüm Go paketleri.

    Keşif BURADA, host'ta değil: bu projede Go host'a kurulmuyor, her şey
    container içinde çalışıyor. Liste `go list` ile dışarıda üretilseydi
    komut Go kurulu olmayan makinede sessizce boş dönerdi.
    """
    kok = kaynak / "internal"
    out = []
    for d in sorted(kok.rglob("*")):
        if not d.is_dir():
            continue
        if any(f.suffix == ".go" and not f.name.endswith("_test.go") for f in d.iterdir()):
            out.append(str(d.relative_to(kaynak)))
    return out


def main() -> int:
    if len(sys.argv) < 2:
        print(
            "kullanım: mutasyon.py <kaynak-dizin> [paket...]\n"
            "  paket verilmezse internal/ altındaki tümü taranır\n"
            "  örn:  mutasyon.py /src internal/workflow internal/runbatch",
            file=sys.stderr,
        )
        return 2

    kaynak = pathlib.Path(sys.argv[1]).resolve()
    paketler = sys.argv[2:]
    emniyet_denetimi(kaynak)
    if not paketler:
        paketler = paketleri_bul(kaynak)
        print(f"{len(paketler)} paket taranacak", file=sys.stderr)

    # TEK KULLANIMLIK KOPYA — betiğin yazdığı tek yer burası (bkz. EMNİYET/1).
    with tempfile.TemporaryDirectory(prefix="mutasyon-") as gecici:
        calisma = pathlib.Path(gecici) / "kaynak"
        print(f"Kaynak kopyalanıyor → {calisma}", file=sys.stderr)
        shutil.copytree(kaynak, calisma, symlinks=True)

        sonuc = tara(calisma, paketler)

    yaz_rapor(sonuc)
    # Korumasız bekçi varsa sıfırdan farklı çık: CI'da uyarı olarak
    # kullanılabilsin (derleme üreten bir işte DEĞİL — bkz. EMNİYET/3).
    return 1 if sonuc["hayatta"] else 0


def yaz_rapor(sonuc: dict) -> None:
    hayatta = sonuc["hayatta"]
    korunan = sonuc["toplam"] - len(hayatta)

    print()
    print(f"Denenen bekçi    : {sonuc['toplam']}")
    if sonuc.get("atlanan"):
        print(f"Atlanan          : {sonuc['atlanan']}  ({ATLA} işaretli)")
    print(f"Testin yakaladığı: {korunan}")
    print(f"KORUNMAYAN       : {len(hayatta)}")

    if not hayatta:
        print("\nHer bekçi korunuyor.")
        return

    print("\nAşağıdaki kurallar koddan kaldırıldığında hiçbir test kırmızıya")
    print("dönmedi. İleride biri bunları kaldırırsa uyaran bir şey yok:\n")
    for x in hayatta:
        print(f"  {x['yer']}")
        print(f"      {x['bekci']}")

    print("\nJSON:")
    print(json.dumps(sonuc, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    sys.exit(main())
