package mcpserver_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestCanliSunucu, ayakta duran gerçek sunucuya bir MCP istemcisiyle bağlanır.
//
// Yalnızca AGENT_CODER_MCP_URL verildiğinde çalışır: birim testlerin çalışan
// bir kuruluma bağımlı olması yanlış olurdu.
func TestCanliSunucu(t *testing.T) {
	url := os.Getenv("AGENT_CODER_MCP_URL")
	if url == "" {
		t.Skip("AGENT_CODER_MCP_URL verilmedi")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	client := sdk.NewClient(&sdk.Implementation{Name: "deneme-istemcisi"}, nil)
	session, err := client.Connect(ctx, &sdk.StreamableClientTransport{
		Endpoint: url, MaxRetries: -1, DisableStandaloneSSE: true,
	}, nil)
	if err != nil {
		t.Fatalf("bağlanılamadı: %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("araçlar okunamadı: %v", err)
	}
	for _, tool := range tools.Tools {
		t.Logf("ARAÇ: %s — %s", tool.Name, tool.Description)
	}

	call := func(name string, args map[string]any) map[string]any {
		res, err := session.CallTool(ctx, &sdk.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("%s çağrılamadı: %v", name, err)
		}
		if res.IsError {
			t.Fatalf("%s hata döndü: %+v", name, res.Content)
		}
		out := map[string]any{}
		if res.StructuredContent != nil {
			b, _ := json.Marshal(res.StructuredContent)
			_ = json.Unmarshal(b, &out)
		}
		return out
	}

	list := call("akislari_listele", map[string]any{"onlyRunnable": true})
	items, _ := list["workflows"].([]any)
	t.Logf("ÇALIŞTIRILABİLİR AKIŞ: %d", len(items))
	if len(items) == 0 {
		t.Skip("çalıştırılabilir akış yok")
	}

	first, _ := items[0].(map[string]any)
	t.Logf("SEÇİLEN: %v (%v)", first["name"], first["project"])

	run := call("akis_calistir", map[string]any{
		"workflowId": first["id"], "input": "MCP istemcisinden başlatıldı",
	})
	t.Logf("BAŞLATILDI: runId=%v status=%v", run["runId"], run["status"])

	// Durum sorgusu: bitmesini beklemiyoruz, aracın cevap verdiğini görüyoruz.
	st := call("calisma_durumu", map[string]any{"runId": run["runId"]})
	t.Logf("DURUM: %v | adım: %d", st["status"], len(st["steps"].([]any)))
}
