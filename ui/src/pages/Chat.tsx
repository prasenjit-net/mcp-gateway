import { useState, useRef, useEffect, useCallback } from 'react'
import OpenAI from 'openai'
import { createMCPClient, type MCPClient, type Transport } from '../lib/mcp-client'
import { Send, Settings, Eye, EyeOff, ChevronDown, ChevronRight, Plus, Loader2 } from 'lucide-react'
import { cn } from '../lib/utils'

interface Message {
  id: string
  role: 'user' | 'assistant' | 'tool_call' | 'tool_result'
  content: string
  toolName?: string
  toolArgs?: Record<string, unknown>
  isStreaming?: boolean
}

function ToolCallBlock({ name, args, expanded: initExpanded = false }: { name: string; args: Record<string, unknown>; expanded?: boolean }) {
  const [expanded, setExpanded] = useState(initExpanded)
  return (
    <div className="bg-gray-800 border border-gray-700 rounded-lg overflow-hidden text-sm">
      <button onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 px-3 py-2 w-full text-left hover:bg-gray-700 transition-colors">
        {expanded ? <ChevronDown className="w-3.5 h-3.5 text-orange-400" /> : <ChevronRight className="w-3.5 h-3.5 text-orange-400" />}
        <span className="text-orange-400 font-mono text-xs">tool_call</span>
        <span className="text-gray-300 font-medium">{name}</span>
      </button>
      {expanded && (
        <pre className="px-3 pb-3 text-gray-400 text-xs overflow-auto">{JSON.stringify(args, null, 2)}</pre>
      )}
    </div>
  )
}

function ToolResultBlock({ content }: { content: string }) {
  const [expanded, setExpanded] = useState(false)
  const isLong = content.length > 200
  return (
    <div className="bg-gray-800 border border-gray-700 rounded-lg overflow-hidden text-sm">
      <button onClick={() => setExpanded(!expanded)}
        className="flex items-center gap-2 px-3 py-2 w-full text-left hover:bg-gray-700 transition-colors">
        {expanded ? <ChevronDown className="w-3.5 h-3.5 text-green-400" /> : <ChevronRight className="w-3.5 h-3.5 text-green-400" />}
        <span className="text-green-400 font-mono text-xs">tool_result</span>
        {!expanded && isLong && <span className="text-gray-500 text-xs">{content.slice(0, 80)}…</span>}
      </button>
      {(expanded || !isLong) && (
        <pre className="px-3 pb-3 text-gray-400 text-xs overflow-auto whitespace-pre-wrap">{content}</pre>
      )}
    </div>
  )
}

function MessageBubble({ msg }: { msg: Message }) {
  if (msg.role === 'tool_call') {
    return <div className="mx-4 my-1"><ToolCallBlock name={msg.toolName!} args={msg.toolArgs ?? {}} /></div>
  }
  if (msg.role === 'tool_result') {
    return <div className="mx-4 my-1"><ToolResultBlock content={msg.content} /></div>
  }
  if (msg.role === 'user') {
    return (
      <div className="flex justify-end px-4 my-2">
        <div className="max-w-[75%] bg-blue-600 text-white rounded-2xl rounded-tr-sm px-4 py-2.5 text-sm whitespace-pre-wrap">{msg.content}</div>
      </div>
    )
  }
  return (
    <div className="flex justify-start px-4 my-2">
      <div className="max-w-[75%] bg-gray-800 text-gray-100 rounded-2xl rounded-tl-sm px-4 py-2.5 text-sm whitespace-pre-wrap">
        {msg.content}
        {msg.isStreaming && <span className="inline-block w-1.5 h-4 bg-blue-400 ml-1 animate-pulse rounded-sm" />}
      </div>
    </div>
  )
}

const MODELS = ['gpt-4o', 'gpt-4-turbo', 'gpt-4', 'gpt-3.5-turbo']

export default function Chat() {
  const [apiKey, setApiKey] = useState(() => localStorage.getItem('chat_api_key') ?? '')
  const [model, setModel] = useState(() => localStorage.getItem('chat_model') ?? 'gpt-4o')
  const [transport, setTransport] = useState<Transport>(() => (localStorage.getItem('chat_transport') as Transport) ?? 'http')
  const [systemPrompt, setSystemPrompt] = useState(() => localStorage.getItem('chat_system_prompt') ?? 'You are a helpful assistant with access to tools.')
  const [showKey, setShowKey] = useState(false)
  const [settingsOpen, setSettingsOpen] = useState(true)
  const [messages, setMessages] = useState<Message[]>([])
  const [input, setInput] = useState('')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const messagesEndRef = useRef<HTMLDivElement>(null)
  const mcpClientRef = useRef<MCPClient | null>(null)
  const idRef = useRef(0)

  const nextId = () => String(++idRef.current)

  useEffect(() => { messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' }) }, [messages])

  const saveSettings = () => {
    localStorage.setItem('chat_api_key', apiKey)
    localStorage.setItem('chat_model', model)
    localStorage.setItem('chat_transport', transport)
    localStorage.setItem('chat_system_prompt', systemPrompt)
  }

  const getMCPClient = useCallback(async () => {
    if (!mcpClientRef.current) {
      mcpClientRef.current = await createMCPClient(transport)
    }
    return mcpClientRef.current
  }, [transport])

  useEffect(() => {
    mcpClientRef.current?.disconnect()
    mcpClientRef.current = null
  }, [transport])

  const appendMessage = (msg: Message) => {
    setMessages(prev => [...prev, msg])
    return msg.id
  }

  const updateMessage = (id: string, updates: Partial<Message>) => {
    setMessages(prev => prev.map(m => m.id === id ? { ...m, ...updates } : m))
  }

  const handleSend = async () => {
    if (!input.trim() || loading) return
    if (!apiKey) { setError('Please set your OpenAI API key in settings'); return }
    setError('')

    const userText = input.trim()
    setInput('')
    setLoading(true)

    appendMessage({ id: nextId(), role: 'user', content: userText })

    try {
      const client = await getMCPClient()
      const tools = await client.listTools()

      const openaiClient = new OpenAI({ apiKey, dangerouslyAllowBrowser: true })

      const history: OpenAI.ChatCompletionMessageParam[] = [
        { role: 'system', content: systemPrompt },
        ...messages.filter(m => m.role === 'user' || m.role === 'assistant').map(m => ({
          role: m.role as 'user' | 'assistant',
          content: m.content,
        })),
        { role: 'user', content: userText },
      ]

      const openaiTools: OpenAI.ChatCompletionTool[] = tools.map(t => ({
        type: 'function' as const,
        function: {
          name: t.name,
          description: t.description,
          parameters: t.inputSchema as OpenAI.FunctionParameters,
        },
      }))

      let continueLoop = true
      const loopHistory = [...history]

      while (continueLoop) {
        const assistantId = nextId()
        appendMessage({ id: assistantId, role: 'assistant', content: '', isStreaming: true })

        let assistantContent = ''

        const stream = await openaiClient.chat.completions.create({
          model,
          messages: loopHistory,
          tools: openaiTools.length > 0 ? openaiTools : undefined,
          stream: true,
        })

        const toolCallsMap: Record<number, { id: string; name: string; args: string }> = {}

        for await (const chunk of stream) {
          const delta = chunk.choices[0]?.delta
          if (delta?.content) {
            assistantContent += delta.content
            updateMessage(assistantId, { content: assistantContent })
          }
          if (delta?.tool_calls) {
            for (const tc of delta.tool_calls) {
              const idx = tc.index
              if (!toolCallsMap[idx]) toolCallsMap[idx] = { id: tc.id ?? '', name: '', args: '' }
              if (tc.id) toolCallsMap[idx].id = tc.id
              if (tc.function?.name) toolCallsMap[idx].name += tc.function.name
              if (tc.function?.arguments) toolCallsMap[idx].args += tc.function.arguments
            }
          }
        }

        updateMessage(assistantId, { content: assistantContent, isStreaming: false })

        const finalToolCalls = Object.values(toolCallsMap)
        if (finalToolCalls.length === 0) {
          continueLoop = false
          loopHistory.push({ role: 'assistant', content: assistantContent })
        } else {
          const openaiToolCalls: OpenAI.ChatCompletionMessageToolCall[] = finalToolCalls.map(tc => ({
            id: tc.id,
            type: 'function' as const,
            function: { name: tc.name, arguments: tc.args },
          }))
          loopHistory.push({ role: 'assistant', content: assistantContent || null, tool_calls: openaiToolCalls } as OpenAI.ChatCompletionMessageParam)

          for (const tc of finalToolCalls) {
            let args: Record<string, unknown> = {}
            try { args = JSON.parse(tc.args) as Record<string, unknown> } catch {}

            appendMessage({ id: nextId(), role: 'tool_call', content: '', toolName: tc.name, toolArgs: args })

            let result = ''
            try {
              result = await client.callTool(tc.name, args)
            } catch (e: unknown) {
              result = `Error: ${e instanceof Error ? e.message : String(e)}`
            }

            appendMessage({ id: nextId(), role: 'tool_result', content: result })
            loopHistory.push({ role: 'tool', tool_call_id: tc.id, content: result })
          }
        }
      }
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Unknown error'
      setError(msg)
      appendMessage({ id: nextId(), role: 'assistant', content: `Error: ${msg}` })
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="flex h-full">
      <div className={cn('border-r border-gray-800 bg-gray-900 flex flex-col transition-all duration-200',
        settingsOpen ? 'w-72' : 'w-12')}>
        <button onClick={() => setSettingsOpen(!settingsOpen)}
          className="flex items-center gap-2 p-3 text-gray-400 hover:text-white transition-colors border-b border-gray-800">
          <Settings className="w-5 h-5 flex-shrink-0" />
          {settingsOpen && <span className="text-sm font-medium">Settings</span>}
        </button>
        {settingsOpen && (
          <div className="flex-1 overflow-y-auto p-4 space-y-4">
            <div>
              <label className="text-xs text-gray-400 block mb-1">OpenAI API Key</label>
              <div className="relative">
                <input
                  type={showKey ? 'text' : 'password'}
                  value={apiKey}
                  onChange={e => setApiKey(e.target.value)}
                  placeholder="sk-..."
                  className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm pr-9 focus:outline-none focus:border-blue-500"
                />
                <button onClick={() => setShowKey(!showKey)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-gray-400 hover:text-white">
                  {showKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                </button>
              </div>
            </div>
            <div>
              <label className="text-xs text-gray-400 block mb-1">Model</label>
              <select value={model} onChange={e => setModel(e.target.value)}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500">
                {MODELS.map(m => <option key={m} value={m}>{m}</option>)}
              </select>
            </div>
            <div>
              <label className="text-xs text-gray-400 block mb-2">Transport</label>
              <div className="flex gap-2">
                {(['sse', 'http'] as Transport[]).map(t => (
                  <button key={t} onClick={() => setTransport(t)}
                    className={cn('flex-1 py-1.5 text-sm rounded-lg border transition-colors',
                      transport === t ? 'bg-blue-600 border-blue-500 text-white' : 'bg-gray-800 border-gray-700 text-gray-400 hover:text-white')}>
                    {t.toUpperCase()}
                  </button>
                ))}
              </div>
            </div>
            <div>
              <label className="text-xs text-gray-400 block mb-1">System Prompt</label>
              <textarea value={systemPrompt} onChange={e => setSystemPrompt(e.target.value)}
                rows={4}
                className="w-full bg-gray-800 border border-gray-700 rounded-lg px-3 py-2 text-white text-sm resize-none focus:outline-none focus:border-blue-500" />
            </div>
            <button onClick={saveSettings}
              className="w-full bg-blue-600 hover:bg-blue-700 text-white rounded-lg py-2 text-sm font-medium transition-colors">
              Save Settings
            </button>
          </div>
        )}
      </div>

      <div className="flex-1 flex flex-col">
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-800">
          <span className="text-sm text-gray-400">MCP Chat — {model} via {transport.toUpperCase()}</span>
          <button onClick={() => { setMessages([]); mcpClientRef.current?.disconnect(); mcpClientRef.current = null }}
            className="flex items-center gap-1.5 text-sm text-gray-400 hover:text-white transition-colors">
            <Plus className="w-4 h-4" /> New Chat
          </button>
        </div>

        <div className="flex-1 overflow-y-auto py-4">
          {messages.length === 0 && (
            <div className="text-center text-gray-500 mt-20">
              <p className="text-lg font-medium">MCP Tool Tester</p>
              <p className="text-sm mt-1">Ask anything — tools from connected MCP servers are available</p>
            </div>
          )}
          {messages.map(msg => <MessageBubble key={msg.id} msg={msg} />)}
          <div ref={messagesEndRef} />
        </div>

        {error && (
          <div className="mx-4 mb-2 px-3 py-2 bg-red-900/30 border border-red-800 text-red-400 text-sm rounded-lg">{error}</div>
        )}

        <div className="p-4 border-t border-gray-800">
          <div className="flex gap-3 items-end">
            <textarea
              value={input}
              onChange={e => setInput(e.target.value)}
              onKeyDown={e => { if (e.key === 'Enter' && !e.shiftKey) { e.preventDefault(); void handleSend() } }}
              placeholder="Message (Enter to send, Shift+Enter for newline)"
              rows={2}
              disabled={loading}
              className="flex-1 bg-gray-800 border border-gray-700 rounded-xl px-4 py-3 text-white text-sm resize-none focus:outline-none focus:border-blue-500 disabled:opacity-50"
            />
            <button onClick={() => { void handleSend() }} disabled={loading || !input.trim()}
              className="p-3 bg-blue-600 hover:bg-blue-700 disabled:bg-gray-700 text-white rounded-xl transition-colors">
              {loading ? <Loader2 className="w-5 h-5 animate-spin" /> : <Send className="w-5 h-5" />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
