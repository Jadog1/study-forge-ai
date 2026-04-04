import { useCallback, useEffect, useMemo, useState } from 'react';
import {
  ArrowDown,
  ArrowUp,
  BookOpen,
  ChevronRight,
  FileText,
  GripVertical,
  Layers,
  Loader2,
  Plus,
  Save,
  Search,
  Settings,
  Trash2,
  WandSparkles,
  X,
} from 'lucide-react';
import {
  Cell,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
} from 'recharts';
import { api } from '../api/client';
import type {
  ClassDetail,
  CoverageScope,
  RosterEntry,
} from '../types';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const PIE_COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#f59e0b',
  '#10b981', '#3b82f6', '#ef4444', '#14b8a6',
];

function arraysEqual(a: string[], b: string[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((v, i) => v === b[i]);
}

function normalizeRosterEntries(entries: RosterEntry[]): RosterEntry[] {
  return entries.map((entry, i) => ({ ...entry, order: i + 1 }));
}

function extractDateValue(value: string): number | null {
  const text = value.trim();
  if (!text) return null;

  const isoMatch = text.match(/\b(\d{4}-\d{1,2}-\d{1,2})\b/);
  if (isoMatch) {
    const ts = Date.parse(isoMatch[1]);
    if (!Number.isNaN(ts)) return ts;
  }

  const slashMatch = text.match(/\b(\d{1,2}[\/\-]\d{1,2}[\/\-]\d{2,4})\b/);
  if (slashMatch) {
    const ts = Date.parse(slashMatch[1]);
    if (!Number.isNaN(ts)) return ts;
  }

  const monthMatch = text.match(
    /\b(?:jan|january|feb|february|mar|march|apr|april|may|jun|june|jul|july|aug|august|sep|sept|september|oct|october|nov|november|dec|december)\s+\d{1,2}(?:,\s*\d{4})?\b/i,
  );
  if (monthMatch) {
    const ts = Date.parse(monthMatch[0]);
    if (!Number.isNaN(ts)) return ts;
  }

  return null;
}

function extractNumericValue(entry: RosterEntry): number | null {
  if (typeof entry.week === 'number' && !Number.isNaN(entry.week)) {
    return entry.week;
  }
  const text = `${entry.label} ${entry.source_pattern ?? ''}`;
  const match = text.match(/\d+(?:\.\d+)?/);
  if (!match) return null;
  return Number(match[0]);
}

function smartOrderRosterEntries(entries: RosterEntry[]): RosterEntry[] {
  const enriched = entries.map((entry, idx) => {
    const haystack = `${entry.label} ${entry.source_pattern ?? ''}`;
    return {
      entry,
      idx,
      dateValue: extractDateValue(haystack),
      numberValue: extractNumericValue(entry),
      labelLower: entry.label.toLowerCase(),
    };
  });

  enriched.sort((a, b) => {
    const aHasDate = a.dateValue != null;
    const bHasDate = b.dateValue != null;
    if (aHasDate && bHasDate) {
      if (a.dateValue !== b.dateValue) return (a.dateValue ?? 0) - (b.dateValue ?? 0);
      return a.labelLower.localeCompare(b.labelLower);
    }
    if (aHasDate !== bHasDate) return aHasDate ? -1 : 1;

    const aHasNumber = a.numberValue != null;
    const bHasNumber = b.numberValue != null;
    if (aHasNumber && bHasNumber) {
      if (a.numberValue !== b.numberValue) return (a.numberValue ?? 0) - (b.numberValue ?? 0);
      return a.labelLower.localeCompare(b.labelLower);
    }
    if (aHasNumber !== bHasNumber) return aHasNumber ? -1 : 1;

    const labelCmp = a.labelLower.localeCompare(b.labelLower);
    if (labelCmp !== 0) return labelCmp;
    return a.idx - b.idx;
  });

  return normalizeRosterEntries(enriched.map((item) => item.entry));
}

type TabId = 'overview' | 'context' | 'roster' | 'coverage';

const TABS: { id: TabId; label: string; icon: React.ReactNode }[] = [
  { id: 'overview', label: 'Overview', icon: <BookOpen size={16} /> },
  { id: 'context', label: 'Context', icon: <FileText size={16} /> },
  { id: 'roster', label: 'Note Roster', icon: <Layers size={16} /> },
  { id: 'coverage', label: 'Coverage', icon: <Settings size={16} /> },
];

// ---------------------------------------------------------------------------
// Create Class Modal
// ---------------------------------------------------------------------------

function CreateClassModal({
  onClose,
  onCreate,
}: {
  onClose: () => void;
  onCreate: (name: string) => void;
}) {
  const [name, setName] = useState('');
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40">
      <div className="relative w-full max-w-sm rounded-xl bg-white p-6 shadow-xl">
        <button
          onClick={onClose}
          className="absolute right-4 top-4 text-slate-400 hover:text-slate-600"
        >
          <X size={20} />
        </button>
        <h2 className="mb-4 text-lg font-semibold text-slate-900">
          New Class
        </h2>
        <input
          autoFocus
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder="e.g. CS-101"
          className="mb-4 w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          onKeyDown={(e) => {
            if (e.key === 'Enter' && name.trim()) onCreate(name.trim());
          }}
        />
        <button
          onClick={() => name.trim() && onCreate(name.trim())}
          disabled={!name.trim()}
          className="flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700 disabled:opacity-50"
        >
          <Plus size={16} />
          Create
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab: Overview
// ---------------------------------------------------------------------------

function OverviewTab({ detail }: { detail: ClassDetail }) {
  return (
    <div className="space-y-6">
      {/* Rules */}
      <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="mb-3 text-sm font-semibold text-slate-700">
          Exam Rules
        </h3>
        {detail.rules.exam_expectations ? (
          <p className="text-sm text-slate-600 whitespace-pre-wrap">
            {detail.rules.exam_expectations}
          </p>
        ) : (
          <p className="text-sm text-slate-400 italic">
            No exam expectations set
          </p>
        )}
        {detail.rules.question_styles &&
          detail.rules.question_styles.length > 0 && (
            <div className="mt-3">
              <p className="mb-1 text-xs font-medium text-slate-500">
                Question Styles
              </p>
              <div className="flex flex-wrap gap-1.5">
                {detail.rules.question_styles.map((s) => (
                  <span
                    key={s}
                    className="rounded-full bg-indigo-50 px-2.5 py-0.5 text-xs font-medium text-indigo-700"
                  >
                    {s}
                  </span>
                ))}
              </div>
            </div>
          )}
        {detail.rules.notes && (
          <p className="mt-3 text-xs text-slate-500">{detail.rules.notes}</p>
        )}
      </div>

      {/* Syllabus */}
      <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <h3 className="mb-3 text-sm font-semibold text-slate-700">
          Syllabus Topics
        </h3>
        {detail.syllabus.topics.length === 0 ? (
          <p className="text-sm text-slate-400 italic">No topics defined</p>
        ) : (
          <div className="space-y-2">
            {detail.syllabus.topics.map((topic, i) => (
              <div
                key={i}
                className="flex items-start gap-3 rounded-lg border border-slate-100 px-3 py-2"
              >
                <div className="flex h-6 w-6 shrink-0 items-center justify-center rounded bg-slate-100 text-xs font-medium text-slate-600">
                  {topic.week ?? i + 1}
                </div>
                <div className="min-w-0">
                  <p className="text-sm font-medium text-slate-800">
                    {topic.title}
                  </p>
                  {topic.description && (
                    <p className="text-xs text-slate-500">
                      {topic.description}
                    </p>
                  )}
                  {topic.tags && topic.tags.length > 0 && (
                    <div className="mt-1 flex flex-wrap gap-1">
                      {topic.tags.map((t) => (
                        <span
                          key={t}
                          className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px] text-slate-500"
                        >
                          {t}
                        </span>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Stats */}
      <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div className="rounded-lg border border-slate-200 bg-white p-4 text-center shadow-sm">
          <p className="text-2xl font-semibold text-slate-900">
            {detail.syllabus.topics.length}
          </p>
          <p className="text-xs text-slate-500">Topics</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-white p-4 text-center shadow-sm">
          <p className="text-2xl font-semibold text-slate-900">
            {detail.roster.entries.length}
          </p>
          <p className="text-xs text-slate-500">Roster Entries</p>
        </div>
        <div className="rounded-lg border border-slate-200 bg-white p-4 text-center shadow-sm">
          <p className="text-2xl font-semibold text-slate-900">
            {Object.keys(detail.profiles).length}
          </p>
          <p className="text-xs text-slate-500">Profiles</p>
        </div>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab: Context
// ---------------------------------------------------------------------------

function ContextTab({
  detail,
  className: cls,
  onDetailRefresh,
}: {
  detail: ClassDetail;
  className: string;
  onDetailRefresh: () => Promise<void>;
}) {
  const [files, setFiles] = useState<string[]>(
    detail.context.context_files ?? [],
  );
  const [savedFiles, setSavedFiles] = useState<string[]>(
    detail.context.context_files ?? [],
  );
  const [newFile, setNewFile] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeProfile, setActiveProfile] = useState<string>(
    Object.keys(detail.profiles)[0] ?? '',
  );
  const [profileTexts, setProfileTexts] = useState<Record<string, string>>(
    () => ({ ...detail.profiles }),
  );
  const [savedProfileTexts, setSavedProfileTexts] = useState<Record<string, string>>(
    () => ({ ...detail.profiles }),
  );
  const [profileSaving, setProfileSaving] = useState(false);

  useEffect(() => {
    const nextFiles = detail.context.context_files ?? [];
    const nextProfiles = { ...detail.profiles };
    setFiles(nextFiles);
    setSavedFiles(nextFiles);
    setProfileTexts(nextProfiles);
    setSavedProfileTexts(nextProfiles);
    setError(null);
    const keys = Object.keys(detail.profiles);
    setActiveProfile((prev) => {
      if (keys.length === 0) return '';
      return keys.includes(prev) ? prev : keys[0];
    });
  }, [cls, detail]);

  const filesDirty = !arraysEqual(files, savedFiles);
  const activeProfileDirty =
    activeProfile !== ''
      && (profileTexts[activeProfile] ?? '') !== (savedProfileTexts[activeProfile] ?? '');
  const anyProfileDirty = Object.keys(profileTexts).some(
    (kind) => (profileTexts[kind] ?? '') !== (savedProfileTexts[kind] ?? ''),
  );

  const handleAddFile = () => {
    const trimmed = newFile.trim();
    if (!trimmed || files.includes(trimmed)) return;
    setFiles([...files, trimmed]);
    setNewFile('');
  };

  const handleRemoveFile = (f: string) =>
    setFiles(files.filter((x) => x !== f));

  const handleSaveFiles = async () => {
    setSaving(true);
    setError(null);
    try {
      await api.updateClassContext(cls, files);
      setSavedFiles([...files]);
      await onDetailRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save context files');
    } finally {
      setSaving(false);
    }
  };

  const handleSaveProfile = async (profileKind: string) => {
    if (!profileKind) return;
    setProfileSaving(true);
    setError(null);
    try {
      await api.updateProfileContext(
        cls,
        profileKind,
        profileTexts[profileKind] ?? '',
      );
      setSavedProfileTexts((prev) => ({
        ...prev,
        [profileKind]: profileTexts[profileKind] ?? '',
      }));
      await onDetailRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save profile context');
    } finally {
      setProfileSaving(false);
    }
  };

  const handleProfileTabClick = async (kind: string) => {
    if (kind === activeProfile) return;
    if (activeProfileDirty && !profileSaving) {
      await handleSaveProfile(activeProfile);
    }
    setActiveProfile(kind);
  };

  return (
    <div className="space-y-6">
      <div className="rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm text-sky-900">
        Class context sharpens generation quality by anchoring the model to your files and profile-specific instructions. Changes marked as unsaved stay local until you save.
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      {/* Context Files */}
      <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-slate-700">Context Files</h3>
          <span className={`text-xs font-medium ${filesDirty ? 'text-amber-600' : 'text-emerald-600'}`}>
            {filesDirty ? 'Unsaved changes' : 'Saved'}
          </span>
        </div>
        <div className="space-y-2">
          {files.map((f) => (
            <div
              key={f}
              className="flex items-center justify-between rounded-lg border border-slate-100 px-3 py-2 text-sm"
            >
              <span className="truncate font-mono text-xs text-slate-600">
                {f}
              </span>
              <button
                onClick={() => handleRemoveFile(f)}
                className="shrink-0 text-slate-400 hover:text-red-500"
              >
                <Trash2 size={14} />
              </button>
            </div>
          ))}
          {files.length === 0 && (
            <p className="text-sm text-slate-400 italic">
              No context files added
            </p>
          )}
        </div>
        <div className="mt-3 flex gap-2">
          <input
            value={newFile}
            onChange={(e) => setNewFile(e.target.value)}
            placeholder="path/to/file.md"
            className="flex-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            onKeyDown={(e) => {
              if (e.key === 'Enter') handleAddFile();
            }}
          />
          <button
            onClick={handleAddFile}
            className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <Plus size={14} />
          </button>
        </div>
        <button
          onClick={handleSaveFiles}
          disabled={saving || !filesDirty}
          className="mt-3 flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700 disabled:opacity-50"
        >
          <Save size={14} />
          {saving ? 'Saving...' : filesDirty ? 'Save Context Files' : 'Context Files Saved'}
        </button>
      </div>

      {/* Profile Contexts */}
      <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
        <div className="mb-3 flex items-center justify-between gap-3">
          <h3 className="text-sm font-semibold text-slate-700">Profile Contexts</h3>
          <span className={`text-xs font-medium ${anyProfileDirty ? 'text-amber-600' : 'text-emerald-600'}`}>
            {anyProfileDirty ? 'Unsaved profile edits' : 'All profiles saved'}
          </span>
        </div>
        {Object.keys(detail.profiles).length === 0 ? (
          <p className="text-sm text-slate-400 italic">No profiles defined</p>
        ) : (
          <>
            <div className="mb-3 flex gap-1 border-b border-slate-200">
              {Object.keys(detail.profiles).map((kind) => (
                <button
                  key={kind}
                  onClick={() => void handleProfileTabClick(kind)}
                  className={`border-b-2 px-3 py-2 text-sm font-medium transition-colors duration-150 ${
                    activeProfile === kind
                      ? 'border-indigo-600 text-indigo-600'
                      : 'border-transparent text-slate-500 hover:text-slate-700'
                  }`}
                >
                  {kind}
                  {(profileTexts[kind] ?? '') !== (savedProfileTexts[kind] ?? '') && (
                    <span className="ml-1 text-amber-600">*</span>
                  )}
                </button>
              ))}
            </div>
            <textarea
              value={profileTexts[activeProfile] ?? ''}
              onChange={(e) =>
                setProfileTexts({
                  ...profileTexts,
                  [activeProfile]: e.target.value,
                })
              }
              rows={8}
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm font-mono focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
            <button
              onClick={() => void handleSaveProfile(activeProfile)}
              disabled={profileSaving || !activeProfileDirty}
              className="mt-2 flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700 disabled:opacity-50"
            >
              <Save size={14} />
              {profileSaving
                ? 'Saving...'
                : activeProfileDirty
                  ? `Save ${activeProfile} Profile`
                  : `${activeProfile} Profile Saved`}
            </button>
          </>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab: Roster
// ---------------------------------------------------------------------------

function RosterTab({
  detail,
  className: cls,
  onDetailRefresh,
}: {
  detail: ClassDetail;
  className: string;
  onDetailRefresh: () => Promise<void>;
}) {
  const [entries, setEntries] = useState<RosterEntry[]>(() => [
    ...detail.roster.entries,
  ]);
  const [savedEntries, setSavedEntries] = useState<RosterEntry[]>(() => [
    ...detail.roster.entries,
  ]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [showAdd, setShowAdd] = useState(false);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [newEntry, setNewEntry] = useState<RosterEntry>({
    label: '',
    source_pattern: '',
    tags: [],
    week: undefined,
    order: undefined,
  });
  const [tagInput, setTagInput] = useState('');

  useEffect(() => {
    setEntries([...detail.roster.entries]);
    setSavedEntries([...detail.roster.entries]);
    setError(null);
  }, [cls, detail]);

  const entriesDirty =
    JSON.stringify(normalizeRosterEntries(entries))
    !== JSON.stringify(normalizeRosterEntries(savedEntries));

  const handleAdd = () => {
    if (!newEntry.label.trim()) return;
    setEntries([
      ...entries,
      {
        ...newEntry,
        label: newEntry.label.trim(),
        source_pattern: newEntry.source_pattern?.trim() || undefined,
      },
    ]);
    setNewEntry({
      label: '',
      source_pattern: '',
      tags: [],
      week: undefined,
      order: undefined,
    });
    setTagInput('');
    setShowAdd(false);
  };

  const handleRemove = (idx: number) =>
    setEntries(entries.filter((_, i) => i !== idx));

  const handleMove = (idx: number, dir: -1 | 1) => {
    const target = idx + dir;
    if (target < 0 || target >= entries.length) return;
    const next = [...entries];
    [next[idx], next[target]] = [next[target], next[idx]];
    setEntries(next);
  };

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const normalized = normalizeRosterEntries(entries);
      await api.updateRoster(cls, normalized);
      setEntries(normalized);
      setSavedEntries(normalized);
      await onDetailRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save roster');
    } finally {
      setSaving(false);
    }
  };

  const handleSmartOrder = () => {
    setEntries((prev) => smartOrderRosterEntries(prev));
  };

  const handleDropOn = (targetIndex: number) => {
    if (dragIndex == null || dragIndex === targetIndex) return;
    setEntries((prev) => {
      const next = [...prev];
      const [dragged] = next.splice(dragIndex, 1);
      next.splice(targetIndex, 0, dragged);
      return normalizeRosterEntries(next);
    });
    setDragIndex(null);
  };

  const handleTagKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      const tag = tagInput.trim().replace(/,$/, '');
      if (tag && !newEntry.tags?.includes(tag)) {
        setNewEntry({
          ...newEntry,
          tags: [...(newEntry.tags ?? []), tag],
        });
      }
      setTagInput('');
    }
  };

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm text-sky-900">
        The roster controls note sequencing for studying and quiz generation. Drag to reorder manually, or use Smart Order to infer timeline order from dates first, then numbers.
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-semibold text-slate-700">
            Note Roster ({entries.length} entries)
          </h3>
          <p className={`text-xs font-medium ${entriesDirty ? 'text-amber-600' : 'text-emerald-600'}`}>
            {entriesDirty ? 'Unsaved order/entry changes' : 'Saved'}
          </p>
        </div>
        <div className="flex gap-2">
          <button
            onClick={handleSmartOrder}
            className="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <WandSparkles size={14} />
            Smart Order
          </button>
          <button
            onClick={() => setShowAdd(!showAdd)}
            className="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <Plus size={14} />
            Add Entry
          </button>
          <button
            onClick={handleSave}
            disabled={saving || !entriesDirty}
            className="flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white transition-colors duration-150 hover:bg-indigo-700 disabled:opacity-50"
          >
            <Save size={14} />
            {saving ? 'Saving...' : entriesDirty ? 'Save' : 'Saved'}
          </button>
        </div>
      </div>

      {/* Add form */}
      {showAdd && (
        <div className="rounded-lg border border-indigo-200 bg-indigo-50/50 p-4">
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">
                Label *
              </label>
              <input
                value={newEntry.label}
                onChange={(e) =>
                  setNewEntry({ ...newEntry, label: e.target.value })
                }
                className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">
                Source Pattern
              </label>
              <input
                value={newEntry.source_pattern ?? ''}
                onChange={(e) =>
                  setNewEntry({ ...newEntry, source_pattern: e.target.value })
                }
                className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            </div>
            <div>
              <label className="mb-1 block text-xs font-medium text-slate-600">
                Tags (enter to add)
              </label>
              <div className="flex flex-wrap gap-1 rounded-lg border border-slate-300 bg-white px-2.5 py-1.5">
                {newEntry.tags?.map((t) => (
                  <span
                    key={t}
                    className="inline-flex items-center gap-0.5 rounded bg-indigo-100 px-1.5 py-0.5 text-xs text-indigo-700"
                  >
                    {t}
                    <button
                      onClick={() =>
                        setNewEntry({
                          ...newEntry,
                          tags: newEntry.tags?.filter((x) => x !== t),
                        })
                      }
                      className="hover:text-indigo-900"
                    >
                      <X size={10} />
                    </button>
                  </span>
                ))}
                <input
                  value={tagInput}
                  onChange={(e) => setTagInput(e.target.value)}
                  onKeyDown={handleTagKeyDown}
                  className="min-w-[60px] flex-1 border-none bg-transparent text-sm outline-none"
                  placeholder={
                    newEntry.tags?.length ? '' : 'Type and press Enter'
                  }
                />
              </div>
            </div>
            <div className="flex gap-3">
              <div className="flex-1">
                <label className="mb-1 block text-xs font-medium text-slate-600">
                  Week
                </label>
                <input
                  type="number"
                  value={newEntry.week ?? ''}
                  onChange={(e) =>
                    setNewEntry({
                      ...newEntry,
                      week: e.target.value ? Number(e.target.value) : undefined,
                    })
                  }
                  className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
              <div className="flex-1">
                <label className="mb-1 block text-xs font-medium text-slate-600">
                  Order
                </label>
                <input
                  type="number"
                  value={newEntry.order ?? ''}
                  onChange={(e) =>
                    setNewEntry({
                      ...newEntry,
                      order: e.target.value
                        ? Number(e.target.value)
                        : undefined,
                    })
                  }
                  className="w-full rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                />
              </div>
            </div>
          </div>
          <div className="mt-3 flex justify-end gap-2">
            <button
              onClick={() => setShowAdd(false)}
              className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm text-slate-600 hover:bg-slate-50"
            >
              Cancel
            </button>
            <button
              onClick={handleAdd}
              disabled={!newEntry.label.trim()}
              className="rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
            >
              Add
            </button>
          </div>
        </div>
      )}

      {/* Entries list */}
      <div className="space-y-1">
        {entries.map((entry, i) => (
          <div
            key={`${entry.label}-${i}`}
            draggable
            onDragStart={() => setDragIndex(i)}
            onDragEnd={() => setDragIndex(null)}
            onDragOver={(e) => e.preventDefault()}
            onDrop={() => handleDropOn(i)}
            className={`flex items-center gap-2 rounded-lg border bg-white px-3 py-2 shadow-sm transition-colors duration-100 ${
              dragIndex === i
                ? 'border-indigo-300 bg-indigo-50/40'
                : 'border-slate-100 hover:border-slate-200'
            }`}
          >
            <span className="shrink-0 cursor-grab text-slate-400" title="Drag to reorder">
              <GripVertical size={14} />
            </span>
            <div className="flex shrink-0 flex-col gap-0.5">
              <button
                onClick={() => handleMove(i, -1)}
                disabled={i === 0}
                className="text-slate-400 hover:text-slate-600 disabled:opacity-30"
              >
                <ArrowUp size={12} />
              </button>
              <button
                onClick={() => handleMove(i, 1)}
                disabled={i === entries.length - 1}
                className="text-slate-400 hover:text-slate-600 disabled:opacity-30"
              >
                <ArrowDown size={12} />
              </button>
            </div>
            <div className="min-w-0 flex-1">
              <p className="text-sm font-medium text-slate-800">
                {entry.label}
              </p>
              <div className="flex flex-wrap items-center gap-2 text-xs text-slate-500">
                {entry.source_pattern && (
                  <span className="font-mono">{entry.source_pattern}</span>
                )}
                {entry.week != null && <span>Week {entry.week}</span>}
                {entry.tags?.map((t) => (
                  <span
                    key={t}
                    className="rounded bg-slate-100 px-1.5 py-0.5 text-[10px]"
                  >
                    {t}
                  </span>
                ))}
              </div>
            </div>
            <button
              onClick={() => handleRemove(i)}
              className="shrink-0 text-slate-400 hover:text-red-500"
            >
              <Trash2 size={14} />
            </button>
          </div>
        ))}
        {entries.length === 0 && (
          <p className="py-8 text-center text-sm text-slate-400">
            No roster entries
          </p>
        )}
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tab: Coverage
// ---------------------------------------------------------------------------

function CoverageTab({
  detail,
  className: cls,
  onDetailRefresh,
}: {
  detail: ClassDetail;
  className: string;
  onDetailRefresh: () => Promise<void>;
}) {
  const profileKinds = useMemo(() => {
    const keys = Object.keys(detail.profiles);
    return keys.length > 0 ? keys : ['quiz', 'exam', 'focused'];
  }, [detail.profiles]);

  const [activeKind, setActiveKind] = useState(Object.keys(detail.coverage)[0] ?? '');
  const [scopes, setScopes] = useState<Record<string, CoverageScope | null>>(
    () => ({ ...detail.coverage }),
  );
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [pendingKind, setPendingKind] = useState(profileKinds[0] ?? 'quiz');
  const [dirtyKinds, setDirtyKinds] = useState<Record<string, boolean>>({});

  const coverageKinds = useMemo(() => {
    const localKinds = Object.keys(scopes);
    const ordered: string[] = [];
    for (const kind of profileKinds) {
      if (localKinds.includes(kind)) ordered.push(kind);
    }
    for (const kind of localKinds) {
      if (!ordered.includes(kind)) ordered.push(kind);
    }
    return ordered;
  }, [profileKinds, scopes]);

  useEffect(() => {
    setScopes({ ...detail.coverage });
    setDirtyKinds({});
    setError(null);
    const kinds = Object.keys(detail.coverage);
    setActiveKind((prev) => {
      if (kinds.length === 0) return '';
      return kinds.includes(prev) ? prev : kinds[0];
    });
  }, [cls, detail]);

  useEffect(() => {
    if (!pendingKind && profileKinds.length > 0) {
      setPendingKind(profileKinds[0]);
    }
  }, [pendingKind, profileKinds]);

  const scope = scopes[activeKind] ?? null;

  const updateScope = (updated: CoverageScope) => {
    setScopes({ ...scopes, [activeKind]: updated });
    setDirtyKinds((prev) => ({ ...prev, [activeKind]: true }));
  };

  const addScope = () => {
    const kind = pendingKind.trim();
    if (!kind) return;
    if (scopes[kind]) {
      setActiveKind(kind);
      return;
    }
    setScopes((prev) => ({
      ...prev,
      [kind]: {
        class: cls,
        kind,
        exclude_unmatched: false,
        groups: [],
      },
    }));
    setDirtyKinds((prev) => ({ ...prev, [kind]: true }));
    setActiveKind(kind);
  };

  const handleSave = async () => {
    if (!scope) return;
    setSaving(true);
    setError(null);
    try {
      await api.updateCoverage(cls, activeKind, scope);
      setDirtyKinds((prev) => ({ ...prev, [activeKind]: false }));
      await onDetailRefresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save coverage scope');
    } finally {
      setSaving(false);
    }
  };

  const addGroup = () => {
    if (!scope) return;
    updateScope({
      ...scope,
      groups: [...scope.groups, { labels: [], tags: [], weight: 1 }],
    });
  };

  const removeGroup = (idx: number) => {
    if (!scope) return;
    updateScope({
      ...scope,
      groups: scope.groups.filter((_, i) => i !== idx),
    });
  };

  const updateGroup = (idx: number, patch: Partial<CoverageScope['groups'][number]>) => {
    if (!scope) return;
    updateScope({
      ...scope,
      groups: scope.groups.map((g, i) => (i === idx ? { ...g, ...patch } : g)),
    });
  };

  const pieData = useMemo(() => {
    if (!scope) return [];
    return scope.groups.map((g, i) => ({
      name: g.labels?.[0] ?? g.tags?.[0] ?? `Group ${i + 1}`,
      value: g.weight,
    }));
  }, [scope]);

  return (
    <div className="space-y-4">
      <div className="rounded-lg border border-sky-200 bg-sky-50 p-3 text-sm text-sky-900">
        Coverage scopes weight what material each profile emphasizes. Create scopes to focus quizzes and exams on the right units, tags, or source patterns.
      </div>

      {error && (
        <div className="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
          {error}
        </div>
      )}

      <div className="rounded-lg border border-slate-200 bg-white p-3 shadow-sm">
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={pendingKind}
            onChange={(e) => setPendingKind(e.target.value)}
            className="rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
          >
            {profileKinds.map((kind) => (
              <option key={kind} value={kind}>{kind}</option>
            ))}
          </select>
          <button
            onClick={addScope}
            className="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
          >
            <Plus size={14} />
            Add Scope
          </button>
        </div>
      </div>

      {coverageKinds.length === 0 ? (
        <p className="text-sm text-slate-400 italic">
          No coverage scopes defined yet. Choose a profile kind above and click Add Scope.
        </p>
      ) : (
        <>
          {/* Kind tabs */}
          <div className="flex gap-1 border-b border-slate-200">
            {coverageKinds.map((kind) => (
              <button
                key={kind}
                onClick={() => setActiveKind(kind)}
                className={`border-b-2 px-3 py-2 text-sm font-medium transition-colors duration-150 ${
                  activeKind === kind
                    ? 'border-indigo-600 text-indigo-600'
                    : 'border-transparent text-slate-500 hover:text-slate-700'
                }`}
              >
                {kind}
                {dirtyKinds[kind] && <span className="ml-1 text-amber-600">*</span>}
              </button>
            ))}
          </div>

          {scope === null ? (
            <p className="text-sm text-slate-400 italic">
              No scope configured for {activeKind}
            </p>
          ) : (
            <div className="grid grid-cols-1 gap-6 lg:grid-cols-3">
              {/* Groups editor */}
              <div className="lg:col-span-2 space-y-3">
                <div className="flex items-center justify-between">
                  <label className="flex items-center gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={scope.exclude_unmatched ?? false}
                      onChange={(e) =>
                        updateScope({
                          ...scope,
                          exclude_unmatched: e.target.checked,
                        })
                      }
                      className="rounded border-slate-300 text-indigo-600 focus:ring-indigo-500"
                    />
                    <span className="text-slate-600">Exclude unmatched</span>
                  </label>
                  <div className="flex gap-2">
                    <button
                      onClick={addGroup}
                      className="flex items-center gap-1 rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50"
                    >
                      <Plus size={14} />
                      Add Group
                    </button>
                    <button
                      onClick={handleSave}
                      disabled={saving || !dirtyKinds[activeKind]}
                      className="flex items-center gap-2 rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 disabled:opacity-50"
                    >
                      <Save size={14} />
                      {saving ? 'Saving...' : dirtyKinds[activeKind] ? 'Save' : 'Saved'}
                    </button>
                  </div>
                </div>

                {scope.groups.map((group, gi) => (
                  <div
                    key={gi}
                    className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm"
                  >
                    <div className="mb-3 flex items-center justify-between">
                      <span className="text-sm font-medium text-slate-700">
                        Group {gi + 1}
                      </span>
                      <button
                        onClick={() => removeGroup(gi)}
                        className="text-slate-400 hover:text-red-500"
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                    <div className="space-y-3">
                      <MultiInput
                        label="Labels"
                        values={group.labels ?? []}
                        onChange={(labels) => updateGroup(gi, { labels })}
                      />
                      <MultiInput
                        label="Source Patterns"
                        values={group.source_patterns ?? []}
                        onChange={(source_patterns) =>
                          updateGroup(gi, { source_patterns })
                        }
                      />
                      <MultiInput
                        label="Tags"
                        values={group.tags ?? []}
                        onChange={(tags) => updateGroup(gi, { tags })}
                      />
                      <div>
                        <label className="mb-1 block text-xs font-medium text-slate-600">
                          Weight
                        </label>
                        <input
                          type="number"
                          min={0}
                          step={0.1}
                          value={group.weight}
                          onChange={(e) =>
                            updateGroup(gi, {
                              weight: Number(e.target.value),
                            })
                          }
                          className="w-32 rounded-lg border border-slate-300 px-2.5 py-1.5 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
                        />
                      </div>
                    </div>
                  </div>
                ))}
                {scope.groups.length === 0 && (
                  <p className="py-4 text-center text-sm text-slate-400">
                    No groups defined
                  </p>
                )}
              </div>

              {/* Pie chart */}
              {pieData.length > 0 && (
                <div className="rounded-lg border border-slate-200 bg-white p-4 shadow-sm">
                  <h4 className="mb-2 text-xs font-semibold text-slate-500">
                    Weight Distribution
                  </h4>
                  <ResponsiveContainer width="100%" height={220}>
                    <PieChart>
                      <Pie
                        data={pieData}
                        dataKey="value"
                        nameKey="name"
                        cx="50%"
                        cy="50%"
                        outerRadius={80}
                        label={({ name }: { name?: string }) => name ?? ''}
                      >
                        {pieData.map((_, i) => (
                          <Cell
                            key={i}
                            fill={PIE_COLORS[i % PIE_COLORS.length]}
                          />
                        ))}
                      </Pie>
                      <Tooltip />
                    </PieChart>
                  </ResponsiveContainer>
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Multi-input helper
// ---------------------------------------------------------------------------

function MultiInput({
  label,
  values,
  onChange,
}: {
  label: string;
  values: string[];
  onChange: (values: string[]) => void;
}) {
  const [input, setInput] = useState('');

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      const val = input.trim().replace(/,$/, '');
      if (val && !values.includes(val)) {
        onChange([...values, val]);
      }
      setInput('');
    }
    if (e.key === 'Backspace' && !input && values.length > 0) {
      onChange(values.slice(0, -1));
    }
  };

  return (
    <div>
      <label className="mb-1 block text-xs font-medium text-slate-600">
        {label}
      </label>
      <div className="flex flex-wrap gap-1 rounded-lg border border-slate-300 bg-white px-2.5 py-1.5">
        {values.map((v) => (
          <span
            key={v}
            className="inline-flex items-center gap-0.5 rounded bg-slate-100 px-1.5 py-0.5 text-xs text-slate-700"
          >
            {v}
            <button
              onClick={() => onChange(values.filter((x) => x !== v))}
              className="hover:text-red-500"
            >
              <X size={10} />
            </button>
          </span>
        ))}
        <input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          className="min-w-[80px] flex-1 border-none bg-transparent text-sm outline-none"
          placeholder={values.length === 0 ? 'Type and press Enter' : ''}
        />
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Main Page
// ---------------------------------------------------------------------------

export default function ClassesPage() {
  const [classes, setClasses] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  const [detail, setDetail] = useState<ClassDetail | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState<TabId>('overview');

  const loadClasses = useCallback(async () => {
    setLoading(true);
    try {
      const list = await api.fetchClasses();
      setClasses(list);
      if (list.length > 0 && !selected) setSelected(list[0]);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load classes');
    } finally {
      setLoading(false);
    }
  }, [selected]);

  const loadDetail = useCallback(async (name: string) => {
    setDetailLoading(true);
    try {
      setDetail(await api.fetchClassDetail(name));
    } catch {
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  }, []);

  const refreshSelectedDetail = useCallback(async () => {
    if (!selected) return;
    await loadDetail(selected);
  }, [selected, loadDetail]);

  useEffect(() => {
    void loadClasses();
  }, [loadClasses]);

  useEffect(() => {
    if (selected) void loadDetail(selected);
  }, [selected, loadDetail]);

  const handleCreate = async (name: string) => {
    setShowCreate(false);
    try {
      await api.createClass(name);
      await loadClasses();
      setSelected(name);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create class');
    }
  };

  const filteredClasses = useMemo(() => {
    if (!searchQuery.trim()) return classes;
    const q = searchQuery.toLowerCase();
    return classes.filter((c) => c.toLowerCase().includes(q));
  }, [classes, searchQuery]);

  if (loading) {
    return (
      <div className="flex h-96 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-indigo-600" />
      </div>
    );
  }

  return (
    <div className="mx-auto flex max-w-7xl gap-6 p-6">
      {/* Sidebar */}
      <div className="w-64 shrink-0">
        <div className="sticky top-6 space-y-3">
          <div className="flex items-center justify-between">
            <h1 className="text-lg font-bold text-slate-900">Classes</h1>
            <button
              onClick={() => setShowCreate(true)}
              className="flex h-8 w-8 items-center justify-center rounded-lg bg-indigo-600 text-white hover:bg-indigo-700"
            >
              <Plus size={16} />
            </button>
          </div>

          <div className="relative">
            <Search
              size={14}
              className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
            />
            <input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search classes..."
              className="w-full rounded-lg border border-slate-300 py-1.5 pl-8 pr-3 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>

          <div className="space-y-1">
            {filteredClasses.map((c) => (
              <button
                key={c}
                onClick={() => setSelected(c)}
                className={`flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm font-medium transition-colors duration-150 ${
                  selected === c
                    ? 'bg-indigo-50 text-indigo-700'
                    : 'text-slate-600 hover:bg-slate-100'
                }`}
              >
                <ChevronRight
                  size={14}
                  className={
                    selected === c ? 'text-indigo-500' : 'text-slate-400'
                  }
                />
                {c}
              </button>
            ))}
            {filteredClasses.length === 0 && (
              <p className="py-4 text-center text-sm text-slate-400">
                {classes.length === 0
                  ? 'No classes yet'
                  : 'No matches'}
              </p>
            )}
          </div>
        </div>
      </div>

      {/* Main content */}
      <div className="min-w-0 flex-1">
        {error && (
          <div className="mb-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700">
            {error}
          </div>
        )}

        {!selected ? (
          <div className="flex h-64 items-center justify-center text-sm text-slate-400">
            Select a class to view details
          </div>
        ) : detailLoading ? (
          <div className="flex h-64 items-center justify-center">
            <Loader2 className="h-6 w-6 animate-spin text-indigo-600" />
          </div>
        ) : !detail ? (
          <div className="flex h-64 items-center justify-center text-sm text-slate-400">
            Could not load class details
          </div>
        ) : (
          <>
            <div className="mb-4">
              <h2 className="text-xl font-bold text-slate-900">
                {detail.name}
              </h2>
            </div>

            {/* Tabs */}
            <div className="mb-6 flex gap-1 border-b border-slate-200">
              {TABS.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`flex items-center gap-1.5 border-b-2 px-4 py-2.5 text-sm font-medium transition-colors duration-150 ${
                    activeTab === tab.id
                      ? 'border-indigo-600 text-indigo-600'
                      : 'border-transparent text-slate-500 hover:text-slate-700'
                  }`}
                >
                  {tab.icon}
                  {tab.label}
                </button>
              ))}
            </div>

            <div className={activeTab === 'overview' ? '' : 'hidden'}>
              <OverviewTab detail={detail} />
            </div>
            <div className={activeTab === 'context' ? '' : 'hidden'}>
              <ContextTab
                detail={detail}
                className={selected}
                onDetailRefresh={refreshSelectedDetail}
              />
            </div>
            <div className={activeTab === 'roster' ? '' : 'hidden'}>
              <RosterTab
                detail={detail}
                className={selected}
                onDetailRefresh={refreshSelectedDetail}
              />
            </div>
            <div className={activeTab === 'coverage' ? '' : 'hidden'}>
              <CoverageTab
                detail={detail}
                className={selected}
                onDetailRefresh={refreshSelectedDetail}
              />
            </div>
          </>
        )}
      </div>

      {/* Create Modal */}
      {showCreate && (
        <CreateClassModal
          onClose={() => setShowCreate(false)}
          onCreate={handleCreate}
        />
      )}
    </div>
  );
}
