// Package sandbox, çalıştırma container'larının yaşam döngüsünü yönetir.
//
// Tasarımın çekirdeği: container OLUŞTURULUR ama başlatılmaz, yapılandırma içine
// kopyalanır, sonra başlatılır. Çalıştırma motoru agent tanımlarını yalnızca
// açılışta okuduğu için (ölçüldü) yapılandırmanın önceden yerinde olması şart.
package sandbox

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// LabelRunID, container'ları çalıştırmaya bağlayan etiket.
// Sahipsiz container taraması bunu kullanır.
const LabelRunID = "agent-coder.run-id"

// LabelManaged, bu sistemin oluşturduğu container'ları işaretler.
const LabelManaged = "agent-coder.managed"

var (
	// ErrCreate, container oluşturulamadı.
	ErrCreate = errors.New("çalışma ortamı oluşturulamadı")
	// ErrStart, container başlatılamadı veya hemen öldü.
	ErrStart = errors.New("çalışma ortamı başlatılamadı")
	// ErrNotReady, container zamanında hazır olmadı.
	ErrNotReady = errors.New("çalışma ortamı zamanında hazır olmadı")
)

// Spec, oluşturulacak container'ın tanımı.
type Spec struct {
	RunID   string
	Image   string
	Network string
	Env     map[string]string

	CPUCores int
	MemoryGB int

	// Files, container BAŞLATILMADAN ÖNCE içine kopyalanacak dosyalar.
	Files []File

	/*
	 * ExtraCACert, HOST üzerindeki bir kök sertifika dosyasının (PEM) yolu.
	 *
	 * Boşsa hiçbir şey bağlanmaz — varsayılan davranış değişmez. Doluysa
	 * dosya container'a SALT OKUNUR bağlanır ve `NODE_EXTRA_CA_CERTS` ile
	 * gösterilir (env'i çağıran veriyor).
	 *
	 * NEDEN İMAJA GÖMÜLMÜYOR: imaj herkese dağıtılıyor ve bir kurumun kök
	 * sertifikası oraya girerse o sertifika bütün kullanıcılara dağıtılmış
	 * olur. Kurumsal ağın çözümü CA'yı TANITMAK; TLS doğrulamasını kapatmak
	 * (NODE_TLS_REJECT_UNAUTHORIZED=0, strict-ssl=false, sslVerify=false)
	 * hiçbir koşulda kabul edilmez — sorunu görünmez yapar, ortadan
	 * kaldırmaz.
	 */
	ExtraCACert string
}

/** Kurumsal CA'nın container içindeki yolu. */
const ExtraCACertPath = "/etc/ssl/certs/kurumsal-ca.pem"

// File, container'a kopyalanacak bir dosya.
type File struct {
	Path    string
	Content []byte
	Mode    int64
}

// Container, çalışan bir sandbox.
type Container struct {
	ID string
	// Host, backend'in container'a erişeceği adres (container adı:port).
	Host string

	docker *client.Client
	name   string
}

// Manager, Docker ile konuşur.
type Manager struct {
	docker *client.Client
	// port, container içinde çalıştırma motorunun dinlediği port.
	port int
}

// NewManager, host'un Docker daemon'ına bağlanır.
func NewManager(port int) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker istemcisi kurulamadı: %w", err)
	}
	return &Manager{docker: cli, port: port}, nil
}

// Ping, Docker daemon'ına erişilebildiğini doğrular.
func (m *Manager) Ping(ctx context.Context) error {
	if _, err := m.docker.Ping(ctx); err != nil {
		return fmt.Errorf("docker daemon'ına erişilemiyor: %w", err)
	}
	return nil
}

// Close, Docker istemcisini kapatır.
func (m *Manager) Close() error { return m.docker.Close() }

// EnsureImage, imajın yerelde bulunduğunu doğrular.
//
// Otomatik indirme YAPILMAZ: runner imajı bu depodan build ediliyor. Eksikse
// kullanıcıya ne yapması gerektiği söylenir — Node sürümü seçilmişse o sürümün
// imajı hiç hazırlanmamış olabilir, mesaj bunu ayırt ediyor.
func (m *Manager) EnsureImage(ctx context.Context, ref string) error {
	_, err := m.docker.ImageInspect(ctx, ref)
	if err == nil {
		return nil
	}
	if client.IsErrNotFound(err) {
		return fmt.Errorf("%w: %q imajı bulunamadı — %s", ErrCreate, ref, imageHint(ref))
	}
	return fmt.Errorf("%w: imaj kontrol edilemedi: %w", ErrCreate, err)
}

// imageHint, eksik imaj için ne yapılacağını söyler.
//
// "make runner" demek sürümlü imajlarda yanıltıcı olurdu: hazır imajla kuran
// biri hiçbir şey derlemiyor, `docker pull` etmesi gerekiyor.
func imageHint(ref string) string {
	if strings.Contains(ref, ":"+nodeTagPrefix) {
		if strings.HasPrefix(ref, "ghcr.io/") {
			return "`docker pull " + ref + "` ile çekin"
		}
		return "`make runner` bu sürümü de derler"
	}
	return "`make runner` ile oluşturun"
}

// nodeTagPrefix, sürümlü runner etiketlerinin öneki (runner.ImageFor ile aynı).
//
// Sabit burada tekrarlanıyor çünkü sandbox paketi runner paketini import
// edemez — runner zaten sandbox'ı kullanıyor, ters bağımlılık döngü olurdu.
const nodeTagPrefix = "node-"

/*
 * caBind, kurumsal CA için salt okunur bağlama listesini üretir.
 *
 * Yol boşsa nil döner: `Binds: nil` ile hiçbir bağlama yapılmaz ve
 * varsayılan davranış aynen korunur.
 *
 * `:ro` PAZARLIKSIZ. Container non-root koşuyor ama yazılabilir bir
 * sertifika deposu, çalıştırılan agent'ın güven zincirini değiştirebilmesi
 * demek olurdu.
 */
func caBind(hostPath string) []string {
	if hostPath == "" {
		return nil
	}
	return []string{hostPath + ":" + ExtraCACertPath + ":ro"}
}

// Create, container'ı oluşturur, dosyaları içine kopyalar ve başlatır.
//
// Hata durumunda yarım kalan container temizlenir — çağıranın elinde sızıntı kalmaz.
func (m *Manager) Create(ctx context.Context, spec Spec) (c *Container, err error) {
	name := "agent-coder-run-" + spec.RunID

	env := make([]string, 0, len(spec.Env))
	for k, v := range spec.Env {
		env = append(env, k+"="+v)
	}

	portSpec := nat.Port(fmt.Sprintf("%d/tcp", m.port))

	created, err := m.docker.ContainerCreate(ctx,
		&container.Config{
			Image:        spec.Image,
			Env:          env,
			ExposedPorts: nat.PortSet{portSpec: struct{}{}},
			Labels: map[string]string{
				LabelManaged: "true",
				LabelRunID:   spec.RunID,
			},
		},
		&container.HostConfig{
			// Dışarıya port yayınlanmaz; backend izole ağ üzerinden erişir.
			NetworkMode: container.NetworkMode(spec.Network),
			// Kurumsal CA yalnızca AÇIKÇA tanımlandıysa bağlanır ve her
			// zaman salt okunur.
			Binds: caBind(spec.ExtraCACert),
			Resources: container.Resources{
				NanoCPUs: int64(spec.CPUCores) * 1_000_000_000,
				Memory:   int64(spec.MemoryGB) << 30,
			},
			// Container kendi kendine yeniden başlamaz: ölürse çalıştırma başarısızdır.
			RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
		},
		nil, nil, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreate, err)
	}

	c = &Container{ID: created.ID, Host: name, docker: m.docker, name: name}

	// Buradan sonraki her hatada yarım container silinir.
	defer func() {
		if err != nil {
			c.Remove(context.WithoutCancel(ctx))
			c = nil
		}
	}()

	// Yapılandırma container BAŞLATILMADAN önce içine kopyalanır.
	if len(spec.Files) > 0 {
		if err = m.copyFiles(ctx, created.ID, spec.Files); err != nil {
			return nil, err
		}
	}

	if err = m.docker.ContainerStart(ctx, created.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrStart, err)
	}

	return c, nil
}

// copyFiles, dosyaları tar akışı olarak container'a yazar.
func (m *Manager) copyFiles(ctx context.Context, id string, files []File) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)

	for _, f := range files {
		// Docker API tar içinde köke göre yol bekler.
		name := strings.TrimPrefix(f.Path, "/")

		mode := f.Mode
		if mode == 0 {
			mode = 0o600
		}
		hdr := &tar.Header{
			Name:    name,
			Mode:    mode,
			Size:    int64(len(f.Content)),
			ModTime: time.Unix(0, 0),
			// Runner container'ı agent kullanıcısıyla (uid 10001) çalışıyor;
			// dosyalar onun okuyabileceği sahiplikte olmalı.
			Uid: 10001,
			Gid: 10001,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return fmt.Errorf("%w: tar başlığı yazılamadı: %w", ErrCreate, err)
		}
		if _, err := tw.Write(f.Content); err != nil {
			return fmt.Errorf("%w: tar içeriği yazılamadı: %w", ErrCreate, err)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("%w: tar kapatılamadı: %w", ErrCreate, err)
	}

	if err := m.docker.CopyToContainer(ctx, id, "/", &buf,
		container.CopyToContainerOptions{}); err != nil {
		return fmt.Errorf("%w: yapılandırma kopyalanamadı: %w", ErrCreate, err)
	}
	return nil
}

// CleanupOrphans, bu sistemin bıraktığı çalışan container'ları siler.
//
// Sunucu çalışma ortasında öldüğünde container'lar sahipsiz kalır. Açılışta
// çağrılır: sızıntı birikip diski doldurmasın.
func (m *Manager) CleanupOrphans(ctx context.Context) (int, error) {
	list, err := m.docker.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", LabelManaged+"=true")),
	})
	if err != nil {
		return 0, fmt.Errorf("sahipsiz container'lar listelenemedi: %w", err)
	}

	removed := 0
	for _, ct := range list {
		if err := m.docker.ContainerRemove(ctx, ct.ID,
			container.RemoveOptions{Force: true, RemoveVolumes: true}); err != nil {
			slog.WarnContext(ctx, "sahipsiz container silinemedi", "id", ct.ID, "error", err)
			continue
		}
		removed++
	}
	if removed > 0 {
		slog.InfoContext(ctx, "sahipsiz çalışma container'ları temizlendi", "adet", removed)
	}
	return removed, nil
}

// Remove, container'ı ve volume'lerini siler.
//
// HER YOLDA çağrılmalı: başarı, hata, panik, iptal ve zaman aşımı.
// İptal edilmiş context ile silme çalışmayacağı için çağıran
// context.WithoutCancel kullanmalıdır.
func (c *Container) Remove(ctx context.Context) {
	if c == nil || c.docker == nil {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := c.docker.ContainerRemove(ctx, c.ID,
		container.RemoveOptions{Force: true, RemoveVolumes: true})
	if err != nil && !client.IsErrNotFound(err) {
		// Silinemeyen container disk sızıntısıdır; sessizce geçilmez.
		slog.ErrorContext(ctx, "çalışma container'ı silinemedi",
			"id", c.ID, "name", c.name, "error", err)
	}
}

// Logs, container'ın o ana kadarki çıktısını döner. Hata ayıklama için.
func (c *Container) Logs(ctx context.Context, tail string) (string, error) {
	rc, err := c.docker.ContainerLogs(ctx, c.ID, container.LogsOptions{
		ShowStdout: true, ShowStderr: true, Tail: tail,
	})
	if err != nil {
		return "", fmt.Errorf("container logları okunamadı: %w", err)
	}
	defer rc.Close()

	// Docker log akışı 8 baytlık başlıklarla çerçevelenir; hata ayıklama için
	// ham okumak yeterli, çerçeveler gözle ayıklanabilir.
	const maxLog = 256 << 10
	data, err := io.ReadAll(io.LimitReader(rc, maxLog))
	if err != nil {
		return "", fmt.Errorf("container logları okunamadı: %w", err)
	}
	return string(data), nil
}

// Wait, container'ın ölmesini bekler ve çıkış kodunu döner.
func (c *Container) Wait(ctx context.Context) (int64, error) {
	okCh, errCh := c.docker.ContainerWait(ctx, c.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		return 0, err
	case ok := <-okCh:
		return ok.StatusCode, nil
	case <-ctx.Done():
		return 0, ctx.Err()
	}
}
