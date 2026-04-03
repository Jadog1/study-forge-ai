import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BarChart3,
  BookOpen,
  CheckCircle2,
  ChevronDown,
  ChevronRight,
  Clock,
  Loader2,
  Plus,
  RefreshCw,
  Target,
  TrendingUp,
  X,
} from 'lucide-react';
import {
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { api } from '../api/client';
import type {
  ChatStreamEvent,
  ContextProfile,
  QuestionHistoryEntry,
  QuizDashboardSnapshot,
  SyncReport,
} from '../types';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function collectQuestionHistory(
  snap: QuizDashboardSnapshot,
): QuestionHistoryEntry[] {
  const entries: QuestionHistoryEntry[] = [];
  for (const s of snap.sections ?? []) {
    if (s.question_history) entries.push(...s.question_history);
  }
  for (const c of snap.components ?? []) {
    if (c.question_history) entries.push(...c.question_history);
  }
  return entries;
}

function accuracyPercent(history: QuestionHistoryEntry[]): number {
  if (history.length === 0) return 0;
  const correct = history.filter((h) => h.correct).length;
  return Math.round((correct / history.length) * 100);
}

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

const BAR_COLORS = [
  '#6366f1', '#8b5cf6', '#a78bfa', '#818cf8',
  '#4f46e5', '#7c3aed', '#6d28d9', '#4338ca',
];

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function StatCard({
  icon,
  label,
  value,
  sub,
}: {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  sub?: string;
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600">
          {icon}
        </div>
        <div className="min-w-0">
          <p className="text-sm font-medium text-slate-500">{label}</p>
          <p className="text-2xl font-semibold text-slate-900">{value}</p>
          {sub && <p className="text-xs text-slate-400">{sub}</p>}
        </div>
      </div>
    </div>
  );
}

function SyncReportBanner({ report }: { report: SyncReport }) {
  return (
    <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800">
      <p className="font-medium">Sync complete</p>
      <p>
        Imported {report.imported_sessions} session(s) &middot; Backfilled{' '}
        {report.backfilled_sessions} &middot; Failed {report.failed_sessions}{' '}
        &middot; Pending quizzes {report.pending_quizzes} &middot; Unmapped{' '}
        {report.unmapped_answers}
      </p>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Generate Quiz Modal
// ---------------------------------------------------------------------------

function GenerateQuizModal({
  snapshot,
  onClose,
  onDone,
}: {
  snapshot: QuizDashboardSnapshot;
  onClose: () => void;
  onDone: () => void;
}) {
  const classes = useMemo(() => {
    const set = new Set<string>();
    for (const s of snapshot.sections) set.add(s.class);
    for (const q of snapshot.quizzes) set.add(q.class);
    return [...set].sort();
  }, [snapshot]);

  const [selectedClass, setSelectedClass] = useState(classes[0] ?? '');
  const [numQuestions, setNumQuestions] = useState(10);
  const [assessmentType, setAssessmentType] = useState('quiz');
  const [questionType, setQuestionType] = useState('multiple-choice');
  const [questionTypes, setQuestionTypes] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<ContextProfile[]>([]);
  const [selectedSections, setSelectedSections] = useState<string[]>([]);
  const [showSections, setShowSections] = useState(false);

  const [generating, setGenerating] = useState(false);
  const [events, setEvents] = useState<ChatStreamEvent[]>([]);
  const [error, setError] = useState<string | null>(null);

  const classSections = useMemo(
    () => snapshot.sections.filter((s) => s.class === selectedClass),
    [snapshot.sections, selectedClass],
  );

  useEffect(() => {
    api.fetchQuestionTypes().then(setQuestionTypes).catch(() => {});
    api.fetchContextProfiles().then(setProfiles).catch(() => {});
  }, []);

  const assessmentTypes = useMemo(
    () => [...new Set(profiles.map((p) => p.kind))],
    [profiles],
  );

  const handleGenerate = async () => {
    setGenerating(true);
    setEvents([]);
    setError(null);
    try {
      await api.generateQuiz(
        {
          class: selectedClass,
          count: numQuestions,
          assessment_type: assessmentType,
          question_type: questionType,
          focused_sections:
            selectedSections.length > 0 ? selectedSections : undefined,
        },
        (e) => setEvents((prev) => [...prev, e]),
      );
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Generation failed');
    } finally {
      setGenerating(false);
    }
  };

  const toggleSection = (id: string) =>
    setSelectedSections((prev) =>
      prev.includes(id) ? prev.filter((s) => s !== id) : [...prev, id],
    );

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="relative max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl bg-white p-6 shadow-xl">
        <button
          onClick={onClose}
          className="absolute right-4 top-4 text-slate-400 hover:text-slate-600"
        >
          <X size={20} />
        </button>
        <h2 className="mb-4 text-lg font-semibold text-slate-900">
          Generate Quiz
        </h2>

        <div className="space-y-4">
          {/* Class */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Class
            </label>
            <select
              value={selectedClass}
              onChange={(e) => setSelectedClass(e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              {classes.map((c) => (
                <option key={c} value={c}>
                  {c}
                </option>
              ))}
            </select>
          </div>

          {/* Num questions */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Number of Questions
            </label>
            <input
              type="number"
              min={1}
              max={50}
              value={numQuestions}
              onChange={(e) => setNumQuestions(Number(e.target.value))}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>

          {/* Assessment type */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Assessment Type
            </label>
            <select
              value={assessmentType}
              onChange={(e) => setAssessmentType(e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              {(assessmentTypes.length > 0
                ? assessmentTypes
                : ['quiz', 'exam', 'focused']
              ).map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>

          {/* Question type */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Question Type
            </label>
            <select
              value={questionType}
              onChange={(e) => setQuestionType(e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              {(questionTypes.length > 0
                ? questionTypes
                : [
                    'multiple-choice',
                    'multi-select',
                    'true-false',
                    'short-answer',
                    'fill-in-the-blank',
                  ]
              ).map((t) => (
                <option key={t} value={t}>
                  {t}
                </option>
              ))}
            </select>
          </div>

          {/* Focused sections */}
          <div>
            <button
              type="button"
              onClick={() => setShowSections(!showSections)}
              className="flex items-center gap-1 text-sm font-medium text-indigo-600 hover:text-indigo-700"
            >
              {showSections ? (
                <ChevronDown size={16} />
              ) : (
                <ChevronRight size={16} />
              )}
              Focus on specific sections ({selectedSections.length} selected)
            </button>
            {showSections && (
              <div className="mt-2 max-h-40 overflow-y-auto rounded-lg border border-slate-200 p-2">
                {classSections.length === 0 ? (
                  <p className="text-xs text-slate-400">
                    No sections for this class
                  </p>
                ) : (
                  classSections.map((s) => (
                    <label
                      key={s.id}
                      className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-slate-50"
                    >
                      <input
                        type="checkbox"
                        checked={selectedSections.includes(s.id)}
                        onChange={() => toggleSection(s.id)}
                        className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                      />
                      <span className="truncate">{s.title}</span>
                    </label>
                  ))
                )}
              </div>
            )}
          </div>

          {/* Progress */}
          {events.length > 0 && (
            <div className="max-h-40 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs font-mono text-slate-600">
              {events.map((e, i) => (
                <div key={i} className="py-0.5">
                  {e.type === 'action-start' && (
                    <span className="text-indigo-600">
                      ▸ {e.label ?? 'working'}...
                    </span>
                  )}
                  {e.type === 'action-done' && (
                    <span className="text-emerald-600">
                      ✓ {e.label ?? 'done'}
                    </span>
                  )}
                  {e.type === 'chunk' && (
                    <span>{e.text}</span>
                  )}
                  {e.type === 'error' && (
                    <span className="text-red-600">✗ {e.error}</span>
                  )}
                  {e.type === 'done' && (
                    <span className="font-semibold text-emerald-700">
                      ✓ Complete
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}

          {error && (
            <p className="text-sm text-red-600">{error}</p>
          )}

          <button
            onClick={handleGenerate}
            disabled={generating || !selectedClass}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700 disabled:opacity-50"
          >
            {generating ? (
              <>
                <Loader2 size={16} className="animate-spin" />
                Generating...
              </>
            ) : (
              <>
                <Plus size={16} />
                Generate
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function QuizDashboardPage() {
  const [snapshot, setSnapshot] = useState<QuizDashboardSnapshot | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncing, setSyncing] = useState(false);
  const [syncReport, setSyncReport] = useState<SyncReport | null>(null);
  const [showGenerate, setShowGenerate] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const raw = await api.fetchQuizDashboard();
      setSnapshot({
        ...raw,
        sections: raw.sections ?? [],
        components: raw.components ?? [],
        quizzes: raw.quizzes ?? [],
        tracked: {
          ...raw.tracked,
          quizzes: raw.tracked?.quizzes ?? [],
          imported_session_ids: raw.tracked?.imported_session_ids ?? [],
        },
      });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const handleSync = async () => {
    setSyncing(true);
    setSyncReport(null);
    try {
      const report = await api.syncTrackedSessions();
      setSyncReport(report);
      await load();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Sync failed');
    } finally {
      setSyncing(false);
    }
  };

  // ---------- Computed stats ----------

  const allHistory = useMemo(
    () => (snapshot ? collectQuestionHistory(snapshot) : []),
    [snapshot],
  );

  const overallAccuracy = useMemo(
    () => accuracyPercent(allHistory),
    [allHistory],
  );

  const importedSessions = useMemo(
    () => snapshot?.tracked.imported_session_ids?.length ?? 0,
    [snapshot],
  );

  const classAccuracy = useMemo(() => {
    if (!snapshot) return [];
    const byClass = new Map<
      string,
      { correct: number; total: number }
    >();
    for (const entry of allHistory) {
      const component = snapshot.components.find((c) =>
        c.question_history?.some((h) => h.id === entry.id),
      );
      const section = snapshot.sections.find((s) =>
        s.question_history?.some((h) => h.id === entry.id),
      );
      const cls = component?.class ?? section?.class ?? 'Unknown';
      const acc = byClass.get(cls) ?? { correct: 0, total: 0 };
      acc.total++;
      if (entry.correct) acc.correct++;
      byClass.set(cls, acc);
    }
    return [...byClass.entries()].map(([name, { correct, total }]) => ({
      name,
      accuracy: total > 0 ? Math.round((correct / total) * 100) : 0,
      total,
    }));
  }, [snapshot, allHistory]);

  const coverageByClass = useMemo(() => {
    if (!snapshot) return [];
    const byClass = new Map<
      string,
      { quizzed: number; total: number }
    >();
    for (const s of snapshot.sections) {
      const acc = byClass.get(s.class) ?? { quizzed: 0, total: 0 };
      acc.total++;
      if (s.question_history && s.question_history.length > 0) acc.quizzed++;
      byClass.set(s.class, acc);
    }
    return [...byClass.entries()].map(([name, { quizzed, total }]) => ({
      name,
      quizzed,
      total,
      pct: total > 0 ? Math.round((quizzed / total) * 100) : 0,
    }));
  }, [snapshot]);

  const needsAttention = useMemo(() => {
    if (!snapshot) return [];
    return snapshot.sections
      .filter((s) => {
        if (!s.question_history || s.question_history.length === 0) return true;
        return accuracyPercent(s.question_history) < 60;
      })
      .slice(0, 8);
  }, [snapshot]);

  // ---------- Render ----------

  if (loading) {
    return (
      <div className="flex h-96 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-indigo-600" />
      </div>
    );
  }

  if (error && !snapshot) {
    return (
      <div className="mx-auto max-w-xl p-8 text-center">
        <p className="mb-4 text-red-600">{error}</p>
        <button
          onClick={load}
          className="rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          Retry
        </button>
      </div>
    );
  }

  if (!snapshot) return null;

  return (
    <div className="mx-auto max-w-7xl space-y-6 p-6">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Quiz Dashboard</h1>
          <p className="text-sm text-slate-500">
            Loaded {formatDate(snapshot.loaded_at)}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={() => setShowGenerate(true)}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-2 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700"
          >
            <Plus size={16} />
            Generate Quiz
          </button>
          <button
            onClick={handleSync}
            disabled={syncing}
            className="flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition-colors duration-150 hover:bg-slate-50 disabled:opacity-50"
          >
            <RefreshCw size={16} className={syncing ? 'animate-spin' : ''} />
            Sync
          </button>
          <button
            onClick={load}
            className="flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 transition-colors duration-150 hover:bg-slate-50"
          >
            <RefreshCw size={16} />
            Refresh
          </button>
        </div>
      </div>

      {syncReport && <SyncReportBanner report={syncReport} />}

      {/* Overview Cards */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <StatCard
          icon={<BarChart3 size={20} />}
          label="Total Quizzes"
          value={snapshot.quizzes.length}
        />
        <StatCard
          icon={<BookOpen size={20} />}
          label="Questions Answered"
          value={allHistory.length}
        />
        <StatCard
          icon={<Target size={20} />}
          label="Overall Accuracy"
          value={`${overallAccuracy}%`}
          sub={`${allHistory.filter((h) => h.correct).length} / ${allHistory.length} correct`}
        />
        <StatCard
          icon={<TrendingUp size={20} />}
          label="Tracked Sessions"
          value={importedSessions}
          sub={`${snapshot.tracked.quizzes.length} quiz(es) tracked`}
        />
      </div>

      {/* Performance by Class */}
      {classAccuracy.length > 0 && (
        <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
          <h2 className="mb-4 text-base font-semibold text-slate-900">
            Performance by Class
          </h2>
          <ResponsiveContainer width="100%" height={280}>
            <BarChart data={classAccuracy}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
              <XAxis
                dataKey="name"
                tick={{ fontSize: 12, fill: '#64748b' }}
              />
              <YAxis
                domain={[0, 100]}
                tick={{ fontSize: 12, fill: '#64748b' }}
                tickFormatter={(v: number) => `${v}%`}
              />
              <Tooltip
                formatter={(value) => [`${value}%`, 'Accuracy']}
                contentStyle={{
                  borderRadius: 8,
                  border: '1px solid #e2e8f0',
                }}
              />
              <Bar dataKey="accuracy" radius={[4, 4, 0, 0]}>
                {classAccuracy.map((_, i) => (
                  <Cell
                    key={i}
                    fill={BAR_COLORS[i % BAR_COLORS.length]}
                  />
                ))}
              </Bar>
            </BarChart>
          </ResponsiveContainer>
        </section>
      )}

      {/* Recent Quizzes */}
      <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
        <div className="border-b border-slate-100 px-5 py-4">
          <h2 className="text-base font-semibold text-slate-900">
            Recent Quizzes
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-slate-100 text-xs font-medium uppercase tracking-wide text-slate-500">
                <th className="px-5 py-3">Title</th>
                <th className="px-5 py-3">Class</th>
                <th className="px-5 py-3">Questions</th>
                <th className="px-5 py-3">Generated</th>
                <th className="px-5 py-3">Status</th>
              </tr>
            </thead>
            <tbody>
              {[...snapshot.quizzes]
                .sort(
                  (a, b) =>
                    new Date(b.generated_at).getTime() -
                    new Date(a.generated_at).getTime(),
                )
                .map((quiz) => {
                  const tracked = snapshot.tracked.quizzes.find(
                    (t) => t.quiz_id === quiz.id,
                  );
                  return (
                    <tr
                      key={quiz.id}
                      className="border-b border-slate-50 transition-colors duration-100 hover:bg-slate-50"
                    >
                      <td className="px-5 py-3 font-medium text-slate-900">
                        {quiz.title}
                      </td>
                      <td className="px-5 py-3 text-slate-600">
                        {quiz.class}
                      </td>
                      <td className="px-5 py-3 text-slate-600">
                        {quiz.question_count}
                      </td>
                      <td className="px-5 py-3 text-slate-500">
                        {formatDate(quiz.generated_at)}
                      </td>
                      <td className="px-5 py-3">
                        {tracked?.last_imported_at ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700">
                            <CheckCircle2 size={12} />
                            Synced
                          </span>
                        ) : tracked ? (
                          <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-0.5 text-xs font-medium text-amber-700">
                            <Clock size={12} />
                            Pending
                          </span>
                        ) : (
                          <span className="inline-flex items-center rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-500">
                            Untracked
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              {snapshot.quizzes.length === 0 && (
                <tr>
                  <td
                    colSpan={5}
                    className="px-5 py-8 text-center text-slate-400"
                  >
                    No quizzes generated yet
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Knowledge Coverage */}
      <section className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h2 className="mb-4 text-base font-semibold text-slate-900">
          Knowledge Coverage
        </h2>
        {coverageByClass.length === 0 ? (
          <p className="text-sm text-slate-400">No sections found</p>
        ) : (
          <div className="space-y-4">
            {coverageByClass.map((c) => (
              <div key={c.name}>
                <div className="mb-1 flex items-center justify-between text-sm">
                  <span className="font-medium text-slate-700">{c.name}</span>
                  <span className="text-slate-500">
                    {c.quizzed}/{c.total} sections ({c.pct}%)
                  </span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-slate-100">
                  <div
                    className="h-full rounded-full bg-indigo-500 transition-all duration-300"
                    style={{ width: `${c.pct}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        )}

        {needsAttention.length > 0 && (
          <div className="mt-6">
            <h3 className="mb-2 text-sm font-semibold text-slate-700">
              Needs Attention
            </h3>
            <div className="grid grid-cols-1 gap-2 sm:grid-cols-2">
              {needsAttention.map((s) => {
                const acc =
                  s.question_history && s.question_history.length > 0
                    ? accuracyPercent(s.question_history)
                    : null;
                return (
                  <div
                    key={s.id}
                    className="flex items-center justify-between rounded-lg border border-slate-100 px-3 py-2 text-sm"
                  >
                    <span className="truncate font-medium text-slate-700">
                      {s.title}
                    </span>
                    {acc !== null ? (
                      <span className="shrink-0 rounded-full bg-red-50 px-2 py-0.5 text-xs font-medium text-red-600">
                        {acc}%
                      </span>
                    ) : (
                      <span className="shrink-0 rounded-full bg-slate-100 px-2 py-0.5 text-xs font-medium text-slate-500">
                        No data
                      </span>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}
      </section>

      {/* Tracked Quiz Sessions */}
      <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
        <div className="border-b border-slate-100 px-5 py-4">
          <h2 className="text-base font-semibold text-slate-900">
            Tracked Quiz Sessions
          </h2>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-sm">
            <thead>
              <tr className="border-b border-slate-100 text-xs font-medium uppercase tracking-wide text-slate-500">
                <th className="px-5 py-3">Quiz ID</th>
                <th className="px-5 py-3">Class</th>
                <th className="px-5 py-3">Registered</th>
                <th className="px-5 py-3">Last Import</th>
                <th className="px-5 py-3">Status</th>
              </tr>
            </thead>
            <tbody>
              {snapshot.tracked.quizzes.map((t) => (
                <tr
                  key={t.quiz_id}
                  className="border-b border-slate-50 transition-colors duration-100 hover:bg-slate-50"
                >
                  <td className="px-5 py-3 font-mono text-xs text-slate-700">
                    {t.quiz_id.slice(0, 12)}...
                  </td>
                  <td className="px-5 py-3 text-slate-600">{t.class}</td>
                  <td className="px-5 py-3 text-slate-500">
                    {formatDate(t.registered_at)}
                  </td>
                  <td className="px-5 py-3 text-slate-500">
                    {t.last_imported_at
                      ? formatDate(t.last_imported_at)
                      : '—'}
                  </td>
                  <td className="px-5 py-3">
                    {t.last_imported_at ? (
                      <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-medium text-emerald-700">
                        <CheckCircle2 size={12} />
                        Imported
                      </span>
                    ) : (
                      <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-0.5 text-xs font-medium text-amber-700">
                        <Clock size={12} />
                        Pending
                      </span>
                    )}
                  </td>
                </tr>
              ))}
              {snapshot.tracked.quizzes.length === 0 && (
                <tr>
                  <td
                    colSpan={5}
                    className="px-5 py-8 text-center text-slate-400"
                  >
                    No tracked quizzes
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>

      {/* Generate Modal */}
      {showGenerate && (
        <GenerateQuizModal
          snapshot={snapshot}
          onClose={() => setShowGenerate(false)}
          onDone={() => {
            setShowGenerate(false);
            void load();
          }}
        />
      )}
    </div>
  );
}
