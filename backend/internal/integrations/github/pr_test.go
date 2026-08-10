package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct{ adres, sahip, depo string }{
		{"https://github.com/omer/agentTestProject.git", "omer", "agentTestProject"},
		{"https://github.com/omer/agentTestProject", "omer", "agentTestProject"},
		{"https://github.com/omer/agentTestProject/", "omer", "agentTestProject"},
		{"https://git.sirket.local/takim/depo.git", "takim", "depo"},
	}
	for _, tt := range tests {
		sahip, depo, err := ParseRepoURL(tt.adres)
		require.NoError(t, err, tt.adres)
		require.Equal(t, tt.sahip, sahip)
		require.Equal(t, tt.depo, depo)
	}

	for _, bozuk := range []string{"github.com/omer/depo", "https://github.com/omer", ""} {
		_, _, err := ParseRepoURL(bozuk)
		require.Error(t, err, bozuk)
	}
}

func TestOpen_Basarili(t *testing.T) {
	var alinan map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/repos/omer/depo/pulls", r.URL.Path)
		require.Equal(t, "Bearer gizli-token", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&alinan))

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(PullRequest{
			Number: 7, URL: "https://github.com/omer/depo/pull/7", Title: "Başlık",
		})
	}))
	defer srv.Close()

	pr, err := New(srv.URL).Open(context.Background(), OpenInput{
		RepoURL: "https://github.com/omer/depo.git", Token: "gizli-token",
		Title: "Başlık", Body: "Gövde", Head: "agent-coder/x", Base: "main",
	})
	require.NoError(t, err)
	require.Equal(t, 7, pr.Number)
	require.Equal(t, "https://github.com/omer/depo/pull/7", pr.URL)

	require.Equal(t, "agent-coder/x", alinan["head"])
	require.Equal(t, "main", alinan["base"])
	require.Equal(t, "Gövde", alinan["body"])
}

// TestOpen_422IkiFarkliSeyAnlatir — "zaten var" ile "fark yok" farklı eylemler
// gerektirir; ikisini tek hataya sıkıştırmak kullanıcıyı yanıltır.
func TestOpen_422IkiFarkliSeyAnlatir(t *testing.T) {
	tests := []struct {
		ad      string
		mesaj   string
		beklian error
	}{
		{"zaten açık PR var", "A pull request already exists for omer:agent-coder/x.", ErrAlreadyExists},
		{"değişiklik yok", "No commits between main and agent-coder/x", ErrNoChanges},
	}

	for _, tt := range tests {
		t.Run(tt.ad, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"message": "Validation Failed",
					"errors":  []map[string]string{{"message": tt.mesaj}},
				})
			}))
			defer srv.Close()

			_, err := New(srv.URL).Open(context.Background(), OpenInput{
				RepoURL: "https://github.com/omer/depo", Token: "t", Head: "h", Base: "b",
			})
			require.ErrorIs(t, err, tt.beklian)
		})
	}
}

func TestOpen_HataDurumlari(t *testing.T) {
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
			_ = json.NewEncoder(w).Encode(map[string]string{"message": "Not Found"})
		}))

		_, err := New(srv.URL).Open(context.Background(), OpenInput{
			RepoURL: "https://github.com/omer/depo", Token: "t", Head: "h", Base: "b",
		})
		require.ErrorIs(t, err, tt.beklian, "durum %d", tt.kod)
		srv.Close()
	}
}

// TestOpen_TokenHataMesajinaSizmaz — hata metni loglara ve arayüze gidiyor.
func TestOpen_TokenHataMesajinaSizmaz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"message": "Bad credentials"})
	}))
	defer srv.Close()

	_, err := New(srv.URL).Open(context.Background(), OpenInput{
		RepoURL: "https://github.com/omer/depo", Token: "ghp_cokgizli12345", Head: "h", Base: "b",
	})
	require.Error(t, err)
	require.NotContains(t, err.Error(), "ghp_cokgizli12345")
}
