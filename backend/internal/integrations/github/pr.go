// Package github, kod deposu üzerinde pull request açar.
//
// Yalnızca ihtiyaç duyulan uç: PR açma. Genel bir GitHub istemcisi değil —
// kullanılmayan yüzey, bakımı gereken ama doğrulanmayan kod demek.
package github

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
	// ErrNoChanges, branch ile hedef arasında fark yok.
	ErrNoChanges = errors.New("branch ile hedef arasında fark yok")
	// ErrAlreadyExists, bu branch için zaten açık bir PR var.
	ErrAlreadyExists = errors.New("bu branch için zaten açık bir PR var")
	// ErrUnauthorized, token yetkisiz veya geçersiz.
	ErrUnauthorized = errors.New("depo erişimi yetkisiz")
	// ErrNotFound, depo veya branch bulunamadı.
	ErrNotFound = errors.New("depo veya branch bulunamadı")
)

// Client, GitHub API istemcisi.
type Client struct {
	http    *http.Client
	baseURL string
}

// New yeni istemci üretir. baseURL boşsa github.com kullanılır;
// GitHub Enterprise kurulumları için değiştirilebilir.
// rt nil ise varsayılan taşıyıcı kullanılır (bkz. tlstrust).
func New(baseURL string, rt http.RoundTripper) *Client {
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	return &Client{
		http:    &http.Client{Timeout: 30 * time.Second, Transport: rt},
		baseURL: strings.TrimRight(baseURL, "/"),
	}
}

// PullRequest, açılan PR'ın özeti.
type PullRequest struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
	Title  string `json:"title"`
}

// OpenInput, PR açma isteği.
type OpenInput struct {
	// RepoURL, projenin depo adresi. Sahip/ad buradan çıkarılır.
	RepoURL string
	Token   string
	Title   string
	Body    string
	// Head, değişikliği taşıyan branch.
	Head string
	// Base, PR'ın açılacağı hedef branch.
	Base string
}

// Open, pull request açar.
func (c *Client) Open(ctx context.Context, in OpenInput) (PullRequest, error) {
	owner, repo, err := ParseRepoURL(in.RepoURL)
	if err != nil {
		return PullRequest{}, err
	}

	payload, err := json.Marshal(map[string]string{
		"title": in.Title,
		"body":  in.Body,
		"head":  in.Head,
		"base":  in.Base,
	})
	if err != nil {
		return PullRequest{}, fmt.Errorf("istek gövdesi hazırlanamadı: %w", err)
	}

	url := fmt.Sprintf("%s/repos/%s/%s/pulls", c.baseURL, owner, repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return PullRequest{}, fmt.Errorf("istek oluşturulamadı: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+in.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return PullRequest{}, fmt.Errorf("depoya ulaşılamadı: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusCreated {
		var pr PullRequest
		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			return PullRequest{}, fmt.Errorf("yanıt okunamadı: %w", err)
		}
		return pr, nil
	}
	return PullRequest{}, classify(resp)
}

// classify, GitHub yanıtını anlaşılır bir hataya çevirir.
//
// 422 iki farklı şeyi anlatabiliyor — zaten açık bir PR ve fark olmaması.
// İkisini ayırmak gerekiyor: ilki kullanıcıya "zaten var" der, ikincisi
// "agent bir şey değiştirmemiş" der ve bunlar farklı eylemler gerektirir.
func classify(resp *http.Response) error {
	var body struct {
		Message string `json:"message"`
		Errors  []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)

	detail := body.Message
	for _, e := range body.Errors {
		if e.Message != "" {
			detail = e.Message
			break
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%w: %s", ErrUnauthorized, detail)
	case http.StatusNotFound:
		// GitHub, yetkisiz erişimde de 404 döner (deponun varlığını gizlemek
		// için). Mesajı olduğu gibi taşımak kullanıcıyı yanıltmaz.
		return fmt.Errorf("%w: %s", ErrNotFound, detail)
	case http.StatusUnprocessableEntity:
		lower := strings.ToLower(detail)
		switch {
		case strings.Contains(lower, "already exists"):
			return fmt.Errorf("%w: %s", ErrAlreadyExists, detail)
		case strings.Contains(lower, "no commits between"):
			return fmt.Errorf("%w: %s", ErrNoChanges, detail)
		}
		return fmt.Errorf("pull request açılamadı: %s", detail)
	default:
		return fmt.Errorf("pull request açılamadı: durum %d: %s", resp.StatusCode, detail)
	}
}

// ParseRepoURL, depo adresinden sahip ve ad çıkarır.
//
// Hem `https://github.com/sahip/depo.git` hem `.git`siz hali kabul edilir;
// kullanıcı iki biçimi de yazabiliyor ve ikisi de geçerli.
func ParseRepoURL(raw string) (owner, repo string, err error) {
	trimmed := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(raw), "/"), ".git")

	i := strings.Index(trimmed, "://")
	if i < 0 {
		return "", "", fmt.Errorf("depo adresi anlaşılamadı: %q", raw)
	}
	path := trimmed[i+3:]

	// Alan adını at, kalan yol sahip/ad olmalı.
	if j := strings.Index(path, "/"); j >= 0 {
		path = path[j+1:]
	} else {
		return "", "", fmt.Errorf("depo adresi anlaşılamadı: %q", raw)
	}

	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("depo adresi sahip/ad içermiyor: %q", raw)
	}
	return parts[0], parts[1], nil
}
