package scripts_test

import (
	"context"
	"testing"

	"github.com/google/uuid"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/scripts"
	"github.com/agent-coder/backend/internal/testutil"
)

/*
 * Klasör benzersizliği — spec 022'nin en riskli parçası, ve riski
 * VERİTABANININ KENDİSİNDE.
 *
 * Kural: aynı klasörde aynı ad olamaz, farklı klasörlerde olabilir. Bunu
 * `UNIQUE (folder_id, name)` ile yazmak yetmez — Postgres iki NULL'ı
 * birbirinden FARKLI sayar, dolayısıyla klasörsüz iki script aynı adı alır ve
 * container'da AYNI dosyaya yazılırdı. Biri diğerini sessizce ezerdi.
 *
 * Aşağıdaki testlerden üçüncüsü (`KlasorsuzAyniAdAlamaz`) tam olarak bunu
 * yakalamak için var: `NULLS NOT DISTINCT` olmadan geçmez.
 */

func newFolderStore(t *testing.T) *scripts.Store {
	t.Helper()
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool,
		"agent_script_folders", "agent_scripts", "scripts", "script_folders",
		"runs", "agents")
	return scripts.NewStore(pool)
}

func klasor(t *testing.T, s *scripts.Store, ad string) scripts.Folder {
	t.Helper()
	f, err := s.CreateFolder(context.Background(), scripts.FolderInput{Name: ad})
	require.NoError(t, err)
	return f
}

func TestKlasor_AyniKlasordeAyniAdReddedilir(t *testing.T) {
	s := newFolderStore(t)
	f := klasor(t, s, "node-24")

	_, err := s.Create(context.Background(), scripts.CreateInput{
		Name: "01-baslat", Content: "echo bir", FolderID: &f.ID})
	require.NoError(t, err)

	_, err = s.Create(context.Background(), scripts.CreateInput{
		Name: "01-baslat", Content: "echo iki", FolderID: &f.ID})

	require.ErrorIs(t, err, scripts.ErrDuplicateName)
}

func TestKlasor_FarkliKlasordeAyniAdKabulEdilir(t *testing.T) {
	s := newFolderStore(t)
	a := klasor(t, s, "node-24")
	b := klasor(t, s, "spring-3")

	_, err := s.Create(context.Background(), scripts.CreateInput{
		Name: "01-baslat", Content: "echo bir", FolderID: &a.ID})
	require.NoError(t, err)

	_, err = s.Create(context.Background(), scripts.CreateInput{
		Name: "01-baslat", Content: "echo iki", FolderID: &b.ID})

	require.NoError(t, err, "farklı klasörlerde aynı ad çakışmaz — yolları farklı")
}

/*
 * KLASÖRSÜZ İKİ SCRIPT AYNI ADI ALAMAZ.
 *
 * Bu testin varlık sebebi: `NULLS NOT DISTINCT` olmadan Postgres bu iki kaydı
 * kabul eder ve ikisi container'da `/home/agent/scripts/ortak.sh` dosyasına
 * yazılır. Hata ancak agent yanlış içeriği çalıştırınca görülürdü.
 */
func TestKlasor_KlasorsuzAyniAdAlamaz(t *testing.T) {
	s := newFolderStore(t)

	_, err := s.Create(context.Background(), scripts.CreateInput{
		Name: "ortak", Content: "echo bir"})
	require.NoError(t, err)

	_, err = s.Create(context.Background(), scripts.CreateInput{
		Name: "ortak", Content: "echo iki"})

	require.ErrorIs(t, err, scripts.ErrDuplicateName)
}

func TestKlasor_AdBenzersiz(t *testing.T) {
	s := newFolderStore(t)
	klasor(t, s, "node-24")

	_, err := s.CreateFolder(context.Background(), scripts.FolderInput{Name: "node-24"})

	require.ErrorIs(t, err, scripts.ErrDuplicateFolder)
}

// Klasör adı script adıyla AYNI kurala tabi: ikisi de dizin/dosya adına
// dönüşüyor. Kural kopyalanmadığı için bu test onu kilitliyor.
func TestKlasor_AdDogrulamasiScriptKuralininAynisi(t *testing.T) {
	s := newFolderStore(t)

	gecersizler := []string{"", "Node 24", "node_24", "node/24", "NODE24"}
	for _, ad := range gecersizler {
		t.Run(ad, func(t *testing.T) {
			_, err := s.CreateFolder(context.Background(), scripts.FolderInput{Name: ad})
			require.Error(t, err)
		})
	}

	_, err := s.CreateFolder(context.Background(), scripts.FolderInput{Name: "node-24-upgrade"})
	require.NoError(t, err)
}

/*
 * KLASÖR SİLİNİNCE SCRIPT'LER KALIR (spec 022 H5).
 *
 * Bir düzenleme kararı veri kaybına dönüşmemeli: kullanıcı kampanyayı
 * kaldırıyor, yazdığı script'leri değil.
 */
func TestKlasor_SilininceScriptlerKlasorsuzKalir(t *testing.T) {
	s := newFolderStore(t)
	f := klasor(t, s, "node-24")

	sc, err := s.Create(context.Background(), scripts.CreateInput{
		Name: "01-baslat", Content: "echo bir", FolderID: &f.ID})
	require.NoError(t, err)

	require.NoError(t, s.DeleteFolder(context.Background(), f.ID))

	kalan, err := s.Get(context.Background(), sc.ID)
	require.NoError(t, err, "script silinmemeli")
	require.Nil(t, kalan.FolderID, "klasörsüz kalmalı")
}

func TestKlasor_ListeScriptSayisiniTasir(t *testing.T) {
	s := newFolderStore(t)
	f := klasor(t, s, "node-24")
	klasor(t, s, "bos-klasor")

	for _, ad := range []string{"01-a", "02-b"} {
		_, err := s.Create(context.Background(), scripts.CreateInput{
			Name: ad, Content: "echo", FolderID: &f.ID})
		require.NoError(t, err)
	}

	liste, err := s.ListFolders(context.Background())
	require.NoError(t, err)

	sayi := map[string]int{}
	for _, k := range liste {
		sayi[k.Name] = k.ScriptCount
	}
	require.Equal(t, 2, sayi["node-24"])
	require.Equal(t, 0, sayi["bos-klasor"], "boş klasör de listelenir")
}

func TestKlasor_KullanimSayilari(t *testing.T) {
	s := newFolderStore(t)
	f := klasor(t, s, "node-24")

	_, err := s.Create(context.Background(), scripts.CreateInput{
		Name: "01-a", Content: "echo", FolderID: &f.ID})
	require.NoError(t, err)

	scriptSayisi, agentSayisi, err := s.FolderUsage(context.Background(), f.ID)

	require.NoError(t, err)
	require.Equal(t, 1, scriptSayisi)
	require.Equal(t, 0, agentSayisi)
}

/*
 * ForAgent — İKİ KÜMENİN BİRLEŞİMİ.
 *
 * Agent'a doğrudan atanmış script'ler ve atanmış KLASÖRLERİN tüm script'leri
 * birlikte döner. Bu birleşim çalıştırma anında yapılıyor; atama sırasında
 * çözülseydi klasöre sonradan eklenen script o agent'ta geçerli olmazdı.
 */
func TestForAgent_KlasorVeTekilBirlesir(t *testing.T) {
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_script_folders", "agent_scripts",
		"scripts", "script_folders", "runs", "agents")
	s := scripts.NewStore(pool)
	ctx := context.Background()

	f := klasor(t, s, "node-24")
	icinde, err := s.Create(ctx, scripts.CreateInput{
		Name: "01-baslat", Content: "echo", FolderID: &f.ID})
	require.NoError(t, err)

	tekil, err := s.Create(ctx, scripts.CreateInput{Name: "ortak", Content: "echo"})
	require.NoError(t, err)

	// Atanmamış bir script: listeye SIZMAMALI.
	_, err = s.Create(ctx, scripts.CreateInput{Name: "alakasiz", Content: "echo"})
	require.NoError(t, err)

	agentID := seedAgent(t, pool, "kampanya-agent")
	require.NoError(t, s.SetAgentScripts(ctx, agentID, []uuid.UUID{tekil.ID}))
	require.NoError(t, s.SetAgentFolders(ctx, agentID, []uuid.UUID{f.ID}))

	liste, err := s.ForAgent(ctx, agentID)
	require.NoError(t, err)

	adlar := make([]string, 0, len(liste))
	for _, sc := range liste {
		adlar = append(adlar, sc.Name)
	}
	require.ElementsMatch(t, []string{"ortak", "01-baslat"}, adlar)
	require.Contains(t, adlar, icinde.Name)
}

/*
 * KLASÖRE SONRADAN EKLENEN SCRIPT, ATAMA TAZELENMEDEN GEÇERLİ OLUR.
 *
 * Spec 022 H3'ün kanıtı. Klasörü "içindeki script'leri agent_scripts'e yaz"
 * diye çözen bir tasarım bu testte düşer — ve kullanıcı her yeni adımda bütün
 * agent'ları tekrar düzenlemek zorunda kalırdı.
 */
func TestForAgent_KlasoreSonradanEklenenGorunur(t *testing.T) {
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_script_folders", "agent_scripts",
		"scripts", "script_folders", "runs", "agents")
	s := scripts.NewStore(pool)
	ctx := context.Background()

	f := klasor(t, s, "node-24")
	agentID := seedAgent(t, pool, "kampanya-agent")
	require.NoError(t, s.SetAgentFolders(ctx, agentID, []uuid.UUID{f.ID}))

	once, err := s.ForAgent(ctx, agentID)
	require.NoError(t, err)
	require.Empty(t, once)

	_, err = s.Create(ctx, scripts.CreateInput{
		Name: "08-yeni-adim", Content: "echo", FolderID: &f.ID})
	require.NoError(t, err)

	sonra, err := s.ForAgent(ctx, agentID)
	require.NoError(t, err)
	require.Len(t, sonra, 1, "atama tazelenmeden görünmeli")
}

// Hem tekil hem klasör üzerinden atanmış script BİR KEZ döner: iki kez
// dönseydi talimatta iki kez yazılırdı.
func TestForAgent_MukerrerDonmez(t *testing.T) {
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_script_folders", "agent_scripts",
		"scripts", "script_folders", "runs", "agents")
	s := scripts.NewStore(pool)
	ctx := context.Background()

	f := klasor(t, s, "node-24")
	sc, err := s.Create(ctx, scripts.CreateInput{
		Name: "01-baslat", Content: "echo", FolderID: &f.ID})
	require.NoError(t, err)

	agentID := seedAgent(t, pool, "kampanya-agent")
	require.NoError(t, s.SetAgentScripts(ctx, agentID, []uuid.UUID{sc.ID}))
	require.NoError(t, s.SetAgentFolders(ctx, agentID, []uuid.UUID{f.ID}))

	liste, err := s.ForAgent(ctx, agentID)

	require.NoError(t, err)
	require.Len(t, liste, 1)
}

/*
 * SIRA: önce klasörsüzler, sonra klasör adı, sonra script adı.
 *
 * Talimat metni bu sıraya dayanıyor; sıra değişirse aynı agent'ın farklı
 * çalıştırmalarında farklı bir talimat dosyası üretilirdi.
 */
func TestForAgent_Sira(t *testing.T) {
	pool := testutil.TestDB(t)
	testutil.Truncate(t, pool, "agent_script_folders", "agent_scripts",
		"scripts", "script_folders", "runs", "agents")
	s := scripts.NewStore(pool)
	ctx := context.Background()

	b := klasor(t, s, "b-kampanya")
	a := klasor(t, s, "a-kampanya")

	for _, kv := range []struct {
		ad  string
		kls *uuid.UUID
	}{
		{"02-ikinci", &a.ID}, {"01-ilk", &a.ID}, {"tek", &b.ID}, {"ortak", nil},
	} {
		_, err := s.Create(ctx, scripts.CreateInput{
			Name: kv.ad, Content: "echo", FolderID: kv.kls})
		require.NoError(t, err)
	}

	agentID := seedAgent(t, pool, "kampanya-agent")
	require.NoError(t, s.SetAgentFolders(ctx, agentID, []uuid.UUID{a.ID, b.ID}))
	require.NoError(t, s.SetAgentScripts(ctx, agentID, nil))

	// Klasörsüz olanı da ekleyelim.
	hepsi, _, err := s.List(ctx, 100, 0)
	require.NoError(t, err)
	for _, sc := range hepsi {
		if sc.Name == "ortak" {
			require.NoError(t, s.SetAgentScripts(ctx, agentID, []uuid.UUID{sc.ID}))
		}
	}

	liste, err := s.ForAgent(ctx, agentID)
	require.NoError(t, err)

	adlar := make([]string, 0, len(liste))
	for _, sc := range liste {
		adlar = append(adlar, sc.Name)
	}
	require.Equal(t, []string{"ortak", "01-ilk", "02-ikinci", "tek"}, adlar)
}

// Klasörlü script alt dizine düşer; klasörsüz olan bugünkü yolunda kalır.
func TestPath_KlasorluVeKlasorsuz(t *testing.T) {
	klasorlu := scripts.Script{Name: "01-baslat", FolderName: "node-24"}
	klasorsuz := scripts.Script{Name: "ortak"}

	require.Equal(t, "/home/agent/scripts/node-24/01-baslat.sh", klasorlu.Path())
	require.Equal(t, "/home/agent/scripts/ortak.sh", klasorsuz.Path())
	require.Equal(t, "/home/agent/scripts/node-24",
		scripts.Folder{Name: "node-24"}.Path())
}
