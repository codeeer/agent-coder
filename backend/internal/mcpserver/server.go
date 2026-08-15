package mcpserver

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agent-coder/backend/internal/paging"
	"github.com/agent-coder/backend/internal/workflow"
)

// serverName, dışarıya kendimizi tanıttığımız ad.
const serverName = "agent-coder"

// Server, akışları MCP aracı olarak sunar.
type Server struct {
	store    *workflow.Store
	launcher *workflow.Launcher
	version  string

	// handler BİR KEZ kurulur ve paylaşılır.
	//
	// İstek başına yeni bir handler üretmek oturum durumunu kaybettiriyor:
	// MCP el sıkışması iki isteğe yayılıyor ve ikincisi "session not found"
	// alıyor (ölçüldü — spec 011 Ölçüm 5).
	handler http.Handler
}

// New yeni MCP sunucusu üretir.
func New(store *workflow.Store, launcher *workflow.Launcher, version string) *Server {
	s := &Server{store: store, launcher: launcher, version: version}
	s.handler = sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server {
		return s.build()
	}, nil)
	return s
}

/* ── Araç girdileri ve çıktıları ─────────────────────────────────────────── */

type listInput struct {
	// Yalnızca çalıştırılabilir akışlar — tanımı olmayanı listelemek, çağıranı
	// başlatamayacağı bir kimliğe yönlendirmek olurdu.
	OnlyRunnable bool `json:"onlyRunnable,omitempty" jsonschema:"yalnızca çalıştırılabilir akışları döner"`
}

type workflowInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Project     string `json:"project"`
	// Runnable false ise akış ya duraklatılmış ya da hiç kaydedilmemiştir.
	Runnable bool   `json:"runnable"`
	Reason   string `json:"reason,omitempty"`
}

type listOutput struct {
	Workflows []workflowInfo `json:"workflows"`
}

type runInput struct {
	WorkflowID string `json:"workflowId" jsonschema:"akislari_listele ile alınan akış kimliği"`
	Input      string `json:"input,omitempty" jsonschema:"akışa verilecek görev metni"`
}

type runOutput struct {
	RunID  string `json:"runId"`
	Status string `json:"status"`
	// Note, çağıranın ne yapması gerektiğini söyler: akış arka planda sürer.
	Note string `json:"note"`
}

type statusInput struct {
	RunID string `json:"runId" jsonschema:"akis_calistir ile alınan çalışma kimliği"`
}

type stepInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Result string `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
}

type statusOutput struct {
	Status  string     `json:"status"`
	Steps   []stepInfo `json:"steps"`
	CostUSD float64    `json:"costUsd"`
}

/* ── Sunucu kurulumu ─────────────────────────────────────────────────────── */

// Handler, MCP isteklerini karşılayan HTTP handler'ı döner.
func (s *Server) Handler() http.Handler { return s.handler }

// build, araçları tanımlanmış bir MCP sunucusu kurar.
//
// Oturum başına çağrılır (handler değil, sunucu): kurulum ucuz ve durum
// taşımıyor.
func (s *Server) build() *sdk.Server {
	srv := sdk.NewServer(&sdk.Implementation{
		Name:    serverName,
		Title:   "Agent Coder",
		Version: s.version,
	}, nil)

	sdk.AddTool(srv, &sdk.Tool{
		Name: "akislari_listele",
		Description: "Agent Coder'da tanımlı akışları listeler. " +
			"Bir akışı başlatmadan önce kimliğini buradan alın.",
	}, s.listWorkflows)

	sdk.AddTool(srv, &sdk.Tool{
		Name: "akis_calistir",
		Description: "Bir akışı başlatır ve hemen döner — akış arka planda sürer. " +
			"Sonucu öğrenmek için calisma_durumu aracını kullanın.",
	}, s.runWorkflow)

	sdk.AddTool(srv, &sdk.Tool{
		Name:        "calisma_durumu",
		Description: "Başlatılmış bir akış çalışmasının durumunu ve adım sonuçlarını döner.",
	}, s.runStatus)

	return srv
}

func (s *Server) listWorkflows(ctx context.Context, _ *sdk.CallToolRequest, in listInput) (
	*sdk.CallToolResult, listOutput, error,
) {
	items, _, err := s.store.List(ctx, nil, paging.Clamp(paging.Max, 0))
	if err != nil {
		return nil, listOutput{}, err
	}

	out := listOutput{Workflows: []workflowInfo{}}
	for _, w := range items {
		info := workflowInfo{
			ID: w.ID.String(), Name: w.Name, Description: w.Description,
			Project: w.ProjectName, Runnable: true,
		}
		switch {
		case w.ActiveVersionID == nil:
			info.Runnable, info.Reason = false, "akışın kaydedilmiş bir tanımı yok"
		case !w.IsActive:
			info.Runnable, info.Reason = false, "akış duraklatıldı"
		}

		if in.OnlyRunnable && !info.Runnable {
			continue
		}
		out.Workflows = append(out.Workflows, info)
	}
	return nil, out, nil
}

func (s *Server) runWorkflow(ctx context.Context, _ *sdk.CallToolRequest, in runInput) (
	*sdk.CallToolResult, runOutput, error,
) {
	id, err := uuid.Parse(strings.TrimSpace(in.WorkflowID))
	if err != nil {
		return nil, runOutput{}, fmt.Errorf(
			"geçersiz akış kimliği — akislari_listele ile alınan kimliği kullanın")
	}

	/*
		DURAKLATILMIŞ AKIŞ BAŞLATILMAZ — ve bu kontrol burada olmak zorunda.

		`Launcher` yalnızca tanımsız akışı ve eşzamanlılık sınırını görüyor;
		duraklatılmış olmayı görmüyor. Bu bilinçli: elle başlatma ve toplu
		çalıştırma kullanıcının O AN verdiği bir karar ve duraklatılmış bir akışı
		bilerek çalıştırabilmeli.

		MCP çağıranı ise bir insan değil, bir agent — yani dışarıdan gelen bir
		yol. Diğer dış yolların hepsi engelliyor: webhook ve tetikleme adresi
		"akış pasif durumda" ile reddediyor, Jira taraması duraklatılmış akışı
		hiç okumuyor.

		Kontrol edilmezse sunucu kendi söylediğinin tersini yapar: aynı sunucu
		`akislari_listele` çağrısında bu akış için "çalıştırılamaz —
		duraklatıldı" diyor, sonra kimliği doğrudan verilince başlatıyordu.
	*/
	wf, err := s.store.Get(ctx, id)
	if err != nil {
		return nil, runOutput{}, err
	}
	if !wf.IsActive {
		return nil, runOutput{}, fmt.Errorf(
			"akış duraklatılmış durumda ve başlatılamaz — " +
				"Agent Coder arayüzünden etkinleştirilmeli")
	}

	// Tanımsız akış ve eşzamanlılık sınırı kontrolleri `Launcher`'da; elle,
	// webhook ve Jira tetiklemesi de aynı kapıdan geçiyor.
	run, err := s.launcher.Launch(ctx, workflow.LaunchInput{
		WorkflowID: id,
		Trigger:    workflow.TriggerMCP,
		Input:      in.Input,
	})
	if err != nil {
		return nil, runOutput{}, err
	}

	return nil, runOutput{
		RunID:  run.ID.String(),
		Status: string(run.Status),
		Note: "Akış arka planda çalışıyor. Sonucu için calisma_durumu aracını " +
			"bu kimlikle çağırın.",
	}, nil
}

func (s *Server) runStatus(ctx context.Context, _ *sdk.CallToolRequest, in statusInput) (
	*sdk.CallToolResult, statusOutput, error,
) {
	id, err := uuid.Parse(strings.TrimSpace(in.RunID))
	if err != nil {
		return nil, statusOutput{}, fmt.Errorf("geçersiz çalışma kimliği")
	}

	run, err := s.store.GetRun(ctx, id)
	if err != nil {
		return nil, statusOutput{}, err
	}

	out := statusOutput{Status: string(run.Status), CostUSD: run.CostUSD, Steps: []stepInfo{}}
	for _, st := range run.Steps {
		info := stepInfo{
			Name:   firstNonEmpty(st.Name, st.NodeID),
			Status: string(st.Status),
			Result: st.ResultText,
		}
		if st.Error != nil {
			info.Error = *st.Error
		}
		out.Steps = append(out.Steps, info)
	}
	return nil, out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
