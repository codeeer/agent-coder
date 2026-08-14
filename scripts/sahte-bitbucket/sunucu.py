"""Kurumsal Bitbucket'ın OKUMA uçlarını taklit eden yerel sunucu (spec 021).

NEDEN VAR: gruptan toplu proje ekleme yalnızca kendi sunucusunda çalışan
kurumsal Bitbucket için geçerli ve elimizde deneme lisansı olmadığı için
gerçek bir örnek yok. Bu sunucu **kendi kodumuzu** doğrular: adres
ayrıştırma, sayfalama döngüsü, klonlama adresi seçimi, mükerrer atlama ve
arayüzün bütün durumları.

BUNUN KANITLAMADIĞI ŞEY: Atlassian'ın gerçek yanıtı. Varsayımımız yanlışsa bu
sunucu aynı yanlışı tekrarlar. Bu yüzden buradan geçen bir doğrulama "gerçek
sunucuda çalışıyor" diye sunulamaz.

İki iş yapar:

  GET /rest/api/1.0/projects/{ANAHTAR}/repos   → sayfalı repository listesi
  /scm/{ANAHTAR}/{slug}.git/…                  → gerçek git (smart HTTP)

Sayfalama BİLEREK iki sayfaya bölünüyor: tek sayfa dönseydi istemcinin
`isLastPage` döngüsü hiç sınanmazdı.

Çalıştırma:

    python3 scripts/sahte-bitbucket/sunucu.py

Backend container'ından erişim: http://host.docker.internal:7990
"""

import json
import os
import subprocess
import sys
import tempfile
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import parse_qs, urlparse

PORT = int(os.environ.get("PORT", "7990"))
ANAHTAR = os.environ.get("GRUP", "ODEME")

# Sayfa başına kayıt — Bitbucket'ın varsayılanı 25, burada küçük tutuluyor ki
# birkaç depoyla bile ikinci sayfa oluşsun.
SAYFA = int(os.environ.get("SAYFA", "2"))

# `git http-backend` bir alt komut DEĞİL, ayrı bir ikili; yeri dağıtıma göre
# değişiyor, o yüzden git'in kendisine soruluyor.
BACKEND = os.path.join(
    subprocess.run(["git", "--exec-path"], capture_output=True, text=True,
                   check=True).stdout.strip(),
    "git-http-backend",
)

DEPO_KOK = tempfile.mkdtemp(prefix="sahte-bitbucket-")

# slug → görünen ad, arşivli mi
DEPOLAR = [
    ("odeme-api", "Ödeme API", False),
    ("odeme-worker", "Ödeme Worker", False),
    ("odeme-web", "Ödeme Web", False),
    ("odeme-legacy", "Ödeme Legacy", True),
]

# YÜK ÖLÇÜMÜ: `DEPO_SAYISI=100` verilirse liste doldurulur. Spec 021'in
# "yüz repository'de makul sürede biter" kriteri ancak yüz repository ile
# ölçülebilir; dört depoyla yapılan bir ölçüm o kriteri sınamaz.
_EK = int(os.environ.get("DEPO_SAYISI", "0"))
for _i in range(len(DEPOLAR), _EK):
    DEPOLAR.append((f"yuk-{_i:03d}", f"Yük {_i:03d}", False))

# Varsayılan branch'i her depo için farklı tutuyoruz: ürün branch'i git'ten
# okuduğunu iddia ediyor, hepsi "main" olsaydı iddia sınanmazdı.
BRANCHLER = {"odeme-api": "develop", "odeme-worker": "main",
             "odeme-web": "release/2026", "odeme-legacy": "main"}
for _slug, _, _ in DEPOLAR:
    BRANCHLER.setdefault(_slug, "main")


def depolari_hazirla():
    """Her depo için tek commit'lik gerçek bir bare repository üretir."""
    for slug, _, _ in DEPOLAR:
        branch = BRANCHLER[slug]
        calisma = tempfile.mkdtemp()
        ortam = {**os.environ,
                 "GIT_AUTHOR_NAME": "sahte", "GIT_AUTHOR_EMAIL": "s@e.com",
                 "GIT_COMMITTER_NAME": "sahte", "GIT_COMMITTER_EMAIL": "s@e.com"}
        subprocess.run(["git", "init", "-b", branch, calisma], check=True,
                       capture_output=True)
        subprocess.run(["git", "commit", "--allow-empty", "-m", "ilk"],
                       cwd=calisma, env=ortam, check=True, capture_output=True)
        subprocess.run(["git", "clone", "--bare", calisma,
                        os.path.join(DEPO_KOK, f"{slug}.git")],
                       check=True, capture_output=True)
    print(f"depolar hazır: {DEPO_KOK}", file=sys.stderr)


def klon_adresi(host, slug):
    # Kullanıcı adı BİLEREK gömülü: Bitbucket çoğu kurulumda böyle veriyor ve
    # ürünün onu ayıklaması gerekiyor. Ayıklamazsa kayıt reddedilir.
    return f"http://ahmet@{host}/scm/{ANAHTAR}/{slug}.git"


class Isleyici(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"

    def log_message(self, bicim, *args):
        print(f"  {self.command} {self.path}", file=sys.stderr)

    def do_GET(self):
        yol = urlparse(self.path).path
        if yol == f"/rest/api/1.0/projects/{ANAHTAR}/repos":
            return self.repolari_yaz()
        # Sağlayıcı doğrulaması bu uca bakıyor (gitprovider/validator.go):
        # kurumsal kurulumda "projects" her yerde var ve okuma yetkisi yeter.
        if yol == "/rest/api/1.0/projects":
            return self.yanit(200, json.dumps({
                "size": 1, "isLastPage": True,
                "values": [{"key": ANAHTAR, "name": "Ödeme"}],
            }).encode())
        if yol.startswith("/scm/"):
            return self.git()
        self.yanit(404, b'{"errors":[]}')

    def do_POST(self):
        if urlparse(self.path).path.startswith("/scm/"):
            return self.git()
        self.yanit(404, b"")

    def repolari_yaz(self):
        sorgu = parse_qs(urlparse(self.path).query)
        start = int(sorgu.get("start", ["0"])[0])
        limit = min(int(sorgu.get("limit", [str(SAYFA)])[0]), SAYFA)

        dilim = DEPOLAR[start:start + limit]
        son = start + limit >= len(DEPOLAR)
        host = self.headers.get("Host", f"localhost:{PORT}")

        govde = {
            "size": len(dilim), "limit": limit, "start": start,
            "isLastPage": son,
            "values": [
                {
                    "slug": slug, "id": i, "name": ad, "scmId": "git",
                    "state": "AVAILABLE", "archived": arsiv,
                    "links": {"clone": [
                        {"href": f"ssh://git@{host}:7999/{ANAHTAR}/{slug}.git",
                         "name": "ssh"},
                        {"href": klon_adresi(host, slug), "name": "http"},
                    ]},
                }
                for i, (slug, ad, arsiv) in enumerate(dilim)
            ],
        }
        if not son:
            govde["nextPageStart"] = start + limit

        self.yanit(200, json.dumps(govde).encode())

    def git(self):
        yol = urlparse(self.path).path
        # /scm/ANAHTAR/... → git http-backend'in beklediği yol
        parcalar = yol.split("/", 3)
        ic_yol = "/" + parcalar[3] if len(parcalar) > 3 else "/"

        uzunluk = int(self.headers.get("Content-Length") or 0)
        govde = self.rfile.read(uzunluk) if uzunluk else b""

        ortam = {
            **os.environ,
            "GIT_PROJECT_ROOT": DEPO_KOK,
            "GIT_HTTP_EXPORT_ALL": "1",
            "REQUEST_METHOD": self.command,
            "PATH_INFO": ic_yol,
            "QUERY_STRING": urlparse(self.path).query,
            "CONTENT_TYPE": self.headers.get("Content-Type", ""),
            "CONTENT_LENGTH": str(uzunluk),
            "REMOTE_USER": "sahte",
        }

        p = subprocess.run([BACKEND], input=govde, env=ortam, capture_output=True)
        basliklar, _, cikti = p.stdout.partition(b"\r\n\r\n")

        self.send_response(200)
        for satir in basliklar.split(b"\r\n"):
            if b":" in satir:
                ad, _, deger = satir.partition(b":")
                self.send_header(ad.decode(), deger.strip().decode())
        self.send_header("Content-Length", str(len(cikti)))
        self.end_headers()
        self.wfile.write(cikti)

    def yanit(self, kod, govde):
        self.send_response(kod)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(govde)))
        self.end_headers()
        self.wfile.write(govde)


if __name__ == "__main__":
    depolari_hazirla()
    print(f"sahte Bitbucket: http://localhost:{PORT}/projects/{ANAHTAR}", file=sys.stderr)
    ThreadingHTTPServer(("0.0.0.0", PORT), Isleyici).serve_forever()
