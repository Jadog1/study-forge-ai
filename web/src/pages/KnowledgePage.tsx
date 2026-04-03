import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BookOpen,
  RefreshCw,
  Tag,
  Layers,
  Clock,
  CheckCircle,
  XCircle,
  FileText,
} from 'lucide-react';
import { fetchClasses, fetchComponents, fetchSections } from '../api/client';
import { Badge } from '../components/Badge';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { PageHeader } from '../components/PageHeader';
import { SearchInput } from '../components/SearchInput';
import { useToast } from '../contexts/ToastContext';
import type { Component, Section } from '../types';

export function KnowledgePage() {
  const { toast } = useToast();
  const [sections, setSections] = useState<Section[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [classes, setClasses] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [classFilter, setClassFilter] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [s, c, cl] = await Promise.all([
        fetchSections(),
        fetchComponents(),
        fetchClasses(),
      ]);
      setSections(s);
      setComponents(c);
      setClasses(cl);
    } catch (err) {
      toast(err instanceof Error ? err.message : 'Failed to load knowledge', 'error');
    } finally {
      setLoading(false);
    }
  }, [toast]);

  useEffect(() => {
    load();
  }, [load]);

  const filtered = useMemo(() => {
    const query = search.toLowerCase();
    return sections.filter((s) => {
      if (classFilter && s.class !== classFilter) return false;
      if (!query) return true;
      return (
        s.title.toLowerCase().includes(query) ||
        s.tags?.some((t) => t.toLowerCase().includes(query)) ||
        s.concepts?.some((c) => c.toLowerCase().includes(query))
      );
    });
  }, [sections, search, classFilter]);

  const selected = useMemo(
    () => sections.find((s) => s.id === selectedId) ?? null,
    [sections, selectedId],
  );

  const selectedComponents = useMemo(
    () =>
      selected
        ? components.filter(
            (c) =>
              c.section_id === selected.id ||
              selected.component_ids?.includes(c.id),
          )
        : [],
    [components, selected],
  );

  const sectionMetrics = useCallback(
    (section: Section) => {
      const sectionComps = components.filter(
        (c) =>
          c.section_id === section.id ||
          section.component_ids?.includes(c.id),
      );
      const allHistory = [
        ...(section.question_history ?? []),
        ...sectionComps.flatMap((c) => c.question_history ?? []),
      ];
      const total = allHistory.length;
      const correct = allHistory.filter((h) => h.correct).length;
      return { total, correct, accuracy: total > 0 ? Math.round((correct / total) * 100) : null };
    },
    [components],
  );

  if (loading) {
    return (
      <div className="p-6 lg:p-8">
        <PageHeader title="Knowledge" description="Browse and explore your knowledge base." />
        <LoadingSpinner label="Loading knowledge base..." />
      </div>
    );
  }

  if (sections.length === 0) {
    return (
      <div className="p-6 lg:p-8">
        <PageHeader
          title="Knowledge"
          description="Browse and explore your knowledge base."
          actions={
            <button
              onClick={load}
              className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>
          }
        />
        <EmptyState
          icon={BookOpen}
          title="No knowledge ingested yet"
          description="Ingest study materials through the chat interface or CLI to start building your knowledge base."
        />
      </div>
    );
  }

  return (
    <div className="flex h-full flex-col">
      <div className="border-b border-slate-200 bg-white px-4 py-3 lg:px-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-lg font-semibold text-slate-900">Knowledge</h1>
            <p className="text-xs text-slate-500">
              {sections.length} section{sections.length !== 1 ? 's' : ''} · {components.length} component{components.length !== 1 ? 's' : ''}
            </p>
          </div>
          <button
            onClick={load}
            className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
          >
            <RefreshCw className="h-4 w-4" />
            Refresh
          </button>
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Left panel: section list */}
        <div className="w-80 shrink-0 border-r border-slate-200 bg-white flex flex-col overflow-hidden lg:w-96">
          <div className="space-y-2 border-b border-slate-100 px-3 py-3">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder="Search sections..."
            />
            {classes.length > 1 && (
              <select
                value={classFilter}
                onChange={(e) => setClassFilter(e.target.value)}
                className="w-full rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm text-slate-700 focus:border-indigo-300 focus:outline-none focus:ring-2 focus:ring-indigo-100"
              >
                <option value="">All classes</option>
                {classes.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
            )}
          </div>

          <div className="flex-1 overflow-y-auto">
            {filtered.length === 0 ? (
              <div className="p-4 text-center text-sm text-slate-400">
                No sections match your search.
              </div>
            ) : (
              <ul className="divide-y divide-slate-100">
                {filtered.map((section) => {
                  const metrics = sectionMetrics(section);
                  const isActive = section.id === selectedId;
                  return (
                    <li key={section.id}>
                      <button
                        onClick={() => setSelectedId(section.id)}
                        className={`w-full px-4 py-3 text-left transition-colors ${
                          isActive
                            ? 'bg-indigo-50 border-l-2 border-indigo-500'
                            : 'hover:bg-slate-50 border-l-2 border-transparent'
                        }`}
                      >
                        <div className="flex items-start justify-between gap-2">
                          <h3 className="text-sm font-medium text-slate-900 line-clamp-2">
                            {section.title}
                          </h3>
                          {metrics.accuracy !== null && (
                            <span
                              className={`shrink-0 text-xs font-semibold ${
                                metrics.accuracy >= 80
                                  ? 'text-emerald-600'
                                  : metrics.accuracy >= 50
                                    ? 'text-amber-600'
                                    : 'text-red-600'
                              }`}
                            >
                              {metrics.accuracy}%
                            </span>
                          )}
                        </div>
                        <div className="mt-1 flex items-center gap-3 text-xs text-slate-500">
                          <span>{section.class}</span>
                          {section.tags && section.tags.length > 0 && (
                            <span className="flex items-center gap-1">
                              <Tag className="h-3 w-3" />
                              {section.tags.length}
                            </span>
                          )}
                          {section.component_ids && (
                            <span className="flex items-center gap-1">
                              <Layers className="h-3 w-3" />
                              {section.component_ids.length}
                            </span>
                          )}
                        </div>
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        </div>

        {/* Right panel: section detail */}
        <div className="flex-1 overflow-y-auto bg-slate-50 p-4 lg:p-6">
          {!selected ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState
                icon={BookOpen}
                title="Select a section"
                description="Choose a section from the list to view its details and components."
              />
            </div>
          ) : (
            <SectionDetail
              section={selected}
              components={selectedComponents}
            />
          )}
        </div>
      </div>
    </div>
  );
}

function SectionDetail({
  section,
  components,
}: {
  section: Section;
  components: Component[];
}) {
  const history = [
    ...(section.question_history ?? []),
    ...components.flatMap((c) => c.question_history ?? []),
  ].sort((a, b) => b.answered_at.localeCompare(a.answered_at));

  const correct = history.filter((h) => h.correct).length;
  const total = history.length;
  const accuracy = total > 0 ? Math.round((correct / total) * 100) : null;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h2 className="text-xl font-bold text-slate-900">{section.title}</h2>
        <p className="mt-1 text-sm text-slate-500">{section.class}</p>
      </div>

      {/* Summary */}
      <Card>
        <p className="text-sm text-slate-700 leading-relaxed">{section.summary}</p>
      </Card>

      {/* Metrics */}
      {accuracy !== null && (
        <div className="grid grid-cols-3 gap-3">
          <Card className="text-center">
            <p className="text-2xl font-bold text-indigo-600">{accuracy}%</p>
            <p className="text-xs text-slate-500">Accuracy</p>
          </Card>
          <Card className="text-center">
            <p className="text-2xl font-bold text-emerald-600">{correct}</p>
            <p className="text-xs text-slate-500">Correct</p>
          </Card>
          <Card className="text-center">
            <p className="text-2xl font-bold text-slate-700">{total}</p>
            <p className="text-xs text-slate-500">Answered</p>
          </Card>
        </div>
      )}

      {/* Tags & Concepts */}
      {((section.tags && section.tags.length > 0) ||
        (section.concepts && section.concepts.length > 0)) && (
        <Card
          header={
            <h3 className="text-sm font-semibold text-slate-700">Tags & Concepts</h3>
          }
        >
          <div className="space-y-2">
            {section.tags && section.tags.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {section.tags.map((tag) => (
                  <Badge key={tag} color="indigo">
                    {tag}
                  </Badge>
                ))}
              </div>
            )}
            {section.concepts && section.concepts.length > 0 && (
              <div className="flex flex-wrap gap-1.5">
                {section.concepts.map((concept) => (
                  <Badge key={concept} color="purple">
                    {concept}
                  </Badge>
                ))}
              </div>
            )}
          </div>
        </Card>
      )}

      {/* Source Paths */}
      {section.source_paths && section.source_paths.length > 0 && (
        <Card
          header={
            <h3 className="text-sm font-semibold text-slate-700">Sources</h3>
          }
        >
          <ul className="space-y-1">
            {section.source_paths.map((path) => (
              <li
                key={path}
                className="flex items-center gap-2 text-xs text-slate-600 font-mono"
              >
                <FileText className="h-3 w-3 shrink-0 text-slate-400" />
                {path}
              </li>
            ))}
          </ul>
        </Card>
      )}

      {/* Components */}
      {components.length > 0 && (
        <div>
          <h3 className="mb-3 text-sm font-semibold text-slate-700">
            Components ({components.length})
          </h3>
          <div className="space-y-3">
            {components.map((comp) => (
              <Card key={comp.id}>
                <div className="flex items-start justify-between gap-2 mb-2">
                  <div className="flex items-center gap-2">
                    <Badge color="blue">{comp.kind}</Badge>
                    {comp.tags?.map((tag) => (
                      <Badge key={tag} color="slate">
                        {tag}
                      </Badge>
                    ))}
                  </div>
                </div>
                <p className="text-sm text-slate-700 line-clamp-4 whitespace-pre-wrap">
                  {comp.content}
                </p>
              </Card>
            ))}
          </div>
        </div>
      )}

      {/* Question History */}
      {history.length > 0 && (
        <div>
          <h3 className="mb-3 text-sm font-semibold text-slate-700">
            Question History ({history.length})
          </h3>
          <div className="space-y-2">
            {history.slice(0, 20).map((entry) => (
              <Card key={entry.id}>
                <div className="flex items-start gap-2">
                  {entry.correct ? (
                    <CheckCircle className="mt-0.5 h-4 w-4 shrink-0 text-emerald-500" />
                  ) : (
                    <XCircle className="mt-0.5 h-4 w-4 shrink-0 text-red-500" />
                  )}
                  <div className="flex-1 min-w-0">
                    {entry.question && (
                      <p className="text-sm font-medium text-slate-800 line-clamp-2">
                        {entry.question}
                      </p>
                    )}
                    <div className="mt-1 flex items-center gap-3 text-xs text-slate-500">
                      {entry.user_answer && (
                        <span>
                          Answer: <span className="font-medium">{entry.user_answer}</span>
                        </span>
                      )}
                      <span className="flex items-center gap-1">
                        <Clock className="h-3 w-3" />
                        {new Date(entry.answered_at).toLocaleDateString()}
                      </span>
                    </div>
                  </div>
                </div>
              </Card>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
