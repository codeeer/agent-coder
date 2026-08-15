package runbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/workflow"
)

/*
 * MCP argümanlarının çözümlenmesi.
 *
 * Bu dönüşüm ürünün DIŞARIYA çıkan yüzlerinden biri: buradan çıkan nesne bir
 * dış araç sunucusuna gidiyor. Yanlış çözümlenen bir argüman, yanlış bir
 * kayda yazmak ya da yanlış bir sorguyu çalıştırmak demek.
 */

// girdi, verilen argüman JSON'u ve şablon bağlamıyla bir çalıştırma girdisi kurar.
func girdi(args string, ctx workflow.Context) workflow.ExecInput {
	return workflow.ExecInput{
		Node:    workflow.Node{Config: workflow.NodeConfig{Arguments: args}},
		Context: ctx,
	}
}

func TestRenderArguments_BosArgumanBosNesne(t *testing.T) {
	out, err := (&MCPHandler{}).renderArguments(girdi("  ", workflow.Context{}))

	require.NoError(t, err)
	require.NotNil(t, out, "nil değil boş nesne dönmeli — çağıran doğrudan gönderiyor")
	require.Empty(t, out)
}

func TestRenderArguments_SablonCozulur(t *testing.T) {
	out, err := (&MCPHandler{}).renderArguments(
		girdi(`{"gorev": "{{ input }}"}`, workflow.Context{Input: "Node 24'e yükselt"}))

	require.NoError(t, err)
	require.Equal(t, map[string]any{"gorev": "Node 24'e yükselt"}, out)
}

/*
ÖNCE JSON, SONRA ŞABLON — sıranın kanıtı.

Ters sırada (önce şablon, sonra JSON) agent çıktısındaki bir tırnak ya da satır
sonu JSON'u bozardı ve hata ancak belirli bir çıktı geldiğinde patlardı: en geç
anlaşılan hata türü.

Buradaki çıktı bilerek en kötü hâli — tırnak, satır sonu, ters bölü ve süslü
parantez. Doğru sırada hiçbiri JSON'a dokunmuyor çünkü şablon, JSON zaten
ayrıştırıldıktan SONRA uygulanıyor.
*/
func TestRenderArguments_CiktidakiTirnakVeSatirSonuJSONuBozmaz(t *testing.T) {
	kotu := "o dedi ki \"tamam\"\nikinci satır \\ ters bölü {\"sahte\": 1}"

	out, err := (&MCPHandler{}).renderArguments(
		girdi(`{"aciklama": "{{ input }}"}`, workflow.Context{Input: kotu}))

	require.NoError(t, err, "agent çıktısı JSON'u bozmamalı")
	require.Equal(t, kotu, out["aciklama"], "değer olduğu gibi taşınmalı")
	require.Len(t, out, 1, "sahte JSON metni yeni bir alan üretmemeli")
}

// İç içe nesne ve dizilerdeki değerler de çözülür: argüman şeması düz olmak
// zorunda değil ve dış araçların çoğu iç içe alan istiyor.
func TestRenderArguments_IcIceYapilarDaCozulur(t *testing.T) {
	ctx := workflow.Context{
		Input:   "kök",
		Trigger: map[string]string{"key": "SCRUM-7"},
	}

	out, err := (&MCPHandler{}).renderArguments(girdi(
		`{"dis": {"ic": "{{ input }}"}, "liste": ["{{ trigger.key }}", "sabit"]}`, ctx))

	require.NoError(t, err)
	require.Equal(t, map[string]any{"ic": "kök"}, out["dis"])
	require.Equal(t, []any{"SCRUM-7", "sabit"}, out["liste"])
}

// Sayı, bool ve null şablon içeremez; tipleri korunarak geçerler. Metne
// çevrilselerdi dış araç şemayı reddederdi.
func TestRenderArguments_SayiBoolNullTipiniKorur(t *testing.T) {
	out, err := (&MCPHandler{}).renderArguments(
		girdi(`{"n": 42, "b": true, "y": null}`, workflow.Context{}))

	require.NoError(t, err)
	require.Equal(t, float64(42), out["n"])
	require.Equal(t, true, out["b"])
	require.Nil(t, out["y"])
}

func TestRenderArguments_BozukJSONSebebiyleReddedilir(t *testing.T) {
	_, err := (&MCPHandler{}).renderArguments(girdi(`{"a": `, workflow.Context{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "JSON")
}

/*
ÇÖZÜLEMEYEN REFERANS HANGİ ARGÜMANDA OLDUĞUNU SÖYLER.

Şablon katmanı bilinmeyen referansı boş metne çevirmiyor, hata veriyor — sessiz
boşluk, dış araca eksik veri göndermek olurdu. Ama hata tek başına "çözümlenemedi"
deseydi kullanıcı on argümanlık bir nesnede hangisinin bozuk olduğunu aramak
zorunda kalırdı.
*/
func TestRenderArguments_CozulemeyenReferansArgumaniAdlandirir(t *testing.T) {
	_, err := (&MCPHandler{}).renderArguments(
		girdi(`{"hedef": "{{ steps.olmayan.output }}"}`, workflow.Context{}))

	require.Error(t, err)
	require.Contains(t, err.Error(), "hedef", "hata hangi argümanda olduğunu söylemeli")
}
