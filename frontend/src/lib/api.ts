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
  McpServer,
  CreateMcpServerRequest,
  UpdateMcpServerRequest,
  ModelList,
  Paged,
  ModelQuery,
  Project,
  ProjectRequest,
  ReportQuery,
  ReportSummary,
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
  UpdateAgentRequest,
  PutCredentialRequest,
  RefreshResponse,
  SyncResult,
  UpdateGitProviderRequest,
  UpdateLLMProviderRequest,
} from "./types";

const BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

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
    const res = await fetch(`${BASE_URL}${path}`, {
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

  runs: {
    list: (params: PageQuery & { project?: string } = {}) =>
      apiFetch<RunList>(`/api/runs${pageQuery(params)}`),

    get: (id: string) => apiFetch<Run>(`/api/runs/${id}`),

    /** Çalıştırmayı başlatır ve hemen döner; iş arka planda sürer. */
    start: (body: StartRunRequest) =>
      apiFetch<Run>("/api/runs", { method: "POST", body, timeoutMs: 30_000 }),

    cancel: (id: string) =>
      apiFetch<null>(`/api/runs/${id}/cancel`, { method: "POST" }),

    push: (id: string, branch?: string) =>
      apiFetch<{ branch: string }>(`/api/runs/${id}/push`, {
        method: "POST",
        body: { branch: branch ?? "" },
        timeoutMs: 120_000,
      }),

    /** Canlı olay akışının adresi — EventSource ile açılır. */
    eventsUrl: (id: string) => `${BASE_URL}/api/runs/${id}/events`,
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

    start: (id: string, input: string) =>
      apiFetch<StartWorkflowResponse>(`/api/workflows/${id}/runs`, {
        method: "POST",
        body: { input },
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
    eventsUrl: (id: string) => `${BASE_URL}/api/workflow-runs/${id}/events`,

    /** Dışarıdan tetikleme adresi — kullanıcıya gösterilir, kopyalanır. */
    hookUrl: (token: string) => `${BASE_URL}/hooks/${token}`,

    /** Jira'nın çağıracağı adres — genel webhook'tan ayrı, gövdesi Jira biçimindedir. */
    jiraHookUrl: (token: string) => `${BASE_URL}/hooks/jira/${token}`,
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
