// Package jira, Jira Cloud üzerinde issue'lara yorum yazar.
//
// Hedef JIRA CLOUD REST API v3'tür (`https://<site>.atlassian.net`, e-posta +
// API token ile basic auth). Data Center farklı kimlik doğrulama ve kısmen
// farklı bir API yüzeyi kullanır; desteklenmesi istenirse ayrı bir iştir.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var (
	// ErrUnauthorized, e-posta veya token geçersiz.
	ErrUnauthorized = errors.New("Jira erişimi yetkisiz")
	// ErrNotFound, issue bulunamadı veya görme yetkisi yok.
	ErrNotFound = errors.New("issue bulunamadı")
)

// Client, Jira Cloud istemcisi.
type Client struct {
	http *http.Client
}

// New yeni istemci üretir.
func New() *Client {
	return &Client{http: &http.Client{Timeout: 30 * time.Second}}
}

// CommentInput, yorum yazma isteği.
type CommentInput struct {
	// BaseURL, Jira sitesinin adresi (`https://sirket.atlassian.net`).
	BaseURL string
	Email   string
	Token   string
	// IssueKey, örn. "ABC-123".
	IssueKey string
	Body     string
}

// Comment, yazılan yorumun özeti.
type Comment struct {
	ID  string `json:"id"`
	URL string `json:"self"`
}

// AddComment, issue'ya yorum yazar.
func (c *Client) AddComment(ctx context.Context, in CommentInput) (Comment, error) {
	if strings.TrimSpace(in.IssueKey) == "" {
		return Comment{}, errors.New("issue anahtarı boş")
	}

	payload, err := json.Marshal(map[string]any{"body": document(in.Body)})
	if err != nil {
		return Comment{}, fmt.Errorf("istek gövdesi hazırlanamadı: %w", err)
	}

	url := fmt.Sprintf("%s/rest/api/3/issue/%s/comment",
		strings.TrimRight(in.BaseURL, "/"), in.IssueKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return Comment{}, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.SetBasicAuth(in.Email, in.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Comment{}, fmt.Errorf("Jira'ya ulaşılamadı: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusCreated {
		var out Comment
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return Comment{}, fmt.Errorf("yanıt okunamadı: %w", err)
		}
		// Kullanıcıya API adresi değil, tarayıcıda açılan adres gösterilmeli.
		out.URL = fmt.Sprintf("%s/browse/%s",
			strings.TrimRight(in.BaseURL, "/"), in.IssueKey)
		return out, nil
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return Comment{}, ErrUnauthorized
	case http.StatusNotFound:
		return Comment{}, fmt.Errorf("%w: %s", ErrNotFound, in.IssueKey)
	default:
		return Comment{}, fmt.Errorf("yorum yazılamadı: durum %d", resp.StatusCode)
	}
}

/*
 * Atlassian Document Format.
 *
 * API v3 yorum gövdesini DÜZ METİN kabul etmiyor, yapılandırılmış belge
 * istiyor. Metin satırlara bölünerek paragraflara çevriliyor — tek paragrafa
 * sıkıştırılsaydı agent'ın çok satırlı çıktısı Jira'da tek bloğa yapışırdı.
 */
func document(text string) map[string]any {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")

	content := make([]map[string]any, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": []map[string]any{{"type": "text", "text": line}},
		})
	}

	// Boş gövde ADF'de geçersiz; en azından bir paragraf olmalı.
	if len(content) == 0 {
		content = append(content, map[string]any{
			"type":    "paragraph",
			"content": []map[string]any{{"type": "text", "text": "(boş)"}},
		})
	}

	return map[string]any{"type": "doc", "version": 1, "content": content}
}

/* ── Issue okuma ─────────────────────────────────────────────────────────── */

// Issue, bir Jira iş kaydının akışa geçen alanları.
//
// Tüm alanlar DEĞİL, şablonda kullanılabilecek olanlar: Jira'nın issue yanıtı
// yüzlerce alan taşıyor ve hepsini taşımak, kullanılmayan yüzeyi bakım yüküne
// çevirirdi.
type Issue struct {
	Key         string `json:"key"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	Status      string `json:"status"`
	IssueType   string `json:"issueType"`
	Assignee    string `json:"assignee"`
	Reporter    string `json:"reporter"`
	UpdatedAt   string `json:"updatedAt"`
	URL         string `json:"url"`
}

// Fields, şablon bağlamına (`{{ trigger.<alan> }}`) geçen alanlar.
func (i Issue) Fields() map[string]string {
	return map[string]string{
		"key":         i.Key,
		"summary":     i.Summary,
		"description": i.Description,
		"status":      i.Status,
		"issueType":   i.IssueType,
		"assignee":    i.Assignee,
		"reporter":    i.Reporter,
		"url":         i.URL,
	}
}

// GetIssue, bir issue'yu okur.
func (c *Client) GetIssue(ctx context.Context, in CommentInput) (Issue, error) {
	base := strings.TrimRight(in.BaseURL, "/")
	url := fmt.Sprintf("%s/rest/api/3/issue/%s", base, in.IssueKey)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Issue{}, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.SetBasicAuth(in.Email, in.Token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Issue{}, fmt.Errorf("Jira'ya ulaşılamadı: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return Issue{}, ErrUnauthorized
	case http.StatusNotFound:
		return Issue{}, fmt.Errorf("%w: %s", ErrNotFound, in.IssueKey)
	default:
		return Issue{}, fmt.Errorf("issue okunamadı: durum %d", resp.StatusCode)
	}

	var raw rawIssue
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Issue{}, fmt.Errorf("yanıt okunamadı: %w", err)
	}
	return raw.toIssue(base), nil
}

// rawIssue, Jira'nın issue gövdesinin okuduğumuz kısmı.
type rawIssue struct {
	Key    string `json:"key"`
	Fields struct {
		Summary     string                       `json:"summary"`
		Description json.RawMessage              `json:"description"`
		Updated     string                       `json:"updated"`
		Status      struct{ Name string }        `json:"status"`
		IssueType   struct{ Name string }        `json:"issuetype"`
		Assignee    struct{ DisplayName string } `json:"assignee"`
		Reporter    struct{ DisplayName string } `json:"reporter"`
	} `json:"fields"`
}

func (r rawIssue) toIssue(base string) Issue {
	return Issue{
		Key:     r.Key,
		Summary: r.Fields.Summary,
		// Açıklama ADF gelir; şablonda kullanılabilmesi için düz metne çevrilir.
		Description: plainText(r.Fields.Description),
		Status:      r.Fields.Status.Name,
		IssueType:   r.Fields.IssueType.Name,
		Assignee:    r.Fields.Assignee.DisplayName,
		Reporter:    r.Fields.Reporter.DisplayName,
		UpdatedAt:   r.Fields.Updated,
		URL:         fmt.Sprintf("%s/browse/%s", base, r.Key),
	}
}

// plainText, Atlassian Document Format belgesini düz metne indirger.
//
// Agent'a ADF ağacı vermenin anlamı yok; talimatına giren şey okunur metin
// olmalı. Ağaçtaki tüm `text` düğümleri toplanır, paragraflar satır sonuyla
// ayrılır.
func plainText(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	// Eski API sürümlerinde açıklama düz metin olabiliyor.
	var asString string
	if err := json.Unmarshal(raw, &asString); err == nil {
		return asString
	}

	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		return ""
	}

	var b strings.Builder
	var walk func(node map[string]any)
	walk = func(node map[string]any) {
		if t, _ := node["type"].(string); t == "text" {
			if s, ok := node["text"].(string); ok {
				b.WriteString(s)
			}
		}
		children, _ := node["content"].([]any)
		for _, c := range children {
			if m, ok := c.(map[string]any); ok {
				walk(m)
			}
		}
		if t, _ := node["type"].(string); t == "paragraph" {
			b.WriteString("\n")
		}
	}
	walk(doc)

	return strings.TrimSpace(b.String())
}
