package opencode

import (
	"strings"
	"testing"

	"github.com/agent-coder/backend/internal/runner"
	"github.com/stretchr/testify/require"
)

/*
Container'a giden `PROJECT_DIR`, çalıştırma başında hesaplanan değeri AYNEN
taşır — burada yeniden hesaplanmaz (spec 025).

İki test bir arada duruyor çünkü asıl iddia ikisinin farkında: alan boşken
bugünkü davranış, doluyken verilen değer. Yalnızca ikincisi olsaydı,
alanı doldurmayan çağıranın ne aldığı sınanmamış kalırdı.
*/
func TestBuildEnv_ProjeDiziniBosSaVarsayilan(t *testing.T) {
	env := buildEnv(runner.Request{}, "")

	require.Equal(t, runner.WorkRoot, env["PROJECT_DIR"])
}

func TestBuildEnv_ProjeDiziniVerilenDegeriTasir(t *testing.T) {
	env := buildEnv(runner.Request{ProjectDir: "/work/agent-coder"}, "")

	require.Equal(t, "/work/agent-coder", env["PROJECT_DIR"])
}

/*
ENV İLE TALİMAT METNİ AYRIŞAMAZ — bu spec 025'in asıl iddiası.

İki tüketici var: container'ın `PROJECT_DIR` değişkeni (betikler bunu okur) ve
agent'a verilen talimat metni (model bunu okur). Aynı `Request` alanından
besleniyorlar. Ayrışsalardı ikisi de tek başına "çalışıyor" görünür, hata
ancak betik var olmayan bir dizine baktığında ortaya çıkardı — ve o an
kullanıcının çalıştırmasının ortasıdır.

Bu yüzden test iki tarafı da AYNI istekten üretip karşılaştırıyor; birini
sabit bir beklentiyle sınamak bu ayrışmayı yakalamazdı.
*/
func TestProjeDizini_EnvVeTalimatAyni(t *testing.T) {
	for _, dir := range []string{runner.WorkRoot, "/work/agent-coder"} {
		t.Run(dir, func(t *testing.T) {
			req := runner.Request{
				ProjectDir: dir,
				Provider:   runner.ProviderSpec{Slug: "or", Kind: "openrouter"},
				Agent: runner.AgentSpec{
					Slug: "coder", Prompt: "x", AllowBash: true,
					Scripts: []runner.ScriptSpec{{Name: "kur", Content: "echo"}},
				},
				Model: "model-x",
			}

			env := buildEnv(req, "")

			files, err := runner.BuildConfigFiles(req.Provider, req.Agent, req.Model,
				req.Packages, req.ProjectDir)
			require.NoError(t, err)

			var talimat string
			for _, f := range files {
				if strings.HasSuffix(f.Path, "/agents/coder.md") {
					talimat = string(f.Content)
				}
			}
			require.NotEmpty(t, talimat, "agent talimat dosyası üretilmeliydi")

			require.Contains(t, talimat, "`"+env["PROJECT_DIR"]+"` altında",
				"talimat metni env ile aynı yolu söylemeli")
		})
	}
}
