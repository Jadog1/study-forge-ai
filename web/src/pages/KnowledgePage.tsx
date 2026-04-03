import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  BookOpen,
  CheckCircle,
  CheckSquare,
  ChevronDown,
  ChevronRight,
  Clock,
  Download,
  FileText,
  Layers,
  Loader2,
  Plus,
  RefreshCw,
  Square,
  Tag,
  Upload,
  XCircle,
  Zap,
} from 'lucide-react';
import { fetchClasses, fetchComponents, fetchSections } from '../api/client';
import { api } from '../api/client';
import { Badge } from '../components/Badge';
import { Card } from '../components/Card';
import { EmptyState } from '../components/EmptyState';
import { ExportModal } from '../components/ExportModal';
import { IngestModal } from '../components/IngestModal';
import { LoadingSpinner } from '../components/LoadingSpinner';
import { PageHeader } from '../components/PageHeader';
import { SearchInput } from '../components/SearchInput';
import { useToast } from '../contexts/ToastContext';
import type {
  ChatStreamEvent,
  Component,
  ContextProfile,
  OrchestratorDirective,
  Section,
} from '../types';

type LeftTab = 'sections' | 'sources';

interface QuizSelection {
  componentIds: Set<string>;
  sourcePaths: Set<string>;
}

const EMPTY_SELECTION: QuizSelection = {
  componentIds: new Set(),
  sourcePaths: new Set(),
};

function selectionCount(sel: QuizSelection): number {
  return sel.componentIds.size + sel.sourcePaths.size;
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export function KnowledgePage() {
  const { toast } = useToast();
  const [sections, setSections] = useState<Section[]>([]);
  const [components, setComponents] = useState<Component[]>([]);
  const [classes, setClasses] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [classFilter, setClassFilter] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [leftTab, setLeftTab] = useState<LeftTab>('sections');
  const [selection, setSelection] = useState<QuizSelection>(EMPTY_SELECTION);
  const [showQuizPanel, setShowQuizPanel] = useState(false);
  const [showIngest, setShowIngest] = useState(false);
  const [showExport, setShowExport] = useState(false);

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

  // ---------- Derived data ----------

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

  const allSources = useMemo(() => {
    const sourceMap = new Map<string, { classes: Set<string>; sectionCount: number }>();
    for (const s of sections) {
      if (classFilter && s.class !== classFilter) continue;
      for (const path of s.source_paths ?? []) {
        const existing = sourceMap.get(path);
        if (existing) {
          existing.classes.add(s.class);
          existing.sectionCount++;
        } else {
          sourceMap.set(path, { classes: new Set([s.class]), sectionCount: 1 });
        }
      }
    }
    return [...sourceMap.entries()]
      .map(([path, info]) => ({
        path,
        classes: [...info.classes],
        sectionCount: info.sectionCount,
        name: path.split('/').pop() ?? path,
      }))
      .sort((a, b) => a.name.localeCompare(b.name));
  }, [sections, classFilter]);

  const filteredSources = useMemo(() => {
    if (!search) return allSources;
    const query = search.toLowerCase();
    return allSources.filter(
      (s) => s.name.toLowerCase().includes(query) || s.path.toLowerCase().includes(query),
    );
  }, [allSources, search]);

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

  // ---------- Selection helpers ----------

  const toggleComponent = useCallback((id: string) => {
    setSelection((prev) => {
      const next = new Set(prev.componentIds);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return { ...prev, componentIds: next };
    });
  }, []);

  const toggleSource = useCallback((path: string) => {
    setSelection((prev) => {
      const next = new Set(prev.sourcePaths);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return { ...prev, sourcePaths: next };
    });
  }, []);

  const selectAllComponents = useCallback((comps: Component[]) => {
    setSelection((prev) => {
      const next = new Set(prev.componentIds);
      const allSelected = comps.every((c) => next.has(c.id));
      if (allSelected) {
        for (const c of comps) next.delete(c.id);
      } else {
        for (const c of comps) next.add(c.id);
      }
      return { ...prev, componentIds: next };
    });
  }, []);

  const clearSelection = useCallback(() => {
    setSelection(EMPTY_SELECTION);
    setShowQuizPanel(false);
  }, []);

  // ---------- Resolved selection for quiz generation ----------

  const resolvedSelection = useMemo(() => {
    const selectedComps = components.filter((c) => selection.componentIds.has(c.id));

    const sourceFilteredComps = selection.sourcePaths.size > 0
      ? components.filter((c) =>
          c.source_paths?.some((sp) => selection.sourcePaths.has(sp)),
        )
      : [];

    const allComps = new Map<string, Component>();
    for (const c of selectedComps) allComps.set(c.id, c);
    for (const c of sourceFilteredComps) allComps.set(c.id, c);
    return [...allComps.values()];
  }, [components, selection]);

  const selectionClasses = useMemo(() => {
    const cls = new Set<string>();
    for (const c of resolvedSelection) cls.add(c.class);
    return [...cls].sort();
  }, [resolvedSelection]);

  // ---------- Render ----------

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
            <div className="flex gap-2">
              <button
                onClick={() => setShowIngest(true)}
                className="flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-700 transition-colors"
              >
                <Upload className="h-4 w-4" />
                Ingest Notes
              </button>
              <button
                onClick={load}
                className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
              >
                <RefreshCw className="h-4 w-4" />
                Refresh
              </button>
            </div>
          }
        />
        <EmptyState
          icon={BookOpen}
          title="No knowledge ingested yet"
          description="Ingest study materials to start building your knowledge base. Click 'Ingest Notes' above to get started."
        />
        <IngestModal
          open={showIngest}
          onClose={() => setShowIngest(false)}
          onDone={() => {
            setShowIngest(false);
            load();
          }}
        />
      </div>
    );
  }

  const totalSelected = selectionCount(selection);

  return (
    <div className="flex h-full flex-col">
      {/* Top bar */}
      <div className="border-b border-slate-200 bg-white px-4 py-3 lg:px-6">
        <div className="flex items-center justify-between gap-4">
          <div>
            <h1 className="text-lg font-semibold text-slate-900">Knowledge</h1>
            <p className="text-xs text-slate-500">
              {sections.length} section{sections.length !== 1 ? 's' : ''} · {components.length} component{components.length !== 1 ? 's' : ''} · {allSources.length} source{allSources.length !== 1 ? 's' : ''}
            </p>
          </div>
          <div className="flex items-center gap-2">
            <button
              onClick={() => setShowIngest(true)}
              className="flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-2 text-sm font-medium text-white hover:bg-indigo-700 transition-colors"
            >
              <Upload className="h-4 w-4" />
              Ingest
            </button>
            <button
              onClick={() => setShowExport(true)}
              className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              <Download className="h-4 w-4" />
              Export
            </button>
            <button
              onClick={load}
              className="flex items-center gap-2 rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
            >
              <RefreshCw className="h-4 w-4" />
              Refresh
            </button>
          </div>
        </div>
      </div>

      <div className="flex flex-1 overflow-hidden">
        {/* Left panel */}
        <div className="w-80 shrink-0 border-r border-slate-200 bg-white flex flex-col overflow-hidden lg:w-96">
          {/* Tab switcher */}
          <div className="flex border-b border-slate-100">
            <button
              onClick={() => setLeftTab('sections')}
              className={`flex-1 px-4 py-2.5 text-sm font-medium transition-colors ${
                leftTab === 'sections'
                  ? 'border-b-2 border-indigo-500 text-indigo-600'
                  : 'text-slate-500 hover:text-slate-700'
              }`}
            >
              <span className="flex items-center justify-center gap-1.5">
                <Layers className="h-4 w-4" />
                Sections
              </span>
            </button>
            <button
              onClick={() => setLeftTab('sources')}
              className={`flex-1 px-4 py-2.5 text-sm font-medium transition-colors ${
                leftTab === 'sources'
                  ? 'border-b-2 border-indigo-500 text-indigo-600'
                  : 'text-slate-500 hover:text-slate-700'
              }`}
            >
              <span className="flex items-center justify-center gap-1.5">
                <FileText className="h-4 w-4" />
                Sources
                {selection.sourcePaths.size > 0 && (
                  <Badge color="indigo">{selection.sourcePaths.size}</Badge>
                )}
              </span>
            </button>
          </div>

          {/* Search + filter */}
          <div className="space-y-2 border-b border-slate-100 px-3 py-3">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder={leftTab === 'sections' ? 'Search sections...' : 'Search sources...'}
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

          {/* List */}
          <div className="flex-1 overflow-y-auto">
            {leftTab === 'sections' ? (
              <SectionList
                sections={filtered}
                selectedId={selectedId}
                onSelect={setSelectedId}
                sectionMetrics={sectionMetrics}
              />
            ) : (
              <SourceList
                sources={filteredSources}
                selectedPaths={selection.sourcePaths}
                onToggle={toggleSource}
              />
            )}
          </div>
        </div>

        {/* Right panel */}
        <div className="flex-1 overflow-y-auto bg-slate-50 p-4 lg:p-6">
          {!selected ? (
            <div className="flex h-full items-center justify-center">
              <EmptyState
                icon={BookOpen}
                title="Select a section"
                description="Choose a section from the list to view its details, select components, or browse sources to build a quiz."
              />
            </div>
          ) : (
            <SectionDetail
              section={selected}
              components={selectedComponents}
              selectedComponentIds={selection.componentIds}
              selectedSourcePaths={selection.sourcePaths}
              onToggleComponent={toggleComponent}
              onToggleSource={toggleSource}
              onSelectAllComponents={selectAllComponents}
            />
          )}
        </div>
      </div>

      {/* Floating selection bar */}
      {totalSelected > 0 && !showQuizPanel && (
        <SelectionBar
          selection={selection}
          resolvedComponentCount={resolvedSelection.length}
          onClear={clearSelection}
          onGenerate={() => setShowQuizPanel(true)}
        />
      )}

      {/* Quiz generation panel */}
      {showQuizPanel && (
        <QuizGenerationPanel
          resolvedComponents={resolvedSelection}
          selectionClasses={selectionClasses}
          selection={selection}
          sections={sections}
          onClose={() => setShowQuizPanel(false)}
          onClear={clearSelection}
          onDone={() => {
            clearSelection();
            toast('Quiz generated successfully!', 'success');
          }}
        />
      )}

      {/* Ingest modal */}
      <IngestModal
        open={showIngest}
        onClose={() => setShowIngest(false)}
        onDone={() => {
          setShowIngest(false);
          load();
          toast('Notes ingested successfully!', 'success');
        }}
      />

      {/* Export modal */}
      <ExportModal
        open={showExport}
        onClose={() => setShowExport(false)}
      />
    </div>
  );
}

// ---------------------------------------------------------------------------
// Section List
// ---------------------------------------------------------------------------

function SectionList({
  sections,
  selectedId,
  onSelect,
  sectionMetrics,
}: {
  sections: Section[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  sectionMetrics: (s: Section) => { total: number; correct: number; accuracy: number | null };
}) {
  if (sections.length === 0) {
    return (
      <div className="p-4 text-center text-sm text-slate-400">
        No sections match your search.
      </div>
    );
  }

  return (
    <ul className="divide-y divide-slate-100">
      {sections.map((section) => {
        const metrics = sectionMetrics(section);
        const isActive = section.id === selectedId;
        return (
          <li key={section.id}>
            <button
              onClick={() => onSelect(section.id)}
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
  );
}

// ---------------------------------------------------------------------------
// Source List (new tab)
// ---------------------------------------------------------------------------

function SourceList({
  sources,
  selectedPaths,
  onToggle,
}: {
  sources: { path: string; name: string; classes: string[]; sectionCount: number }[];
  selectedPaths: Set<string>;
  onToggle: (path: string) => void;
}) {
  if (sources.length === 0) {
    return (
      <div className="p-4 text-center text-sm text-slate-400">
        No sources match your search.
      </div>
    );
  }

  return (
    <ul className="divide-y divide-slate-100">
      {sources.map((source) => {
        const isSelected = selectedPaths.has(source.path);
        return (
          <li key={source.path}>
            <button
              onClick={() => onToggle(source.path)}
              className={`w-full px-4 py-3 text-left transition-colors ${
                isSelected
                  ? 'bg-indigo-50'
                  : 'hover:bg-slate-50'
              }`}
            >
              <div className="flex items-start gap-3">
                <div className="mt-0.5 shrink-0">
                  {isSelected ? (
                    <CheckSquare className="h-4 w-4 text-indigo-600" />
                  ) : (
                    <Square className="h-4 w-4 text-slate-300" />
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <p className="text-sm font-medium text-slate-900 truncate">
                    {source.name}
                  </p>
                  <p className="mt-0.5 text-xs text-slate-500 truncate font-mono">
                    {source.path}
                  </p>
                  <div className="mt-1 flex items-center gap-2 text-xs text-slate-400">
                    <span>{source.sectionCount} section{source.sectionCount !== 1 ? 's' : ''}</span>
                    {source.classes.length > 0 && (
                      <span>· {source.classes.join(', ')}</span>
                    )}
                  </div>
                </div>
              </div>
            </button>
          </li>
        );
      })}
    </ul>
  );
}

// ---------------------------------------------------------------------------
// Section Detail (enhanced with selection)
// ---------------------------------------------------------------------------

function SectionDetail({
  section,
  components,
  selectedComponentIds,
  selectedSourcePaths,
  onToggleComponent,
  onToggleSource,
  onSelectAllComponents,
}: {
  section: Section;
  components: Component[];
  selectedComponentIds: Set<string>;
  selectedSourcePaths: Set<string>;
  onToggleComponent: (id: string) => void;
  onToggleSource: (path: string) => void;
  onSelectAllComponents: (comps: Component[]) => void;
}) {
  const history = [
    ...(section.question_history ?? []),
    ...components.flatMap((c) => c.question_history ?? []),
  ].sort((a, b) => b.answered_at.localeCompare(a.answered_at));

  const correct = history.filter((h) => h.correct).length;
  const total = history.length;
  const accuracy = total > 0 ? Math.round((correct / total) * 100) : null;
  const allComponentsSelected = components.length > 0 && components.every((c) => selectedComponentIds.has(c.id));
  const someComponentsSelected = components.some((c) => selectedComponentIds.has(c.id));

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

      {/* Source Paths (selectable) */}
      {section.source_paths && section.source_paths.length > 0 && (
        <Card
          header={
            <h3 className="text-sm font-semibold text-slate-700">
              Sources
              <span className="ml-2 text-xs font-normal text-slate-400">Click to select for quiz</span>
            </h3>
          }
        >
          <ul className="space-y-1">
            {section.source_paths.map((path) => {
              const isSelected = selectedSourcePaths.has(path);
              return (
                <li key={path}>
                  <button
                    onClick={() => onToggleSource(path)}
                    className={`flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-xs transition-colors ${
                      isSelected
                        ? 'bg-indigo-50 text-indigo-700'
                        : 'text-slate-600 hover:bg-slate-50'
                    }`}
                  >
                    {isSelected ? (
                      <CheckSquare className="h-3.5 w-3.5 shrink-0 text-indigo-600" />
                    ) : (
                      <Square className="h-3.5 w-3.5 shrink-0 text-slate-300" />
                    )}
                    <FileText className="h-3 w-3 shrink-0 text-slate-400" />
                    <span className="font-mono truncate">{path}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </Card>
      )}

      {/* Components (selectable) */}
      {components.length > 0 && (
        <div>
          <div className="mb-3 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-700">
              Components ({components.length})
            </h3>
            <button
              onClick={() => onSelectAllComponents(components)}
              className="flex items-center gap-1.5 text-xs font-medium text-indigo-600 hover:text-indigo-700 transition-colors"
            >
              {allComponentsSelected ? (
                <CheckSquare className="h-3.5 w-3.5" />
              ) : someComponentsSelected ? (
                <CheckSquare className="h-3.5 w-3.5 opacity-50" />
              ) : (
                <Square className="h-3.5 w-3.5" />
              )}
              {allComponentsSelected ? 'Deselect all' : 'Select all'}
            </button>
          </div>
          <div className="space-y-3">
            {components.map((comp) => {
              const isSelected = selectedComponentIds.has(comp.id);
              return (
                <div
                  key={comp.id}
                  className={`rounded-lg border p-4 transition-all ${
                    isSelected
                      ? 'border-indigo-300 bg-indigo-50/50 ring-1 ring-indigo-200'
                      : 'border-slate-200 bg-white'
                  }`}
                >
                  <div className="flex items-start gap-3">
                    <button
                      onClick={() => onToggleComponent(comp.id)}
                      className="mt-0.5 shrink-0"
                    >
                      {isSelected ? (
                        <CheckSquare className="h-4 w-4 text-indigo-600" />
                      ) : (
                        <Square className="h-4 w-4 text-slate-300 hover:text-slate-400" />
                      )}
                    </button>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2 mb-2">
                        <Badge color="blue">{comp.kind}</Badge>
                        {comp.tags?.map((tag) => (
                          <Badge key={tag} color="slate">
                            {tag}
                          </Badge>
                        ))}
                      </div>
                      <p className="text-sm text-slate-700 line-clamp-4 whitespace-pre-wrap">
                        {comp.content}
                      </p>
                    </div>
                  </div>
                </div>
              );
            })}
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

// ---------------------------------------------------------------------------
// Selection Bar (floating)
// ---------------------------------------------------------------------------

function SelectionBar({
  selection,
  resolvedComponentCount,
  onClear,
  onGenerate,
}: {
  selection: QuizSelection;
  resolvedComponentCount: number;
  onClear: () => void;
  onGenerate: () => void;
}) {
  const parts: string[] = [];
  if (selection.componentIds.size > 0) {
    parts.push(`${selection.componentIds.size} component${selection.componentIds.size !== 1 ? 's' : ''}`);
  }
  if (selection.sourcePaths.size > 0) {
    parts.push(`${selection.sourcePaths.size} source${selection.sourcePaths.size !== 1 ? 's' : ''}`);
  }

  return (
    <div className="border-t border-slate-200 bg-white px-4 py-3 shadow-lg">
      <div className="mx-auto flex max-w-7xl items-center justify-between">
        <div className="flex items-center gap-3">
          <Zap className="h-4 w-4 text-indigo-600" />
          <div className="text-sm">
            <span className="font-medium text-slate-900">{parts.join(' + ')}</span>
            <span className="text-slate-500"> selected</span>
            {resolvedComponentCount > 0 && (
              <span className="text-slate-400 ml-1">
                ({resolvedComponentCount} component{resolvedComponentCount !== 1 ? 's' : ''} resolved)
              </span>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onClear}
            className="rounded-lg border border-slate-200 px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50 transition-colors"
          >
            Clear
          </button>
          <button
            onClick={onGenerate}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-4 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 transition-colors"
          >
            <Zap className="h-4 w-4" />
            Generate Quiz
          </button>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Quiz Generation Panel (slide-up panel)
// ---------------------------------------------------------------------------

function QuizGenerationPanel({
  resolvedComponents,
  selectionClasses,
  selection,
  sections,
  onClose,
  onClear,
  onDone,
}: {
  resolvedComponents: Component[];
  selectionClasses: string[];
  selection: QuizSelection;
  sections: Section[];
  onClose: () => void;
  onClear: () => void;
  onDone: () => void;
}) {
  const [numQuestions, setNumQuestions] = useState(
    Math.min(Math.max(resolvedComponents.length, 5), 20),
  );
  const [questionType, setQuestionType] = useState('multiple-choice');
  const [assessmentType, setAssessmentType] = useState('quiz');
  const [questionTypes, setQuestionTypes] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<ContextProfile[]>([]);
  const [generating, setGenerating] = useState(false);
  const [events, setEvents] = useState<ChatStreamEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [showDetails, setShowDetails] = useState(false);

  useEffect(() => {
    api.fetchQuestionTypes().then(setQuestionTypes).catch(() => {});
    api.fetchContextProfiles().then(setProfiles).catch(() => {});
  }, []);

  const assessmentTypes = useMemo(
    () => [...new Set(profiles.map((p) => p.kind))],
    [profiles],
  );

  const targetClass = selectionClasses[0] ?? '';

  const buildDirectives = useCallback((): OrchestratorDirective[] => {
    if (resolvedComponents.length === 0) return [];

    const perComponent = Math.max(1, Math.floor(numQuestions / resolvedComponents.length));
    let remainder = numQuestions - perComponent * resolvedComponents.length;

    return resolvedComponents.map((comp) => {
      const count = perComponent + (remainder > 0 ? 1 : 0);
      if (remainder > 0) remainder--;

      const parentSection = sections.find(
        (s) => s.id === comp.section_id || s.component_ids?.includes(comp.id),
      );

      return {
        component_id: comp.id,
        section_id: comp.section_id ?? parentSection?.id ?? '',
        section_title: parentSection?.title ?? '',
        question_count: count,
        question_types: [questionType],
        angle: '',
      };
    });
  }, [resolvedComponents, numQuestions, questionType, sections]);

  const handleGenerate = async () => {
    setGenerating(true);
    setEvents([]);
    setError(null);
    try {
      const directives = buildDirectives();
      await api.generateQuiz(
        {
          class: targetClass,
          count: numQuestions,
          assessment_type: assessmentType,
          question_type: questionType,
          directives: directives.length > 0 ? directives : undefined,
        },
        (e) => setEvents((prev) => [...prev, e]),
      );
      onDone();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Quiz generation failed');
    } finally {
      setGenerating(false);
    }
  };

  const summaryParts: string[] = [];
  if (selection.componentIds.size > 0) {
    summaryParts.push(`${selection.componentIds.size} component${selection.componentIds.size !== 1 ? 's' : ''}`);
  }
  if (selection.sourcePaths.size > 0) {
    summaryParts.push(`${selection.sourcePaths.size} source${selection.sourcePaths.size !== 1 ? 's' : ''}`);
  }

  return (
    <div className="border-t border-slate-200 bg-white shadow-xl">
      <div className="mx-auto max-w-5xl px-4 py-4 lg:px-6">
        {/* Header */}
        <div className="mb-4 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-100">
              <Zap className="h-4 w-4 text-indigo-600" />
            </div>
            <div>
              <h3 className="text-sm font-semibold text-slate-900">
                Generate Quiz from Selection
              </h3>
              <p className="text-xs text-slate-500">
                {summaryParts.join(' + ')} → {resolvedComponents.length} component{resolvedComponents.length !== 1 ? 's' : ''} for <span className="font-medium">{targetClass || 'unknown class'}</span>
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 text-slate-400 hover:bg-slate-100 hover:text-slate-600 transition-colors"
          >
            <XCircle className="h-5 w-5" />
          </button>
        </div>

        {/* Config row */}
        <div className="mb-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">Questions</label>
            <input
              type="number"
              min={1}
              max={50}
              value={numQuestions}
              onChange={(e) => setNumQuestions(Number(e.target.value))}
              className="w-full rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">Assessment</label>
            <select
              value={assessmentType}
              onChange={(e) => setAssessmentType(e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              {(assessmentTypes.length > 0
                ? assessmentTypes
                : ['quiz', 'exam', 'focused']
              ).map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
          <div>
            <label className="mb-1 block text-xs font-medium text-slate-600">Question Type</label>
            <select
              value={questionType}
              onChange={(e) => setQuestionType(e.target.value)}
              className="w-full rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            >
              {(questionTypes.length > 0
                ? questionTypes
                : ['multiple-choice', 'multi-select', 'true-false', 'short-answer', 'fill-in-the-blank']
              ).map((t) => (
                <option key={t} value={t}>{t}</option>
              ))}
            </select>
          </div>
          <div className="flex items-end gap-2">
            <button
              onClick={handleGenerate}
              disabled={generating || !targetClass || resolvedComponents.length === 0}
              className="flex flex-1 items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-1.5 text-sm font-medium text-white transition-colors hover:bg-indigo-700 disabled:opacity-50"
            >
              {generating ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  Generating...
                </>
              ) : (
                <>
                  <Plus size={14} />
                  Generate
                </>
              )}
            </button>
          </div>
        </div>

        {/* Expandable details */}
        <button
          onClick={() => setShowDetails(!showDetails)}
          className="mb-2 flex items-center gap-1 text-xs font-medium text-slate-500 hover:text-slate-700 transition-colors"
        >
          {showDetails ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
          {showDetails ? 'Hide' : 'Show'} selection details
        </button>

        {showDetails && (
          <div className="mb-3 max-h-32 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-3">
            {selection.componentIds.size > 0 && (
              <div className="mb-2">
                <p className="text-xs font-semibold text-slate-600 mb-1">Selected Components:</p>
                <div className="flex flex-wrap gap-1">
                  {resolvedComponents
                    .filter((c) => selection.componentIds.has(c.id))
                    .map((c) => (
                      <span
                        key={c.id}
                        className="inline-flex items-center rounded-full bg-indigo-100 px-2 py-0.5 text-xs text-indigo-700"
                      >
                        {c.kind}: {c.content.slice(0, 40)}…
                      </span>
                    ))}
                </div>
              </div>
            )}
            {selection.sourcePaths.size > 0 && (
              <div>
                <p className="text-xs font-semibold text-slate-600 mb-1">Selected Sources:</p>
                <div className="flex flex-wrap gap-1">
                  {[...selection.sourcePaths].map((p) => (
                    <span
                      key={p}
                      className="inline-flex items-center rounded-full bg-blue-100 px-2 py-0.5 text-xs text-blue-700 font-mono"
                    >
                      {p.split('/').pop()}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {/* Progress events */}
        {events.length > 0 && (
          <div className="mb-3 max-h-32 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs font-mono text-slate-600">
            {events.map((e, i) => (
              <div key={i} className="py-0.5">
                {e.type === 'action-start' && (
                  <span className="text-indigo-600">▸ {e.label ?? 'working'}...</span>
                )}
                {e.type === 'action-done' && (
                  <span className="text-emerald-600">✓ {e.label ?? 'done'}</span>
                )}
                {e.type === 'chunk' && <span>{e.text}</span>}
                {e.type === 'error' && (
                  <span className="text-red-600">✗ {e.error}</span>
                )}
                {e.type === 'done' && (
                  <span className="font-semibold text-emerald-700">✓ Complete</span>
                )}
              </div>
            ))}
          </div>
        )}

        {error && <p className="mb-2 text-sm text-red-600">{error}</p>}

        {/* Clear link */}
        <div className="flex justify-end">
          <button
            onClick={onClear}
            className="text-xs text-slate-400 hover:text-slate-600 transition-colors"
          >
            Clear selection
          </button>
        </div>
      </div>
    </div>
  );
}
