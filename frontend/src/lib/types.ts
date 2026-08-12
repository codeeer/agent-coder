/**
 * Backend API tipleri — tek kaynak.
 *
 * Bu dosyadaki tipler `backend/internal/httpapi` içindeki Go struct'larıyla
 * birebir eşleşir. Backend'de bir alan değişirse burası da değişmeli.
 */

/**
 * Sayfalı liste yanıtı — tüm liste uçlarının ortak zarfı.
 *
 * Kimi uç çıplak dizi, kimi zarf dönseydi her uç için ayrı bir okuma yolu ve
 * ayrı bir sayfalama davranışı gerekirdi.
 */
export interface Paged<T> {
  items: T[];
  /** Süzgeçten geçen toplam kayıt — sayfadaki kayıt sayısı değil. */
  total: number;
  limit: number;
  offset: number;
}

/** GET /health */
export interface HealthResponse {
  status: string;
  version: string;
  env: string;
  uptime: string;
}

/** GET /readyz */
export interface ReadyResponse {
  ready: boolean;
  database: "ok" | "unavailable" | "not_configured";
}

/** Tüm hata yanıtlarının ortak gövdesi. */
export interface ErrorBody {
  error: {
    code: string;
    message: string;
  };
}

/** Bir çalıştırmanın durumu. */
export type RunStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | "timeout"
  /** Sunucu çalışma ortasında yeniden başladı. */
  | "interrupted";

// ─── LLM sağlayıcılar ───────────────────────────────────────────────────────

export type LLMProviderType = "openrouter" | "litellm" | "openai_compatible";

/** Bir sağlayıcının katalog senkron durumu. */
export interface ProviderSync {
  providerId: string;
  providerName: string;
  lastAttemptAt: string | null;
  lastSuccessAt: string | null;
  modelCount: number;
  /** null değilse son senkron başarısız — liste eski olabilir. */
  lastError: string | null;
}

/**
 * Tanımlı bir LLM sağlayıcı.
 *
 * Gizli değeri BİLİNÇLİ OLARAK içermez — backend onu hiçbir yanıtta göndermez.
 * `hint` yalnızca son 4 karakterdir.
 */
export interface LLMProvider {
  id: string;
  type: LLMProviderType;
  name: string;
  slug: string;
  baseUrl: string;
  hint: string;
  isDefault: boolean;
  createdAt: string;
  updatedAt: string;
  sync: ProviderSync | null;
}

export interface CreateLLMProviderRequest {
  type: LLMProviderType;
  name: string;
  baseUrl?: string;
  secret: string;
  isDefault?: boolean;
}

export interface UpdateLLMProviderRequest {
  name?: string;
  baseUrl?: string;
  /** Boş bırakılırsa mevcut anahtar korunur. */
  secret?: string;
  isDefault?: boolean;
}

// ─── Git sağlayıcılar ───────────────────────────────────────────────────────

export type GitProviderType = "github" | "bitbucket" | "generic";

export interface GitProvider {
  id: string;
  type: GitProviderType;
  name: string;
  baseUrl: string;
  username: string;
  hint: string;
  createdAt: string;
  updatedAt: string;
  /** false ise erişim bilgisi doğrulanamadan kaydedilmiştir (genel Git). */
  verified: boolean;
}

export interface CreateGitProviderRequest {
  type: GitProviderType;
  name: string;
  baseUrl?: string;
  username?: string;
  secret: string;
}

export interface UpdateGitProviderRequest {
  name?: string;
  baseUrl?: string;
  username?: string;
  secret?: string;
}

// ─── MCP sunucuları (spec 011) ──────────────────────────────────────────────

/** Yalnızca uzak sunucular; yerel (stdio) sunucular bu fazda desteklenmiyor. */
export type McpTransport = "http" | "sse";

/**
 * Tanımlı bir MCP sunucusu.
 *
 * Gizli değeri BİLİNÇLİ OLARAK içermez — backend hiçbir yanıtta göndermez.
 */
export interface McpServer {
  id: string;
  /** Araç adlarının önekidir: `sentry` → `sentry_issue`. */
  name: string;
  transport: McpTransport;
  url: string;
  hint: string;
  hasSecret: boolean;
  /** Son doğrulamada sunucunun bildirdiği araçlar. */
  tools: string[];
  createdAt: string;
  updatedAt: string;
}

export interface CreateMcpServerRequest {
  name: string;
  transport: McpTransport;
  url: string;
  /** Anahtarsız çalışan sunucular için boş bırakılabilir. */
  secret?: string;
}

export interface UpdateMcpServerRequest {
  name?: string;
  transport?: McpTransport;
  url?: string;
  /** Boş bırakılırsa mevcut anahtar korunur. */
  secret?: string;
  /** Kayıtlı anahtarı siler — boş `secret` "değiştirme" anlamına geldiği için ayrı. */
  clearSecret?: boolean;
}

// ─── Betikler ───────────────────────────────────────────────────────────────

/**
 * Agent'ların çalıştırabileceği hazır kabuk betiği.
 *
 * Gizli değer TAŞIMAZ ve şifrelenmez: içerik container içinde zaten düz metin
 * olarak duruyor. Gizli değerler betiğe değil ortam değişkenine konur.
 */
export interface Script {
  id: string;
  /** Dosya adına dönüşür: `<name>.sh`. Küçük harf, rakam ve `-`. */
  name: string;
  /** Agent'ın talimatına yazılır — betiğin NE ZAMAN çağrılacağını anlatır. */
  description: string;
  content: string;
  createdAt: string;
  updatedAt: string;
}

export interface CreateScriptRequest {
  name: string;
  description?: string;
  content: string;
}

export interface UpdateScriptRequest {
  name?: string;
  description?: string;
  content?: string;
}

/** Betiklerin container içindeki dizini — arayüzde yol göstermek için. */
export const SCRIPT_DIR = "/home/agent/scripts";

// ─── Jira (credentials) ─────────────────────────────────────────────────────

/** Spec 002'den sonra bu uç yalnızca Jira ile ilgilenir. */
export type CredentialKind = "jira";

export interface Credential {
  kind: CredentialKind;
  hint: string;
  metadata: Record<string, string>;
  updatedAt: string;
}

export interface PutCredentialRequest {
  secret: string;
  metadata?: Record<string, string>;
}

// ─── Modeller ───────────────────────────────────────────────────────────────

/**
 * Katalogdaki bir model.
 *
 * `contextLength` ve `supportsTools` null olabilir — bu "bilinmiyor" demektir,
 * sıfır veya false DEĞİL. Fiyatlar bilinmiyorsa 0'dır ve model ücretsiz görünür.
 */
export interface Model {
  providerId: string;
  providerName: string;
  id: string;
  name: string;
  description: string;
  contextLength: number | null;
  maxOutputTokens: number | null;
  promptPricePerMTok: number;
  completionPricePerMTok: number;
  supportsTools: boolean | null;
  isFree: boolean;
  isPreview: boolean;
  modality: string;
}

export interface ModelList {
  items: Model[];
  total: number;
  /** Her sağlayıcının senkron durumu; biri düşse diğerleri etkilenmez. */
  providers: ProviderSync[];
  /** En az bir LLM sağlayıcı tanımlı mı. */
  configured: boolean;
}

export type ModelSort = "name" | "price" | "context" | "provider";

/** Araç desteği filtresi — üç durumlu, çünkü "bilinmiyor" ayrı bir durum. */
export type ToolsFilter = "" | "yes" | "unknown";

export interface ModelQuery {
  provider?: string;
  q?: string;
  tools?: ToolsFilter;
  free?: boolean;
  sort?: ModelSort;
  order?: "asc" | "desc";
  limit?: number;
  offset?: number;
}

/** Tek bir sağlayıcının senkron sonucu. */
export interface SyncResult {
  providerId: string;
  providerName: string;
  ok: boolean;
  count?: number;
  error?: string;
}

export interface RefreshResponse {
  results: SyncResult[];
}

// ─── Çalışma ayarları (spec 003 H7) ─────────────────────────────────────────

export type SettingKind = "int" | "bool" | "text";

/**
 * Bir ayarın tanımı ve mevcut değeri.
 *
 * Arayüz kendini bu tanımdan çizer: etiket, açıklama, birim ve aralık
 * backend'den gelir. Yeni parametre eklendiğinde frontend'e dokunmak gerekmez.
 */
export interface SettingValue {
  key: string;
  group: string;
  label: string;
  help: string;
  kind: SettingKind;
  default: string;
  unit?: string;
  min?: number;
  max?: number;
  value: string;
  /** Değer varsayılandan farklı mı. */
  isCustom: boolean;
}

export interface SettingsResponse {
  items: SettingValue[];
  /** Grup kimliği → insan okunur başlık. */
  groups: Record<string, string>;
}

// ─── Projeler ───────────────────────────────────────────────────────────────

export interface Project {
  id: string;
  name: string;
  repoUrl: string;
  defaultBranch: string;
  gitProviderId: string | null;
  /** Bu projede varsayılan Node sürümü. Boşsa runner imajının kendi sürümü. */
  defaultNodeVersion: string;
  runCount: number;
  createdAt: string;
  updatedAt: string;
}

export interface ProjectRequest {
  name: string;
  repoUrl: string;
  defaultBranch?: string;
  gitProviderId?: string | null;
  clearGitProvider?: boolean;
  defaultNodeVersion?: string;
}

// ─── Agent'lar ──────────────────────────────────────────────────────────────

export type AgentSource = "builtin" | "custom";

export interface Agent {
  id: string;
  slug: string;
  name: string;
  description: string;
  prompt: string;
  source: AgentSource;
  /** Hazır agent özgün halinden farklı mı. */
  isModified: boolean;
  defaultProviderId: string | null;
  defaultModel: string;
  allowEdit: boolean;
  allowBash: boolean;
  allowWebfetch: boolean;
  /** Bu agent'ın erişebileceği MCP sunucuları. */
  mcpServerIds: string[];
  /** Bu agent'ın çalıştırabileceği hazır betikler. */
  scriptIds: string[];
  createdAt: string;
  updatedAt: string;
}

export interface CreateAgentRequest {
  slug?: string;
  name: string;
  description?: string;
  prompt: string;
  defaultProviderId?: string | null;
  defaultModel?: string;
  allowEdit: boolean;
  allowBash: boolean;
  allowWebfetch: boolean;
}

export interface UpdateAgentRequest {
  name?: string;
  description?: string;
  prompt?: string;
  defaultProviderId?: string | null;
  clearProvider?: boolean;
  defaultModel?: string;
  allowEdit?: boolean;
  allowBash?: boolean;
  allowWebfetch?: boolean;
  /** nil ise dokunulmaz; boş dizi "hiçbiri" demektir. */
  mcpServerIds?: string[];
  /** Aynı kural: nil dokunulmaz, boş dizi "hiçbiri". */
  scriptIds?: string[];
}

// ─── Çalıştırmalar ──────────────────────────────────────────────────────────

export interface FileChange {
  file: string;
  additions: number;
  deletions: number;
  status: string;
}

/**
 * Bir agent çalıştırması.
 *
 * `agentPrompt` ve `modelId` ANLIK KOPYADIR: agent veya model sonradan
 * değiştirilse bile geçmiş kayıt neyle çalıştığını doğru gösterir.
 */
export interface Run {
  id: string;
  projectId: string;
  agentId: string;
  providerId: string | null;

  projectName: string;
  agentSlug: string;
  agentPrompt: string;
  providerSlug: string;
  modelId: string;

  branch: string;
  task: string;
  /**
   * Çalıştırmanın koştuğu Node sürümü. Boşsa runner imajının kendi sürümü.
   * `agentPrompt` gibi ANLIK KOPYA: projenin varsayılanı sonradan değişse de
   * geçmiş kayıt neyle koştuğunu doğru gösterir.
   */
  nodeVersion: string;

  status: RunStatus;
  error: string | null;
  output: string;
  diff: string;
  files: FileChange[];

  promptTokens: number;
  completionTokens: number;
  costUsd: number;

  /** Bu çalıştırma bir akış adımıysa dolu. */
  workflowRunId: string | null;
  workflowName: string;
  stepName: string;

  pushedBranch: string | null;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
}

export interface RunList {
  items: Run[];
  total: number;
  /** Sayfa penceresi — diğer liste uçlarıyla aynı anlam. */
  limit: number;
  offset: number;
  /** O an çalışan iş sayısı. Sayfalamayla ilgisi yok. */
  active: number;
  /** Aynı anda kaç işe izin verildiği (ayarlardan). */
  concurrencyLimit: number;
}

export interface StartRunRequest {
  projectId: string;
  agentId: string;
  providerId?: string;
  model?: string;
  branch?: string;
  task: string;
  /** Boşsa projenin varsayılanı kullanılır. */
  nodeVersion?: string;
}

/** SSE ile gelen tek bir ilerleme olayı. */
export interface RunEvent {
  seq: number;
  level: "info" | "warn" | "error";
  message: string;
  ts: string;
  /** true ise çalıştırma bitmiştir; bağlantı kapatılır. */
  terminal?: boolean;
  status?: RunStatus;
}

/* ── Rapor ───────────────────────────────────────────────────────────────── */

/** Bir dönemin toplamları. */
export interface ReportTotals {
  runs: number;

  succeeded: number;
  failed: number;
  cancelled: number;
  timeout: number;
  interrupted: number;
  /** Henüz bitmemiş (bekleyen/çalışan) kayıtlar. */
  active: number;

  costUsd: number;
  promptTokens: number;
  completionTokens: number;

  filesChanged: number;
  additions: number;
  deletions: number;

  pushedBranches: number;

  /** Gerçekten kod değiştiren çalıştırmalar — "çalıştı" ile "üretti" aynı şey değil. */
  runsWithCode: number;
  /**
   * Açılan pull request sayısı — BİRLEŞTİRİLEN değil.
   *
   * Bu sayı `runs` tablosunda yoktur; PR açan düğüm model çağırmadığı için
   * çalıştırma kaydı üretmiyor. Arayüz, birleştirme takibi yapılmadığını
   * açıkça yazmak zorunda (spec 012 K4).
   */
  prsOpened: number;
  /** İnsan dokunmadan işlenen Jira task'ı sayısı. */
  jiraTasks: number;
  /** Yalnızca başlayıp bitmiş kayıtların ortalaması (saniye). */
  avgDurationSec: number;
}

/** Tek bir takvim gününün kırılımı. */
export interface ReportDay {
  date: string;
  runs: number;
  succeeded: number;
  failed: number;
  cancelled: number;
  timeout: number;
  interrupted: number;
  active: number;
  costUsd: number;
  /** O gün açılan pull request sayısı. */
  prsOpened: number;
}

/** Agent, model veya proje kırılımının bir satırı. */
export interface ReportGroup {
  key: string;
  label: string;
  runs: number;
  succeeded: number;
  failed: number;
  costUsd: number;
  tokens: number;
  filesChanged: number;
  additions: number;
  deletions: number;
  avgDurationSec: number;
}

/** Yinelenen bir hata metni. */
export interface ReportFailure {
  message: string;
  count: number;
  lastAt: string;
}

/** Rapor sayfasının tüm verisi — tek istekte gelir. */
export interface ReportSummary {
  from: string;
  to: string;
  days: number;
  timezone: string;

  totals: ReportTotals;
  /** Aynı uzunlukta bir önceki dönem — değişim oranı için. */
  previous: ReportTotals;

  daily: ReportDay[];
  byAgent: ReportGroup[];
  byModel: ReportGroup[];
  byProject: ReportGroup[];
  failures: ReportFailure[];
}

export interface ReportQuery {
  days?: number;
  project?: string;
}

/* ── Akışlar (workflow) ──────────────────────────────────────────────────── */

export type NodeKind =
  | "trigger.manual"
  | "trigger.webhook"
  | "trigger.jira"
  | "agent"
  | "github.pr"
  | "jira.comment"
  | "mcp.call";

export interface WorkflowNodeConfig {
  // Agent adımı
  agentId?: string;
  providerId?: string;
  model?: string;
  prompt?: string;
  branch?: string;
  /** Üretilen değişikliği kendiliğinden branch'e gönderir. Varsayılan kapalı. */
  autoPush?: boolean;

  // PR adımı
  title?: string;
  body?: string;
  /** Boşsa gönderim yapan tek ata adımdan çıkarılır. */
  headBranch?: string;
  baseBranch?: string;

  // Jira yorum adımı
  issueKey?: string;

  // Jira tetikleyici
  /** Hangi task'ların akışı başlatacağını belirleyen JQL sorgusu. */
  jql?: string;

  // MCP araç çağrısı
  mcpServerId?: string;
  /** Sunucunun kendi adlandırmasıyla araç adı (`ask_question`). */
  toolName?: string;
  /**
   * Argümanlar — JSON nesnesi.
   *
   * Şablonlar JSON'un İÇİNDE, string değerlerinde durur. Backend önce JSON'u
   * ayrıştırır, sonra değerleri şablondan geçirir; böylece içinde tırnak olan
   * bir agent çıktısı JSON'u bozmaz.
   */
  arguments?: string;
}

/** Agent Coder'ı dışarıya MCP olarak açan adres. */
export interface McpAccess {
  /** MCP istemcisine yapıştırılacak tam adres — kendisi anahtardır. */
  url: string;
}

/** Bir akışın son Jira taramasının sonucu. */
export interface JiraScanState {
  workflowId: string;
  scannedAt: string | null;
  found: number;
  started: number;
  error: string | null;
}

export interface WorkflowNode {
  id: string;
  kind: NodeKind;
  name?: string;
  config: WorkflowNodeConfig;
  /** Tuval konumu — Faz 4 için saklanıyor, bu fazda kullanılmıyor. */
  position: { x: number; y: number };
}

export interface WorkflowEdge {
  from: string;
  to: string;
}

export interface WorkflowGraph {
  nodes: WorkflowNode[];
  edges: WorkflowEdge[];
}

/**
 * Akışın nasıl başladığı.
 *
 * Ayrı bir tip: iki ekranda `kind === "webhook" ? … : "elle"` yazılmıştı ve
 * Jira eklenince ikisi de sessizce "elle" göstermeye başladı. Kapalı bir küme
 * ternary ile değil, eksiksiz bir eşlemeyle karşılanır.
 */
export type TriggerKind = "manual" | "webhook" | "jira" | "mcp";

/** Tetikleyicinin ekranda görünen adı. Kayıt eksiksiz olmak zorunda. */
export const TRIGGER_TEXT: Record<TriggerKind, { short: string; long: string }> = {
  manual: { short: "elle", long: "elle başlatıldı" },
  webhook: { short: "dışarıdan", long: "dışarıdan tetiklendi" },
  jira: { short: "Jira", long: "Jira task'ı tetikledi" },
  mcp: { short: "MCP", long: "dış bir MCP istemcisi başlattı" },
};

export type WorkflowRunStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | "interrupted";

export type WorkflowStepStatus =
  | "pending"
  | "running"
  | "succeeded"
  | "failed"
  | "skipped"
  | "cancelled";

export interface WorkflowRunSummary {
  id: string;
  status: WorkflowRunStatus;
  createdAt: string;
  finishedAt: string | null;
}

export interface Workflow {
  id: string;
  /** Akışın VARSAYILAN projesi — çalıştırırken değiştirilebilir. */
  projectId: string;
  projectName: string;
  /** Toplam çalışma sayısı; silme onayında ne kaybedileceğini yazmak için. */
  runCount: number;
  name: string;
  description: string;
  isActive: boolean;
  activeVersionId: string | null;
  activeVersion: number | null;
  /** Dışarıdan tetikleme adresinin gizli parçası. */
  hookToken: string;
  createdAt: string;
  updatedAt: string;
  lastRun: WorkflowRunSummary | null;
}

/** Detay ucu etkin sürümün grafını da döner. */
export interface WorkflowDetail extends Workflow {
  graph: WorkflowGraph | null;
}

export interface WorkflowStep {
  id: string;
  nodeId: string;
  nodeKind: string;
  nodeName: string;
  level: number;
  status: WorkflowStepStatus;
  error: string | null;
  runId: string | null;
  /** Agent olmayan adımların sonucu (açılan PR, yazılan yorum). */
  resultText: string;
  resultUrl: string;
  agentSlug: string;
  modelId: string;
  costUsd: number;
  tokens: number;
  startedAt: string | null;
  finishedAt: string | null;
}

export interface WorkflowRun {
  id: string;
  workflowId: string;
  workflowName: string;
  /** Çalışmanın koştuğu proje — akışın varsayılanından farklı olabilir. */
  projectId: string;
  projectName: string;
  versionId: string;
  version: number;
  status: WorkflowRunStatus;
  triggerKind: TriggerKind;
  triggerPayload: Record<string, string>;
  input: string;
  error: string | null;
  createdAt: string;
  startedAt: string | null;
  finishedAt: string | null;
  steps: WorkflowStep[];
  costUsd: number;
  tokens: number;
}

/** Başlatma yanıtı — aynı akışın süren diğer çalışmalarını da bildirir. */
export interface StartWorkflowResponse extends WorkflowRun {
  activeOthers: number;
}

/** Geçersiz graf hatasının düğüm bazında kusur listesi. */
export interface GraphProblem {
  nodeId?: string;
  message: string;
}

export interface WorkflowVersion {
  id: string;
  workflowId: string;
  version: number;
  graph: WorkflowGraph;
  createdAt: string;
}
