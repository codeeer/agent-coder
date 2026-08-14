#!/usr/bin/env bash
#
# Kanarya değerlerini üretir.
#
# NEDEN KANARYA: bir mitmproxy dökümüne bakıp "şüpheli bir şey görmedim"
# demek kanıt değil — bir koşu on binlerce satır üretiyor ve göz kaçırır.
# Kanarya, sızıntı sorusunu MEKANİK bir aramaya çeviriyor: bu dizge dökümde
# var mı, yok mu.
#
# Değerler benzersiz ve yüksek entropili: "test123" gibi bir dizge model
# yanıtında tesadüfen geçebilir ve yanlış pozitif üretirdi.
#
# BİR KEZ üretilir ve iki koşuda da aynı kalır; yeniden üretmek önceki
# koşunun analizini geçersiz kılar.
set -euo pipefail

kok="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cikti="$kok/cikti"
dosya="$cikti/kanaryalar.env"

mkdir -p "$cikti"

if [[ -f "$dosya" ]]; then
    echo "kanaryalar zaten üretilmiş: $dosya"
    echo "yeniden üretmek için önce silin (önceki koşuların analizi geçersiz olur)"
    exit 0
fi

id="$(head -c 8 /dev/urandom | od -An -tx1 | tr -d ' \n')"

cat > "$dosya" <<EOF
# Sızıntı analizi kanaryaları — koşu kimliği $id
#
# Hepsi SAHTE. Hiçbiri gerçek bir sisteme erişim vermez. Amaçları tek şey:
# ağ dökümünde arandıklarında bulunup bulunmadıklarını ölçmek.

# Depo içindeki kaynak kodda geçen işaret — kaynak kodun nereye gittiğini ölçer.
KANARYA_KAYNAK_KODU=KANARYAKAYNAK${id}KOD

# Depo içindeki .env benzeri dosyada duran sahte sır — depodaki sırları ölçer.
KANARYA_DEPO_SIRRI=KANARYADEPO${id}SIRRI

# Depo erişim token'ı. Hem nginx basic auth parolası HEM kanarya:
# gerçekten kullanılıyor, dolayısıyla container ortamına gerçekten giriyor.
KANARYA_GIT_TOKEN=KANARYAGIT${id}TOKEN

# Görev metnine gömülen işaret — prompt'un nereye gittiğini ölçer.
KANARYA_PROMPT=KANARYAPROMPT${id}GOREV

# Dosya adı — dizin listesinin/dosya adlarının nereye gittiğini ölçer.
KANARYA_DOSYA_ADI=kanaryadosya${id}

# Koşu kimliği — çıktı adlandırmasında kullanılır.
KANARYA_ID=$id
EOF

echo "üretildi: $dosya"
cat "$dosya" | grep -v '^#' | grep -v '^$'
