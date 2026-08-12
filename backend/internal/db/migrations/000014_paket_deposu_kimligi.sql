-- +goose Up

-- Kurumsal paket deposu (Nexus, Artifactory…) kimlik bilgisi.
--
-- Adres ve kullanıcı adı AYARLARDA durur (packages.npm_*), token burada:
-- token bir sırdır ve bu tablo tam da onun için var — `secret_enc` şifreli
-- ve NOT NULL.
--
-- Kimlik doğrulama OPSİYONEL. Bunu "boş sır" ile değil, KAYDIN HİÇ
-- OLMAMASIYLA anlatıyoruz: `Put` boş sırrı açıkça reddediyor ve
-- "kimlik bilgisi = sır" bu tablonun değişmezi. Anonim okumaya açık bir
-- depoda satır hiç oluşmaz.
--
-- ADD VALUE bu geçişte yalnızca TANIMLANIR, kullanılmaz — PostgreSQL yeni
-- değerin aynı işlem içinde kullanılmasına izin vermez.
ALTER TYPE credential_kind ADD VALUE IF NOT EXISTS 'nexus';

-- +goose Down
-- PostgreSQL enum değeri KALDIRAMAZ; geri alma türü yeniden yaratmayı ve
-- tabloyu taşımayı gerektirirdi. Kullanılmayan bir enum değeri zararsız
-- olduğu için burada bilinçli olarak hiçbir şey yapılmıyor.
SELECT 1;
