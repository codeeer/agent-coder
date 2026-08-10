package workflow

import (
	"encoding/json"
	"strings"
)

/*
 * Düğüm türü kayıt defteri.
 *
 * Her tür kendini tanıtır: nasıl doğrulanır, hangi alanları şablon içerir,
 * çalıştırılır mı. Bu olmadan her yeni tür `graph.go` ve `executor.go` içindeki
 * `switch`'lere birer dal eklemek demekti — üç yeni tür için aynı iki dosyayı
 * üç kez değiştirmek.
 *
 * Doğrulama burada, ÇALIŞTIRMA `handler.go` içinde: doğrulama türün sabit
 * bilgisi (bağımlılığı yok), çalıştırma ise Docker'a ve dış servislere bağlı.
 * Ayrı durmaları, grafın veritabanı ve ağ olmadan doğrulanabilmesini sağlıyor.
 */

// KindSpec, bir düğüm türünün sabit tanımı.
type KindSpec struct {
	// Label, arayüzde ve hata mesajlarında görünen ad.
	Label string
	// Executable, motorun bu düğümü çalıştırıp çalıştırmayacağı.
	// Tetikleyiciler giriş noktasıdır, çalıştırılmazlar.
	Executable bool
	// Fields, türe özel alan kontrolleri. Yalnızca mesaj döner; düğüm kimliğini
	// çağıran ekler.
	Fields func(n Node) []string
	// Templates, şablon içerebilecek alanların değerleri. Referans doğrulaması
	// bunlara bakar.
	Templates func(n Node) []string
}

var kindSpecs = map[NodeKind]KindSpec{
	KindTriggerManual: {
		Label:      "Başlangıç",
		Executable: false,
	},
	KindTriggerWebhook: {
		Label:      "Dışarıdan tetikleme",
		Executable: false,
	},
	KindTriggerJira: {
		Label:      "Jira'dan task çek",
		Executable: false,
		Fields: func(n Node) []string {
			if strings.TrimSpace(n.Config.JQL) == "" {
				return []string{"JQL sorgusu boş — hangi task'lar akışı başlatacak?"}
			}
			return nil
		},
	},
	KindMCPCall: {
		Label:      "MCP aracı çağır",
		Executable: true,
		Fields: func(n Node) []string {
			var out []string
			if strings.TrimSpace(n.Config.MCPServerID) == "" {
				out = append(out, "MCP sunucusu seçilmedi")
			}
			if strings.TrimSpace(n.Config.ToolName) == "" {
				out = append(out, "araç seçilmedi")
			}
			// Argümanlar bir JSON NESNESİ olmalı. Bozuk JSON'un çalışma anında
			// ortaya çıkması, akışın yarısı koştuktan sonra durması demekti.
			if raw := strings.TrimSpace(n.Config.Arguments); raw != "" {
				var probe map[string]any
				if err := json.Unmarshal([]byte(raw), &probe); err != nil {
					out = append(out, "argümanlar geçerli bir JSON nesnesi değil")
				}
			}
			return out
		},
		Templates: func(n Node) []string {
			// Şablonlar argüman JSON'unun içindeki string değerlerde; ham metni
			// vermek referans doğrulaması için yeterli.
			return []string{n.Config.Arguments}
		},
	},
	KindAgent: {
		Label:      "Agent adımı",
		Executable: true,
		Fields: func(n Node) []string {
			var out []string
			if n.Config.AgentID == "" {
				out = append(out, "agent seçilmemiş")
			}
			if strings.TrimSpace(n.Config.Prompt) == "" {
				out = append(out, "talimat boş")
			}
			return out
		},
		Templates: func(n Node) []string { return []string{n.Config.Prompt} },
	},
	KindGitHubPR: {
		Label:      "PR aç",
		Executable: true,
		Fields: func(n Node) []string {
			if strings.TrimSpace(n.Config.Title) == "" {
				return []string{"PR başlığı boş"}
			}
			return nil
		},
		Templates: func(n Node) []string {
			return []string{n.Config.Title, n.Config.Body, n.Config.HeadBranch, n.Config.BaseBranch}
		},
	},
	KindJiraComment: {
		Label:      "Jira'ya yorum yaz",
		Executable: true,
		Fields: func(n Node) []string {
			var out []string
			if strings.TrimSpace(n.Config.IssueKey) == "" {
				out = append(out, "issue anahtarı boş")
			}
			if strings.TrimSpace(n.Config.Body) == "" {
				out = append(out, "yorum metni boş")
			}
			return out
		},
		Templates: func(n Node) []string { return []string{n.Config.IssueKey, n.Config.Body} },
	},
}

// Spec, türün tanımını döner.
func Spec(kind NodeKind) (KindSpec, bool) {
	s, ok := kindSpecs[kind]
	return s, ok
}

// Kinds, tanımlı tüm türleri döner (arayüzün paleti için).
func Kinds() map[NodeKind]KindSpec {
	out := make(map[NodeKind]KindSpec, len(kindSpecs))
	for k, v := range kindSpecs {
		out[k] = v
	}
	return out
}
