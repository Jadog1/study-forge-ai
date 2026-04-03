import { useEffect, useState } from 'react';
import { CheckSquare, Download, Loader2, Square } from 'lucide-react';
import { Modal } from './Modal';
import { exportKnowledge, fetchClasses } from '../api/client';
import type { ExportKnowledgeResult } from '../types';

interface ExportModalProps {
  open: boolean;
  onClose: () => void;
}

export function ExportModal({ open, onClose }: ExportModalProps) {
  const [outputPath, setOutputPath] = useState('');
  const [className, setClassName] = useState('');
  const [classes, setClasses] = useState<string[]>([]);
  const [includeEmbeddings, setIncludeEmbeddings] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<ExportKnowledgeResult | null>(null);

  useEffect(() => {
    if (open) {
      fetchClasses().then(setClasses).catch(() => {});
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
      setOutputPath(`./knowledge-export-${timestamp}.json`);
      setClassName('');
      setIncludeEmbeddings(false);
      setError(null);
      setResult(null);
    }
  }, [open]);

  const handleExport = async () => {
    if (!outputPath.trim()) return;
    setExporting(true);
    setError(null);
    setResult(null);
    try {
      const res = await exportKnowledge({
        output_path: outputPath.trim(),
        class: className || undefined,
        include_embeddings: includeEmbeddings,
      });
      setResult(res);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Export failed');
    } finally {
      setExporting(false);
    }
  };

  return (
    <Modal open={open} onClose={exporting ? () => {} : onClose} title="Export Knowledge">
      {!result ? (
        <div className="space-y-4">
          {/* Output path */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Output File Path
            </label>
            <input
              type="text"
              value={outputPath}
              onChange={(e) => setOutputPath(e.target.value)}
              placeholder="./knowledge-export.json"
              className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
            />
          </div>

          {/* Class filter */}
          <div>
            <label className="mb-1 block text-sm font-medium text-slate-700">
              Class Filter <span className="text-slate-400 font-normal">(optional)</span>
            </label>
            {classes.length > 0 ? (
              <select
                value={className}
                onChange={(e) => setClassName(e.target.value)}
                className="w-full rounded-lg border border-slate-300 px-3 py-2 text-sm focus:border-indigo-500 focus:outline-none focus:ring-1 focus:ring-indigo-500"
              >
                <option value="">All classes</option>
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

          {/* Include embeddings */}
          <label className="flex items-center gap-2 cursor-pointer">
            <button
              onClick={() => setIncludeEmbeddings(!includeEmbeddings)}
              className="shrink-0"
            >
              {includeEmbeddings ? (
                <CheckSquare className="h-4 w-4 text-indigo-600" />
              ) : (
                <Square className="h-4 w-4 text-slate-300" />
              )}
            </button>
            <span className="text-sm text-slate-700">Include embeddings</span>
          </label>

          {error && <p className="text-sm text-red-600">{error}</p>}

          <button
            onClick={handleExport}
            disabled={exporting || !outputPath.trim()}
            className="flex w-full items-center justify-center gap-2 rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-indigo-700 disabled:opacity-50"
          >
            {exporting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                Exporting...
              </>
            ) : (
              <>
                <Download className="h-4 w-4" />
                Export
              </>
            )}
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          <div className="rounded-lg border border-emerald-200 bg-emerald-50 p-4">
            <p className="text-sm font-medium text-emerald-800">Export complete</p>
            <div className="mt-2 space-y-1 text-sm text-emerald-700">
              <p>Path: <span className="font-mono font-semibold">{result.output_path}</span></p>
              <p>Sections: <span className="font-semibold">{result.sections}</span></p>
              <p>Components: <span className="font-semibold">{result.components}</span></p>
              {result.class && <p>Class filter: <span className="font-semibold">{result.class}</span></p>}
              <p>Embeddings: <span className="font-semibold">{result.include_embeddings ? 'included' : 'excluded'}</span></p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="flex w-full items-center justify-center rounded-lg bg-indigo-600 px-4 py-2.5 text-sm font-medium text-white transition-colors hover:bg-indigo-700"
          >
            Done
          </button>
        </div>
      )}
    </Modal>
  );
}
