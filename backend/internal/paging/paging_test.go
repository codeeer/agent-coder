package paging_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agent-coder/backend/internal/paging"
)

func TestClamp_GecerliDegerlerKorunur(t *testing.T) {
	p := paging.Clamp(50, 100)
	require.Equal(t, 50, p.Limit)
	require.Equal(t, 100, p.Offset)
}

// TestClamp_BozukDegerVarsayilanaDuser — liste ucu insan eliyle de çağrılıyor;
// `?limit=abc` (0'a çözülür) yüzünden hata dönmek listeyi hiç göstermemek olurdu.
func TestClamp_BozukDegerVarsayilanaDuser(t *testing.T) {
	for _, limit := range []int{0, -5} {
		require.Equal(t, paging.Default, paging.Clamp(limit, 0).Limit)
	}
	require.Equal(t, 0, paging.Clamp(10, -1).Offset, "eksi offset sıfıra çekilmeli")
}

// TestClamp_AsiriLimitReddedilir — sınır kullanıcı için değil veritabanı için:
// tek bir istek tüm tabloyu belleğe çekememeli.
func TestClamp_AsiriLimitReddedilir(t *testing.T) {
	require.Equal(t, paging.Default, paging.Clamp(100_000, 0).Limit)
	require.Equal(t, paging.Max, paging.Clamp(paging.Max, 0).Limit,
		"azami sınırın kendisi geçerli olmalı")
}
