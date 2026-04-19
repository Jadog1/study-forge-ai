import type {
  BrowseResponse,
  ChatStreamEvent,
  ClassDetail,
  Component,
  Config,
  ContextProfile,
  ExportKnowledgeParams,
  ExportKnowledgeResult,
  GenerateQuizParams,
  IngestParams,
  IngestSSEEvent,
  QuizDashboardSnapshot,
  Section,
  SyncReport,
  UsageEvent,
  UsageFilter,
  UsageTotals,
  ChatMode,
} from '../types';

const API_BASE = '/api';

type RawStreamEvent = {
  type?: string;
  text?: string;
  label?: string;
  detail?: string;
  error?: string;
  done?: boolean | string;
  message?: string;
};

function normalizeStreamEvent(raw: unknown): ChatStreamEvent | null {
  if (!raw || typeof raw !== 'object') return null;
  const e = raw as RawStreamEvent;
  if (!e.type) return null;

  if (
    e.type === 'chunk' ||
    e.type === 'action-start' ||
    e.type === 'action-done' ||
    e.type === 'done' ||
    e.type === 'error'
  ) {
    return {
      type: e.type,
      text: e.text,
      label: e.label,
      detail: e.detail,
      error: e.error,
    };
  }

  if (e.type === 'progress') {
    if (e.error) {
      return { type: 'error', error: e.error, label: e.label, detail: e.detail };
    }
    const isDone = e.done === true || e.done === 'true';
    return {
      type: isDone ? 'action-done' : 'action-start',
      label: e.label ?? e.detail ?? (isDone ? 'done' : 'working'),
      detail: e.detail,
    };
  }

  if (e.type === 'warning') {
    return {
      type: 'chunk',
      text: `Warning: ${e.message ?? e.detail ?? e.label ?? 'unknown warning'}`,
    };
  }

  return null;
}

function normalizeClassDetail(raw: unknown): ClassDetail {
  const d = (raw ?? {}) as Record<string, unknown>;
  const syllabus = (d.syllabus ?? {}) as Record<string, unknown>;
  const rules = (d.rules ?? {}) as Record<string, unknown>;
  const context = (d.context ?? {}) as Record<string, unknown>;
  const roster = (d.roster ?? {}) as Record<string, unknown>;

  return {
    name: typeof d.name === 'string' ? d.name : '',
    syllabus: {
      class: typeof syllabus.class === 'string' ? syllabus.class : '',
      topics: Array.isArray(syllabus.topics) ? (syllabus.topics as ClassDetail['syllabus']['topics']) : [],
    },
    rules: {
      class: typeof rules.class === 'string' ? rules.class : '',
      exam_expectations:
        typeof rules.exam_expectations === 'string' ? rules.exam_expectations : undefined,
      question_styles: Array.isArray(rules.question_styles) ? (rules.question_styles as string[]) : [],
      notes: typeof rules.notes === 'string' ? rules.notes : undefined,
    },
    context: {
      class: typeof context.class === 'string' ? context.class : '',
      context_files: Array.isArray(context.context_files) ? (context.context_files as string[]) : [],
    },
    profiles:
      d.profiles && typeof d.profiles === 'object' ? (d.profiles as Record<string, string>) : {},
    roster: {
      class: typeof roster.class === 'string' ? roster.class : '',
      entries: Array.isArray(roster.entries) ? (roster.entries as ClassDetail['roster']['entries']) : [],
    },
    coverage:
      d.coverage && typeof d.coverage === 'object'
        ? (d.coverage as ClassDetail['coverage'])
        : {},
  };
}

async function fetchJSON<T>(url: string, init?: RequestInit): Promise<T> {
  const resp = await fetch(`${API_BASE}${url}`, init);
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
          const parsed = JSON.parse(line.slice(6));
          const event = normalizeStreamEvent(parsed);
          if (event) onEvent(event);
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
  mode: ChatMode,
  onEvent: (event: ChatStreamEvent) => void,
): Promise<void> {
  return streamSSE('/chat', { message, class: className, mode }, onEvent);
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

export function exportKnowledge(params: ExportKnowledgeParams): Promise<ExportKnowledgeResult> {
  return postJSON<ExportKnowledgeResult>('/export', params);
}

export function browseFiles(dir?: string): Promise<BrowseResponse> {
  const qs = dir ? `?dir=${encodeURIComponent(dir)}` : '';
  return fetchJSON<BrowseResponse>(`/browse${qs}`);
}

export async function streamIngest(
  params: IngestParams,
  onEvent: (event: IngestSSEEvent) => void,
): Promise<void> {
  const resp = await fetch(`${API_BASE}/ingest`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
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
          onEvent(JSON.parse(line.slice(6)) as IngestSSEEvent);
        } catch {
          // skip malformed
        }
      }
    }
  }
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
    fetchJSON<unknown>(
      `/classes/${encodeURIComponent(name)}?_=${Date.now()}`,
      { cache: 'no-store' },
    ).then(normalizeClassDetail),

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
  browseFiles: (dir?: string) => browseFiles(dir),
  streamIngest: (params: IngestParams, onEvent: (e: IngestSSEEvent) => void) =>
    streamIngest(params, onEvent),
  exportKnowledge: (params: ExportKnowledgeParams) => exportKnowledge(params),
};
