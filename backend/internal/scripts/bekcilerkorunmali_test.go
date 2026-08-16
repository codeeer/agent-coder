package scripts_test

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/scripts"
)

/*
 * Klasör ve betik kapılarının HATA TÜRÜ.
 *
 * Bu testler bir mutasyon taramasının çıktısı: aşağıdaki bekçiler koddan
 * kaldırıldığında hiçbir test kırmızıya dönmüyordu.
 *
 * Hata TÜRÜ ayrıntı değil, ürünün cevabıdır. HTTP katmanı `ErrFolderNotFound`
 * gördüğünde 404, `ErrDuplicateFolder` gördüğünde 409 ve kullanıcıya "bu adda
 * bir klasör zaten var" der. Bekçi kalkarsa yerine ham veritabanı hatası geçer
 * ve üçü de 500 "işlem tamamlanamadı" olur: kullanıcı adı değiştirmesi
 * gerektiğini asla öğrenemez, sistemin bozulduğunu sanır.
 */

func TestGetFolder_OlmayanKlasor(t *testing.T) {
	s := newFolderStore(t)

	_, err := s.GetFolder(context.Background(), uuid.New())
	require.ErrorIs(t, err, scripts.ErrFolderNotFound)
}

func TestUpdateFolder_OlmayanKlasor(t *testing.T) {
	s := newFolderStore(t)

	_, err := s.UpdateFolder(context.Background(), uuid.New(),
		scripts.FolderInput{Name: "yeni-ad"})
	require.ErrorIs(t, err, scripts.ErrFolderNotFound)
}

/*
YENİDEN ADLANDIRMA DA BENZERSİZLİĞE TAKILIR.

Oluşturmada kontrol edilip güncellemede edilmeseydi, kural tek bir adımla
delinirdi: "a" ve "b" klasörlerini kur, sonra "b"yi "a" yap. Sonuç, oluşturmada
en baştan yasaklanan durumun ta kendisi.
*/
func TestUpdateFolder_AyniAdaTasinamaz(t *testing.T) {
	s := newFolderStore(t)
	klasor(t, s, "node-24")
	hedef := klasor(t, s, "node-22")

	_, err := s.UpdateFolder(context.Background(), hedef.ID,
		scripts.FolderInput{Name: "node-24"})
	require.ErrorIs(t, err, scripts.ErrDuplicateFolder)
}

/*
OLMAYAN KLASÖR AGENT'A BAĞLANAMAZ.

Yabancı anahtar ihlali ham hâliyle dönseydi kullanıcı 500 görürdü. Oysa
yapılacak şey belli: silinmiş bir klasörü seçmiş, listeyi tazelemesi gerek.
*/
func TestSetAgentFolders_OlmayanKlasor(t *testing.T) {
	s := newFolderStore(t)

	err := s.SetAgentFolders(context.Background(), uuid.New(), []uuid.UUID{uuid.New()})
	require.ErrorIs(t, err, scripts.ErrFolderNotFound)
}

/*
BETİK YENİDEN ADLANDIRILIRKEN DE BENZERSİZLİK KORUNUR.

Aynı klasörde iki betiğin aynı adı alması container'da AYNI dosyaya yazmak
demek — biri diğerini sessizce ezerdi. Kural oluşturmada sınanıyordu;
güncellemede sınanmasaydı iki adımda delinirdi.
*/
func TestUpdate_AyniAdaTasinamaz(t *testing.T) {
	s := newFolderStore(t)
	ctx := context.Background()
	f := klasor(t, s, "node-24")

	_, err := s.Create(ctx, scripts.CreateInput{
		Name: "01-baslat", Content: "echo bir", FolderID: &f.ID})
	require.NoError(t, err)

	ikinci, err := s.Create(ctx, scripts.CreateInput{
		Name: "02-kur", Content: "echo iki", FolderID: &f.ID})
	require.NoError(t, err)

	ad := "01-baslat"
	_, err = s.Update(ctx, ikinci.ID, scripts.UpdateInput{Name: &ad}, false)
	require.ErrorIs(t, err, scripts.ErrDuplicateName)
}
