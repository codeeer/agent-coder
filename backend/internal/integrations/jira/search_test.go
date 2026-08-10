package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSearch_SayfalamaTokenIleIlerler(t *testing.T) {
	var istekler []map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Eski uç Ağustos 2025'te kaldırıldı; yeni uca gitmeliyiz.
		require.Equal(t, "/rest/api/3/search/jql", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)

		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		istekler = append(istekler, body)

		if body["nextPageToken"] == nil {
			_, _ = w.Write([]byte(`{"issues":[
				{"key":"A-1","fields":{"summary":"bir"}},
				{"key":"A-2","fields":{"summary":"iki"}}
			],"nextPageToken":"tok2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"issues":[{"key":"A-3","fields":{"summary":"üç"}}]}`))
	}))
	defer srv.Close()

	issues, err := New().Search(context.Background(), SearchInput{
		BaseURL: srv.URL, JQL: "project = A", Limit: 10,
	})
	require.NoError(t, err)
	require.Len(t, issues, 3)
	require.Equal(t, "A-3", issues[2].Key)
	require.Equal(t, srv.URL+"/browse/A-1", issues[0].URL)

	require.Len(t, istekler, 2)
	require.Equal(t, "tok2", istekler[1]["nextPageToken"])
}

// TestSearch_SinirAsilmaz — bir tarama yüzlerce akış başlatmamalı.
func TestSearch_SinirAsilmaz(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"issues":[
			{"key":"A-1","fields":{}},{"key":"A-2","fields":{}},{"key":"A-3","fields":{}}
		],"nextPageToken":"devam"}`))
	}))
	defer srv.Close()

	issues, err := New().Search(context.Background(), SearchInput{
		BaseURL: srv.URL, JQL: "project = A", Limit: 2,
	})
	require.NoError(t, err)
	require.Len(t, issues, 2)
}

// TestSearch_BosSayfadaDurur — bozuk bir yanıt sonsuz döngü yapmamalı.
func TestSearch_BosSayfaSonsuzDonguYapmaz(t *testing.T) {
	cagri := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		cagri++
		_, _ = w.Write([]byte(`{"issues":[],"nextPageToken":"hep-ayni"}`))
	}))
	defer srv.Close()

	issues, err := New().Search(context.Background(), SearchInput{
		BaseURL: srv.URL, JQL: "project = A", Limit: 50,
	})
	require.NoError(t, err)
	require.Empty(t, issues)
	require.Equal(t, 1, cagri, "boş sayfa geldiğinde durulmalı")
}

func TestSearch_GecersizJQLAnlasilirHataVerir(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"errorMessages":["Field 'proje' does not exist"]}`))
	}))
	defer srv.Close()

	_, err := New().Search(context.Background(), SearchInput{
		BaseURL: srv.URL, JQL: "proje = A", Token: "gizli-token",
	})
	require.Error(t, err)
	require.Contains(t, err.Error(), "does not exist")
	require.NotContains(t, err.Error(), "gizli-token")
}

func TestSearch_BosJQLReddedilir(t *testing.T) {
	_, err := New().Search(context.Background(), SearchInput{BaseURL: "http://x", JQL: "  "})
	require.Error(t, err)
}
