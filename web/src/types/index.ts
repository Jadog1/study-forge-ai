export interface Section {
  id: string;
  class: string;
  title: string;
  summary: string;
  tags?: string[];
  concepts?: string[];
  source_paths?: string[];
  source_tags?: string[];
  component_ids?: string[];
  question_history?: QuestionHistoryEntry[];
  created_at: string;
  updated_at: string;
}

export interface Component {
  id: string;
  section_id: string;
  class: string;
  kind: string;
  content: string;
  tags?: string[];
  concepts?: string[];
  source_paths?: string[];
  source_tags?: string[];
  question_history?: QuestionHistoryEntry[];
  created_at: string;
  updated_at: string;
}

export interface QuestionHistoryEntry {
  id: string;
  quiz_id?: string;
  question_id?: string;
  question?: string;
  user_answer?: string;
  expected?: string;
  correct: boolean;
  answered_at: string;
}

export interface Note {
  id: string;
  source: string;
  source_tag?: string;
  sources?: string[];
  class: string;
  summary: string;
  tags: string[];
  concepts: string[];
  created_at: string;
}

export interface QuizChoice {
  text: string;
  correct: boolean;
}

export interface QuizSection {
  type: string;
  id: string;
  question: string;
  hint: string;
  answer?: string;
  reasoning: string;
  section_id?: string;
  component_id?: string;
  tags: string[];
  choices?: QuizChoice[];
}

export interface Quiz {
  title: string;
  class: string;
  tags: string[];
  sections: QuizSection[];
}

export interface QuizResult {
  question_id: string;
  correct: boolean;
  time_spent: number;
  user_answer?: string;
  answered_at?: string;
  section_id?: string;
  component_id?: string;
}

export interface QuizResults {
  quiz_id: string;
  completed_at: string;
  results: QuizResult[];
}

export interface TrackedQuizRecord {
  quiz_id: string;
  class: string;
  quiz_path: string;
  sfq_path: string;
  registered_at: string;
  last_session_id?: string;
  last_imported_at?: string;
}

export interface TrackedQuizCache {
  schema_version: number;
  quizzes: TrackedQuizRecord[];
  imported_session_ids?: string[];
}

export interface UsageEvent {
  id: string;
  operation: string;
  provider: string;
  model: string;
  request_id?: string;
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_usd?: number;
  class?: string;
  source_path?: string;
  created_at: string;
}

export interface UsageTotals {
  total_input_tokens: number;
  total_output_tokens: number;
  total_tokens: number;
  total_cost_usd: number;
  by_model: Record<string, ModelTotals>;
  updated_at: string;
}

export interface ModelTotals {
  input_tokens: number;
  output_tokens: number;
  total_tokens: number;
  cost_usd: number;
}

export interface Config {
  provider: string;
  embeddings: { provider: string; model: string };
  openai: { model: string };
  claude: { model: string };
  voyage: { model: string };
  local: { endpoint: string; embeddings_endpoint?: string; model: string };
  sfq: { command: string };
  agent_models?: {
    chat?: { provider?: string; model?: string };
    ingestion?: { provider?: string; model?: string };
    quiz_orchestrator?: { provider?: string; model?: string };
    quiz_component?: { provider?: string; model?: string };
  };
  custom_prompt_context?: string;
  model_prices?: Record<string, { input_per_million: number; output_per_million: number }>;
}

export interface ClassDetail {
  name: string;
  syllabus: {
    class: string;
    topics: {
      week?: number;
      day?: string;
      title: string;
      description?: string;
      tags?: string[];
    }[];
  };
  rules: {
    class: string;
    exam_expectations?: string;
    question_styles?: string[];
    notes?: string;
  };
  context: { class: string; context_files: string[] };
  profiles: Record<string, string>;
  roster: { class: string; entries: RosterEntry[] };
  coverage: Record<string, CoverageScope | null>;
}

export interface RosterEntry {
  label: string;
  source_pattern?: string;
  tags?: string[];
  week?: number;
  order?: number;
}

export interface CoverageScope {
  class: string;
  kind: string;
  exclude_unmatched?: boolean;
  groups: {
    labels?: string[];
    source_patterns?: string[];
    tags?: string[];
    weight: number;
  }[];
}

export interface ContextProfile {
  kind: string;
  label: string;
  file_name: string;
  default_question_type: string;
}

export interface QuizDashboardSnapshot {
  sections: Section[];
  components: Component[];
  tracked: TrackedQuizCache;
  quizzes: QuizDashboardDoc[];
  loaded_at: string;
}

export interface QuizDashboardDoc {
  id: string;
  class: string;
  title: string;
  path: string;
  question_count: number;
  generated_at: string;
}

export interface SyncReport {
  imported_sessions: number;
  backfilled_sessions: number;
  failed_sessions: number;
  pending_quizzes: number;
  unmapped_answers: number;
}

export interface ChatStreamEvent {
  type: 'chunk' | 'action-start' | 'action-done' | 'done' | 'error';
  text?: string;
  label?: string;
  detail?: string;
  error?: string;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  actions?: ChatAction[];
  streaming?: boolean;
}

export interface ChatAction {
  label: string;
  detail?: string;
  done: boolean;
}

export interface OrchestratorDirective {
  component_id: string;
  section_id: string;
  section_title: string;
  question_count: number;
  question_types: string[];
  angle: string;
}

export interface GenerateQuizParams {
  class: string;
  count: number;
  assessment_type: string;
  question_type: string;
  focused_sections?: string[];
  tags?: string[];
  directives?: OrchestratorDirective[];
}

export interface ExportKnowledgeParams {
  output_path: string;
  class?: string;
  include_embeddings?: boolean;
}

export interface ExportKnowledgeResult {
  output_path: string;
  class: string;
  sections: number;
  components: number;
  include_embeddings: boolean;
}

export interface BrowseEntry {
  name: string;
  path: string;
  is_dir: boolean;
}

export interface BrowseResponse {
  dir: string;
  entries: BrowseEntry[];
}

export interface IngestParams {
  path?: string;
  class?: string;
  files?: string[];
  clean?: boolean;
}

export interface IngestSSEEvent {
  type: 'progress' | 'done' | 'error';
  label?: string;
  detail?: string;
  done?: string;
  error?: string;
  notes?: number;
  sections_added?: number;
  components_added?: number;
  usage_events?: number;
}

export interface UsageFilter {
  class?: string;
  model?: string;
  operation?: string;
  since?: string;
}
