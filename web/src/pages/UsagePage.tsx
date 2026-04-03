import { useState, useEffect, useMemo, useCallback } from 'react';
import type { UsageTotals, UsageEvent } from '../types';
import { fetchUsageTotals, fetchUsageLedger } from '../api/client';
import {
  formatTokens,
  formatCost,
  formatCompactTokens,
  relativeTime,
  sinceDate,
  csvEscape,
} from '../lib/format';
import type { TimeFilter } from '../lib/format';
import {
  DollarSign,
  Zap,
  ArrowDown,
  ArrowUp,
  Download,
  TrendingUp,
  ChevronDown,
  ChevronUp,
  Loader2,
  AlertCircle,
  X,
} from 'lucide-react';
import {
  PieChart,
  Pie,
  Cell,
  Tooltip,
  ResponsiveContainer,
  BarChart,
  Bar,
  XAxis,
  YAxis,
  Legend,
  CartesianGrid,
  AreaChart,
  Area,
} from 'recharts';

// ── Constants ────────────────────────────────────────────────────────

const CHART_COLORS = [
  '#6366f1', '#a855f7', '#10b981', '#f59e0b', '#f43f5e',
  '#3b82f6', '#8b5cf6', '#14b8a6',
];

const PROVIDER_COLORS: Record<string, string> = {
  openai: '#6366f1',
  claude: '#a855f7',
  anthropic: '#a855f7',
  local: '#10b981',
  voyage: '#3b82f6',
};

const PAGE_SIZE = 25;

const TIME_FILTER_OPTIONS: { value: TimeFilter; label: string }[] = [
  { value: 'all', label: 'All Time' },
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7d' },
  { value: '30d', label: 'Last 30d' },
];

type SortField =
  | 'created_at'
  | 'operation'
  | 'provider'
  | 'model'
  | 'input_tokens'
  | 'output_tokens'
  | 'cost_usd';

type SortDir = 'asc' | 'desc';

interface PieDatum {
  name: string;
  value: number;
  color: string;
}

interface BarDatum {
  name: string;
  input: number;
  output: number;
}

interface DailyDatum {
  date: string;
  tokens: number;
  cost: number;
}

interface Projection {
  daily: number;
  weekly: number;
  monthly: number;
}

// ── Tooltip components ───────────────────────────────────────────────

interface TooltipPayloadItem {
  name: string;
  value: number;
  color: string;
  dataKey: string;
}

interface ChartTooltipProps {
  active?: boolean;
  payload?: TooltipPayloadItem[];
  label?: string;
}

function CostTooltip({ active, payload }: ChartTooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-3 py-2 shadow-lg">
      {payload.map((entry) => (
        <div key={entry.dataKey ?? entry.name} className="flex items-center gap-2 text-sm">
          <span
            className="h-2.5 w-2.5 rounded-full"
            style={{ backgroundColor: entry.color }}
          />
          <span className="text-slate-500">{entry.name}:</span>
          <span className="font-medium text-slate-900">{formatCost(entry.value)}</span>
        </div>
      ))}
    </div>
  );
}

function TokenTooltip({ active, payload }: ChartTooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-3 py-2 shadow-lg">
      {payload.map((entry) => (
        <div key={entry.dataKey ?? entry.name} className="flex items-center gap-2 text-sm">
          <span
            className="h-2.5 w-2.5 rounded-full"
            style={{ backgroundColor: entry.color }}
          />
          <span className="text-slate-500">{entry.name}:</span>
          <span className="font-medium text-slate-900">{formatTokens(entry.value)}</span>
        </div>
      ))}
    </div>
  );
}

function DailyTooltip({ active, payload, label }: ChartTooltipProps) {
  if (!active || !payload?.length) return null;
  return (
    <div className="rounded-lg border border-slate-200 bg-white px-3 py-2 shadow-lg">
      <p className="mb-1 text-xs font-medium text-slate-600">{label}</p>
      {payload.map((entry) => (
        <div key={entry.dataKey} className="flex items-center gap-2 text-sm">
          <span
            className="h-2.5 w-2.5 rounded-full"
            style={{ backgroundColor: entry.color }}
          />
          <span className="text-slate-500">{entry.name}:</span>
          <span className="font-medium text-slate-900">
            {entry.dataKey === 'cost' ? formatCost(entry.value) : formatTokens(entry.value)}
          </span>
        </div>
      ))}
    </div>
  );
}

// ── Summary card ─────────────────────────────────────────────────────

function SummaryCard({
  icon: Icon,
  label,
  value,
  sub,
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string;
  sub?: string;
}) {
  return (
    <div className="rounded-lg border border-slate-200 bg-white p-5 shadow-sm">
      <div className="flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-indigo-50">
          <Icon className="h-5 w-5 text-indigo-600" />
        </div>
        <div>
          <p className="text-sm text-slate-500">{label}</p>
          <p className="text-xl font-semibold text-slate-900">{value}</p>
          {sub && <p className="text-xs text-slate-400">{sub}</p>}
        </div>
      </div>
    </div>
  );
}

// ── Helpers ──────────────────────────────────────────────────────────

function operationBadge(op: string): string {
  if (op.includes('chat')) return 'bg-blue-100 text-blue-700';
  if (op.includes('ingest')) return 'bg-emerald-100 text-emerald-700';
  if (op.includes('quiz')) return 'bg-purple-100 text-purple-700';
  return 'bg-slate-100 text-slate-700';
}

function getModelColor(
  model: string,
  providerMap: Map<string, string>,
  index: number,
): string {
  const provider = providerMap.get(model);
  if (provider && PROVIDER_COLORS[provider]) return PROVIDER_COLORS[provider];
  return CHART_COLORS[index % CHART_COLORS.length];
}

function exportAsCSV(events: UsageEvent[]): void {
  const headers = [
    'Time', 'Operation', 'Provider', 'Model', 'Input Tokens',
    'Output Tokens', 'Total Tokens', 'Cost (USD)', 'Class', 'Source Path',
  ];
  const rows = events.map((e) =>
    [
      e.created_at, e.operation, e.provider, e.model,
      e.input_tokens, e.output_tokens, e.total_tokens,
      e.cost_usd ?? 0, e.class ?? '', e.source_path ?? '',
    ]
      .map(csvEscape)
      .join(','),
  );
  const csv = [headers.map(csvEscape).join(','), ...rows].join('\n');
  const blob = new Blob([csv], { type: 'text/csv;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `usage-${new Date().toISOString().slice(0, 10)}.csv`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  URL.revokeObjectURL(url);
}

function aggregateByDay(events: UsageEvent[]): DailyDatum[] {
  const byDay: Record<string, { tokens: number; cost: number }> = {};
  for (const e of events) {
    const day = e.created_at.slice(0, 10);
    const entry = byDay[day] ?? { tokens: 0, cost: 0 };
    entry.tokens += e.total_tokens;
    entry.cost += e.cost_usd ?? 0;
    byDay[day] = entry;
  }
  return Object.entries(byDay)
    .map(([date, data]) => ({ date, ...data }))
    .sort((a, b) => a.date.localeCompare(b.date));
}

function sortEvents(
  events: UsageEvent[],
  field: SortField,
  dir: SortDir,
): UsageEvent[] {
  return [...events].sort((a, b) => {
    let cmp = 0;
    switch (field) {
      case 'created_at':
        cmp = a.created_at.localeCompare(b.created_at);
        break;
      case 'operation':
        cmp = a.operation.localeCompare(b.operation);
        break;
      case 'provider':
        cmp = a.provider.localeCompare(b.provider);
        break;
      case 'model':
        cmp = a.model.localeCompare(b.model);
        break;
      case 'input_tokens':
        cmp = a.input_tokens - b.input_tokens;
        break;
      case 'output_tokens':
        cmp = a.output_tokens - b.output_tokens;
        break;
      case 'cost_usd':
        cmp = (a.cost_usd ?? 0) - (b.cost_usd ?? 0);
        break;
    }
    return dir === 'asc' ? cmp : -cmp;
  });
}

// ── Main component ───────────────────────────────────────────────────

export default function UsagePage() {
  const [totals, setTotals] = useState<UsageTotals | null>(null);
  const [ledger, setLedger] = useState<UsageEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [timeFilter, setTimeFilter] = useState<TimeFilter>('all');
  const [showLedger, setShowLedger] = useState(false);
  const [ledgerPage, setLedgerPage] = useState(0);
  const [sortField, setSortField] = useState<SortField>('created_at');
  const [sortDir, setSortDir] = useState<SortDir>('desc');
  const [expandedRow, setExpandedRow] = useState<string | null>(null);
  const [selectedModel, setSelectedModel] = useState<string | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const since = sinceDate(timeFilter);
      const [t, l] = await Promise.all([
        fetchUsageTotals(since ? { since } : undefined),
        fetchUsageLedger(),
      ]);
      setTotals(t);
      setLedger(l);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load usage data');
    } finally {
      setLoading(false);
    }
  }, [timeFilter]);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  // Build a model → provider mapping from ledger events
  const modelProviderMap = useMemo(() => {
    const map = new Map<string, string>();
    for (const e of ledger) {
      map.set(e.model, e.provider);
    }
    return map;
  }, [ledger]);

  // Filter ledger by time
  const timeFilteredLedger = useMemo(() => {
    const since = sinceDate(timeFilter);
    if (!since) return ledger;
    return ledger.filter((e) => e.created_at >= since);
  }, [ledger, timeFilter]);

  // Filter ledger by selected model (from chart click)
  const filteredLedger = useMemo(() => {
    if (!selectedModel) return timeFilteredLedger;
    return timeFilteredLedger.filter((e) => e.model === selectedModel);
  }, [timeFilteredLedger, selectedModel]);

  // Chart data
  const pieData: PieDatum[] = useMemo(() => {
    if (!totals) return [];
    return Object.entries(totals.by_model)
      .filter(([, m]) => m.cost_usd > 0)
      .map(([name, m], idx) => ({
        name,
        value: m.cost_usd,
        color: getModelColor(name, modelProviderMap, idx),
      }))
      .sort((a, b) => b.value - a.value);
  }, [totals, modelProviderMap]);

  const barData: BarDatum[] = useMemo(() => {
    if (!totals) return [];
    return Object.entries(totals.by_model)
      .map(([name, m]) => ({
        name,
        input: m.input_tokens,
        output: m.output_tokens,
      }))
      .sort((a, b) => b.input + b.output - (a.input + a.output));
  }, [totals]);

  const dailyData: DailyDatum[] = useMemo(
    () => aggregateByDay(timeFilteredLedger),
    [timeFilteredLedger],
  );

  const projection: Projection | null = useMemo(() => {
    if (timeFilteredLedger.length === 0) return null;
    const sorted = [...timeFilteredLedger].sort((a, b) =>
      a.created_at.localeCompare(b.created_at),
    );
    const firstMs = new Date(sorted[0].created_at).getTime();
    const lastMs = new Date(sorted[sorted.length - 1].created_at).getTime();
    const daySpan = Math.max(1, (lastMs - firstMs) / 86_400_000);
    const totalCost = timeFilteredLedger.reduce(
      (sum, e) => sum + (e.cost_usd ?? 0),
      0,
    );
    const daily = totalCost / daySpan;
    return { daily, weekly: daily * 7, monthly: daily * 30 };
  }, [timeFilteredLedger]);

  // Sorted + paginated ledger
  const sortedLedger = useMemo(
    () => sortEvents(filteredLedger, sortField, sortDir),
    [filteredLedger, sortField, sortDir],
  );

  const totalPages = Math.ceil(sortedLedger.length / PAGE_SIZE);
  const startIdx = ledgerPage * PAGE_SIZE;
  const endIdx = Math.min(startIdx + PAGE_SIZE, sortedLedger.length);
  const pagedLedger = sortedLedger.slice(startIdx, endIdx);

  const handleSort = useCallback(
    (field: SortField) => {
      if (sortField === field) {
        setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
      } else {
        setSortField(field);
        setSortDir('desc');
      }
      setLedgerPage(0);
    },
    [sortField],
  );

  const handleTimeFilter = useCallback((filter: TimeFilter) => {
    setTimeFilter(filter);
    setLedgerPage(0);
    setSelectedModel(null);
  }, []);

  const handleExportCSV = useCallback(() => {
    exportAsCSV(filteredLedger);
  }, [filteredLedger]);

  // ── Render ──────────────────────────────────────────────────────

  if (loading && !totals) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
      </div>
    );
  }

  if (error && !totals) {
    return (
      <div className="p-6 text-center">
        <AlertCircle className="mx-auto mb-2 h-8 w-8 text-red-500" />
        <p className="text-red-600">{error}</p>
        <button
          onClick={() => void loadData()}
          className="mt-3 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl space-y-6 px-4 py-6">
      {/* Header */}
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Usage Analytics</h1>
          <p className="mt-1 text-sm text-slate-500">
            Track AI model usage, costs, and token consumption
          </p>
        </div>
        <button
          onClick={handleExportCSV}
          className="inline-flex items-center gap-2 rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50"
        >
          <Download className="h-4 w-4" />
          Export CSV
        </button>
      </div>

      {/* Summary cards */}
      {totals && (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <SummaryCard
            icon={DollarSign}
            label="Total Cost"
            value={formatCost(totals.total_cost_usd)}
          />
          <SummaryCard
            icon={Zap}
            label="Total Tokens"
            value={formatTokens(totals.total_tokens)}
          />
          <SummaryCard
            icon={ArrowDown}
            label="Input Tokens"
            value={formatTokens(totals.total_input_tokens)}
          />
          <SummaryCard
            icon={ArrowUp}
            label="Output Tokens"
            value={formatTokens(totals.total_output_tokens)}
          />
        </div>
      )}

      {/* Time filter */}
      <div className="flex flex-wrap items-center gap-2">
        {TIME_FILTER_OPTIONS.map((f) => (
          <button
            key={f.value}
            onClick={() => handleTimeFilter(f.value)}
            className={`rounded-md px-3 py-1.5 text-sm font-medium transition ${
              timeFilter === f.value
                ? 'bg-indigo-600 text-white'
                : 'bg-white text-slate-600 border border-slate-300 hover:bg-slate-50'
            }`}
          >
            {f.label}
          </button>
        ))}
        {loading && <Loader2 className="ml-2 h-4 w-4 animate-spin text-indigo-500" />}
      </div>

      {/* Model filter chip */}
      {selectedModel && (
        <div className="flex items-center gap-2 rounded-lg bg-indigo-50 px-3 py-2 text-sm text-indigo-700">
          <span>
            Filtered by model: <strong>{selectedModel}</strong>
          </span>
          <button
            onClick={() => setSelectedModel(null)}
            className="rounded p-0.5 hover:bg-indigo-100"
          >
            <X className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Charts row */}
      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Cost by Model */}
        <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="mb-4 text-lg font-semibold text-slate-900">Cost by Model</h2>
          {pieData.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <PieChart>
                <Pie
                  data={pieData}
                  dataKey="value"
                  nameKey="name"
                  cx="50%"
                  cy="50%"
                  innerRadius={60}
                  outerRadius={100}
                  paddingAngle={2}
                  cursor="pointer"
                  onClick={(_d: unknown) => {
                    const d = _d as PieDatum;
                    setSelectedModel(d.name);
                  }}
                >
                  {pieData.map((entry) => (
                    <Cell key={entry.name} fill={entry.color} />
                  ))}
                </Pie>
                <Tooltip content={<CostTooltip />} />
                <Legend
                  formatter={(value: string) => (
                    <span className="text-sm text-slate-600">{value}</span>
                  )}
                />
              </PieChart>
            </ResponsiveContainer>
          ) : (
            <p className="py-12 text-center text-sm text-slate-400">No cost data available</p>
          )}
        </div>

        {/* Token Usage by Model */}
        <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <h2 className="mb-4 text-lg font-semibold text-slate-900">
            Token Usage by Model
          </h2>
          {barData.length > 0 ? (
            <ResponsiveContainer width="100%" height={300}>
              <BarChart data={barData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
                <XAxis
                  dataKey="name"
                  tick={{ fontSize: 12 }}
                  tickLine={false}
                  axisLine={false}
                />
                <YAxis
                  tickFormatter={(v: number) => formatCompactTokens(v)}
                  tick={{ fontSize: 12 }}
                  tickLine={false}
                  axisLine={false}
                />
                <Tooltip content={<TokenTooltip />} />
                <Legend />
                <Bar
                  dataKey="input"
                  name="Input"
                  stackId="tokens"
                  fill="#818cf8"
                  radius={[0, 0, 0, 0]}
                />
                <Bar
                  dataKey="output"
                  name="Output"
                  stackId="tokens"
                  fill="#c084fc"
                  radius={[4, 4, 0, 0]}
                />
              </BarChart>
            </ResponsiveContainer>
          ) : (
            <p className="py-12 text-center text-sm text-slate-400">
              No token usage data available
            </p>
          )}
        </div>
      </div>

      {/* Usage Over Time */}
      <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="mb-4 text-lg font-semibold text-slate-900">Usage Over Time</h2>
        {dailyData.length > 0 ? (
          <ResponsiveContainer width="100%" height={300}>
            <AreaChart data={dailyData}>
              <CartesianGrid strokeDasharray="3 3" stroke="#e2e8f0" />
              <XAxis
                dataKey="date"
                tickFormatter={(d: string) => d.slice(5)}
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                yAxisId="tokens"
                tickFormatter={(v: number) => formatCompactTokens(v)}
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
              />
              <YAxis
                yAxisId="cost"
                orientation="right"
                tickFormatter={(v: number) => `$${v.toFixed(2)}`}
                tick={{ fontSize: 12 }}
                tickLine={false}
                axisLine={false}
              />
              <Tooltip content={<DailyTooltip />} />
              <Legend />
              <Area
                yAxisId="tokens"
                type="monotone"
                dataKey="tokens"
                name="Tokens"
                fill="#6366f1"
                stroke="#6366f1"
                fillOpacity={0.15}
              />
              <Area
                yAxisId="cost"
                type="monotone"
                dataKey="cost"
                name="Cost"
                fill="#10b981"
                stroke="#10b981"
                fillOpacity={0.1}
              />
            </AreaChart>
          </ResponsiveContainer>
        ) : (
          <p className="py-12 text-center text-sm text-slate-400">
            No time-series data available
          </p>
        )}
      </div>

      {/* Cost Projection */}
      {projection && (
        <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
          <div className="flex items-center gap-2 mb-4">
            <TrendingUp className="h-5 w-5 text-indigo-600" />
            <h2 className="text-lg font-semibold text-slate-900">Cost Projection</h2>
          </div>
          <p className="mb-3 text-sm text-slate-500">
            Based on your recent usage rate:
          </p>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
            <div className="rounded-md bg-slate-50 p-4">
              <p className="text-sm text-slate-500">Daily Average</p>
              <p className="text-lg font-semibold text-slate-900">
                {formatCost(projection.daily)}
              </p>
            </div>
            <div className="rounded-md bg-slate-50 p-4">
              <p className="text-sm text-slate-500">Weekly Estimate</p>
              <p className="text-lg font-semibold text-slate-900">
                {formatCost(projection.weekly)}
              </p>
            </div>
            <div className="rounded-md bg-slate-50 p-4">
              <p className="text-sm text-slate-500">Monthly Estimate</p>
              <p className="text-lg font-semibold text-slate-900">
                {formatCost(projection.monthly)}
              </p>
            </div>
          </div>
        </div>
      )}

      {/* Detailed Ledger */}
      <div className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <div className="mb-4 flex items-center justify-between">
          <h2 className="text-lg font-semibold text-slate-900">Usage Ledger</h2>
          <button
            onClick={() => setShowLedger((v) => !v)}
            className="inline-flex items-center gap-1.5 rounded-md border border-slate-300 bg-white px-3 py-1.5 text-sm font-medium text-slate-600 hover:bg-slate-50"
          >
            {showLedger ? (
              <>
                <ChevronUp className="h-4 w-4" /> Hide Details
              </>
            ) : (
              <>
                <ChevronDown className="h-4 w-4" /> Show Details
              </>
            )}
          </button>
        </div>

        {showLedger && (
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-slate-200 text-left">
                    {(
                      [
                        ['created_at', 'Time'],
                        ['operation', 'Operation'],
                        ['provider', 'Provider'],
                        ['model', 'Model'],
                        ['input_tokens', 'Input'],
                        ['output_tokens', 'Output'],
                        ['cost_usd', 'Cost'],
                      ] as const
                    ).map(([field, label]) => (
                      <th
                        key={field}
                        onClick={() => handleSort(field)}
                        className="cursor-pointer whitespace-nowrap px-3 py-2 font-medium text-slate-600 hover:text-slate-900 select-none"
                      >
                        <span className="inline-flex items-center gap-1">
                          {label}
                          {sortField === field && (
                            <span className="text-indigo-500">
                              {sortDir === 'asc' ? '↑' : '↓'}
                            </span>
                          )}
                        </span>
                      </th>
                    ))}
                    <th className="px-3 py-2 font-medium text-slate-600">Class</th>
                  </tr>
                </thead>
                <tbody>
                  {pagedLedger.map((event, idx) => (
                    <LedgerRow
                      key={event.id}
                      event={event}
                      expanded={expandedRow === event.id}
                      onToggle={() =>
                        setExpandedRow((prev) => (prev === event.id ? null : event.id))
                      }
                      striped={idx % 2 === 1}
                    />
                  ))}
                  {pagedLedger.length === 0 && (
                    <tr>
                      <td colSpan={8} className="py-8 text-center text-slate-400">
                        No usage events found
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>

            {/* Pagination */}
            {totalPages > 1 && (
              <div className="mt-4 flex items-center justify-between border-t border-slate-200 pt-4">
                <span className="text-sm text-slate-500">
                  Showing {startIdx + 1}–{endIdx} of {sortedLedger.length} events
                </span>
                <div className="flex items-center gap-2">
                  <button
                    disabled={ledgerPage === 0}
                    onClick={() => setLedgerPage((p) => p - 1)}
                    className="rounded-md border border-slate-300 px-3 py-1 text-sm font-medium text-slate-600 enabled:hover:bg-slate-50 disabled:opacity-40"
                  >
                    Previous
                  </button>
                  <span className="text-sm text-slate-600">
                    {ledgerPage + 1} / {totalPages}
                  </span>
                  <button
                    disabled={ledgerPage >= totalPages - 1}
                    onClick={() => setLedgerPage((p) => p + 1)}
                    className="rounded-md border border-slate-300 px-3 py-1 text-sm font-medium text-slate-600 enabled:hover:bg-slate-50 disabled:opacity-40"
                  >
                    Next
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// ── Ledger row ───────────────────────────────────────────────────────

function LedgerRow({
  event,
  expanded,
  onToggle,
  striped,
}: {
  event: UsageEvent;
  expanded: boolean;
  onToggle: () => void;
  striped: boolean;
}) {
  return (
    <>
      <tr
        onClick={onToggle}
        className={`cursor-pointer border-b border-slate-100 transition hover:bg-slate-50 ${
          striped ? 'bg-slate-50/50' : ''
        }`}
      >
        <td className="whitespace-nowrap px-3 py-2 text-slate-500">
          {relativeTime(event.created_at)}
        </td>
        <td className="px-3 py-2">
          <span
            className={`inline-block rounded-full px-2 py-0.5 text-xs font-medium ${operationBadge(event.operation)}`}
          >
            {event.operation}
          </span>
        </td>
        <td className="px-3 py-2 text-slate-700">{event.provider}</td>
        <td className="px-3 py-2 font-mono text-xs text-slate-700">{event.model}</td>
        <td className="px-3 py-2 text-right tabular-nums text-slate-700">
          {formatTokens(event.input_tokens)}
        </td>
        <td className="px-3 py-2 text-right tabular-nums text-slate-700">
          {formatTokens(event.output_tokens)}
        </td>
        <td className="px-3 py-2 text-right tabular-nums text-slate-700">
          {event.cost_usd != null ? formatCost(event.cost_usd) : '—'}
        </td>
        <td className="px-3 py-2 text-slate-500">{event.class ?? '—'}</td>
      </tr>
      {expanded && (
        <tr className="bg-indigo-50/40">
          <td colSpan={8} className="px-6 py-3">
            <div className="grid grid-cols-2 gap-x-8 gap-y-1 text-sm sm:grid-cols-4">
              <div>
                <span className="text-slate-500">Request ID</span>
                <p className="font-mono text-xs text-slate-700 break-all">
                  {event.request_id ?? '—'}
                </p>
              </div>
              <div>
                <span className="text-slate-500">Total Tokens</span>
                <p className="font-medium text-slate-700">
                  {formatTokens(event.total_tokens)}
                </p>
              </div>
              <div>
                <span className="text-slate-500">Source Path</span>
                <p className="font-mono text-xs text-slate-700 break-all">
                  {event.source_path ?? '—'}
                </p>
              </div>
              <div>
                <span className="text-slate-500">Full Timestamp</span>
                <p className="text-slate-700">
                  {new Date(event.created_at).toLocaleString()}
                </p>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  );
}
