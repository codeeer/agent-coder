package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddComment_Basarili(t *testing.T) {
	var govde map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/3/issue/ABC-12/comment", r.URL.Path)

		email, token, ok := r.BasicAuth()
		require.True(t, ok, "basic auth bekleniyor")
		require.Equal(t, "a@b.test", email)
		require.Equal(t, "gizli", token)

		require.NoError(t, json.NewDecoder(r.Body).Decode(&govde))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "10001"})
	}))
	defer srv.Close()

	out, err := New(nil).AddComment(context.Background(), CommentInput{
		BaseURL: srv.URL, Email: "a@b.test", Token: "gizli",
		IssueKey: "ABC-12", Body: "Birinci satır\n\nİkinci satır",
	})
	require.NoError(t, err)
	require.Equal(t, "10001", out.ID)
	// Kullanıcıya API adresi değil tarayıcı adresi gösterilmeli.
	require.Equal(t, srv.URL+"/browse/ABC-12", out.URL)

	// v3 düz metin kabul etmiyor: gövde yapılandırılmış belge olmalı.
	doc := govde["body"].(map[string]any)
	require.Equal(t, "doc", doc["type"])
	// Boş satır atlanır, iki paragraf kalır — çok satırlı çıktı tek bloğa yapışmasın.
	require.Len(t, doc["content"], 2)
}

func TestAddComment_BosGovdeGecerliBelgeUretir(t *testing.T) {
	var govde map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&govde))
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]string{"id": "1"})
	}))
	defer srv.Close()

	_, err := New(nil).AddComment(context.Background(), CommentInput{
		BaseURL: srv.URL, IssueKey: "ABC-1", Body: "   \n\n  ",
	})
	require.NoError(t, err)

	doc := govde["body"].(map[string]any)
	require.Len(t, doc["content"], 1, "boş gövde ADF'de geçersiz olurdu")
}

func TestAddComment_HataDurumlari(t *testing.T) {
	tests := []struct {
		kod     int
		beklian error
	}{
		{http.StatusUnauthorized, ErrUnauthorized},
		{http.StatusForbidden, ErrUnauthorized},
		{http.StatusNotFound, ErrNotFound},
	}

	for _, tt := range tests {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(tt.kod)
		}))

		_, err := New(nil).AddComment(context.Background(), CommentInput{
			BaseURL: srv.URL, IssueKey: "ABC-1", Body: "x", Token: "gizli-token",
		})
		require.ErrorIs(t, err, tt.beklian, "durum %d", tt.kod)
		require.NotContains(t, err.Error(), "gizli-token")
		srv.Close()
	}
}

func TestAddComment_BosIssueAnahtari(t *testing.T) {
	_, err := New(nil).AddComment(context.Background(), CommentInput{BaseURL: "http://x", Body: "y"})
	require.Error(t, err)
}

/* ── Issue okuma ─────────────────────────────────────────────────────────── */

func TestGetIssue_AlanlariCozer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/rest/api/3/issue/SCRUM-1", r.URL.Path)
		_, _ = w.Write([]byte(`{
			"key": "SCRUM-1",
			"fields": {
				"summary": "Test yazılım geliştir",
				"updated": "2026-08-10T00:00:00.000+0300",
				"status":    {"name": "Yapılacaklar"},
				"issuetype": {"name": "Görev"},
				"assignee":  {"displayName": "Ömer"},
				"reporter":  {"displayName": "Ömer"},
				"description": {"type":"doc","version":1,"content":[
					{"type":"paragraph","content":[{"type":"text","text":"Birinci satır"}]},
					{"type":"paragraph","content":[{"type":"text","text":"İkinci satır"}]}
				]}
			}
		}`))
	}))
	defer srv.Close()

	issue, err := New(nil).GetIssue(context.Background(), CommentInput{
		BaseURL: srv.URL, IssueKey: "SCRUM-1",
	})
	require.NoError(t, err)
	require.Equal(t, "SCRUM-1", issue.Key)
	require.Equal(t, "Test yazılım geliştir", issue.Summary)
	require.Equal(t, "Görev", issue.IssueType)
	require.Equal(t, "Yapılacaklar", issue.Status)
	require.Equal(t, srv.URL+"/browse/SCRUM-1", issue.URL)

	// ADF ağacı agent'a verilmez; okunur düz metne indirgenir.
	require.Equal(t, "Birinci satır\nİkinci satır", issue.Description)

	// Şablon bağlamına geçen alanlar.
	require.Equal(t, "Test yazılım geliştir", issue.Fields()["summary"])
	require.Equal(t, "SCRUM-1", issue.Fields()["key"])
}

// TestGetIssue_AciklamaBiciminden Bagimsiz — eski kurulumlar düz metin döndürür.
func TestGetIssue_AciklamaDuzMetinDeOlabilir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"A-1","fields":{"summary":"x","description":"düz metin"}}`))
	}))
	defer srv.Close()

	issue, err := New(nil).GetIssue(context.Background(), CommentInput{BaseURL: srv.URL, IssueKey: "A-1"})
	require.NoError(t, err)
	require.Equal(t, "düz metin", issue.Description)
}

func TestGetIssue_AciklamaBosOlabilir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"key":"A-1","fields":{"summary":"x"}}`))
	}))
	defer srv.Close()

	issue, err := New(nil).GetIssue(context.Background(), CommentInput{BaseURL: srv.URL, IssueKey: "A-1"})
	require.NoError(t, err)
	require.Empty(t, issue.Description)
}

func TestGetIssue_BulunamadiVeYetkisiz(t *testing.T) {
	for kod, beklian := range map[int]error{
		http.StatusNotFound:     ErrNotFound,
		http.StatusUnauthorized: ErrUnauthorized,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(kod)
		}))
		_, err := New(nil).GetIssue(context.Background(), CommentInput{
			BaseURL: srv.URL, IssueKey: "YOK-1", Token: "gizli-token",
		})
		require.ErrorIs(t, err, beklian)
		require.NotContains(t, err.Error(), "gizli-token")
		srv.Close()
	}
}
