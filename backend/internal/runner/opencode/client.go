// Package opencode, runner.Runner arayüzünün opencode ile uygulamasıdır.
//
// opencode'a dair her şey bu pakette ve sandbox paketinde kalır; sistemin geri
// kalanı yalnızca runner.Runner arayüzünü bilir.
package opencode

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/agent-coder/backend/internal/runner"
)

// Port, container içinde opencode'un dinlediği port.
const Port = 4096

var (
	ErrNotReady = errors.New("opencode zamanında hazır olmadı")
	ErrSession  = errors.New("oturum açılamadı")
	ErrMessage  = errors.New("mesaj gönderilemedi")
)

// client, tek bir container'daki opencode ile konuşur.
type client struct {
	base string
	http *http.Client
}

func newClient(host string) *client {
	return &client{
		base: fmt.Sprintf("http://%s:%d", host, Port),
		// Model çağrısı uzun sürebilir; zaman aşımı context ile yönetilir.
		http: &http.Client{},
	}
}

// waitReady, opencode ayağa kalkana kadar bekler.
func (c *client) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for {
		if err := c.ping(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return ErrNotReady
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func (c *client) ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/global/health", nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sağlık kontrolü %d döndü", resp.StatusCode)
	}
	return nil
}

// mcpState, tek bir MCP sunucusunun motordaki durumu.
type mcpState struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// mcpStatus, tanımlı MCP sunucularının bağlantı durumunu okur.
//
// Motor bağlanamayan sunucuyu sessizce yok saydığı için bu tek görünürlük
// noktası: `connected` dışındaki her durum kullanıcıya bildirilmeli.
func (c *client) mcpStatus(ctx context.Context) (map[string]mcpState, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/mcp", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		return nil, fmt.Errorf("MCP durumu %d döndü", resp.StatusCode)
	}

	var out map[string]mcpState
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("MCP durumu okunamadı: %w", err)
	}
	return out, nil
}

// createSession, yetki kurallarıyla yeni bir oturum açar.
//
// Yetkiler burada gönderilir; agent dosyasına yazılmaz (ölçüldü: bu yol çalışıyor
// ve tek kaynak olması karışıklığı önler).
func (c *client) createSession(ctx context.Context, title string, perms []runner.PermissionRule) (string, error) {
	body := map[string]any{"title": title, "permission": perms}

	var out struct {
		ID string `json:"id"`
	}
	if err := c.postJSON(ctx, "/session", body, &out); err != nil {
		return "", fmt.Errorf("%w: %w", ErrSession, err)
	}
	if out.ID == "" {
		return "", fmt.Errorf("%w: oturum kimliği boş döndü", ErrSession)
	}
	return out.ID, nil
}

// messageResponse, prompt yanıtının ilgilendiğimiz kısmı.
type messageResponse struct {
	Info struct {
		ProviderID   string  `json:"providerID"`
		ModelID      string  `json:"modelID"`
		Cost         float64 `json:"cost"`
		FinishReason string  `json:"finishReason"`
		Error        any     `json:"error"`
		Tokens       struct {
			Input     int `json:"input"`
			Output    int `json:"output"`
			Reasoning int `json:"reasoning"`
			Cache     struct {
				Read  int `json:"read"`
				Write int `json:"write"`
			} `json:"cache"`
		} `json:"tokens"`
	} `json:"info"`
	Parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"parts"`
}

// Text, yanıttaki metin parçalarını birleştirir.
func (m messageResponse) Text() string {
	var b strings.Builder
	for _, p := range m.Parts {
		if p.Type == "text" && p.Text != "" {
			if b.Len() > 0 {
				b.WriteString("\n")
			}
			b.WriteString(p.Text)
		}
	}
	return strings.TrimSpace(b.String())
}

// sendMessage, agent'a görevi verir ve yanıtı bekler.
//
// SENKRON uç kullanılır: /api/session/:id/prompt asenkron ve tamamlanmayı beklemek
// için gereken /api/session/:id/wait bu sürümde uygulanmamış (ölçüldü).
func (c *client) sendMessage(ctx context.Context, sessionID, agent, providerID, modelID, task string) (*messageResponse, error) {
	body := map[string]any{
		"agent": agent,
		"model": map[string]string{"providerID": providerID, "modelID": modelID},
		"parts": []map[string]string{{"type": "text", "text": task}},
	}

	var out messageResponse
	if err := c.postJSON(ctx, "/session/"+sessionID+"/message", body, &out); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrMessage, err)
	}
	return &out, nil
}

// FileChange, değişen bir dosyanın özeti.
type FileChange struct {
	File      string `json:"file"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Status    string `json:"status"` // modified | added | deleted
}

// diff, ÇALIŞMA DİZİNİNDEKİ değişikliklerin unified diff'ini döner.
//
// Oturuma özel /session/:id/diff DEĞİL, çalışma alanına bakan /vcs/diff/raw
// kullanılır: oturum ucu yalnızca o oturumun kendi araçlarıyla yaptığı
// değişiklikleri izliyor ve boşken JSON dizi `[]` döndürüyor (ölçüldü).
// Her çalıştırma kendi container'ını aldığı için çalışma alanı = bu çalıştırma.
func (c *client) diff(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/vcs/diff/raw", nil)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	const maxDiff = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDiff))
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		// Diff alınamaması çalıştırmayı düşürmez; değişiklik yok sayılır.
		return "", nil
	}
	return string(data), nil
}

// changedFiles, değişen dosyaların özetini döner.
//
// Diff'i ayrıştırmak yerine sağlayıcının kendi özetini kullanıyoruz: arayüz
// "3 dosya, +42 −8" göstermek için bunu kullanacak.
func (c *client) changedFiles(ctx context.Context) ([]FileChange, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/vcs/status", nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	const maxBody = 4 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var out []FileChange
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, nil // özet alınamazsa diff yine geçerli
	}
	return out, nil
}

// abort, çalışan bir oturumu durdurur.
func (c *client) abort(ctx context.Context, sessionID string) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.base+"/session/"+sessionID+"/abort", nil)
	if err != nil {
		return
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
}

// streamEvents, opencode'un olay akışını dinler ve emit ile bildirir.
//
// ctx iptal edilince akış kapanır. Akışın kopması çalıştırmayı DÜŞÜRMEZ:
// olaylar yalnızca bilgilendirme amaçlıdır, sonuç mesaj yanıtından gelir.
func (c *client) streamEvents(ctx context.Context, emit func(runner.Event)) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/event", nil)
	if err != nil {
		return
	}
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.http.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64<<10), 1<<20)

	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		if msg := describeEvent(line[len("data: "):]); msg != "" {
			emit(runner.Event{Level: runner.LevelInfo, Message: msg})
		}
	}
}

// describeEvent, ham opencode olayını kullanıcıya gösterilebilir bir satıra çevirir.
//
// Tüm olaylar gösterilmez: iç durum güncellemeleri gürültü yapar. Yalnızca
// kullanıcının "ne oluyor" sorusuna cevap veren olaylar geçirilir.
func describeEvent(raw string) string {
	var e struct {
		Type       string `json:"type"`
		Properties struct {
			Part struct {
				Type  string `json:"type"`
				Tool  string `json:"tool"`
				State struct {
					Status string `json:"status"`
					Title  string `json:"title"`
				} `json:"state"`
			} `json:"part"`
		} `json:"properties"`
	}
	if json.Unmarshal([]byte(raw), &e) != nil {
		return ""
	}

	switch e.Type {
	case "server.connected":
		return "çalışma ortamına bağlanıldı"

	case "message.part.updated":
		p := e.Properties.Part
		if p.Type != "tool" || p.Tool == "" {
			return ""
		}
		// Aynı aracın her durum güncellemesi değil, yalnızca tamamlananlar.
		if p.State.Status != "completed" {
			return ""
		}
		if p.State.Title != "" {
			return p.Tool + ": " + p.State.Title
		}
		return "araç çalıştı: " + p.Tool
	}
	return ""
}

// postJSON, ortak POST yolu.
func (c *client) postJSON(ctx context.Context, path string, body, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("gövde kodlanamadı: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+path, strings.NewReader(string(data)))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	const maxBody = 32 << 20
	respData, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fmt.Errorf("yanıt okunamadı: %w", err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("durum %d: %s", resp.StatusCode, truncate(string(respData), 300))
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(respData, out); err != nil {
		return fmt.Errorf("yanıt ayrıştırılamadı: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

/*
 * sessionTranscript, bir oturumun tam mesaj ve parça geçmişini döner.
 *
 * NEDEN DOSYADAN DEĞİL API'DEN: motorun oturum deposu bu sürümde SQLite
 * (`opencode.db`) — ölçüldü. Bir veritabanı dosyasını ham kopyalamak metin
 * üretmez, maskeleme uygulanamaz ve okunamaz. API ise aynı veriyi JSON
 * olarak veriyor: her mesajın rolü, metni, akıl yürütmesi ve araç çağrıları.
 *
 * Çıktı GİRİNTİLİ yazılıyor: bu içerik satır numaralı bir görüntüleyicide
 * okunacak, tek satırlık 10 KB'lık bir JSON orada işe yaramaz.
 */
func (c *client) sessionTranscript(ctx context.Context, sessionID string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		c.base+"/session/"+sessionID+"/message", nil)
	if err != nil {
		return "", err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	const maxBody = 32 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", fmt.Errorf("oturum geçmişi okunamadı: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("durum %d: %s", resp.StatusCode, truncate(string(data), 300))
	}

	var guzel bytes.Buffer
	if err := json.Indent(&guzel, data, "", "  "); err != nil {
		// Biçimlendirilemeyen yanıt yine de saklanır: bozuk JSON'un kendisi
		// de teşhis verisidir.
		return string(data), nil
	}
	return guzel.String(), nil
}
