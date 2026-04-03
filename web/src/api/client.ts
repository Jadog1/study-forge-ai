import type {
  ChatStreamEvent,
  ClassDetail,
  Component,
  Config,
  ContextProfile,
  ExportKnowledgeParams,
  GenerateQuizParams,
  IngestParams,
  QuizDashboardSnapshot,
  Section,
  SyncReport,
  UsageEvent,
  UsageFilter,
  UsageTotals,
} from '../types';

const API_BASE = '/api';

async function fetchJSON<T>(url: string): Promise<T> {
  const resp = await fetch(`${API_BASE}${url}`);
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json() as Promise<T>;
}

async function postJSON<T>(url: string, body: unknown): Promise<T> {
  const resp = await fetch(`${API_BASE}${url}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json() as Promise<T>;
}

async function putJSON<T>(url: string, body: unknown): Promise<T> {
  const resp = await fetch(`${API_BASE}${url}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json() as Promise<T>;
}

async function deleteJSON<T>(url: string): Promise<T> {
  const resp = await fetch(`${API_BASE}${url}`, { method: 'DELETE' });
  if (!resp.ok) throw new Error(await resp.text());
  return resp.json() as Promise<T>;
}

async function streamSSE(
  url: string,
  body: unknown,
  onEvent: (event: ChatStreamEvent) => void,
): Promise<void> {
  const resp = await fetch(`${API_BASE}${url}`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  if (!resp.ok) throw new Error(await resp.text());
  const reader = resp.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true });
    const lines = buffer.split('\n');
    buffer = lines.pop() || '';
    for (const line of lines) {
      if (line.startsWith('data: ')) {
        try {
          onEvent(JSON.parse(line.slice(6)) as ChatStreamEvent);
        } catch {
          // skip malformed events
        }
      }
    }
  }
}

// ── Standalone exports (used by ChatPage, KnowledgePage, etc.) ──────

export function fetchConfig(): Promise<Config> {
  return fetchJSON<Config>('/config');
}

export function updateConfig(config: Partial<Config>): Promise<void> {
  return putJSON<void>('/config', config);
}

export function streamChat(
  message: string,
  className: string,
  onEvent: (event: ChatStreamEvent) => void,
): Promise<void> {
  return streamSSE('/chat', { message, class: className }, onEvent);
}

export function fetchSections(): Promise<Section[]> {
  return fetchJSON<{ sections: Section[] }>('/knowledge/sections').then(r => r.sections ?? []);
}

export function fetchComponents(): Promise<Component[]> {
  return fetchJSON<{ components: Component[] }>('/knowledge/components').then(r => r.components ?? []);
}

export function fetchClasses(): Promise<string[]> {
  return fetchJSON<string[]>('/classes');
}

export function fetchUsageTotals(filter?: UsageFilter): Promise<UsageTotals> {
  const params = new URLSearchParams();
  if (filter?.since) params.set('after', filter.since);
  const qs = params.toString();
  return fetchJSON<UsageTotals>(`/usage${qs ? `?${qs}` : ''}`);
}

export function fetchUsageLedger(): Promise<UsageEvent[]> {
  return fetchJSON<{ events: UsageEvent[] }>('/usage/ledger').then(r => r.events ?? []);
}

export function exportKnowledge(params: ExportKnowledgeParams): Promise<{ summary: string }> {
  return postJSON<{ summary: string }>('/export', params);
}

export function ingest(
  params: IngestParams,
  onEvent: (event: ChatStreamEvent) => void,
): Promise<void> {
  return streamSSE('/ingest', params, onEvent);
}

// ── Object-style API (used by QuizDashboardPage, ClassesPage) ───────

export const api = {
  fetchQuizDashboard: () =>
    fetchJSON<QuizDashboardSnapshot>('/quiz/dashboard'),

  generateQuiz: (
    params: GenerateQuizParams,
    onEvent: (e: ChatStreamEvent) => void,
  ) => streamSSE('/quiz/generate', params, onEvent),

  syncTrackedSessions: () => postJSON<SyncReport>('/quiz/sync', {}),

  fetchClasses: () => fetchJSON<string[]>('/classes'),

  createClass: (name: string) => postJSON<void>('/classes', { name }),

  fetchClassDetail: (name: string) =>
    fetchJSON<ClassDetail>(`/classes/${encodeURIComponent(name)}`),

  updateClassContext: (name: string, files: string[]) =>
    putJSON<void>(`/classes/${encodeURIComponent(name)}/context`, {
      context_files: files,
    }),

  updateProfileContext: (name: string, kind: string, text: string) =>
    putJSON<void>(
      `/classes/${encodeURIComponent(name)}/profile/${encodeURIComponent(kind)}`,
      { text },
    ),

  updateRoster: (name: string, entries: unknown[]) =>
    putJSON<void>(`/classes/${encodeURIComponent(name)}/roster`, { entries }),

  updateCoverage: (name: string, kind: string, scope: unknown) =>
    putJSON<void>(
      `/classes/${encodeURIComponent(name)}/coverage/${encodeURIComponent(kind)}`,
      scope,
    ),

  deleteCoverage: (name: string, kind: string) =>
    deleteJSON<void>(
      `/classes/${encodeURIComponent(name)}/coverage/${encodeURIComponent(kind)}`,
    ),

  fetchQuestionTypes: () => fetchJSON<string[]>('/sfq/question-types'),

  fetchContextProfiles: () =>
    fetchJSON<ContextProfile[]>('/classes/profiles'),

  fetchConfig: () => fetchJSON<Config>('/config'),
  updateConfig: (config: Partial<Config>) => putJSON<void>('/config', config),
  streamChat: (message: string, className: string, onEvent: (e: ChatStreamEvent) => void) =>
    streamSSE('/chat', { message, class: className }, onEvent),
  fetchSections: () => fetchJSON<{ sections: Section[] }>('/knowledge/sections').then(r => r.sections ?? []),
  fetchComponents: () => fetchJSON<{ components: Component[] }>('/knowledge/components').then(r => r.components ?? []),
  fetchUsageTotals: (filter?: UsageFilter) => fetchUsageTotals(filter),
  fetchUsageLedger: () => fetchJSON<{ events: UsageEvent[] }>('/usage/ledger').then(r => r.events ?? []),
  ingest: (params: IngestParams, onEvent: (e: ChatStreamEvent) => void) =>
    streamSSE('/ingest', params, onEvent),
  exportKnowledge: (params: ExportKnowledgeParams) =>
    postJSON<{ summary: string }>('/export', params),
};
