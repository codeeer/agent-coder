package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReferences_Cozumleme(t *testing.T) {
	refs, err := References(
		"Görev: {{ input }}\nAnaliz:\n{{steps.a1.output}}\nDiff: {{ steps.a2.diff }}\n" +
			"Konu: {{ trigger.summary }}")
	require.NoError(t, err)
	require.Len(t, refs, 4)

	require.Equal(t, RootInput, refs[0].Root)
	require.Equal(t, "a1", refs[1].Node)
	require.Equal(t, FieldOutput, refs[1].Field)
	require.Equal(t, "a2", refs[2].Node)
	require.Equal(t, FieldDiff, refs[2].Field)
	require.Equal(t, "summary", refs[3].Field)
}

func TestReferences_BozukBicimHataVerir(t *testing.T) {
	tests := []struct {
		ad     string
		metin  string
		icerir string
	}{
		{"eksik alan", "{{ steps.a1 }}", "biçiminde olmalı"},
		{"olmayan alan", "{{ steps.a1.cikti }}", "diye bir alan yok"},
		{"bilinmeyen kök", "{{ sonuc.a1 }}", "bilinmeyen değişken"},
		{"input alan alır", "{{ input.text }}", "alan almaz"},
		{"trigger alansız", "{{ trigger }}", "biçiminde olmalı"},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			_, err := References(tt.metin)
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.icerir)
		})
	}
}

// TestReferences_KalibaUymayanSusluParantezMetindir, kullanıcının kod örneği bozulmamalı.
func TestReferences_KalibaUymayanSusluParantezMetindir(t *testing.T) {
	// Boşluklu ifade, JSON gövdesi, tek parantez — hiçbiri referans değil.
	metin := "Şu JSON'u üret: {{ \"a\": 1 }} ve {{ iki kelime }} ile { tek }"
	refs, err := References(metin)
	require.NoError(t, err)
	require.Empty(t, refs)

	out, err := Render(metin, Context{})
	require.NoError(t, err)
	require.Equal(t, metin, out, "referans olmayan süslü parantez olduğu gibi kalmalı")
}

func TestRender_DegerleriYerlestirir(t *testing.T) {
	out, err := Render(
		"Görev: {{ input }}\n\nAnaliz:\n{{ steps.a1.output }}\n\nBranch: {{steps.a1.branch}}",
		Context{
			Input: "hata düzelt",
			Steps: map[string]StepResult{
				"a1": {Output: "şu dosyaya bak", Branch: "agent/x"},
			},
		})
	require.NoError(t, err)
	require.Equal(t,
		"Görev: hata düzelt\n\nAnaliz:\nşu dosyaya bak\n\nBranch: agent/x", out)
}

// TestRender_BilinmeyenReferansHataVerir — spec'in en önemli maddesi:
// sessizce boş metinle çalışmak yasak.
func TestRender_BilinmeyenReferansHataVerir(t *testing.T) {
	_, err := Render("{{ steps.yok.output }}", Context{Steps: map[string]StepResult{}})
	require.Error(t, err)
	require.Contains(t, err.Error(), "henüz çalışmadı")

	_, err = Render("{{ trigger.key }}", Context{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "tetikleyici böyle bir alan göndermedi")
}

// TestRender_BosDegerHataDegildir — adım çalıştı ama diff üretmediyse bu normaldir.
func TestRender_CalismisAdiminBosCiktisiHataDegil(t *testing.T) {
	out, err := Render("diff:[{{ steps.a1.diff }}]",
		Context{Steps: map[string]StepResult{"a1": {Output: "x"}}})
	require.NoError(t, err)
	require.Equal(t, "diff:[]", out)
}

func TestRender_AyniReferansBirdenFazlaKezGecebilir(t *testing.T) {
	out, err := Render("{{ input }} ve yine {{ input }}", Context{Input: "A"})
	require.NoError(t, err)
	require.Equal(t, "A ve yine A", out)
}
