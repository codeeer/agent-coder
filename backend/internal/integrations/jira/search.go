package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

/*
 * JQL araması.
 *
 * UÇ NOKTA `/rest/api/3/search/jql`. Eski `/rest/api/3/search` Ağustos 2025'te
 * KALDIRILDI ve 410 döndürüyor — kod ona yazılsaydı ilk çalıştırmada patlardı.
 *
 * Yeni uç sayfalamayı `nextPageToken` ile yapıyor (offset değil): bir sayfa
 * okunup token boş gelene kadar devam edilir.
 */

// searchFields, okunan alanlar. Jira varsayılanda yüzlerce alan döndürür;
// istemediğimiz veriyi taşımak ağ ve bellek israfı.
var searchFields = []string{
	"summary", "description", "status", "issuetype", "assignee", "reporter", "updated",
}

// SearchInput, JQL araması.
type SearchInput struct {
	BaseURL string
	Email   string
	Token   string
	JQL     string
	// Limit, en fazla kaç issue okunacağı. Bir taramanın yüzlerce akış
	// başlatmasını engeller.
	Limit int
}

// Search, JQL'e uyan issue'ları döner.
func (c *Client) Search(ctx context.Context, in SearchInput) ([]Issue, error) {
	if strings.TrimSpace(in.JQL) == "" {
		return nil, fmt.Errorf("JQL sorgusu boş")
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}

	base := strings.TrimRight(in.BaseURL, "/")
	out := []Issue{}
	token := ""

	for len(out) < in.Limit {
		page, next, err := c.searchPage(ctx, in, base, token)
		if err != nil {
			return nil, err
		}
		out = append(out, page...)

		// Token boşsa son sayfadayız. Sayfa boş dönerse de dururuz —
		// aksi halde bozuk bir yanıt sonsuz döngü yapardı.
		if next == "" || len(page) == 0 {
			break
		}
		token = next
	}

	if len(out) > in.Limit {
		out = out[:in.Limit]
	}
	return out, nil
}

func (c *Client) searchPage(ctx context.Context, in SearchInput, base, pageToken string) (
	[]Issue, string, error,
) {
	body := map[string]any{
		"jql":        in.JQL,
		"maxResults": in.Limit,
		"fields":     searchFields,
	}
	if pageToken != "" {
		body["nextPageToken"] = pageToken
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, "", fmt.Errorf("istek gövdesi hazırlanamadı: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		base+"/rest/api/3/search/jql", bytes.NewReader(payload))
	if err != nil {
		return nil, "", fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.SetBasicAuth(in.Email, in.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Jira'ya ulaşılamadı: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, "", ErrUnauthorized
	case http.StatusBadRequest:
		// Geçersiz JQL buradan gelir; kullanıcı sorgusunu düzeltmeli.
		var e struct {
			Messages []string `json:"errorMessages"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		detail := strings.Join(e.Messages, "; ")
		if detail == "" {
			detail = "sorgu kabul edilmedi"
		}
		return nil, "", fmt.Errorf("JQL geçersiz: %s", detail)
	default:
		return nil, "", fmt.Errorf("arama başarısız: durum %d", resp.StatusCode)
	}

	var out struct {
		Issues        []rawIssue `json:"issues"`
		NextPageToken string     `json:"nextPageToken"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, "", fmt.Errorf("yanıt okunamadı: %w", err)
	}

	issues := make([]Issue, 0, len(out.Issues))
	for _, raw := range out.Issues {
		issues = append(issues, raw.toIssue(base))
	}
	return issues, out.NextPageToken, nil
}
