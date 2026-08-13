package runner

import (
	"encoding/base64"
	"strings"
)

/*
 * Motorun ham logları.
 *
 * Runner container'ı geçici: iş biter bitmez siliniyor ve opencode'un asıl
 * teşhis bilgisi onunla birlikte gidiyor. Bu tip, o bilgiyi container
 * silinmeden önce dışarı taşıyan kanaldır.
 *
 * `Result` üzerinden dönmüyor çünkü `Run` başarısızlıkta `nil` Result
 * döndürüyor — oysa loglara asıl ihtiyaç tam da o anda var.
 */

// EngineLogSource, logun nereden geldiği.
type EngineLogSource string

const (
	// EngineLogStdout, container'ın stdout/stderr'i.
	EngineLogStdout EngineLogSource = "stdout"
	// EngineLogFile, motorun kendi log dosyaları.
	EngineLogFile EngineLogSource = "file"
	/*
	 * EngineLogSession, motorun oturum deposu: mesaj ve parça JSON'ları.
	 *
	 * İlerleme kayıtları SSE akışından besleniyor ve o akış kopabilir —
	 * kopan bağlantı, olmamış bir konuşma anlamına gelmez ama kaydı olmayan
	 * bir konuşma anlamına gelir. Bu kaynak o kaydın YEDEĞİ: agent'ın tam
	 * konuşma ve araç geçmişi container'la birlikte silinmesin.
	 */
	EngineLogSession EngineLogSource = "session"
)

// EngineLog, tek bir kaynaktan toplanmış ham log.
type EngineLog struct {
	Source  EngineLogSource
	Content string
}

// EngineLogFunc, toplanan logları alır.
//
// Koşu NASIL biterse bitsin çağrılır: başarı, hata, iptal, zaman aşımı.
type EngineLogFunc func([]EngineLog)

/*
 * Redact, bilinen gizli değerleri maskeler.
 *
 * Motor loglarına sır sızabilir: yapılandırma hatasında opencode isteğin
 * tamamını basabiliyor, git ise uzak adresi hata metnine koyabiliyor. Log
 * veritabanına yazılıp arayüzde gösterildiği için maskeleme YAZMADAN ÖNCE
 * yapılır — sonradan temizlemek, bir kez yazılmış sırrı geri almaz.
 *
 * Kısa değerler atlanır: üç karakterlik bir "sır" metnin her yerinde eşleşir
 * ve logu okunamaz hale getirirdi.
 */
func Redact(content string, secrets []string) string {
	const enAzUzunluk = 8

	for _, s := range secrets {
		if len(s) < enAzUzunluk {
			continue
		}
		content = strings.ReplaceAll(content, s, "***")
	}
	return content
}

// SecretsOf, bir isteğin içindeki tüm gizli değerler.
//
// Tek yerde toplanıyor: yeni bir sır eklendiğinde (sağlayıcı anahtarı, MCP
// token'ı, paket deposu parolası) maskeleme listesine de girmesi gerekiyor ve
// bunun unutulduğu yer burası olmamalı.
func SecretsOf(req Request) []string {
	out := []string{
		req.Provider.APIKey,
		req.Repo.Secret,
		req.Packages.Token,
	}
	for _, m := range req.Agent.MCPServers {
		out = append(out, m.Secret)
	}

	// Paket deposu kimliği `~/.npmrc` içinde BASE64 duruyor (`_auth=`).
	// Yalnızca ham token aransaydı, agent o dosyayı okuduğunda kimlik
	// kodlanmış hâliyle loga düşer ve maskeleme onu görmezdi.
	if req.Packages.HasAuth() {
		out = append(out, base64.StdEncoding.EncodeToString(
			[]byte(req.Packages.Username+":"+req.Packages.Token)))
	}
	return out
}
