"""Kanarya deposunu **smart HTTP** ile sunar, basic auth ile korur.

NEDEN NGINX DEĞİL — ÖLÇÜLEREK BULUNDU: ilk kurulumda depo nginx ile "dumb
HTTP" olarak sunuluyordu. Ürün depoyu SIĞ klonluyor (`runner.clone_depth`
varsayılanı 1) ve git şu hatayla düştü:

    fatal: dumb http transport does not support shallow capabilities

Klon derinliği ayarı en az 1 olabiliyor, yani sığ klonlama kapatılamıyor.
Ölçümün gerçek koşu yolunu izlemesi için smart HTTP zorunlu.

Basic auth BİLEREK korunuyor: kimlik bilgisi gerçekten kullanılmazsa kanarya
token'ı telde hiç görünmez ve "kimlik bilgisi sızıyor mu" sorusu ölçülemez.

`git http-backend` bir CGI programı; buradaki tek iş ona doğru ortam
değişkenlerini verip çıktısını HTTP yanıtına çevirmek.
"""

import base64
import os
import subprocess
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

DEPO_KOK = os.environ.get("DEPO_KOK", "/depolar")
KULLANICI = os.environ.get("KANARYA_KULLANICI", "kanarya")
PAROLA = os.environ["KANARYA_GIT_TOKEN"]

# ÖLÇÜLEREK BULUNDU: `git http-backend` bir alt komut DEĞİL — `libexec/git-core`
# altında ayrı bir ikili ve PATH'te yok. Doğrudan "git http-backend" çağrısı
#   git: 'http-backend' is not a git command
# ile düşüyor ve istemciye 500 olarak yansıyordu. Yol dağıtıma göre değiştiği
# için sabit yazılmıyor, git'in kendisine sorduruluyor.
GIT_CORE = subprocess.run(  # noqa: S603
    ["git", "--exec-path"], capture_output=True, text=True, check=True,
).stdout.strip()
BACKEND = os.path.join(GIT_CORE, "git-http-backend")


class Isleyici(BaseHTTPRequestHandler):
    # HTTP/1.1 şart: git smart HTTP chunked yanıt bekleyebiliyor.
    protocol_version = "HTTP/1.1"

    def log_message(self, bicim, *args):
        # Ölçümün gürültüsünü azaltır; asıl kayıt vekilde ve pcap'te.
        pass

    def yetkili_mi(self):
        baslik = self.headers.get("Authorization", "")
        if not baslik.startswith("Basic "):
            return False
        try:
            cozulmus = base64.b64decode(baslik[6:]).decode()
        except Exception:  # noqa: BLE001
            return False
        ad, _, parola = cozulmus.partition(":")
        return ad == KULLANICI and parola == PAROLA

    def yetki_iste(self):
        self.send_response(401)
        self.send_header("WWW-Authenticate", 'Basic realm="kanarya"')
        self.send_header("Content-Length", "0")
        self.end_headers()

    def govde_oku(self):
        if self.headers.get("Transfer-Encoding", "").lower() == "chunked":
            parcalar = []
            while True:
                satir = self.rfile.readline().strip()
                uzunluk = int(satir.split(b";")[0], 16)
                if uzunluk == 0:
                    self.rfile.readline()
                    break
                parcalar.append(self.rfile.read(uzunluk))
                self.rfile.readline()
            return b"".join(parcalar)
        uzunluk = int(self.headers.get("Content-Length") or 0)
        return self.rfile.read(uzunluk) if uzunluk else b""

    def calistir(self):
        if not self.yetkili_mi():
            self.yetki_iste()
            return

        yol, _, sorgu = self.path.partition("?")
        govde = self.govde_oku()

        ortam = {
            "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
            "GIT_PROJECT_ROOT": DEPO_KOK,
            "GIT_HTTP_EXPORT_ALL": "1",
            "REQUEST_METHOD": self.command,
            "PATH_INFO": yol,
            "QUERY_STRING": sorgu,
            "REMOTE_USER": KULLANICI,
            "REMOTE_ADDR": self.client_address[0],
            "CONTENT_TYPE": self.headers.get("Content-Type", ""),
            "CONTENT_LENGTH": str(len(govde)),
        }
        # git protokol sürümü pazarlığı bu başlıkla yapılıyor; aktarılmazsa
        # istemci v0'a düşer ve sığ klonlama yine sorun çıkarabilir.
        if self.headers.get("Git-Protocol"):
            ortam["GIT_PROTOCOL"] = self.headers["Git-Protocol"]

        ortam["GIT_EXEC_PATH"] = GIT_CORE
        sonuc = subprocess.run(  # noqa: S603
            [BACKEND],
            input=govde, capture_output=True, env=ortam, check=False,
        )
        if sonuc.returncode != 0:
            self.send_response(500)
            self.send_header("Content-Length", str(len(sonuc.stderr)))
            self.end_headers()
            self.wfile.write(sonuc.stderr)
            return

        baslik_ham, _, govde_cikti = sonuc.stdout.partition(b"\r\n\r\n")
        durum = 200
        basliklar = []
        for satir in baslik_ham.split(b"\r\n"):
            if not satir:
                continue
            ad, _, deger = satir.decode("latin-1").partition(":")
            deger = deger.strip()
            if ad.lower() == "status":
                durum = int(deger.split()[0])
            else:
                basliklar.append((ad, deger))

        self.send_response(durum)
        for ad, deger in basliklar:
            self.send_header(ad, deger)
        self.send_header("Content-Length", str(len(govde_cikti)))
        self.end_headers()
        self.wfile.write(govde_cikti)

    do_GET = calistir
    do_POST = calistir


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 80), Isleyici).serve_forever()
