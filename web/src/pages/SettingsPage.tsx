import { useState, useEffect, useCallback } from 'react';
import type { Config } from '../types';
import { fetchConfig, updateConfig } from '../api/client';
import {
  Save,
  RotateCcw,
  ChevronDown,
  ChevronUp,
  Plus,
  Trash2,
  Loader2,
  AlertCircle,
  CheckCircle2,
  Info,
} from 'lucide-react';

// ── Types ────────────────────────────────────────────────────────────

type Provider = 'openai' | 'claude' | 'local';
type EmbeddingsProvider = 'openai' | 'voyage' | 'local';

interface Feedback {
  type: 'success' | 'error';
  message: string;
}

const PROVIDERS: { value: Provider; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'claude', label: 'Claude' },
  { value: 'local', label: 'Local' },
];

const EMBEDDING_PROVIDERS: { value: EmbeddingsProvider; label: string }[] = [
  { value: 'openai', label: 'OpenAI' },
  { value: 'voyage', label: 'Voyage' },
  { value: 'local', label: 'Local' },
];

const AGENT_ROLES: { key: string; label: string }[] = [
  { key: 'chat', label: 'Chat' },
  { key: 'ingestion', label: 'Ingestion' },
  { key: 'quiz_orchestrator', label: 'Quiz Orchestrator' },
  { key: 'quiz_component', label: 'Quiz Component' },
];

// ── Validation ───────────────────────────────────────────────────────

function validateConfig(config: Config): string[] {
  const errors: string[] = [];

  if (config.provider === 'local') {
    if (!config.local.endpoint) {
      errors.push('Local endpoint URL is required');
    } else if (
      !config.local.endpoint.startsWith('http://') &&
      !config.local.endpoint.startsWith('https://')
    ) {
      errors.push('Local endpoint must start with http:// or https://');
    }
    if (!config.local.model) errors.push('Local model name is required');
  }

  if (config.provider === 'openai' && !config.openai.model) {
    errors.push('OpenAI model name is required');
  }

  if (config.provider === 'claude' && !config.claude.model) {
    errors.push('Claude model name is required');
  }

  if (!config.embeddings.model) {
    errors.push('Embeddings model name is required');
  }

  return errors;
}

// ── Sub-components ───────────────────────────────────────────────────

function CollapsibleSection({
  title,
  description,
  open,
  onToggle,
  children,
}: {
  title: string;
  description?: string;
  open: boolean;
  onToggle: () => void;
  children: React.ReactNode;
}) {
  return (
    <section className="rounded-lg border border-slate-200 bg-white shadow-sm">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between px-6 py-4"
      >
        <div className="text-left">
          <h2 className="text-lg font-semibold text-slate-900">{title}</h2>
          {description && <p className="mt-0.5 text-sm text-slate-500">{description}</p>}
        </div>
        {open ? (
          <ChevronUp className="h-5 w-5 text-slate-400" />
        ) : (
          <ChevronDown className="h-5 w-5 text-slate-400" />
        )}
      </button>
      {open && <div className="border-t border-slate-200 px-6 py-5">{children}</div>}
    </section>
  );
}

function FieldLabel({ children, htmlFor }: { children: React.ReactNode; htmlFor?: string }) {
  return (
    <label htmlFor={htmlFor} className="block text-sm font-medium text-slate-700">
      {children}
    </label>
  );
}

function HelpText({ children }: { children: React.ReactNode }) {
  return <p className="mt-1 text-sm text-slate-500">{children}</p>;
}

const INPUT_CLASS =
  'mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none';

const SELECT_CLASS =
  'mt-1 block w-full rounded-md border border-slate-300 px-3 py-2 text-sm shadow-sm focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none bg-white';

// ── Main component ───────────────────────────────────────────────────

export default function SettingsPage() {
  const [original, setOriginal] = useState<Config | null>(null);
  const [draft, setDraft] = useState<Config | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [feedback, setFeedback] = useState<Feedback | null>(null);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({
    embeddings: false,
    agentModels: true,
    sfq: true,
    prompt: true,
    pricing: true,
  });
  const [newModelName, setNewModelName] = useState('');
  const [pricingFilter, setPricingFilter] = useState('');

  const loadConfig = useCallback(async () => {
    setLoading(true);
    try {
      const raw = await fetchConfig();
      const config: Config = {
        provider: raw.provider ?? 'openai',
        embeddings: { provider: raw.embeddings?.provider ?? 'openai', model: raw.embeddings?.model ?? '' },
        openai: { model: raw.openai?.model ?? '' },
        claude: { model: raw.claude?.model ?? '' },
        voyage: { model: raw.voyage?.model ?? '' },
        local: {
          endpoint: raw.local?.endpoint ?? '',
          embeddings_endpoint: raw.local?.embeddings_endpoint ?? '',
          model: raw.local?.model ?? '',
        },
        sfq: { command: raw.sfq?.command ?? 'sfq' },
        agent_models: {
          chat: { provider: raw.agent_models?.chat?.provider ?? '', model: raw.agent_models?.chat?.model ?? '' },
          ingestion: { provider: raw.agent_models?.ingestion?.provider ?? '', model: raw.agent_models?.ingestion?.model ?? '' },
          quiz_orchestrator: { provider: raw.agent_models?.quiz_orchestrator?.provider ?? '', model: raw.agent_models?.quiz_orchestrator?.model ?? '' },
          quiz_component: { provider: raw.agent_models?.quiz_component?.provider ?? '', model: raw.agent_models?.quiz_component?.model ?? '' },
        },
        custom_prompt_context: raw.custom_prompt_context ?? '',
        model_prices: raw.model_prices ?? {},
      };
      setOriginal(config);
      setDraft(structuredClone(config));
    } catch (err) {
      setFeedback({
        type: 'error',
        message: err instanceof Error ? err.message : 'Failed to load config',
      });
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadConfig();
  }, [loadConfig]);

  const toggleSection = useCallback((key: string) => {
    setCollapsed((prev) => ({ ...prev, [key]: !prev[key] }));
  }, []);

  const updateDraft = useCallback((updater: (prev: Config) => Config) => {
    setDraft((prev) => (prev ? updater(prev) : prev));
  }, []);

  const handleSave = useCallback(async () => {
    if (!draft) return;

    const errors = validateConfig(draft);
    if (errors.length > 0) {
      setFeedback({ type: 'error', message: errors.join('. ') });
      return;
    }

    setSaving(true);
    setFeedback(null);
    try {
      await updateConfig(draft);
      setOriginal(structuredClone(draft));
      setFeedback({ type: 'success', message: 'Settings saved successfully' });
    } catch (err) {
      setFeedback({
        type: 'error',
        message: err instanceof Error ? err.message : 'Failed to save',
      });
    } finally {
      setSaving(false);
    }
  }, [draft]);

  const handleReset = useCallback(() => {
    if (original) {
      setDraft(structuredClone(original));
      setFeedback(null);
    }
  }, [original]);

  const addModelPrice = useCallback(
    (model: string) => {
      const trimmed = model.trim();
      if (!trimmed) return;
      updateDraft((d) => ({
        ...d,
        model_prices: {
          ...(d.model_prices ?? {}),
          [trimmed]: { input_per_million: 0, output_per_million: 0 },
        },
      }));
      setNewModelName('');
    },
    [updateDraft],
  );

  const removeModelPrice = useCallback(
    (model: string) => {
      updateDraft((d) => {
        const prices = { ...d.model_prices };
        delete prices[model];
        return { ...d, model_prices: prices };
      });
    },
    [updateDraft],
  );

  // ── Render ──────────────────────────────────────────────────────

  if (loading) {
    return (
      <div className="flex h-64 items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-indigo-500" />
      </div>
    );
  }

  if (!draft) {
    return (
      <div className="p-6 text-center">
        <AlertCircle className="mx-auto mb-2 h-8 w-8 text-red-500" />
        <p className="text-red-600">Failed to load configuration</p>
        <button
          onClick={() => void loadConfig()}
          className="mt-3 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white hover:bg-indigo-700"
        >
          Retry
        </button>
      </div>
    );
  }

  const agentModels = draft.agent_models ?? {};
  const modelPrices = draft.model_prices ?? {};
  const filteredPriceEntries = Object.entries(modelPrices).filter(([model]) =>
    model.toLowerCase().includes(pricingFilter.toLowerCase()),
  );

  return (
    <div className="mx-auto max-w-4xl px-4 py-6 space-y-6">
      {/* Sticky header */}
      <div className="sticky top-0 z-10 -mx-4 flex flex-wrap items-center justify-between gap-4 border-b border-slate-200 bg-white/95 px-4 py-4 backdrop-blur-sm">
        <div>
          <h1 className="text-2xl font-bold text-slate-900">Settings</h1>
          <p className="mt-0.5 text-sm text-slate-500">
            Configure AI providers and application settings
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={handleReset}
            className="inline-flex items-center gap-2 rounded-md border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm hover:bg-slate-50"
          >
            <RotateCcw className="h-4 w-4" />
            Reset
          </button>
          <button
            onClick={() => void handleSave()}
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-md bg-indigo-600 px-4 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-60"
          >
            {saving ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Save className="h-4 w-4" />
            )}
            Save Settings
          </button>
        </div>
      </div>

      {/* Feedback */}
      {feedback && (
        <div
          className={`flex items-start gap-2 rounded-lg px-4 py-3 text-sm ${
            feedback.type === 'success'
              ? 'bg-emerald-50 text-emerald-700'
              : 'bg-red-50 text-red-700'
          }`}
        >
          {feedback.type === 'success' ? (
            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0" />
          ) : (
            <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" />
          )}
          {feedback.message}
        </div>
      )}

      {/* Section 1: AI Provider */}
      <section className="rounded-lg border border-slate-200 bg-white p-6 shadow-sm">
        <h2 className="text-lg font-semibold text-slate-900">AI Provider</h2>
        <p className="mt-1 text-sm text-slate-500">
          Select the primary AI provider for completions
        </p>

        <div className="mt-4 flex flex-wrap gap-3">
          {PROVIDERS.map((p) => (
            <label
              key={p.value}
              className={`flex cursor-pointer items-center gap-2 rounded-lg border px-4 py-3 text-sm font-medium transition ${
                draft.provider === p.value
                  ? 'border-indigo-500 bg-indigo-50 text-indigo-700 ring-2 ring-indigo-500'
                  : 'border-slate-200 text-slate-700 hover:border-slate-300'
              }`}
            >
              <input
                type="radio"
                name="provider"
                value={p.value}
                checked={draft.provider === p.value}
                onChange={() => updateDraft((d) => ({ ...d, provider: p.value }))}
                className="sr-only"
              />
              {p.label}
            </label>
          ))}
        </div>

        {/* Provider-specific fields */}
        <div className="mt-5 space-y-4 rounded-md bg-slate-50 p-4">
          {draft.provider === 'openai' && (
            <div>
              <FieldLabel htmlFor="openai-model">Model</FieldLabel>
              <input
                id="openai-model"
                type="text"
                value={draft.openai.model}
                onChange={(e) =>
                  updateDraft((d) => ({ ...d, openai: { ...d.openai, model: e.target.value } }))
                }
                placeholder="gpt-5-mini"
                className={INPUT_CLASS}
              />
            </div>
          )}

          {draft.provider === 'claude' && (
            <div>
              <FieldLabel htmlFor="claude-model">Model</FieldLabel>
              <input
                id="claude-model"
                type="text"
                value={draft.claude.model}
                onChange={(e) =>
                  updateDraft((d) => ({ ...d, claude: { ...d.claude, model: e.target.value } }))
                }
                placeholder="claude-4-5-haiku"
                className={INPUT_CLASS}
              />
            </div>
          )}

          {draft.provider === 'local' && (
            <>
              <div>
                <FieldLabel htmlFor="local-endpoint">Endpoint URL</FieldLabel>
                <input
                  id="local-endpoint"
                  type="text"
                  value={draft.local.endpoint}
                  onChange={(e) =>
                    updateDraft((d) => ({
                      ...d,
                      local: { ...d.local, endpoint: e.target.value },
                    }))
                  }
                  placeholder="http://localhost:11434"
                  className={INPUT_CLASS}
                />
              </div>
              <div>
                <FieldLabel htmlFor="local-model">Model</FieldLabel>
                <input
                  id="local-model"
                  type="text"
                  value={draft.local.model}
                  onChange={(e) =>
                    updateDraft((d) => ({ ...d, local: { ...d.local, model: e.target.value } }))
                  }
                  placeholder="llama3"
                  className={INPUT_CLASS}
                />
              </div>
            </>
          )}
        </div>

        <div className="mt-4 flex items-start gap-2 rounded-md bg-amber-50 px-3 py-2.5 text-sm text-amber-700">
          <Info className="mt-0.5 h-4 w-4 shrink-0" />
          <span>
            API keys are set via environment variables (
            <code className="rounded bg-amber-100 px-1 font-mono text-xs">OPENAI_API_KEY_SFA</code>,{' '}
            <code className="rounded bg-amber-100 px-1 font-mono text-xs">ANTHROPIC_API_KEY_SFA</code>,{' '}
            <code className="rounded bg-amber-100 px-1 font-mono text-xs">VOYAGE_API_KEY_SFA</code>)
            and are never stored in config.
          </span>
        </div>
      </section>

      {/* Section 2: Embeddings */}
      <CollapsibleSection
        title="Embeddings"
        description="Configure the provider and model used for text embeddings"
        open={!collapsed.embeddings}
        onToggle={() => toggleSection('embeddings')}
      >
        <div className="space-y-4">
          <div>
            <FieldLabel htmlFor="embed-provider">Provider</FieldLabel>
            <select
              id="embed-provider"
              value={draft.embeddings.provider}
              onChange={(e) =>
                updateDraft((d) => ({
                  ...d,
                  embeddings: { ...d.embeddings, provider: e.target.value },
                }))
              }
              className={SELECT_CLASS}
            >
              {EMBEDDING_PROVIDERS.map((p) => (
                <option key={p.value} value={p.value}>
                  {p.label}
                </option>
              ))}
            </select>
          </div>
          <div>
            <FieldLabel htmlFor="embed-model">Model</FieldLabel>
            <input
              id="embed-model"
              type="text"
              value={draft.embeddings.model}
              onChange={(e) =>
                updateDraft((d) => ({
                  ...d,
                  embeddings: { ...d.embeddings, model: e.target.value },
                }))
              }
              placeholder="text-embedding-3-small"
              className={INPUT_CLASS}
            />
          </div>
        </div>
      </CollapsibleSection>

      {/* Section 3: Per-Role Model Overrides */}
      <CollapsibleSection
        title="Per-Role Model Overrides"
        description="Override the global provider/model for specific agent roles"
        open={!collapsed.agentModels}
        onToggle={() => toggleSection('agentModels')}
      >
        <HelpText>
          Leave fields empty to inherit from the global provider setting above.
        </HelpText>
        <div className="mt-4 space-y-5">
          {AGENT_ROLES.map((role) => {
            const roleKey = role.key as keyof NonNullable<Config['agent_models']>;
            const roleConfig = agentModels[roleKey];
            return (
              <div
                key={role.key}
                className="rounded-md border border-slate-200 p-4"
              >
                <h3 className="text-sm font-medium text-slate-800">{role.label}</h3>
                <div className="mt-3 grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <FieldLabel htmlFor={`${role.key}-provider`}>Provider</FieldLabel>
                    <select
                      id={`${role.key}-provider`}
                      value={roleConfig?.provider ?? ''}
                      onChange={(e) =>
                        updateDraft((d) => ({
                          ...d,
                          agent_models: {
                            ...d.agent_models,
                            [role.key]: {
                              ...(d.agent_models?.[roleKey] ?? {}),
                              provider: e.target.value || undefined,
                            },
                          },
                        }))
                      }
                      className={SELECT_CLASS}
                    >
                      <option value="">Inherit global</option>
                      {PROVIDERS.map((p) => (
                        <option key={p.value} value={p.value}>
                          {p.label}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <FieldLabel htmlFor={`${role.key}-model`}>Model</FieldLabel>
                    <input
                      id={`${role.key}-model`}
                      type="text"
                      value={roleConfig?.model ?? ''}
                      onChange={(e) =>
                        updateDraft((d) => ({
                          ...d,
                          agent_models: {
                            ...d.agent_models,
                            [role.key]: {
                              ...(d.agent_models?.[roleKey] ?? {}),
                              model: e.target.value || undefined,
                            },
                          },
                        }))
                      }
                      placeholder="Inherit global"
                      className={INPUT_CLASS}
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      </CollapsibleSection>

      {/* Section 4: SFQ Configuration */}
      <CollapsibleSection
        title="SFQ Configuration"
        description="Settings for the SFQ quiz tool integration"
        open={!collapsed.sfq}
        onToggle={() => toggleSection('sfq')}
      >
        <div>
          <FieldLabel htmlFor="sfq-command">SFQ Command</FieldLabel>
          <input
            id="sfq-command"
            type="text"
            value={draft.sfq.command}
            onChange={(e) =>
              updateDraft((d) => ({ ...d, sfq: { ...d.sfq, command: e.target.value } }))
            }
            placeholder="sfq"
            className={INPUT_CLASS}
          />
          <HelpText>
            The command used to run the sfq quiz tool. Defaults to &quot;sfq&quot; (assumes
            it&apos;s in PATH).
          </HelpText>
        </div>
      </CollapsibleSection>

      {/* Section 5: Custom Prompt Context */}
      <CollapsibleSection
        title="Custom Prompt Context"
        description="Additional context appended to every AI prompt"
        open={!collapsed.prompt}
        onToggle={() => toggleSection('prompt')}
      >
        <div>
          <textarea
            value={draft.custom_prompt_context ?? ''}
            onChange={(e) =>
              updateDraft((d) => ({
                ...d,
                custom_prompt_context: e.target.value || undefined,
              }))
            }
            rows={6}
            placeholder="Enter custom prompt context..."
            className={`${INPUT_CLASS} resize-y`}
          />
          <HelpText>
            Appended to every AI prompt. Use to steer output style, add domain-specific
            instructions, or set formatting preferences.
          </HelpText>
        </div>
      </CollapsibleSection>

      {/* Section 6: Model Pricing */}
      <CollapsibleSection
        title="Model Pricing"
        description="Override built-in token pricing for cost calculations"
        open={!collapsed.pricing}
        onToggle={() => toggleSection('pricing')}
      >
        <HelpText>
          Built-in prices exist for common models. Add overrides here to customize cost
          tracking for additional or custom models.
        </HelpText>

        {/* Search */}
        {Object.keys(modelPrices).length > 5 && (
          <input
            type="text"
            value={pricingFilter}
            onChange={(e) => setPricingFilter(e.target.value)}
            placeholder="Search models..."
            className={`${INPUT_CLASS} mt-3`}
          />
        )}

        {/* Pricing table */}
        {filteredPriceEntries.length > 0 && (
          <div className="mt-4 overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-200 text-left">
                  <th className="px-3 py-2 font-medium text-slate-600">Model</th>
                  <th className="px-3 py-2 font-medium text-slate-600">Input ($/M tokens)</th>
                  <th className="px-3 py-2 font-medium text-slate-600">Output ($/M tokens)</th>
                  <th className="w-12 px-3 py-2" />
                </tr>
              </thead>
              <tbody>
                {filteredPriceEntries.map(([model, price]) => (
                  <tr key={model} className="border-b border-slate-100">
                    <td className="px-3 py-2 font-mono text-xs text-slate-700">{model}</td>
                    <td className="px-3 py-2">
                      <input
                        type="number"
                        step="0.01"
                        min="0"
                        value={price.input_per_million}
                        onChange={(e) =>
                          updateDraft((d) => ({
                            ...d,
                            model_prices: {
                              ...(d.model_prices ?? {}),
                              [model]: {
                                ...(d.model_prices?.[model] ?? {
                                  input_per_million: 0,
                                  output_per_million: 0,
                                }),
                                input_per_million: parseFloat(e.target.value) || 0,
                              },
                            },
                          }))
                        }
                        className="w-28 rounded-md border border-slate-300 px-2 py-1 text-sm tabular-nums focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none"
                      />
                    </td>
                    <td className="px-3 py-2">
                      <input
                        type="number"
                        step="0.01"
                        min="0"
                        value={price.output_per_million}
                        onChange={(e) =>
                          updateDraft((d) => ({
                            ...d,
                            model_prices: {
                              ...(d.model_prices ?? {}),
                              [model]: {
                                ...(d.model_prices?.[model] ?? {
                                  input_per_million: 0,
                                  output_per_million: 0,
                                }),
                                output_per_million: parseFloat(e.target.value) || 0,
                              },
                            },
                          }))
                        }
                        className="w-28 rounded-md border border-slate-300 px-2 py-1 text-sm tabular-nums focus:border-indigo-500 focus:ring-2 focus:ring-indigo-500 focus:outline-none"
                      />
                    </td>
                    <td className="px-3 py-2">
                      <button
                        onClick={() => removeModelPrice(model)}
                        className="rounded p-1 text-slate-400 hover:bg-red-50 hover:text-red-500"
                        title="Remove pricing override"
                      >
                        <Trash2 className="h-4 w-4" />
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {filteredPriceEntries.length === 0 && Object.keys(modelPrices).length > 0 && (
          <p className="mt-4 text-sm text-slate-400">No models match your search</p>
        )}

        {/* Add new model price */}
        <div className="mt-4 flex gap-2">
          <input
            type="text"
            value={newModelName}
            onChange={(e) => setNewModelName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') addModelPrice(newModelName);
            }}
            placeholder="Model name (e.g. gpt-5)"
            className={`${INPUT_CLASS} mt-0 flex-1`}
          />
          <button
            onClick={() => addModelPrice(newModelName)}
            disabled={!newModelName.trim()}
            className="inline-flex items-center gap-1.5 rounded-md bg-indigo-600 px-3 py-2 text-sm font-medium text-white shadow-sm hover:bg-indigo-700 disabled:opacity-40"
          >
            <Plus className="h-4 w-4" />
            Add
          </button>
        </div>
      </CollapsibleSection>
    </div>
  );
}
