import { timestampDate } from '@bufbuild/protobuf/wkt'
import { createClient } from '@connectrpc/connect'
import { useQuery } from '@connectrpc/connect-query'
import { MagnifyingGlassIcon } from '@heroicons/react/16/solid'
import { clsx } from 'clsx'
import { Fragment, useEffect, useRef, useState } from 'react'
import { Badge } from '@/components/ui/Badge'
import { FormError } from '@/components/ui/Callout'
import { Heading } from '@/components/ui/Heading'
import { Input, InputGroup } from '@/components/ui/Input'
import PageContentBlock from '@/components/ui/PageContentBlock'
import { Select } from '@/components/ui/Select'
import { Switch } from '@/components/ui/Switch'
import { LogLevel, type LogRecord, LogService, type LogSource } from '@/gen/krill/v1/log_pb'
import { transport } from '@/lib/transport'

const client = createClient(LogService, transport)

const POLL_MS = 2000
// The API keeps at most this many per source, so keeping more here would
// only hold records it has dropped.
const MAX_RECORDS = 2000

const levels: Record<LogLevel, { label: string; color: 'zinc' | 'sky' | 'amber' | 'red' }> = {
  [LogLevel.UNSPECIFIED]: { label: '?', color: 'zinc' },
  [LogLevel.DEBUG]: { label: 'DEBUG', color: 'zinc' },
  [LogLevel.INFO]: { label: 'INFO', color: 'sky' },
  [LogLevel.WARN]: { label: 'WARN', color: 'amber' },
  [LogLevel.ERROR]: { label: 'ERROR', color: 'red' },
}

const time = new Intl.DateTimeFormat(undefined, {
  hour: '2-digit',
  minute: '2-digit',
  second: '2-digit',
  fractionalSecondDigits: 3,
  hour12: false,
})
const day = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

function sourceName(s: LogSource) {
  if (!s.worker) return `API · ${s.hostname}`
  return `Worker · ${s.hostname || s.id.slice(0, 8)}${s.online ? '' : ' (offline)'}`
}

const urlPattern = /(https?:\/\/[^\s"'<>]+)/g

function Linkified({ text }: { text: string }) {
  const parts = text.split(urlPattern)
  return (
    <>
      {parts.map((p, i) =>
        i % 2 === 1 ? (
          <a
            key={i}
            href={p}
            target="_blank"
            rel="noreferrer"
            className="text-sky-700 underline dark:text-sky-400"
          >
            {p}
          </a>
        ) : (
          <Fragment key={i}>{p}</Fragment>
        ),
      )}
    </>
  )
}

function Record({ record }: { record: LogRecord }) {
  const [open, setOpen] = useState(false)
  const at = record.time ? timestampDate(record.time) : null
  const level = levels[record.level]
  const preview = record.attrs
    .map((a) => `${a.key}=${a.value.length > 60 ? `${a.value.slice(0, 60)}…` : a.value}`)
    .join(' ')
    .replaceAll('\n', ' ')

  return (
    <li className="border-b border-zinc-950/5 dark:border-white/5">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        aria-expanded={open}
        disabled={record.attrs.length === 0}
        className="flex w-full items-baseline gap-3 px-2 py-1.5 text-left font-mono text-xs/5 enabled:hover:bg-zinc-950/2.5 dark:enabled:hover:bg-white/2.5"
      >
        <span
          className="shrink-0 text-zinc-500 tabular-nums dark:text-zinc-400"
          title={at ? day.format(at) : undefined}
        >
          {at ? time.format(at) : ''}
        </span>
        <Badge color={level.color} className="w-14 shrink-0 justify-center">
          {level.label}
        </Badge>
        <span className="min-w-0 shrink-0 text-zinc-950 dark:text-white">{record.message}</span>
        {!open && (
          <span className="min-w-0 truncate text-zinc-500 dark:text-zinc-400">{preview}</span>
        )}
      </button>
      {open && (
        <dl className="grid grid-cols-[max-content_1fr] gap-x-4 gap-y-1 px-2 pb-3 pl-30 font-mono text-xs/5">
          {record.attrs.map((a, i) => (
            <Fragment key={i}>
              <dt className="text-zinc-500 dark:text-zinc-400">{a.key}</dt>
              <dd className="min-w-0 break-words whitespace-pre-wrap text-zinc-950 dark:text-white">
                <Linkified text={a.value} />
              </dd>
            </Fragment>
          ))}
        </dl>
      )}
    </li>
  )
}

function useDebounced<T>(value: T, ms: number) {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

type Logs = { key: string; records: LogRecord[]; error: unknown; loaded: boolean }

function useLogs(sourceId: string, minLevel: LogLevel, query: string, live: boolean) {
  const key = `${sourceId}|${minLevel}|${query}`
  const [logs, setLogs] = useState<Logs>({ key, records: [], error: null, loaded: false })
  // Survives pausing, so resuming only fetches what's new.
  const last = useRef({ key, seq: 0n })

  useEffect(() => {
    let cancelled = false
    let timer: ReturnType<typeof setTimeout> | undefined
    if (last.current.key !== key) last.current = { key, seq: 0n }

    const poll = async () => {
      try {
        const res = await client.listLogs({
          sourceId,
          minLevel,
          query,
          afterSeq: last.current.seq,
        })
        if (cancelled) return
        // A restarted source starts its seqs over.
        const restarted = res.lastSeq < last.current.seq
        last.current = { key, seq: res.lastSeq }
        setLogs((prev) => {
          const kept = prev.key === key && !restarted ? prev.records : []
          return {
            key,
            records: [...kept, ...res.records].slice(-MAX_RECORDS),
            error: null,
            loaded: true,
          }
        })
      } catch (error) {
        if (!cancelled) setLogs((prev) => ({ ...prev, key, error }))
      }
      if (!cancelled && live) timer = setTimeout(() => void poll(), POLL_MS)
    }
    void poll()
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [key, sourceId, minLevel, query, live])

  return logs.key === key ? logs : { key, records: [], error: null, loaded: false }
}

export function LogsPage() {
  const [sourceId, setSourceId] = useState('api')
  const [minLevel, setMinLevel] = useState<LogLevel>(LogLevel.UNSPECIFIED)
  const [search, setSearch] = useState('')
  const [live, setLive] = useState(true)
  const query = useDebounced(search.trim(), 300)
  const sources = useQuery(LogService.method.listLogSources, {}, { refetchInterval: 10_000 })
  const { records, error, loaded } = useLogs(sourceId, minLevel, query, live)
  const newestFirst = [...records].reverse()

  return (
    <PageContentBlock title="Logs · Admin · Krill">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading>Logs</Heading>
        <div className="flex w-full flex-wrap items-center gap-3 sm:w-auto">
          <Select
            aria-label="Source"
            value={sourceId}
            onChange={(e) => setSourceId(e.target.value)}
            className="sm:w-56"
          >
            {(
              sources.data?.sources ?? [{ id: 'api', hostname: '', worker: false } as LogSource]
            ).map((s) => (
              <option key={s.id} value={s.id}>
                {sourceName(s)}
              </option>
            ))}
          </Select>
          <Select
            aria-label="Level"
            value={minLevel}
            onChange={(e) => setMinLevel(Number(e.target.value) as LogLevel)}
            className="sm:w-32"
          >
            <option value={LogLevel.UNSPECIFIED}>All levels</option>
            <option value={LogLevel.INFO}>Info</option>
            <option value={LogLevel.WARN}>Warn</option>
            <option value={LogLevel.ERROR}>Error</option>
          </Select>
          <InputGroup>
            <MagnifyingGlassIcon data-slot="icon" />
            <Input
              type="search"
              aria-label="Search"
              placeholder="Search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="sm:w-56"
            />
          </InputGroup>
          <span className="flex items-center gap-2 text-sm/6 text-zinc-700 dark:text-zinc-300">
            <Switch checked={live} onChange={setLive} aria-label="Live" />
            <span aria-hidden="true">Live</span>
          </span>
        </div>
      </div>

      <FormError error={error} className="mt-6" />
      {loaded && records.length === 0 ? (
        <p className="py-10 text-center text-sm/6 text-zinc-500 dark:text-zinc-400">
          {query || minLevel !== LogLevel.UNSPECIFIED ? 'No matching records' : 'No records yet'}
        </p>
      ) : (
        <ul
          className={clsx(
            'mt-6 border-t border-zinc-950/10 dark:border-white/10',
            !loaded && 'opacity-50',
          )}
        >
          {newestFirst.map((r) => (
            <Record key={String(r.seq)} record={r} />
          ))}
        </ul>
      )}
    </PageContentBlock>
  )
}
