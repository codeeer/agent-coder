// Package workflow, akış tanımını (graf), doğrulamasını ve yürütmesini barındırır.
//
// Graf ve şablon çözümleme BİLEREK veritabanından ve HTTP'den bağımsızdır: motorun
// en çok hata barındıran kısmı burasıdır ve en ucuz doğrulama yeri birim testtir.
package workflow

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// NodeKind, düğüm türü.
//
// Açık uçlu bir metin: koşul ve HTTP düğümleri sonradan eklendiğinde migration
// gerekmesin diye (spec 007 K3).
type NodeKind string

const (
	KindTriggerManual  NodeKind = "trigger.manual"
	KindTriggerWebhook NodeKind = "trigger.webhook"
	KindTriggerJira    NodeKind = "trigger.jira"
	KindAgent          NodeKind = "agent"
	KindGitHubPR       NodeKind = "github.pr"
	KindJiraComment    NodeKind = "jira.comment"
	KindMCPCall        NodeKind = "mcp.call"
)

// IsTrigger, düğümün bir giriş noktası olup olmadığı.
func (k NodeKind) IsTrigger() bool {
	return strings.HasPrefix(string(k), "trigger.")
}

// Position, tuvaldeki konum.
//
// Bu fazda KULLANILMIYOR ama saklanıyor: Faz 4'te tuval geldiğinde graf yapısının
// değişmesi, kayıtlı bütün akışların taşınması demek olurdu.
type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// NodeConfig, düğümün yapılandırması. Alanlar türe göre anlamlıdır.
type NodeConfig struct {
	// ── Agent düğümü ────────────────────────────────────────────────────────
	AgentID    string `json:"agentId,omitempty"`
	ProviderID string `json:"providerId,omitempty"`
	Model      string `json:"model,omitempty"`
	Prompt     string `json:"prompt,omitempty"`
	// Branch boşsa projenin varsayılanı kullanılır.
	Branch string `json:"branch,omitempty"`
	// AutoPush, adımın ürettiği değişikliği kendiliğinden branch'e gönderir.
	//
	// Varsayılan KAPALI: bir akışın habersizce depoya yazması sürpriz olurdu.
	// PR açan bir akışta açıkça seçilmesi gerekir.
	AutoPush bool `json:"autoPush,omitempty"`

	// ── PR düğümü ───────────────────────────────────────────────────────────
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
	// HeadBranch, PR'ın açılacağı kaynak branch. Boşsa gönderim yapan TEK ata
	// adımdan çıkarılır; birden fazlaysa doğrulama açıkça sorar.
	HeadBranch string `json:"headBranch,omitempty"`
	// BaseBranch boşsa projenin varsayılan branch'i kullanılır.
	BaseBranch string `json:"baseBranch,omitempty"`

	// ── Jira yorum düğümü ───────────────────────────────────────────────────
	IssueKey string `json:"issueKey,omitempty"`

	// ── Jira tetikleyici ────────────────────────────────────────────────────
	// JQL, hangi task'ların akışı başlatacağını belirler.
	JQL string `json:"jql,omitempty"`

	// ── MCP araç çağrısı ────────────────────────────────────────────────────
	// MCPServerID, hangi sunucudaki araç çağrılacak.
	MCPServerID string `json:"mcpServerId,omitempty"`
	// ToolName, sunucunun kendi adlandırmasıyla araç adı (`ask_question`).
	ToolName string `json:"toolName,omitempty"`
	/*
	 * Arguments, aracın argümanları — JSON NESNESİ olarak.
	 *
	 * Şablonlar JSON'un İÇİNDE, string değerlerinde durur:
	 *   {"repoName": "{{ trigger.key }}", "question": "{{ steps.analiz.output }}"}
	 *
	 * Çözümleme sırası önemlidir: önce JSON ayrıştırılır, SONRA her string değer
	 * şablondan geçirilir. Ters sırayla yapılsaydı, içinde tırnak veya satır sonu
	 * olan bir agent çıktısı JSON'u bozardı — ve bu, ancak belirli bir çıktı
	 * geldiğinde patlayan türden bir hata olurdu.
	 */
	Arguments string `json:"arguments,omitempty"`
}

// Node, akıştaki bir adım veya tetikleyici.
type Node struct {
	ID       string     `json:"id"`
	Kind     NodeKind   `json:"kind"`
	Name     string     `json:"name,omitempty"`
	Config   NodeConfig `json:"config"`
	Position Position   `json:"position"`
}

// Edge, iki düğüm arasındaki bağ.
type Edge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Graph, akışın tanımı.
type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

// ParseGraph, JSON gövdesini grafa çevirir.
func ParseGraph(raw []byte) (Graph, error) {
	var g Graph
	if err := json.Unmarshal(raw, &g); err != nil {
		return Graph{}, fmt.Errorf("akış tanımı okunamadı: %w", err)
	}
	return g, nil
}

// Problem, doğrulamada bulunan tek bir kusur.
//
// NodeID taşır çünkü arayüz kullanıcıya HANGİ ADIMDA ne yanlış olduğunu
// göstermeli; "graf geçersiz" demek kullanıcıyı hatayı aramaya bırakır.
type Problem struct {
	NodeID  string `json:"nodeId,omitempty"`
	Message string `json:"message"`
}

// ValidationError, doğrulamada bulunan tüm kusurlar.
//
// Kusurlar TEK SEFERDE toplanır, ilkinde durulmaz: kullanıcı akışını tek turda
// düzeltebilsin diye (config.Load ile aynı yaklaşım).
type ValidationError struct {
	Problems []Problem
}

func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Problems))
	for _, p := range e.Problems {
		if p.NodeID != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", p.NodeID, p.Message))
		} else {
			parts = append(parts, p.Message)
		}
	}
	return "akış geçersiz: " + strings.Join(parts, "; ")
}

// Node, kimliğe göre düğümü döner.
func (g Graph) Node(id string) (Node, bool) {
	for _, n := range g.Nodes {
		if n.ID == id {
			return n, true
		}
	}
	return Node{}, false
}

// Trigger, akışın giriş düğümünü döner.
func (g Graph) Trigger() (Node, bool) {
	for _, n := range g.Nodes {
		if n.Kind.IsTrigger() {
			return n, true
		}
	}
	return Node{}, false
}

// Validate, grafı kaydedilmeye uygun mu diye sınar.
func (g Graph) Validate() error {
	var problems []Problem
	add := func(nodeID, format string, args ...any) {
		problems = append(problems, Problem{NodeID: nodeID, Message: fmt.Sprintf(format, args...)})
	}

	if len(g.Nodes) == 0 {
		add("", "akışta hiç adım yok")
		return &ValidationError{Problems: problems}
	}

	// ── Düğümler ────────────────────────────────────────────────────────────
	seen := make(map[string]bool, len(g.Nodes))
	triggers := 0

	for _, n := range g.Nodes {
		if strings.TrimSpace(n.ID) == "" {
			add("", "kimliği olmayan bir adım var")
			continue
		}
		if seen[n.ID] {
			add(n.ID, "bu kimlik birden fazla adımda kullanılmış")
			continue
		}
		seen[n.ID] = true

		// Türe özel kurallar KAYIT DEFTERİNDEN geliyor; buraya yeni bir tür
		// için dal eklemek gerekmiyor.
		spec, known := Spec(n.Kind)
		if !known {
			add(n.ID, "bilinmeyen adım türü %q", n.Kind)
			continue
		}
		if n.Kind.IsTrigger() {
			triggers++
		}
		if spec.Fields != nil {
			for _, msg := range spec.Fields(n) {
				add(n.ID, "%s", msg)
			}
		}
	}

	switch {
	case triggers == 0:
		add("", "akışın başlangıcı yok — bir tetikleyici eklenmeli")
	case triggers > 1:
		add("", "akışta %d tetikleyici var, yalnızca bir tane olabilir", triggers)
	}

	// ── Kenarlar ────────────────────────────────────────────────────────────
	edgeSeen := make(map[Edge]bool, len(g.Edges))
	for _, e := range g.Edges {
		if !seen[e.From] {
			add("", "%q diye bir adım yok ama ondan bir bağ çıkıyor", e.From)
			continue
		}
		if !seen[e.To] {
			add("", "%q diye bir adım yok ama ona bir bağ giriyor", e.To)
			continue
		}
		if e.From == e.To {
			add(e.From, "adım kendine bağlanamaz")
			continue
		}
		if edgeSeen[e] {
			add("", "%s → %s bağı birden fazla kez tanımlanmış", e.From, e.To)
			continue
		}
		edgeSeen[e] = true

		if to, ok := g.Node(e.To); ok && to.Kind.IsTrigger() {
			add(e.To, "tetikleyici akışın başıdır, ona bağ giremez")
		}
	}

	// Yapısal kusur varsa sıra/erişilebilirlik hesabı anlamsız — ve yanıltıcı
	// ikinci bir hata yığını üretirdi.
	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}

	// ── Döngü ve erişilebilirlik ────────────────────────────────────────────
	if _, err := g.Levels(); err != nil {
		add("", "%s", err.Error())
		return &ValidationError{Problems: problems}
	}

	trigger, _ := g.Trigger()
	reachable := g.reachableFrom(trigger.ID)
	for _, n := range g.Nodes {
		if !reachable[n.ID] {
			add(n.ID, "bu adıma akışın başından ulaşılamıyor, hiç çalışmaz")
		}
	}

	// ── Şablon referansları ─────────────────────────────────────────────────
	ancestors := g.ancestors()
	for _, n := range g.Nodes {
		spec, ok := Spec(n.Kind)
		if !ok || spec.Templates == nil {
			continue
		}

		for _, text := range spec.Templates(n) {
			refs, err := References(text)
			if err != nil {
				add(n.ID, "%s", err.Error())
				continue
			}
			for _, ref := range refs {
				if ref.Root != RootSteps {
					continue
				}
				if !seen[ref.Node] {
					add(n.ID, "%s: `%s` diye bir adım yok", ref.Raw, ref.Node)
					continue
				}
				// ÖNEMLİ: referans yalnızca ÖNCE çalışmış bir adıma olabilir.
				// Aksi halde değer çalışma anında boş gelir ve adım eksik
				// bilgiyle çalışır — spec'in yasakladığı sessiz yanlışlık.
				if !ancestors[n.ID][ref.Node] {
					add(n.ID, "%s: `%s` bu adımdan önce çalışmıyor, çıktısı burada kullanılamaz",
						ref.Raw, ref.Node)
				}
			}
		}
	}

	// ── PR düğümünün kaynak branch'i ────────────────────────────────────────
	//
	// "Hangi branch'ten PR açılacak?" sorusu ÇALIŞMA ANINDA tahmin edilmez.
	// Boş bırakıldıysa, gönderim yapan ata adım TEK olmalı; sıfır veya birden
	// fazlaysa kullanıcıya burada sorulur. Aksi halde akış dakikalar sonra,
	// para harcandıktan sonra patlardı.
	for _, n := range g.Nodes {
		if n.Kind != KindGitHubPR || strings.TrimSpace(n.Config.HeadBranch) != "" {
			continue
		}

		var pushers []string
		for id := range ancestors[n.ID] {
			if a, ok := g.Node(id); ok && a.Kind == KindAgent && a.Config.AutoPush {
				pushers = append(pushers, id)
			}
		}
		sort.Strings(pushers)

		switch len(pushers) {
		case 0:
			add(n.ID, "PR açılacak branch yok — önceki bir agent adımında "+
				"\"değişikliği branch'e gönder\" açılmalı")
		case 1:
			// Tek kaynak var, çalışma anında o kullanılacak.
		default:
			add(n.ID, "birden fazla adım branch gönderiyor (%s) — hangisinden "+
				"PR açılacağı belirtilmeli", strings.Join(pushers, ", "))
		}
	}

	if len(problems) > 0 {
		return &ValidationError{Problems: problems}
	}
	return nil
}

// PushingAncestor, PR düğümünün kaynak branch'ini üreten ata adımı döner.
//
// Doğrulama tek olduğunu garanti ettiği için burada ilk bulunan dönebilir.
func (g Graph) PushingAncestor(nodeID string) (string, bool) {
	for id := range g.ancestors()[nodeID] {
		if a, ok := g.Node(id); ok && a.Kind == KindAgent && a.Config.AutoPush {
			return id, true
		}
	}
	return "", false
}

// Levels, düğümleri topolojik seviyelere böler.
//
// Aynı seviyedeki düğümler birbirine bağlı değildir; motor onları AYNI ANDA
// çalıştırır. Düz bir topolojik sıra bunu söyleyemezdi.
//
// Düğüm sırası her seviyede girdi sırasını korur — çıktının deterministik olması
// hem test hem arayüz için gerekli.
func (g Graph) Levels() ([][]Node, error) {
	indegree := make(map[string]int, len(g.Nodes))
	next := make(map[string][]string, len(g.Nodes))
	for _, n := range g.Nodes {
		indegree[n.ID] = 0
	}
	for _, e := range g.Edges {
		next[e.From] = append(next[e.From], e.To)
		indegree[e.To]++
	}

	remaining := len(g.Nodes)
	var levels [][]Node

	for remaining > 0 {
		var level []Node
		for _, n := range g.Nodes {
			if indegree[n.ID] == 0 {
				level = append(level, n)
			}
		}
		if len(level) == 0 {
			// Kalan düğümlerin hepsinin gireni var → aralarında döngü var.
			return nil, fmt.Errorf("akışta döngü var: adımlar birbirini bekliyor")
		}

		for _, n := range level {
			indegree[n.ID] = -1 // bir daha seçilmesin
			remaining--
		}
		// Kenarlar seviye BİTTİKTEN sonra düşülür; aksi halde aynı seviyedeki
		// bir düğüm diğerini serbest bırakıp yanlış seviyeye taşırdı.
		for _, n := range level {
			for _, to := range next[n.ID] {
				if indegree[to] > 0 {
					indegree[to]--
				}
			}
		}
		levels = append(levels, level)
	}
	return levels, nil
}

// reachableFrom, verilen düğümden ulaşılabilen düğümler.
func (g Graph) reachableFrom(start string) map[string]bool {
	next := make(map[string][]string, len(g.Nodes))
	for _, e := range g.Edges {
		next[e.From] = append(next[e.From], e.To)
	}

	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, to := range next[cur] {
			if !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

// ancestors, her düğüm için kendisinden ÖNCE çalışan tüm düğümleri döner.
func (g Graph) ancestors() map[string]map[string]bool {
	prev := make(map[string][]string, len(g.Nodes))
	for _, e := range g.Edges {
		prev[e.To] = append(prev[e.To], e.From)
	}

	out := make(map[string]map[string]bool, len(g.Nodes))
	for _, n := range g.Nodes {
		set := map[string]bool{}
		queue := append([]string(nil), prev[n.ID]...)
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			if set[cur] {
				continue
			}
			set[cur] = true
			queue = append(queue, prev[cur]...)
		}
		out[n.ID] = set
	}
	return out
}

// ExecutableNodes, motorun çalıştıracağı adımları döner (tetikleyici hariç).
func (g Graph) ExecutableNodes() []Node {
	out := make([]Node, 0, len(g.Nodes))
	for _, n := range g.Nodes {
		if spec, ok := Spec(n.Kind); ok && spec.Executable {
			out = append(out, n)
		}
	}
	return out
}
