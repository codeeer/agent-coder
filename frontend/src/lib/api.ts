/**
 * Backend istemcisi.
 *
 * Bileşenler içinde çıplak `fetch` kullanılmaz — her çağrı buradan geçer,
 * böylece hata biçimi, taban adres ve zaman aşımı tek yerde yönetilir.
 */

import type {
  CreateGitProviderRequest,
  CreateLLMProviderRequest,
  Credential,
  CredentialKind,
  ErrorBody,
  GitProvider,
  GraphProblem,
  HealthResponse,
  JiraScanState,
  LLMProvider,
  Agent,
  CreateAgentRequest,
  McpAccess,
  McpServer,
  CreateMcpServerRequest,
  UpdateMcpServerRequest,
  Script,
  CreateScriptRequest,
  UpdateScriptRequest,
  ModelList,
  Paged,
  ModelQuery,
  ImportLine,
  ImportPreview,
  ImportRepo,
  Project,
  ProjectRequest,
  ReportQuery,
  ReportSummary,
  EngineLog,
  Run,
  RunList,
  StartWorkflowResponse,
  Workflow,
  WorkflowDetail,
  WorkflowGraph,
  WorkflowRun,
  WorkflowVersion,
  StartRunRequest,
  SettingsResponse,
  CACertStatus,
  EgressStatus,
  PushResult,
  CANormalizeResult,
  UpdateAgentRequest,
  PutCredentialRequest,
  RefreshResponse,
  SyncResult,
  UpdateGitProviderRequest,
  UpdateLLMProviderRequest,
} from "./types";
import { apiBase } from "./runtime-config";

/**
 * NDJSON akışı okur: her satır ayrı bir JSON nesnesi.
 *
 * `apiFetch` kullanılamıyor — o gövdenin tamamını okuyup çözüyor, oysa buradaki
 * değerin tamamı işin SONUNDA geliyor. Hata biçimi yine ortak: başarısız yanıt
 * `ApiError` olarak fırlatılır.
 *
 * ZAMAN AŞIMI YOK. İş yüz repository sürebilir ve sabit bir sınır, tam da uzun
 * süren doğru işi keserdi. Kullanıcı vazgeçmek isterse `signal` ile keser.
 */
async function streamNDJSON(
  path: string,
  body: unknown,
  onLine: (line: never) => void,
  signal?: AbortSignal,
): Promise<void> {
  let res: Response;
  try {
    res = await fetch(`${apiBase()}${path}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body),
      signal,
    });
  } catch {
    throw new ApiError(0, "network_error", "Backend'e ulaşılamadı");
  }

  if (!res.ok) {
    const text = await res.text();
    const parsed: unknown = text ? JSON.parse(text) : null;
    if (isErrorBody(parsed)) {
      throw new ApiError(res.status, parsed.error.code, parsed.error.message);
    }
    throw new ApiError(res.status, "unexpected_error", `HTTP ${res.status}`);
  }
  if (!res.body) {
    throw new ApiError(0, "unexpected_error", "Yanıt akışı okunamadı");
  }

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let kalan = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;

    // Bir okuma satır ORTASINDA bitebilir; yarım satır bir sonraki parçanın
    // başına eklenir. Aksi halde uzun kayıtlar bölünüp ayrıştırılamazdı.
    kalan += decoder.decode(value, { stream: true });
    const satirlar = kalan.split("\n");
    kalan = satirlar.pop() ?? "";

    for (const satir of satirlar) {
      if (satir.trim() === "") continue;
      onLine(JSON.parse(satir) as never);
    }
  }

  if (kalan.trim() !== "") onLine(JSON.parse(kalan) as never);
}

/*
 * API kök adresi ÇAĞRI BAŞINA çözülür (`apiBase`), modül seviyesinde sabit
 * olarak değil. Sabit olsaydı bundle'a derleme anındaki değer gömülür ve hazır
 * imaj tek bir adrese mühürlü kalırdı — bkz. `runtime-config.ts`.
 */

/** Backend'in döndürdüğü yapılandırılmış hata. */
export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
    /**
     * Alan/düğüm bazında kusurlar. Akış doğrulaması bunu kullanıyor: arayüz
     * "geçersiz" demekle yetinmeyip HANGİ ADIMDA ne yanlış olduğunu
     * gösterebilmeli. Genel mesaja sıkıştırılsaydı o bilgi kaybolurdu.
     */
    readonly problems: GraphProblem[] = [],
  ) {
    super(message);
    this.name = "ApiError";
  }
}

function isErrorBody(value: unknown): value is ErrorBody {
  if (typeof value !== "object" || value === null) return false;
  const err = (value as { error?: unknown }).error;
  return (
    typeof err === "object" &&
    err !== null &&
    typeof (err as { code?: unknown }).code === "string" &&
    typeof (err as { message?: unknown }).message === "string"
  );
}

interface RequestOptions extends Omit<RequestInit, "body"> {
  body?: unknown;
  /** Yanıt beklemeden vazgeçme süresi (ms). */
  timeoutMs?: number;
}

/**
 * Backend'e istek atar ve yanıtı T olarak çözer.
 *
 * Başarısız yanıtlarda ApiError fırlatır; ağ hatası veya zaman aşımında
 * status 0 ile ApiError döner, böylece çağıran tek tip hata yakalar.
 */
export async function apiFetch<T>(
  path: string,
  { body, timeoutMs = 15_000, headers, ...init }: RequestOptions = {},
): Promise<T> {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);

  try {
    const res = await fetch(`${apiBase()}${path}`, {
      ...init,
      signal: controller.signal,
      headers: {
        ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
        ...headers,
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    const text = await res.text();
    const parsed: unknown = text ? JSON.parse(text) : null;

    if (!res.ok) {
      if (isErrorBody(parsed)) {
        const problems = (parsed.error as { problems?: GraphProblem[] }).problems;
        throw new ApiError(
          res.status,
          parsed.error.code,
          parsed.error.message,
          Array.isArray(problems) ? problems : [],
        );
      }
      throw new ApiError(res.status, "unexpected_error", `HTTP ${res.status}`);
    }

    return parsed as T;
  } catch (err) {
    if (err instanceof ApiError) throw err;
    if (err instanceof DOMException && err.name === "AbortError") {
      throw new ApiError(0, "timeout", "Backend zaman aşımına uğradı");
    }
    throw new ApiError(0, "network_error", "Backend'e ulaşılamadı");
  } finally {
    clearTimeout(timer);
  }
}

/** ModelQuery'yi sorgu dizesine çevirir; tanımsız alanlar atlanır. */
function modelQueryString(q: ModelQuery): string {
  const params = new URLSearchParams();
  if (q.provider) params.set("provider", q.provider);
  if (q.q) params.set("q", q.q);
  if (q.tools) params.set("tools", q.tools);
  if (q.free) params.set("free", "1");
  if (q.sort) params.set("sort", q.sort);
  if (q.order) params.set("order", q.order);
  if (q.limit !== undefined) params.set("limit", String(q.limit));
  if (q.offset !== undefined) params.set("offset", String(q.offset));

  const s = params.toString();
  return s ? `?${s}` : "";
}

/** Sağlayıcı doğrulaması harici servise gittiği için varsayılandan uzun. */
const VALIDATE_TIMEOUT_MS = 30_000;

/**
 * Sayfalama parametreleri.
 *
 * `interface` değil `type`: TypeScript arayüzlere örtük indeks imzası vermiyor,
 * dolayısıyla bir arayüz `Record<string, ...>` bekleyen `pageQuery`'ye
 * geçirilemiyor. Tür takma adı geçebiliyor.
 */
export type PageQuery = {
  limit?: number;
  offset?: number;
};

/**
 * Sorgu dizesini kurar.
 *
 * Boş ve sıfır değerler ATILIR: `?offset=0` adresi gereksizce kirletir ve
 * React Query'nin anahtarında da görünmez — iki ayrı adres aynı sonucu döndürür.
 */
function pageQuery(params: Readonly<Record<string, string | number | undefined>>): string {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) {
    if (v === undefined || v === "" || v === 0) continue;
    q.set(k, String(v));
  }
  const s = q.toString();
  return s ? `?${s}` : "";
}

export const api = {
  health: () => apiFetch<HealthResponse>("/health"),

  llmProviders: {
    list: () => apiFetch<LLMProvider[]>("/api/llm-providers"),

    /** Backend, kaydetmeden ÖNCE adresi ve anahtarı doğrular. */
    create: (body: CreateLLMProviderRequest) =>
      apiFetch<LLMProvider>("/api/llm-providers", {
        method: "POST",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    update: (id: string, body: UpdateLLMProviderRequest) =>
      apiFetch<LLMProvider>(`/api/llm-providers/${id}`, {
        method: "PUT",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    remove: (id: string) =>
      apiFetch<null>(`/api/llm-providers/${id}`, { method: "DELETE" }),

    sync: (id: string) =>
      apiFetch<SyncResult>(`/api/llm-providers/${id}/sync`, {
        method: "POST",
        timeoutMs: 60_000,
      }),
  },

  gitProviders: {
    list: () => apiFetch<GitProvider[]>("/api/git-providers"),

    create: (body: CreateGitProviderRequest) =>
      apiFetch<GitProvider>("/api/git-providers", {
        method: "POST",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    update: (id: string, body: UpdateGitProviderRequest) =>
      apiFetch<GitProvider>(`/api/git-providers/${id}`, {
        method: "PUT",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    remove: (id: string) =>
      apiFetch<null>(`/api/git-providers/${id}`, { method: "DELETE" }),
  },

  mcpServers: {
    list: () => apiFetch<McpServer[]>("/api/mcp-servers"),

    /** Backend, kaydetmeden ÖNCE sunucuya bağlanır ve araçlarını okur. */
    create: (body: CreateMcpServerRequest) =>
      apiFetch<McpServer>("/api/mcp-servers", {
        method: "POST",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    update: (id: string, body: UpdateMcpServerRequest) =>
      apiFetch<McpServer>(`/api/mcp-servers/${id}`, {
        method: "PUT",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    remove: (id: string) =>
      apiFetch<null>(`/api/mcp-servers/${id}`, { method: "DELETE" }),
  },

  /** Agent'ların çalıştırabileceği hazır kabuk betikleri (spec 012). */
  scripts: {
    list: (page: PageQuery = {}) =>
      apiFetch<Paged<Script>>(`/api/scripts${pageQuery(page)}`),

    create: (body: CreateScriptRequest) =>
      apiFetch<Script>("/api/scripts", { method: "POST", body }),

    update: (id: string, body: UpdateScriptRequest) =>
      apiFetch<Script>(`/api/scripts/${id}`, { method: "PUT", body }),

    remove: (id: string) =>
      apiFetch<null>(`/api/scripts/${id}`, { method: "DELETE" }),
  },

  /** Agent Coder'ın kendisini dışarıya MCP olarak açması. */
  mcpAccess: {
    get: () => apiFetch<McpAccess>("/api/mcp-access"),
    rotate: () =>
      apiFetch<McpAccess>("/api/mcp-access/rotate", { method: "POST" }),
  },

  credentials: {
    list: () => apiFetch<Credential[]>("/api/credentials"),

    /** Backend değeri kaydetmeden ÖNCE doğrular; geçersizse 422 döner. */
    put: (kind: CredentialKind, body: PutCredentialRequest) =>
      apiFetch<Credential>(`/api/credentials/${kind}`, {
        method: "PUT",
        body,
        timeoutMs: VALIDATE_TIMEOUT_MS,
      }),

    remove: (kind: CredentialKind) =>
      apiFetch<null>(`/api/credentials/${kind}`, { method: "DELETE" }),
  },

  network: {
    /** Sertifikanın durumu ve içinden okunan bilgi. */
    ca: () => apiFetch<CACertStatus>("/api/network/ca"),

    /** Çıkış denetiminin durumu ve her zaman izinli adresler. */
    egress: () => apiFetch<EgressStatus>("/api/network/egress"),

    /**
     * Seçilen dosyayı PEM'e çevirir — KAYDETMEZ.
     *
     * Dönen metin ekrandaki alana düşer; kullanıcı görür ve normal "Kaydet"
     * akışıyla kaydeder. Dosya seçmek tek başına kaydetmez (spec 017 H1).
     */
    normalizeCA: (data: string) =>
      apiFetch<CANormalizeResult>("/api/network/ca/normalize", {
        method: "POST",
        body: { data },
      }),
  },

  settings: {
    list: () => apiFetch<SettingsResponse>("/api/settings"),

    set: (key: string, value: string) =>
      apiFetch<SettingsResponse>(`/api/settings/${key}`, {
        method: "PUT",
        body: { value },
      }),

    /** Ayarı varsayılanına döndürür. */
    reset: (key: string) =>
      apiFetch<SettingsResponse>(`/api/settings/${key}`, { method: "DELETE" }),
  },

  projects: {
    list: (page: PageQuery = {}) =>
      apiFetch<Paged<Project>>(`/api/projects${pageQuery(page)}`),

    /** Backend, kaydetmeden ÖNCE depoya erişimi doğrular. */
    create: (body: ProjectRequest) =>
      apiFetch<Project>("/api/projects", {
        method: "POST",
        body,
        timeoutMs: 40_000,
      }),

    update: (id: string, body: ProjectRequest) =>
      apiFetch<Project>(`/api/projects/${id}`, {
        method: "PUT",
        body,
        timeoutMs: 40_000,
      }),

    remove: (id: string) =>
      apiFetch<null>(`/api/projects/${id}`, { method: "DELETE" }),

    /**
     * Grup adresindeki repository'leri listeler. Yan etkisi YOK — hiçbir şey
     * kaydedilmez, kullanıcı listeyi görüp seçer.
     */
    importPreview: (body: { groupUrl: string; gitProviderId: string }) =>
      apiFetch<ImportPreview>("/api/projects/import/preview", {
        method: "POST",
        body,
        timeoutMs: 60_000,
      }),

    /**
     * Seçilenleri içe aktarır ve sonuçları SATIR SATIR verir.
     *
     * Yanıt beklenmiyor, akış okunuyor: yüz repository'nin her biri için ayrı
     * bir erişim sınaması koşuyor ve kullanıcı hangisinin bittiğini anında
     * görmeli. Tek parça yanıt beklenseydi ekran dakikalarca sessiz kalırdı.
     */
    importRun: (
      body: { gitProviderId: string; repos: ImportRepo[] },
      onLine: (line: ImportLine) => void,
      signal?: AbortSignal,
    ) => streamNDJSON("/api/projects/import", body, onLine, signal),
  },

  agents: {
    list: (page: PageQuery = {}) =>
      apiFetch<Paged<Agent>>(`/api/agents${pageQuery(page)}`),

    create: (body: CreateAgentRequest) =>
      apiFetch<Agent>("/api/agents", { method: "POST", body }),

    update: (id: string, body: UpdateAgentRequest) =>
      apiFetch<Agent>(`/api/agents/${id}`, { method: "PUT", body }),

    /** Hazır agent'ı özgün haline döndürür. */
    reset: (id: string) =>
      apiFetch<Agent>(`/api/agents/${id}/reset`, { method: "POST" }),

    remove: (id: string) =>
      apiFetch<null>(`/api/agents/${id}`, { method: "DELETE" }),
  },

  /** Çalıştırma ortamı hakkında sabit bilgiler (veritabanına dokunmaz). */
  runner: {
    /**
     * İmajı yayınlanmış Node sürümleri.
     *
     * Serbest metin YOK: burada olmayan bir sürüm seçilirse koşu "imaj
     * bulunamadı" ile düşerdi. Liste nadiren değişir (yeni imaj yayınlanınca),
     * bu yüzden çağıran uzun `staleTime` verebilir.
     */
    nodeVersions: () =>
      apiFetch<{ versions: string[] }>("/api/runner/node-versions"),
  },

  runs: {
    list: (params: PageQuery & { project?: string } = {}) =>
      apiFetch<RunList>(`/api/runs${pageQuery(params)}`),

    get: (id: string) => apiFetch<Run>(`/api/runs/${id}`),

    /** Çalıştırmayı başlatır ve hemen döner; iş arka planda sürer. */
    start: (body: StartRunRequest) =>
      apiFetch<Run>("/api/runs", { method: "POST", body, timeoutMs: 30_000 }),

    cancel: (id: string) =>
      apiFetch<null>(`/api/runs/${id}/cancel`, { method: "POST" }),

    /**
     * Kaydı kalıcı olarak siler. Süren bir çalıştırma ve akış adımı olan bir
     * çalıştırma 409 döner — sebebi hata mesajında yazar.
     */
    remove: (id: string) => apiFetch<null>(`/api/runs/${id}`, { method: "DELETE" }),

    push: (id: string, branch?: string) =>
      apiFetch<PushResult>(`/api/runs/${id}/push`, {
        method: "POST",
        body: { branch: branch ?? "" },
        timeoutMs: 120_000,
      }),

    /**
     * Motorun saklanmış ham logları. Container silinmiş olsa da döner —
     * özelliğin varlık sebebi bu.
     */
    engineLogs: (id: string) =>
      apiFetch<{ items: EngineLog[] }>(`/api/runs/${id}/engine-logs`),

    /** Canlı olay akışının adresi — EventSource ile açılır. */
    eventsUrl: (id: string) => `${apiBase()}/api/runs/${id}/events`,
  },

  workflows: {
    list: (page: PageQuery & { project?: string } = {}) =>
      apiFetch<Paged<Workflow>>(`/api/workflows${pageQuery(page)}`),

    get: (id: string) => apiFetch<WorkflowDetail>(`/api/workflows/${id}`),

    create: (body: { projectId: string; name: string; description?: string }) =>
      apiFetch<Workflow>("/api/workflows", { method: "POST", body }),

    update: (id: string, body: { name: string; description: string; isActive: boolean }) =>
      apiFetch<Workflow>(`/api/workflows/${id}`, { method: "PUT", body }),

    remove: (id: string) =>
      apiFetch<null>(`/api/workflows/${id}`, { method: "DELETE" }),

    /** Yeni sürüm kaydeder. Geçersiz graf 400 + kusur listesiyle döner. */
    saveVersion: (id: string, graph: WorkflowGraph) =>
      apiFetch<WorkflowVersion>(`/api/workflows/${id}/versions`, {
        method: "POST",
        body: { graph },
      }),

    /**
     * Akışı başlatır. `projectId` verilmezse akışın varsayılan projesi
     * kullanılır — aynı akış farklı projelerde koşabilsin diye.
     */
    start: (id: string, input: string, projectId?: string) =>
      apiFetch<StartWorkflowResponse>(`/api/workflows/${id}/runs`, {
        method: "POST",
        body: { input, projectId: projectId ?? "" },
      }),

    /** Belirli bir sürümün grafı — geçmiş çalışmayı doğru çizmek için. */
    version: (versionId: string) =>
      apiFetch<WorkflowVersion>(`/api/workflow-versions/${versionId}`),

    rotateHook: (id: string) =>
      apiFetch<{ hookToken: string }>(`/api/workflows/${id}/hook/rotate`, {
        method: "POST",
      }),

    /** Son Jira taramasının sonucu — tarama arka planda çalıştığı için tek görünür iz. */
    jiraScan: (id: string) =>
      apiFetch<JiraScanState>(`/api/workflows/${id}/jira-scan`),
  },

  workflowRuns: {
    list: (params: PageQuery & { workflow?: string } = {}) =>
      apiFetch<Paged<WorkflowRun>>(`/api/workflow-runs${pageQuery(params)}`),

    get: (id: string) => apiFetch<WorkflowRun>(`/api/workflow-runs/${id}`),

    cancel: (id: string) =>
      apiFetch<null>(`/api/workflow-runs/${id}/cancel`, { method: "POST" }),

    /** Canlı ilerleme akışının adresi. */
    eventsUrl: (id: string) => `${apiBase()}/api/workflow-runs/${id}/events`,

    /** Dışarıdan tetikleme adresi — kullanıcıya gösterilir, kopyalanır. */
    hookUrl: (token: string) => `${apiBase()}/hooks/${token}`,

    /** Jira'nın çağıracağı adres — genel webhook'tan ayrı, gövdesi Jira biçimindedir. */
    jiraHookUrl: (token: string) => `${apiBase()}/hooks/jira/${token}`,
  },

  reports: {
    /**
     * Yönetici özeti. Tüm kırılımlar tek istekte gelir: bölümleri ayrı
     * isteklere bölmek, aralarında yeni kayıt düşerse birbirini tutmayan
     * rakamlar gösterirdi.
     */
    summary: (q: ReportQuery = {}) => {
      const p = new URLSearchParams();
      if (q.days) p.set("days", String(q.days));
      if (q.project) p.set("project", q.project);
      const s = p.toString();
      return apiFetch<ReportSummary>(`/api/reports/summary${s ? `?${s}` : ""}`);
    },
  },

  models: {
    list: (q: ModelQuery = {}) =>
      apiFetch<ModelList>(`/api/models${modelQueryString(q)}`),

    /**
     * Tüm sağlayıcıların katalogunu yeniden indirir.
     * Kısmi başarı döner: bir sağlayıcının hatası diğerlerini engellemez.
     */
    refresh: () =>
      apiFetch<RefreshResponse>("/api/models/refresh", {
        method: "POST",
        timeoutMs: 120_000,
      }),
  },
};
