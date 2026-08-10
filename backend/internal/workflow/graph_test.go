package workflow

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// trigger, testler için giriş düğümü.
func trigger(id string) Node {
	return Node{ID: id, Kind: KindTriggerManual}
}

// agent, testler için geçerli bir adım.
func agent(id, prompt string) Node {
	return Node{
		ID:     id,
		Kind:   KindAgent,
		Config: NodeConfig{AgentID: "agent-1", Model: "m", Prompt: prompt},
	}
}

// problems, doğrulama hatasını Problem listesine çevirir.
func problems(t *testing.T, err error) []Problem {
	t.Helper()
	require.Error(t, err)

	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	return ve.Problems
}

// hasProblem, verilen düğüm için mesajı içeren bir kusur var mı?
func hasProblem(ps []Problem, nodeID, contains string) bool {
	for _, p := range ps {
		if p.NodeID == nodeID && containsFold(p.Message, contains) {
			return true
		}
	}
	return false
}

func containsFold(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func TestValidate_GecerliAkis(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			trigger("t1"),
			agent("a1", "Görevi analiz et: {{ input }}"),
			agent("a2", "Analize göre uygula:\n{{ steps.a1.output }}"),
		},
		Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a2"}},
	}
	require.NoError(t, g.Validate())
}

func TestValidate_BosAkis(t *testing.T) {
	ps := problems(t, Graph{}.Validate())
	require.True(t, hasProblem(ps, "", "hiç adım yok"))
}

func TestValidate_TetikleyiciKurallari(t *testing.T) {
	t.Run("tetikleyici yok", func(t *testing.T) {
		g := Graph{Nodes: []Node{agent("a1", "x")}}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "", "başlangıcı yok"))
	})

	t.Run("iki tetikleyici", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), trigger("t2"), agent("a1", "x")},
			Edges: []Edge{{From: "t1", To: "a1"}},
		}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "", "yalnızca bir tane"))
	})

	t.Run("tetikleyiciye bağ giremez", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), agent("a1", "x")},
			Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "t1"}},
		}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "t1", "bağ giremez"))
	})
}

func TestValidate_KenarHedefleri(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x")},
		Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "yok"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "", "\"yok\" diye bir adım yok"))
}

func TestValidate_KendineBaglanamaz(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x")},
		Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a1"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "a1", "kendine bağlanamaz"))
}

func TestValidate_Dongu(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("a2", "y"), agent("a3", "z")},
		Edges: []Edge{
			{From: "t1", To: "a1"},
			{From: "a1", To: "a2"},
			{From: "a2", To: "a3"},
			{From: "a3", To: "a1"}, // döngü
		},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "", "döngü"))
}

func TestValidate_ErisilemeyenAdim(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("kopuk", "y")},
		Edges: []Edge{{From: "t1", To: "a1"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "kopuk", "ulaşılamıyor"))
}

func TestValidate_AgentAlanlari(t *testing.T) {
	g := Graph{
		Nodes: []Node{
			trigger("t1"),
			{ID: "a1", Kind: KindAgent, Config: NodeConfig{Prompt: "x"}},                 // agent yok
			{ID: "a2", Kind: KindAgent, Config: NodeConfig{AgentID: "a", Prompt: "   "}}, // talimat boş
		},
		Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a2"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "a1", "agent seçilmemiş"))
	require.True(t, hasProblem(ps, "a2", "talimat boş"))
}

func TestValidate_BilinmeyenAdimTuru(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), {ID: "x1", Kind: "http.request"}},
		Edges: []Edge{{From: "t1", To: "x1"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "x1", "bilinmeyen adım türü"))
}

// TestValidate_SablonAtaOlmayanAdimaBakamaz — spec'in "sessizce boş metinle
// çalışmaz" maddesinin en ucuz karşılandığı yer: kaydetme anı.
func TestValidate_SablonAtaOlmayanAdimaBakamaz(t *testing.T) {
	t.Run("sonraki adıma bakıyor", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{
				trigger("t1"),
				agent("a1", "{{ steps.a2.output }}"), // a2 daha sonra çalışıyor
				agent("a2", "x"),
			},
			Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a2"}},
		}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "a1", "bu adımdan önce çalışmıyor"))
	})

	t.Run("paralel kardeşe bakıyor", func(t *testing.T) {
		// a1 ve a2 aynı seviyede; a2 bittiğinde a1'in çalışmış olma garantisi yok.
		g := Graph{
			Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("a2", "{{ steps.a1.output }}")},
			Edges: []Edge{{From: "t1", To: "a1"}, {From: "t1", To: "a2"}},
		}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "a2", "bu adımdan önce çalışmıyor"))
	})

	t.Run("var olmayan adıma bakıyor", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), agent("a1", "{{ steps.hayalet.output }}")},
			Edges: []Edge{{From: "t1", To: "a1"}},
		}
		ps := problems(t, g.Validate())
		require.True(t, hasProblem(ps, "a1", "diye bir adım yok"))
	})

	t.Run("dolaylı ata geçerlidir", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("a2", "y"),
				agent("a3", "{{ steps.a1.output }}")},
			Edges: []Edge{{From: "t1", To: "a1"}, {From: "a1", To: "a2"}, {From: "a2", To: "a3"}},
		}
		require.NoError(t, g.Validate())
	})
}

func TestValidate_TumKusurlarTekSeferdeDoner(t *testing.T) {
	// Kullanıcı akışını tek turda düzeltebilsin diye ilkinde durulmaz.
	g := Graph{
		Nodes: []Node{
			{ID: "a1", Kind: KindAgent, Config: NodeConfig{Prompt: "x"}},
			{ID: "a2", Kind: KindAgent, Config: NodeConfig{AgentID: "a"}},
		},
	}
	ps := problems(t, g.Validate())
	require.GreaterOrEqual(t, len(ps), 3, "agent yok + talimat boş + tetikleyici yok")
}

func TestLevels_ParalelAdimlarAyniSeviyede(t *testing.T) {
	//        t1
	//       /  \
	//      a1   a2      ← aynı seviye, aynı anda çalışırlar
	//       \  /
	//        a3
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("a2", "y"), agent("a3", "z")},
		Edges: []Edge{
			{From: "t1", To: "a1"}, {From: "t1", To: "a2"},
			{From: "a1", To: "a3"}, {From: "a2", To: "a3"},
		},
	}

	levels, err := g.Levels()
	require.NoError(t, err)
	require.Len(t, levels, 3)
	require.Equal(t, []string{"t1"}, ids(levels[0]))
	require.Equal(t, []string{"a1", "a2"}, ids(levels[1]))
	require.Equal(t, []string{"a3"}, ids(levels[2]))
}

// TestLevels_UzunDalKisaDaliBeklemez — a3 iki seviye derinde olsa da
// a2'nin onu beklememesi gerekir; seviye hesabı en uzun yola göredir.
func TestLevels_DerinlikEnUzunYolaGore(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), agent("a1", "x"), agent("a2", "y"), agent("son", "z")},
		Edges: []Edge{
			{From: "t1", To: "a1"},
			{From: "a1", To: "a2"},
			{From: "a1", To: "son"},
			{From: "a2", To: "son"},
		},
	}
	levels, err := g.Levels()
	require.NoError(t, err)
	require.Equal(t, []string{"son"}, ids(levels[len(levels)-1]),
		"son adım, kendisine giren tüm dallardan sonra çalışmalı")
}

func TestLevels_DonguHataVerir(t *testing.T) {
	g := Graph{
		Nodes: []Node{agent("a1", "x"), agent("a2", "y")},
		Edges: []Edge{{From: "a1", To: "a2"}, {From: "a2", To: "a1"}},
	}
	_, err := g.Levels()
	require.Error(t, err)
	require.Contains(t, err.Error(), "döngü")
}

func TestParseGraph(t *testing.T) {
	raw := []byte(`{
		"nodes": [
			{"id":"t1","kind":"trigger.manual","position":{"x":10,"y":20}},
			{"id":"a1","kind":"agent","config":{"agentId":"x","model":"m","prompt":"p"}}
		],
		"edges": [{"from":"t1","to":"a1"}]
	}`)

	g, err := ParseGraph(raw)
	require.NoError(t, err)
	require.NoError(t, g.Validate())
	require.Len(t, g.ExecutableNodes(), 1)
	// Konum bu fazda kullanılmıyor ama kaybolmamalı — Faz 4 onu bekliyor.
	require.Equal(t, float64(10), g.Nodes[0].Position.X)
}

func ids(nodes []Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

/* ── PR ve Jira düğümleri (spec 009) ─────────────────────────────────────── */

func prNode(id string) Node {
	return Node{ID: id, Kind: KindGitHubPR, Config: NodeConfig{Title: "Başlık"}}
}

func pushingAgent(id string) Node {
	n := agent(id, "x")
	n.Config.AutoPush = true
	return n
}

func TestValidate_PRDugumuAlanlari(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), pushingAgent("kod"),
			{ID: "pr", Kind: KindGitHubPR, Config: NodeConfig{}}},
		Edges: []Edge{{From: "t1", To: "kod"}, {From: "kod", To: "pr"}},
	}
	require.True(t, hasProblem(problems(t, g.Validate()), "pr", "PR başlığı boş"))
}

// TestValidate_PRKaynakBranchKaydetmeAnindaSorulur — "hangi branch'ten?" sorusu
// çalışma anında tahmin edilmez; para harcandıktan sonra patlamasın.
func TestValidate_PRKaynakBranchKaydetmeAnindaSorulur(t *testing.T) {
	t.Run("gönderim yapan ata yok", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), agent("kod", "x"), prNode("pr")},
			Edges: []Edge{{From: "t1", To: "kod"}, {From: "kod", To: "pr"}},
		}
		require.True(t, hasProblem(problems(t, g.Validate()), "pr", "PR açılacak branch yok"))
	})

	t.Run("tek gönderim yapan ata geçerlidir", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), pushingAgent("kod"), prNode("pr")},
			Edges: []Edge{{From: "t1", To: "kod"}, {From: "kod", To: "pr"}},
		}
		require.NoError(t, g.Validate())

		source, ok := g.PushingAncestor("pr")
		require.True(t, ok)
		require.Equal(t, "kod", source)
	})

	t.Run("birden fazla gönderim yapan ata belirsizdir", func(t *testing.T) {
		g := Graph{
			Nodes: []Node{trigger("t1"), pushingAgent("sol"), pushingAgent("sag"), prNode("pr")},
			Edges: []Edge{
				{From: "t1", To: "sol"}, {From: "t1", To: "sag"},
				{From: "sol", To: "pr"}, {From: "sag", To: "pr"},
			},
		}
		require.True(t, hasProblem(problems(t, g.Validate()), "pr", "birden fazla adım branch gönderiyor"))
	})

	t.Run("açıkça yazılan branch belirsizliği çözer", func(t *testing.T) {
		pr := prNode("pr")
		pr.Config.HeadBranch = "{{ steps.sol.branch }}"
		g := Graph{
			Nodes: []Node{trigger("t1"), pushingAgent("sol"), pushingAgent("sag"), pr},
			Edges: []Edge{
				{From: "t1", To: "sol"}, {From: "t1", To: "sag"},
				{From: "sol", To: "pr"}, {From: "sag", To: "pr"},
			},
		}
		require.NoError(t, g.Validate())
	})
}

func TestValidate_JiraDugumuAlanlari(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), {ID: "yorum", Kind: KindJiraComment}},
		Edges: []Edge{{From: "t1", To: "yorum"}},
	}
	ps := problems(t, g.Validate())
	require.True(t, hasProblem(ps, "yorum", "issue anahtarı boş"))
	require.True(t, hasProblem(ps, "yorum", "yorum metni boş"))
}

// TestValidate_YeniDugumlerdeSablonAtaKuraliGecerli — kural türe özel değil.
func TestValidate_YeniDugumlerdeSablonAtaKurali(t *testing.T) {
	yorum := Node{ID: "yorum", Kind: KindJiraComment,
		Config: NodeConfig{IssueKey: "ABC-1", Body: "{{ steps.sonraki.output }}"}}
	g := Graph{
		Nodes: []Node{trigger("t1"), yorum, agent("sonraki", "x")},
		Edges: []Edge{{From: "t1", To: "yorum"}, {From: "yorum", To: "sonraki"}},
	}
	require.True(t, hasProblem(problems(t, g.Validate()), "yorum", "bu adımdan önce çalışmıyor"))
}

func TestExecutableNodes_AgentOlmayanlarDaCalisir(t *testing.T) {
	g := Graph{
		Nodes: []Node{trigger("t1"), pushingAgent("kod"), prNode("pr"),
			{ID: "yorum", Kind: KindJiraComment, Config: NodeConfig{IssueKey: "A-1", Body: "b"}}},
		Edges: []Edge{{From: "t1", To: "kod"}, {From: "kod", To: "pr"}, {From: "pr", To: "yorum"}},
	}
	require.NoError(t, g.Validate())
	require.Len(t, g.ExecutableNodes(), 3, "tetikleyici hariç üç adım")
}
