import { useCallback, useEffect, useRef, useState } from 'react';
import { Send, Loader2, MessageSquare, ChevronDown, PlusCircle, AlertTriangle } from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { streamChat, fetchClasses } from '../api/client';
import { EmptyState } from '../components/EmptyState';
import type { ChatAction, ChatMessage, ChatMode, ChatStreamEvent } from '../types';

const CHAT_MODE_OPTIONS: Array<{ value: ChatMode; label: string }> = [
  { value: 'standard', label: 'Standard' },
  { value: 'socratic', label: 'Socratic tutor' },
  { value: 'explain_back', label: 'Explain-back coach' },
];

export function ChatPage() {
  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [input, setInput] = useState('');
  const [streaming, setStreaming] = useState(false);
  const [classes, setClasses] = useState<string[]>([]);
  const [selectedClass, setSelectedClass] = useState('');
  const [mode, setMode] = useState<ChatMode>('standard');
  const messagesEndRef = useRef<HTMLDivElement | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement | null>(null);

  useEffect(() => {
    fetchClasses()
      .then((c) => {
        setClasses(c);
        if (c.length > 0) setSelectedClass(c[0]);
      })
      .catch(() => {});
  }, []);

  const scrollToBottom = useCallback(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, []);

  useEffect(() => {
    scrollToBottom();
  }, [messages, scrollToBottom]);

  const handleSend = async () => {
    const text = input.trim();
    if (!text || streaming) return;

    const userMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: 'user',
      content: text,
    };
    const assistantMsg: ChatMessage = {
      id: crypto.randomUUID(),
      role: 'assistant',
      content: '',
      actions: [],
      streaming: true,
    };

    setMessages((prev) => [...prev, userMsg, assistantMsg]);
    setInput('');
    setStreaming(true);

    const assistantId = assistantMsg.id;

    const updateAssistant = (updater: (msg: ChatMessage) => ChatMessage) => {
      setMessages((prev) =>
        prev.map((m) => (m.id === assistantId ? updater(m) : m)),
      );
    };

    try {
      await streamChat(text, selectedClass, mode, (event: ChatStreamEvent) => {
        switch (event.type) {
          case 'chunk':
            updateAssistant((m) => ({
              ...m,
              content: m.content + (event.text ?? ''),
            }));
            break;
          case 'action-start':
            updateAssistant((m) => ({
              ...m,
              actions: [
                ...(m.actions ?? []),
                { label: event.label ?? '', detail: event.detail, done: false },
              ],
            }));
            break;
          case 'action-done':
            updateAssistant((m) => ({
              ...m,
              actions: m.actions?.map((a: ChatAction) =>
                a.label === event.label ? { ...a, done: true } : a,
              ),
            }));
            break;
          case 'done':
            updateAssistant((m) => ({ ...m, streaming: false }));
            break;
          case 'error':
            updateAssistant((m) => ({
              ...m,
              content: m.content + `\n\n**Error:** ${event.error}`,
              streaming: false,
            }));
            break;
        }
      });
    } catch (err) {
      updateAssistant((m) => ({
        ...m,
        content: m.content + `\n\n**Error:** ${err instanceof Error ? err.message : 'Unknown error'}`,
        streaming: false,
      }));
    } finally {
      setStreaming(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const autoResize = () => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  };

  const handleNewChat = () => {
    if (streaming) return;
    setMessages([]);
    setInput('');
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
      textareaRef.current.focus();
    }
  };

  return (
    <div className="flex h-full flex-col">
      {/* Header bar */}
      <div className="flex items-center gap-3 border-b border-slate-200 bg-white px-4 py-3 lg:px-6">
        <h1 className="text-lg font-semibold text-slate-900">Chat</h1>
        <div className="ml-auto flex items-center gap-2">
          <div className="relative">
            <select
              value={mode}
              onChange={(e) => setMode(e.target.value as ChatMode)}
              disabled={streaming}
              className="appearance-none rounded-lg border border-slate-200 bg-white py-1.5 pl-3 pr-8 text-sm text-slate-700 focus:border-indigo-300 focus:outline-none focus:ring-2 focus:ring-indigo-100 disabled:cursor-not-allowed disabled:opacity-60"
              aria-label="Chat mode"
            >
              {CHAT_MODE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
          </div>
          {classes.length > 0 && (
            <div className="relative">
              <select
                value={selectedClass}
                onChange={(e) => setSelectedClass(e.target.value)}
                className="appearance-none rounded-lg border border-slate-200 bg-white py-1.5 pl-3 pr-8 text-sm text-slate-700 focus:border-indigo-300 focus:outline-none focus:ring-2 focus:ring-indigo-100"
              >
                {classes.map((c) => (
                  <option key={c} value={c}>
                    {c}
                  </option>
                ))}
              </select>
              <ChevronDown className="pointer-events-none absolute right-2 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" />
            </div>
          )}
          <button
            onClick={handleNewChat}
            disabled={streaming || messages.length === 0}
            className="inline-flex items-center gap-1.5 rounded-lg border border-slate-200 bg-white px-3 py-1.5 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <PlusCircle className="h-4 w-4" />
            New chat
          </button>
        </div>
      </div>

      <div className="border-b border-amber-200 bg-amber-50 px-4 py-2 lg:px-6">
        <div className="mx-auto flex max-w-3xl items-start gap-2 text-xs text-amber-800">
          <AlertTriangle className="mt-0.5 h-3.5 w-3.5 shrink-0" />
          <p>
            Chats are not saved yet. Starting a new chat or refreshing the page will clear your conversation history.
          </p>
        </div>
      </div>

      {/* Message area */}
      <div className="flex-1 overflow-y-auto">
        {messages.length === 0 ? (
          <div className="flex h-full items-center justify-center">
            <EmptyState
              icon={MessageSquare}
              title="Start a conversation"
              description="Ask questions about your study materials, request explanations, or generate quizzes."
            />
          </div>
        ) : (
          <div className="mx-auto max-w-3xl space-y-4 px-4 py-6">
            {messages.map((msg) => (
              <MessageBubble key={msg.id} message={msg} />
            ))}
            <div ref={messagesEndRef} />
          </div>
        )}
      </div>

      {/* Input area */}
      <div className="border-t border-slate-200 bg-white px-4 py-3 lg:px-6">
        <div className="mx-auto flex max-w-3xl items-end gap-2">
          <textarea
            ref={textareaRef}
            value={input}
            onChange={(e) => {
              setInput(e.target.value);
              autoResize();
            }}
            onKeyDown={handleKeyDown}
            placeholder="Ask a question..."
            rows={1}
            className="flex-1 resize-none rounded-xl border border-slate-200 px-4 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-indigo-300 focus:outline-none focus:ring-2 focus:ring-indigo-100 transition-colors"
          />
          <button
            onClick={handleSend}
            disabled={!input.trim() || streaming}
            className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-indigo-600 text-white transition-colors hover:bg-indigo-700 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            {streaming ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Send className="h-4 w-4" />
            )}
          </button>
        </div>
      </div>
    </div>
  );
}

/**
 * Remove tool call blocks from message content before displaying to user.
 * Tool calls are internal agent protocol and shouldn't be visible in the UI.
 */
function stripToolCalls(content: string): string {
  return content.replace(/<tool_call>[\s\S]*?<\/tool_call>/g, '').trim();
}

function MessageBubble({ message }: { message: ChatMessage }) {
  const isUser = message.role === 'user';

  return (
    <div className={`flex ${isUser ? 'justify-end' : 'justify-start'}`}>
      <div
        className={`max-w-[85%] rounded-2xl px-4 py-3 ${
          isUser
            ? 'bg-indigo-600 text-white'
            : 'bg-white border border-slate-200 text-slate-900 shadow-sm'
        }`}
      >
        {/* Action indicators */}
        {!isUser && message.actions && message.actions.length > 0 && (
          <div className="mb-2 space-y-1">
            {message.actions.map((action, i) => (
              <div
                key={i}
                className="flex items-center gap-2 rounded-lg bg-slate-50 px-3 py-1.5 text-xs text-slate-500"
              >
                {action.done ? (
                  <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                ) : (
                  <Loader2 className="h-3 w-3 animate-spin text-indigo-500" />
                )}
                <span className="font-medium">{action.label}</span>
                {action.detail && (
                  <span className="text-slate-400">— {action.detail}</span>
                )}
              </div>
            ))}
          </div>
        )}

        {/* Content */}
        {isUser ? (
          <p className="text-sm whitespace-pre-wrap">{message.content}</p>
        ) : (
          <div className="prose prose-sm prose-slate max-w-none prose-p:my-1 prose-headings:my-2 prose-pre:bg-slate-800 prose-pre:text-slate-100 prose-code:text-indigo-600 prose-code:before:content-none prose-code:after:content-none prose-table:text-sm">
            <Markdown remarkPlugins={[remarkGfm]}>{stripToolCalls(message.content)}</Markdown>
            {message.streaming && (
              <span className="inline-block h-4 w-1.5 animate-pulse bg-indigo-500 align-middle ml-0.5 rounded-sm" />
            )}
          </div>
        )}
      </div>
    </div>
  );
}
