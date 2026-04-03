import { useCallback, useEffect, useState } from 'react';
import {
  CheckSquare,
  ChevronRight,
  File,
  Folder,
  FolderOpen,
  Loader2,
  Square,
  Trash2,
  Upload,
} from 'lucide-react';
import { Modal } from './Modal';
import { browseFiles, streamIngest, fetchClasses } from '../api/client';
import type { BrowseEntry, IngestSSEEvent } from '../types';

type IngestStep = 'configure' | 'browse' | 'running' | 'done';

interface IngestEvent {
  label: string;
  detail: string;
  done: boolean;
  error?: string;
}

interface IngestResult {
  notes: number;
  sectionsAdded: number;
  componentsAdded: number;
  usageEvents: number;
}

interface IngestModalProps {
  open: boolean;
  onClose: () => void;
  onDone: () => void;
}

export function IngestModal({ open, onClose, onDone }: IngestModalProps) {
  const [step, setStep] = useState<IngestStep>('configure');
  const [folderPath, setFolderPath] = useState('');
  const [className, setClassName] = useState('');
  const [classes, setClasses] = useState<string[]>([]);
  const [cleanBefore, setCleanBefore] = useState(false);
  const [selectedFiles, setSelectedFiles] = useState<string[]>([]);

  // Browser state
  const [browseDir, setBrowseDir] = useState('');
  const [entries, setEntries] = useState<BrowseEntry[]>([]);
  const [browseLoading, setBrowseLoading] = useState(false);
  const [browseError, setBrowseError] = useState<string | null>(null);

  // Progress state
  const [events, setEvents] = useState<IngestEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<IngestResult | null>(null);

  useEffect(() => {
    if (open) {
      fetchClasses().then(setClasses).catch(() => {});
      setStep('configure');
      setFolderPath('');
      setClassName('');
      setCleanBefore(false);
      setSelectedFiles([]);
      setEvents([]);
      setError(null);
      setResult(null);
    }
  }, [open]);

  const loadDir = useCallback(async (dir?: string) => {
    setBrowseLoading(true);
    setBrowseError(null);
    try {
      const resp = await browseFiles(dir);
      setBrowseDir(resp.dir);
      setEntries(resp.entries);
    } catch (err) {
      setBrowseError(err instanceof Error ? err.message : 'Failed to browse');
    } finally {
      setBrowseLoading(false);
    }
  }, []);

  const openBrowser = useCallback(() => {
    setStep('browse');
    loadDir(folderPath || undefined);
  }, [folderPath, loadDir]);

  const toggleFile = useCallback((path: string) => {
    setSelectedFiles((prev) =>
      prev.includes(path) ? prev.filter((p) => p !== path) : [...prev, path],
    );
  }, []);

  const selectAllFiles = useCallback(() => {
    const fileEntries = entries.filter((e) => !e.is_dir);
    const allPaths = fileEntries.map((e) => e.path);
    const allSelected = allPaths.every((p) => selectedFiles.includes(p));
    if (allSelected) {
      setSelectedFiles((prev) => prev.filter((p) => !allPaths.includes(p)));
    } else {
      setSelectedFiles((prev) => [...new Set([...prev, ...allPaths])]);
    }
  }, [entries, selectedFiles]);

  const confirmBrowse = useCallback(() => {
    if (selectedFiles.length === 0) {
      setFolderPath(browseDir);
    }
    setStep('configure');
  }, [selectedFiles, browseDir]);

  const handleIngest = async () => {
    setStep('running');
    setEvents([]);
    setError(null);
    setResult(null);

    try {
      await streamIngest(
        {
          path: selectedFiles.length > 0 ? undefined : folderPath,
          files: selectedFiles.length > 0 ? selectedFiles : undefined,
          class: className || undefined,
          clean: cleanBefore,
        },
        (e: IngestSSEEvent) => {
          if (e.type === 'progress') {
            setEvents((prev) => {
              const label = e.label ?? '';
              const detail = e.detail ?? '';
              const done = e.done === 'true';
              const existing = prev.findIndex((ev) => ev.label === label);
              if (existing >= 0) {
                const updated = [...prev];
                updated[existing] = { label, detail, done, error: e.error };
                return updated;
              }
              return [...prev, { label, detail, done, error: e.error }];
            });
          } else if (e.type === 'done') {
            setResult({
              notes: e.notes ?? 0,
              sectionsAdded: e.sections_added ?? 0,
              componentsAdded: e.components_added ?? 0,
              usageEvents: e.usage_events ?? 0,
            });
            setStep('done');
          } else if (e.type === 'error') {
            setError(e.error ?? 'Unknown error');
            setStep('done');
          }
        },
      );
      if (step !== 'done') setStep('done');
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ingestion failed');
      setStep('done');
    }
  };

  const canStart = selectedFiles.length > 0 || folderPath.trim() !== '';

  return (
    <Modal open={open} onClose={step === 'running' ? () => {} : onClose} title="Ingest Notes">
      {step === 'configure' && (
        <div className="space-y-4">
          {/* Folder path */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Folder Path
            </label>
            <div className="flex gap-2">
              <input
                type="text"
                value={folderPath}
                onChange={(e) => setFolderPath(e.target.value)}
                placeholder="./notes or /path/to/notes"
                className="flex-1 rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
              <button
                onClick={openBrowser}
                className="flex items-center gap-1.5 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
              >
                <FolderOpen className="h-4 w-4" />
                Browse
              </button>
            </div>
          </div>

          {/* Selected files summary */}
          {selectedFiles.length > 0 && (
            <div className="rounded-lg border border-indigo-200 bg-indigo-50 p-3">
              <div className="flex items-center justify-between mb-1.5">
                <p className="text-sm font-medium text-indigo-700">
                  {selectedFiles.length} file{selectedFiles.length !== 1 ? 's' : ''} selected
                </p>
                <button
                  onClick={() => setSelectedFiles([])}
                  className="text-xs text-indigo-500 hover:text-indigo-700"
                >
                  Clear
                </button>
              </div>
              <div className="max-h-24 overflow-y-auto space-y-0.5">
                {selectedFiles.map((f) => (
                  <p key={f} className="text-xs text-indigo-600 font-mono truncate">
                    {f.split('/').pop()}
                  </p>
                ))}
              </div>
            </div>
          )}

          {/* Class */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Class <span className="text-slate-400 font-normal">(optional)</span>
            </label>
            {classes.length > 0 ? (
              <select
                value={className}
                onChange={(e) => setClassName(e.target.value)}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              >
                <option value="">Auto-detect</option>
                {classes.map((c) => (
                  <option key={c} value={c}>{c}</option>
                ))}
              </select>
            ) : (
              <input
                type="text"
                value={className}
                onChange={(e) => setClassName(e.target.value)}
                placeholder="class name (optional)"
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              />
            )}
          </div>

          {/* Clean toggle */}
          <label className="flex items-center gap-2 cursor-pointer">
            <button
              onClick={() => setCleanBefore(!cleanBefore)}
              className="shrink-0"
            >
              {cleanBefore ? (
                <CheckSquare className="h-4 w-4 text-red-600" />
              ) : (
                <Square className="h-4 w-4 text-slate-300" />
              )}
            </button>
            <span className="text-sm text-slate-700">
              Clean existing data before ingesting
            </span>
            {cleanBefore && (
              <span className="flex items-center gap-1 rounded-full bg-red-100 px-2 py-0.5 text-xs font-medium text-red-700">
                <Trash2 className="h-3 w-3" />
                Destructive
              </span>
            )}
          </label>

          {/* Start button */}
          <button
            onClick={handleIngest}
            disabled={!canStart}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-indigo-700 disabled:opacity-50"
          >
            <Upload className="h-4 w-4" />
            Start Ingestion
          </button>
        </div>
      )}

      {step === 'browse' && (
        <div className="space-y-3">
          {/* Current directory */}
          <div className="flex items-center gap-2 rounded-lg bg-slate-100 px-3 py-2">
            <Folder className="h-4 w-4 text-slate-500 shrink-0" />
            <p className="text-sm font-mono text-slate-700 truncate">{browseDir}</p>
          </div>

          {browseError && (
            <p className="text-sm text-red-600">{browseError}</p>
          )}

          {/* File list */}
          {browseLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="h-5 w-5 animate-spin text-indigo-600" />
            </div>
          ) : (
            <>
              {/* Select all files button */}
              {entries.some((e) => !e.is_dir) && (
                <button
                  onClick={selectAllFiles}
                  className="text-xs font-medium text-indigo-600 hover:text-indigo-700"
                >
                  {entries.filter((e) => !e.is_dir).every((e) => selectedFiles.includes(e.path))
                    ? 'Deselect all files'
                    : 'Select all files'}
                </button>
              )}

              <div className="max-h-64 overflow-y-auto rounded-lg border border-slate-200">
                {entries.length === 0 ? (
                  <p className="p-4 text-center text-sm text-slate-400">
                    No files or directories found
                  </p>
                ) : (
                  <ul className="divide-y divide-slate-100">
                    {entries.map((entry) => (
                      <li key={entry.path}>
                        {entry.is_dir ? (
                          <button
                            onClick={() => loadDir(entry.path)}
                            className="flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm hover:bg-slate-50 transition-colors"
                          >
                            {entry.name === '..' ? (
                              <ChevronRight className="h-4 w-4 text-slate-400 rotate-180" />
                            ) : (
                              <Folder className="h-4 w-4 text-amber-500" />
                            )}
                            <span className="text-slate-700">{entry.name}</span>
                          </button>
                        ) : (
                          <button
                            onClick={() => toggleFile(entry.path)}
                            className={`flex w-full items-center gap-2.5 px-3 py-2 text-left text-sm transition-colors ${
                              selectedFiles.includes(entry.path)
                                ? 'bg-indigo-50'
                                : 'hover:bg-slate-50'
                            }`}
                          >
                            {selectedFiles.includes(entry.path) ? (
                              <CheckSquare className="h-4 w-4 text-indigo-600" />
                            ) : (
                              <Square className="h-4 w-4 text-slate-300" />
                            )}
                            <File className="h-4 w-4 text-slate-400" />
                            <span className="text-slate-700 truncate">{entry.name}</span>
                          </button>
                        )}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </>
          )}

          {/* Footer */}
          <div className="flex items-center justify-between">
            <p className="text-xs text-slate-500">
              {selectedFiles.length > 0
                ? `${selectedFiles.length} file${selectedFiles.length !== 1 ? 's' : ''} selected`
                : 'Select files or use folder path'}
            </p>
            <div className="flex gap-2">
              <button
                onClick={() => setStep('configure')}
                className="rounded-lg border border-slate-300 px-3 py-1.5 text-sm font-medium text-slate-700 hover:bg-slate-50 transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={confirmBrowse}
                className="rounded-lg bg-indigo-600 px-3 py-1.5 text-sm font-medium text-white hover:bg-indigo-700 transition-colors"
              >
                Done
              </button>
            </div>
          </div>
        </div>
      )}

      {step === 'running' && (
        <div className="space-y-3">
          <div className="flex items-center gap-2 text-sm text-indigo-600">
            <Loader2 className="h-4 w-4 animate-spin" />
            <span className="font-medium">Ingesting...</span>
          </div>
          <div className="max-h-48 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs font-mono text-slate-600 space-y-1">
            {events.map((e, i) => (
              <div key={i} className="flex items-center gap-2">
                {e.done ? (
                  <span className="text-emerald-600">✓</span>
                ) : (
                  <Loader2 className="h-3 w-3 animate-spin text-indigo-500" />
                )}
                <span>
                  {e.label}
                  {e.detail && <span className="text-slate-400">: {e.detail}</span>}
                </span>
                {e.error && <span className="text-red-600 ml-1">(error: {e.error})</span>}
              </div>
            ))}
          </div>
        </div>
      )}

      {step === 'done' && (
        <div className="space-y-4">
          {error ? (
            <div className="rounded-lg border border-red-200 bg-red-50 p-4">
              <p className="text-sm font-medium text-red-800">Ingestion failed</p>
              <p className="mt-1 text-sm text-red-600">{error}</p>
            </div>
          ) : result ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4">
              <p className="text-sm font-medium text-emerald-800">Ingestion complete</p>
              <div className="mt-2 grid grid-cols-2 gap-2 text-sm text-emerald-700">
                <div>Notes processed: <span className="font-semibold">{result.notes}</span></div>
                <div>Sections added: <span className="font-semibold">{result.sectionsAdded}</span></div>
                <div>Components added: <span className="font-semibold">{result.componentsAdded}</span></div>
                <div>Usage events: <span className="font-semibold">{result.usageEvents}</span></div>
              </div>
            </div>
          ) : (
            <p className="text-sm text-slate-600">Done</p>
          )}

          {/* Progress log */}
          {events.length > 0 && (
            <div className="max-h-32 overflow-y-auto rounded-lg border border-slate-200 bg-slate-50 p-3 text-xs font-mono text-slate-600 space-y-0.5">
              {events.map((e, i) => (
                <div key={i}>
                  {e.done ? '✓' : '▸'} {e.label}{e.detail ? `: ${e.detail}` : ''}
                </div>
              ))}
            </div>
          )}

          <button
            onClick={() => {
              if (!error) onDone();
              else onClose();
            }}
            className="flex w-full items-center justify-center rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-indigo-700"
          >
            {error ? 'Close' : 'Done'}
          </button>
        </div>
      )}
    </Modal>
  );
}
