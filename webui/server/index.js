import http from 'node:http'
import https from 'node:https'
import fs from 'node:fs'
import path from 'node:path'
import crypto from 'node:crypto'
import { fileURLToPath } from 'node:url'
import mysql from 'mysql2/promise'
import bcrypt from 'bcryptjs'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const distDir = path.resolve(__dirname, '../dist')

const HOST = process.env.WEBUI_HOST || '0.0.0.0'
const PORT = Number.parseInt(process.env.WEBUI_PORT || '8080', 10)
const SOCKET_PATH = process.env.RAIND_UI_SOCKET_PATH || '/run/raind/ui.sock'
const TLS_CERT_PATH = process.env.RAIND_WEBUI_TLS_CERT || '/etc/raind/cert/web/raindWeb.crt'
const TLS_KEY_PATH = process.env.RAIND_WEBUI_TLS_KEY || '/etc/raind/cert/web/raindWeb.key'
const AUDIT_LOG_PATH = process.env.RAIND_AUDIT_LOG_PATH || '/var/log/raind/raind_audit.jsonl'
const NETFLOW_LOG_PATH = process.env.RAIND_NETFLOW_LOG_PATH || '/var/log/raind/raind_netflow.jsonl'
const DNS_LOG_PATH = process.env.RAIND_DNS_LOG_PATH || '/var/log/raind/raind_dns.jsonl'
const DB_HOST = process.env.RAIND_WEBUI_DB_HOST || '127.0.0.1'
const DB_PORT = Number.parseInt(process.env.RAIND_WEBUI_DB_PORT || '3306', 10)
const DB_USER = process.env.RAIND_WEBUI_DB_USER || 'raind_webui'
const DB_PASSWORD = process.env.RAIND_WEBUI_DB_PASSWORD || ''
const DB_NAME = process.env.RAIND_WEBUI_DB_NAME || 'raind_webui'
const SESSION_TTL_SECONDS = Number.parseInt(process.env.RAIND_WEBUI_SESSION_TTL || '28800', 10)
const AUTH_COOKIE_NAME = 'raind_webui_session'

const sessionStore = new Map()
let dbPool
let dbInitialized = false

function sendJson(res, status, body) {
  const payload = JSON.stringify(body)
  res.writeHead(status, {
    'Content-Type': 'application/json; charset=utf-8',
    'Content-Length': Buffer.byteLength(payload)
  })
  res.end(payload)
}

function parseCookies(cookieHeader) {
  const out = {}
  const raw = String(cookieHeader || '')
  for (const token of raw.split(';')) {
    const s = token.trim()
    if (!s) continue
    const idx = s.indexOf('=')
    if (idx < 0) continue
    const key = decodeURIComponent(s.slice(0, idx).trim())
    const value = decodeURIComponent(s.slice(idx + 1).trim())
    out[key] = value
  }
  return out
}

function createSession(username) {
  const id = crypto.randomBytes(32).toString('hex')
  const expiresAt = Date.now() + SESSION_TTL_SECONDS * 1000
  sessionStore.set(id, { username, expiresAt })
  return { id, expiresAt }
}

function cleanupExpiredSessions() {
  const now = Date.now()
  for (const [sid, v] of sessionStore.entries()) {
    if (!v || Number(v.expiresAt || 0) <= now) sessionStore.delete(sid)
  }
}

function getSessionFromRequest(req) {
  cleanupExpiredSessions()
  const cookies = parseCookies(req.headers.cookie)
  const sid = cookies[AUTH_COOKIE_NAME]
  if (!sid) return null
  const s = sessionStore.get(sid)
  if (!s) return null
  if (Number(s.expiresAt || 0) <= Date.now()) {
    sessionStore.delete(sid)
    return null
  }
  return { sid, ...s }
}

function setSessionCookie(res, sid, secure) {
  const maxAge = SESSION_TTL_SECONDS
  const attrs = [
    `${AUTH_COOKIE_NAME}=${encodeURIComponent(sid)}`,
    'Path=/',
    `Max-Age=${maxAge}`,
    'HttpOnly',
    'SameSite=Lax'
  ]
  if (secure) attrs.push('Secure')
  res.setHeader('Set-Cookie', attrs.join('; '))
}

function clearSessionCookie(res, secure) {
  const attrs = [
    `${AUTH_COOKIE_NAME}=`,
    'Path=/',
    'Max-Age=0',
    'HttpOnly',
    'SameSite=Lax'
  ]
  if (secure) attrs.push('Secure')
  res.setHeader('Set-Cookie', attrs.join('; '))
}

function readRequestBody(req) {
  return new Promise((resolve, reject) => {
    let data = ''
    req.on('data', (chunk) => {
      data += chunk
      if (data.length > 1024 * 1024) {
        reject(new Error('request body too large'))
      }
    })
    req.on('error', reject)
    req.on('end', () => resolve(data))
  })
}

async function ensureDb() {
  if (!dbPool) {
    dbPool = mysql.createPool({
      host: DB_HOST,
      port: DB_PORT,
      user: DB_USER,
      password: DB_PASSWORD,
      database: DB_NAME,
      connectionLimit: 5
    })
  }

  // Keep retrying initialization until success.
  if (!dbInitialized) {
    await dbPool.query(`
      CREATE TABLE IF NOT EXISTS webui_users (
        id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
        username VARCHAR(128) NOT NULL UNIQUE,
        password_hash VARCHAR(255) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
      ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4
    `)

    const bootstrapUser = String(process.env.RAIND_WEBUI_BOOTSTRAP_USER || '').trim()
    const bootstrapPass = String(process.env.RAIND_WEBUI_BOOTSTRAP_PASSWORD || '')
    if (bootstrapUser && bootstrapPass) {
      const [rows] = await dbPool.query('SELECT COUNT(*) AS cnt FROM webui_users')
      const count = Number(rows?.[0]?.cnt || 0)
      if (count === 0) {
        const hash = await bcrypt.hash(bootstrapPass, 10)
        await dbPool.query('INSERT INTO webui_users (username, password_hash) VALUES (?, ?)', [bootstrapUser, hash])
        console.log(`raind-webui bootstrap user created: ${bootstrapUser}`)
      }
    }
    dbInitialized = true
  }
  return dbPool
}

function contentTypeOf(filePath) {
  if (filePath.endsWith('.html')) return 'text/html; charset=utf-8'
  if (filePath.endsWith('.js')) return 'application/javascript; charset=utf-8'
  if (filePath.endsWith('.css')) return 'text/css; charset=utf-8'
  if (filePath.endsWith('.json')) return 'application/json; charset=utf-8'
  if (filePath.endsWith('.svg')) return 'image/svg+xml'
  if (filePath.endsWith('.png')) return 'image/png'
  if (filePath.endsWith('.jpg') || filePath.endsWith('.jpeg')) return 'image/jpeg'
  return 'application/octet-stream'
}

function serveStatic(req, res) {
  const urlObj = new URL(req.url || '/', 'http://localhost')
  const pathname = urlObj.pathname || '/'
  const normalized = path.normalize(pathname)
  const relPath = normalized.replace(/^[/\\]+/, '')

  let target = path.resolve(distDir, relPath || 'index.html')
  if (target !== distDir && !target.startsWith(distDir + path.sep)) {
    res.writeHead(403)
    res.end('forbidden')
    return
  }

  const exists = fs.existsSync(target)
  const isDir = exists ? fs.statSync(target).isDirectory() : false

  // SPA fallback: route paths without extension should serve index.html.
  if (!exists || isDir) {
    if (!path.extname(relPath) || relPath === '') {
      target = path.join(distDir, 'index.html')
    }
  }

  fs.readFile(target, (err, data) => {
    if (err) {
      res.writeHead(404)
      res.end('not found')
      return
    }
    res.writeHead(200, { 'Content-Type': contentTypeOf(target) })
    res.end(data)
  })
}

async function handleAuth(req, res) {
  const urlObj = new URL(req.url || '/auth/me', 'http://localhost')
  const pathname = urlObj.pathname

  if (pathname === '/auth/me' && req.method === 'GET') {
    const session = getSessionFromRequest(req)
    if (!session) {
      sendJson(res, 401, { status: 'fail', message: 'unauthorized' })
      return true
    }
    sendJson(res, 200, {
      status: 'success',
      data: {
        username: session.username
      }
    })
    return true
  }

  if (pathname === '/auth/logout' && req.method === 'POST') {
    const session = getSessionFromRequest(req)
    if (session?.sid) sessionStore.delete(session.sid)
    clearSessionCookie(res, req.socket.encrypted === true)
    sendJson(res, 200, { status: 'success' })
    return true
  }

  if (pathname === '/auth/login' && req.method === 'POST') {
    try {
      const raw = await readRequestBody(req)
      let body = {}
      try {
        body = raw ? JSON.parse(raw) : {}
      } catch {
        sendJson(res, 400, { status: 'fail', message: 'invalid json' })
        return true
      }
      const username = String(body.username || '').trim()
      const password = String(body.password || '')
      if (!username || !password) {
        sendJson(res, 400, { status: 'fail', message: 'username and password are required' })
        return true
      }
      const pool = await ensureDb()
      const [rows] = await pool.query(
        'SELECT username, password_hash FROM webui_users WHERE username = ? LIMIT 1',
        [username]
      )
      const row = rows?.[0]
      if (!row) {
        sendJson(res, 401, { status: 'fail', message: 'invalid credentials' })
        return true
      }
      const ok = await bcrypt.compare(password, String(row.password_hash || ''))
      if (!ok) {
        sendJson(res, 401, { status: 'fail', message: 'invalid credentials' })
        return true
      }
      const session = createSession(username)
      setSessionCookie(res, session.id, req.socket.encrypted === true)
      sendJson(res, 200, { status: 'success', data: { username } })
      return true
    } catch (err) {
      sendJson(res, 500, { status: 'fail', message: err.message || 'login failed' })
      return true
    }
  }

  return false
}

function requireAuth(req, res) {
  const session = getSessionFromRequest(req)
  if (!session) {
    sendJson(res, 401, { status: 'fail', message: 'unauthorized' })
    return false
  }
  return true
}

function isAuthenticated(req) {
  return Boolean(getSessionFromRequest(req))
}

function parsePositiveInt(value, fallback, min, max) {
  const n = Number.parseInt(String(value || ''), 10)
  if (!Number.isFinite(n)) return fallback
  return Math.max(min, Math.min(max, n))
}

function loadAuditJsonl(text) {
  const lines = String(text || '')
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter(Boolean)

  let parseErrors = 0
  const rows = []
  for (const line of lines) {
    try {
      rows.push(JSON.parse(line))
    } catch {
      parseErrors += 1
    }
  }
  return { rows, parseErrors }
}

function normalizeTimestampString(value) {
  let out = String(value || '').trim()
  if (!out) return ''
  out = out.replace(/([+-]\d{2})(\d{2})$/, '$1:$2')
  out = out.replace(/\.(\d{3})\d+([+-]\d{2}:\d{2}|Z)$/i, '.$1$2')
  return out
}

function parseTimestampMs(value) {
  const normalized = normalizeTimestampString(value)
  if (!normalized) return Number.NaN
  const ts = Date.parse(normalized)
  return Number.isFinite(ts) ? ts : Number.NaN
}

function parseTimezoneOffsetMinutes(value) {
  const normalized = normalizeTimestampString(value)
  if (!normalized) return null
  if (normalized.endsWith('Z')) return 0
  const m = normalized.match(/([+-])(\d{2}):(\d{2})$/)
  if (!m) return null
  const sign = m[1] === '-' ? -1 : 1
  const hh = Number.parseInt(m[2], 10)
  const mm = Number.parseInt(m[3], 10)
  if (!Number.isFinite(hh) || !Number.isFinite(mm)) return null
  return sign * (hh * 60 + mm)
}

function hourBucketStartMs(ts, tzOffsetMinutes) {
  const shiftMs = tzOffsetMinutes * 60 * 1000
  const shifted = ts + shiftMs
  const bucketShifted = Math.floor(shifted / (60 * 60 * 1000)) * 60 * 60 * 1000
  return bucketShifted - shiftMs
}

function formatHourLabel(ts, tzOffsetMinutes) {
  const shifted = ts + tzOffsetMinutes * 60 * 1000
  const d = new Date(shifted)
  const hh = String(d.getUTCHours()).padStart(2, '0')
  const mm = String(d.getUTCMinutes()).padStart(2, '0')
  return `${hh}:${mm}`
}

function buildHourlySeries(nowMs, tzOffsetMinutes) {
  const currentHour = hourBucketStartMs(nowMs, tzOffsetMinutes)
  const startHour = currentHour - 23 * 60 * 60 * 1000
  const rows = []
  for (let i = 0; i < 24; i += 1) {
    const ts = startHour + i * 60 * 60 * 1000
    const label = formatHourLabel(ts, tzOffsetMinutes)
    rows.push({ ts, label, total: 0, allow: 0, deny: 0, error: 0, ok: 0, ng: 0 })
  }
  return rows
}

function inc(map, key) {
  const k = key || 'unknown'
  map[k] = (map[k] || 0) + 1
}

function readJsonlFile(sourcePath, cb) {
  fs.readFile(sourcePath, 'utf8', (err, text) => {
    if (err) {
      if (err.code === 'ENOENT') {
        cb(null, { rows: [], parseErrors: 0, sourcePath })
        return
      }
      cb(err)
      return
    }
    const { rows, parseErrors } = loadAuditJsonl(text)
    cb(null, { rows, parseErrors, sourcePath })
  })
}

function filterLastHours(rows, nowMs, hours) {
  const rangeStart = nowMs - hours * 60 * 60 * 1000
  return rows
    .map((row) => {
      const ts = parseTimestampMs(row.generated_ts)
      return { row, ts }
    })
    .filter((entry) => Number.isFinite(entry.ts) && entry.ts >= rangeStart && entry.ts <= nowMs)
}

function matchesTextQuery(q, ...parts) {
  const query = String(q || '').trim().toLowerCase()
  if (!query) return true
  const hay = parts
    .map((v) => String(v || ''))
    .join(' ')
    .toLowerCase()
  return hay.includes(query)
}

function filterTrafficEntries(entries, filters) {
  const q = String(filters.q || '').trim()
  const kind = String(filters.trafficKind || '').trim().toLowerCase()
  const verdict = String(filters.verdict || '').trim().toLowerCase()
  const proto = String(filters.proto || '').trim().toUpperCase()

  return entries.filter((e) => {
    const row = e.row || {}
    const rowKind = String(row.kind || '').toLowerCase()
    const rowVerdict = String(row.verdict || '').toLowerCase()
    const rowProto = String(row.proto || '').toUpperCase()
    if (kind && rowKind !== kind) return false
    if (verdict && rowVerdict !== verdict) return false
    if (proto && rowProto !== proto) return false

    return matchesTextQuery(
      q,
      row?.src?.ip,
      row?.src?.port,
      row?.src?.container_name,
      row?.src?.container_id,
      row?.dst?.ip,
      row?.dst?.port,
      row?.dst?.container_name,
      row?.dst?.container_id,
      row?.rule_hint,
      row?.proto,
      row?.kind,
      row?.verdict
    )
  })
}

function filterDnsEntries(entries, filters) {
  const q = String(filters.q || '').trim()
  const result = String(filters.result || '').trim().toLowerCase()
  const rcode = String(filters.rcode || '').trim().toUpperCase()
  const cache = String(filters.cache || '').trim().toLowerCase()

  return entries.filter((e) => {
    const row = e.row || {}
    const rowResult = String(row.query_result || '').toLowerCase()
    const rowRcode = String(row?.dns?.response?.rcode || '').toUpperCase()
    const rowCache = row?.cache?.hit === true ? 'hit' : 'miss'
    if (result && rowResult !== result) return false
    if (rcode && rowRcode !== rcode) return false
    if (cache && rowCache !== cache) return false

    return matchesTextQuery(
      q,
      row?.dns?.question?.name,
      row?.dns?.question?.type,
      row?.src?.container_name,
      row?.src?.container_id,
      row?.src?.ip,
      row?.dst?.container_name,
      row?.dst?.container_id,
      row?.dst?.ip,
      row?.upstream?.server,
      row?.query_result,
      row?.dns?.response?.rcode
    )
  })
}

function detectTimezoneOffsetMinutes(entries, allRows) {
  const fromEntries = entries
    .map((e) => parseTimezoneOffsetMinutes(e.row?.generated_ts))
    .find((v) => v != null)
  if (fromEntries != null) return fromEntries
  const fromAll = allRows.map((r) => parseTimezoneOffsetMinutes(r?.generated_ts)).find((v) => v != null)
  if (fromAll != null) return fromAll
  return 0
}

function aggregateTraffic(entries, nowMs, tzOffsetMinutes) {
  const series = buildHourlySeries(nowMs, tzOffsetMinutes)
  const verdict = {}
  const proto = {}
  const kind = {}
  const topContainers = {}

  for (const e of entries) {
    const idx = Math.min(23, Math.max(0, Math.floor((e.ts - series[0].ts) / (60 * 60 * 1000))))
    const slot = series[idx]
    slot.total += 1

    const v = String(e.row.verdict || 'unknown').toLowerCase()
    if (v === 'allow') slot.allow += 1
    else if (v === 'deny') slot.deny += 1
    else slot.error += 1

    inc(verdict, v)
    inc(proto, String(e.row.proto || 'unknown').toUpperCase())
    inc(kind, String(e.row.kind || 'unknown').toLowerCase())

    const containerName = String(e.row?.dst?.container_name || '-')
    if (containerName && containerName !== '-') inc(topContainers, containerName)
  }

  const topDstContainers = Object.entries(topContainers)
    .map(([name, count]) => ({ name, count }))
    .sort((a, b) => b.count - a.count)
    .slice(0, 5)

  return {
    summary: {
      total: entries.length,
      allow: verdict.allow || 0,
      deny: verdict.deny || 0,
      error: entries.length - (verdict.allow || 0) - (verdict.deny || 0),
      verdict,
      proto,
      kind,
      top_dst_containers: topDstContainers
    },
    series: series.map((s) => ({
      hour: s.label,
      total: s.total,
      allow: s.allow,
      deny: s.deny,
      error: s.error
    }))
  }
}

function aggregateDns(entries, nowMs, tzOffsetMinutes) {
  const series = buildHourlySeries(nowMs, tzOffsetMinutes)
  const queryResult = {}
  const rcode = {}
  const transport = {}
  let cacheHit = 0
  let cacheMiss = 0

  for (const e of entries) {
    const idx = Math.min(23, Math.max(0, Math.floor((e.ts - series[0].ts) / (60 * 60 * 1000))))
    const slot = series[idx]
    slot.total += 1

    const result = String(e.row.query_result || 'unknown').toLowerCase()
    if (result === 'ok') slot.ok += 1
    else slot.ng += 1

    inc(queryResult, result)
    inc(rcode, String(e.row?.dns?.response?.rcode || 'unknown').toUpperCase())
    inc(transport, String(e.row?.network?.transport || 'unknown').toLowerCase())

    if (e.row?.cache?.hit === true) cacheHit += 1
    else cacheMiss += 1
  }

  return {
    summary: {
      total: entries.length,
      ok: queryResult.ok || 0,
      ng: entries.length - (queryResult.ok || 0),
      query_result: queryResult,
      rcode,
      transport,
      cache: {
        hit: cacheHit,
        miss: cacheMiss
      }
    },
    series: series.map((s) => ({
      hour: s.label,
      total: s.total,
      ok: s.ok,
      ng: s.ng
    }))
  }
}

function paginateEntries(entries, page, pageSize) {
  const newestFirst = [...entries].sort((a, b) => b.ts - a.ts)
  const total = newestFirst.length
  const totalPages = total === 0 ? 0 : Math.ceil(total / pageSize)
  const begin = (page - 1) * pageSize
  const end = begin + pageSize
  const items = begin >= total ? [] : newestFirst.slice(begin, end).map((entry) => entry.row)
  return { items, total, totalPages }
}

function serveNetworkLogs(req, res) {
  const urlObj = new URL(req.url || '/api/network/logs', 'http://localhost')
  const kind = String(urlObj.searchParams.get('kind') || 'traffic').toLowerCase()
  const page = parsePositiveInt(urlObj.searchParams.get('page'), 1, 1, 1000000)
  const pageSize = parsePositiveInt(urlObj.searchParams.get('page_size'), 50, 1, 200)
  const hours = parsePositiveInt(urlObj.searchParams.get('hours'), 24, 1, 168)
  const filters = {
    q: urlObj.searchParams.get('q') || '',
    trafficKind: urlObj.searchParams.get('traffic_kind') || '',
    verdict: urlObj.searchParams.get('verdict') || '',
    proto: urlObj.searchParams.get('proto') || '',
    result: urlObj.searchParams.get('result') || '',
    rcode: urlObj.searchParams.get('rcode') || '',
    cache: urlObj.searchParams.get('cache') || ''
  }

  const sourcePath = kind === 'dns' ? DNS_LOG_PATH : NETFLOW_LOG_PATH
  readJsonlFile(sourcePath, (err, result) => {
    if (err) {
      sendJson(res, 500, { status: 'fail', message: `failed to read network log: ${err.message}` })
      return
    }

    const nowMs = Date.now()
    const entries = filterLastHours(result.rows, nowMs, hours)
    const filteredEntries = kind === 'dns' ? filterDnsEntries(entries, filters) : filterTrafficEntries(entries, filters)
    const { items, total, totalPages } = paginateEntries(filteredEntries, page, pageSize)
    const tzOffsetMinutes = detectTimezoneOffsetMinutes(filteredEntries, result.rows)
    const aggregated =
      kind === 'dns'
        ? aggregateDns(filteredEntries, nowMs, tzOffsetMinutes)
        : aggregateTraffic(filteredEntries, nowMs, tzOffsetMinutes)

    sendJson(res, 200, {
      status: 'success',
      data: {
        kind,
        timezone_offset_minutes: tzOffsetMinutes,
        window_hours: hours,
        series_timezone_applied: true,
        applied_filters: filters,
        page,
        page_size: pageSize,
        total,
        total_pages: totalPages,
        parse_errors: result.parseErrors,
        source: sourcePath,
        summary: aggregated.summary,
        series: aggregated.series,
        items
      }
    })
  })
}

function serveAuditLogs(req, res) {
  const urlObj = new URL(req.url || '/api/audit/logs', 'http://localhost')
  const page = parsePositiveInt(urlObj.searchParams.get('page'), 1, 1, 1000000)
  const pageSize = parsePositiveInt(urlObj.searchParams.get('page_size'), 50, 1, 200)
  const hours = 24
  const filters = {
    q: urlObj.searchParams.get('q') || '',
    actor: urlObj.searchParams.get('actor') || '',
    severity: urlObj.searchParams.get('severity') || '',
    action: urlObj.searchParams.get('action') || '',
    method: urlObj.searchParams.get('method') || '',
    result_status: urlObj.searchParams.get('result_status') || ''
  }

  fs.readFile(AUDIT_LOG_PATH, 'utf8', (err, text) => {
    if (err) {
      if (err.code === 'ENOENT') {
        sendJson(res, 200, {
          status: 'success',
          data: {
            items: [],
            page,
            page_size: pageSize,
            total: 0,
            total_pages: 0,
            parse_errors: 0,
            source: AUDIT_LOG_PATH
          }
        })
        return
      }
      sendJson(res, 500, { status: 'fail', message: `failed to read audit log: ${err.message}` })
      return
    }

    const { rows, parseErrors } = loadAuditJsonl(text)
    const nowMs = Date.now()
    const recentRows = filterLastHours(rows, nowMs, hours).map((entry) => entry.row)
    const severity = String(filters.severity || '').toLowerCase()
    const actor = String(filters.actor || '')
    const action = String(filters.action || '').toLowerCase()
    const method = String(filters.method || '').toUpperCase()
    const resultStatus = String(filters.result_status || '').toLowerCase()
    const q = String(filters.q || '')
    const actorOptions = Array.from(
      new Set(
        recentRows
          .map((row) => String(row?.actor?.spiffe_id || row?.actor?.peer_ip || '').trim())
          .filter(Boolean)
      )
    ).sort((a, b) => a.localeCompare(b))

    const filtered = recentRows.filter((row) => {
      const rowActor = String(row?.actor?.spiffe_id || row?.actor?.peer_ip || '').trim()
      const rowSeverity = String(row?.severity || '').toLowerCase()
      const rowAction = String(row?.action || '').toLowerCase()
      const rowMethod = String(row?.request?.method || '').toUpperCase()
      const rowResult = String(row?.result?.status || '').toLowerCase()
      if (actor && rowActor !== actor) return false
      if (severity && rowSeverity !== severity) return false
      if (action && rowAction !== action) return false
      if (method && rowMethod !== method) return false
      if (resultStatus && rowResult !== resultStatus) return false

      return matchesTextQuery(
        q,
        row?.event_id,
        row?.correlation_id,
        row?.severity,
        row?.action,
        row?.actor?.spiffe_id,
        row?.actor?.peer_ip,
        row?.request?.method,
        row?.request?.path,
        row?.result?.status,
        row?.result?.code
      )
    })

    const newestFirst = filtered.reverse()
    const total = newestFirst.length
    const totalPages = total === 0 ? 0 : Math.ceil(total / pageSize)
    const begin = (page - 1) * pageSize
    const end = begin + pageSize
    const items = begin >= total ? [] : newestFirst.slice(begin, end)

    sendJson(res, 200, {
      status: 'success',
      data: {
        items,
        page,
        page_size: pageSize,
        window_hours: hours,
        total,
        total_pages: totalPages,
        parse_errors: parseErrors,
        source: AUDIT_LOG_PATH,
        actor_options: actorOptions,
        applied_filters: filters
      }
    })
  })
}

function serveAuditActors(req, res) {
  const urlObj = new URL(req.url || '/api/audit/actors', 'http://localhost')
  const hours = parsePositiveInt(urlObj.searchParams.get('hours'), 24, 1, 168)

  fs.readFile(AUDIT_LOG_PATH, 'utf8', (err, text) => {
    if (err) {
      if (err.code === 'ENOENT') {
        sendJson(res, 200, {
          status: 'success',
          data: {
            items: [],
            window_hours: hours,
            source: AUDIT_LOG_PATH
          }
        })
        return
      }
      sendJson(res, 500, { status: 'fail', message: `failed to read audit log: ${err.message}` })
      return
    }

    const { rows } = loadAuditJsonl(text)
    const nowMs = Date.now()
    const recentRows = filterLastHours(rows, nowMs, hours).map((entry) => entry.row)
    const items = Array.from(
      new Set(
        recentRows
          .map((row) => String(row?.actor?.spiffe_id || row?.actor?.peer_ip || '').trim())
          .filter(Boolean)
      )
    ).sort((a, b) => a.localeCompare(b))

    sendJson(res, 200, {
      status: 'success',
      data: {
        items,
        window_hours: hours,
        source: AUDIT_LOG_PATH
      }
    })
  })
}

function serveAuditSummary(req, res) {
  const urlObj = new URL(req.url || '/api/audit/summary', 'http://localhost')
  const hours = parsePositiveInt(urlObj.searchParams.get('hours'), 24, 1, 168)

  fs.readFile(AUDIT_LOG_PATH, 'utf8', (err, text) => {
    if (err) {
      if (err.code === 'ENOENT') {
        sendJson(res, 200, {
          status: 'success',
          data: {
            window_hours: hours,
            total: 0,
            allow: 0,
            deny: 0,
            error: 0,
            severity: {},
            parse_errors: 0,
            source: AUDIT_LOG_PATH
          }
        })
        return
      }
      sendJson(res, 500, { status: 'fail', message: `failed to read audit log: ${err.message}` })
      return
    }

    const { rows, parseErrors } = loadAuditJsonl(text)
    const nowMs = Date.now()
    const entries = filterLastHours(rows, nowMs, hours)
    const severity = {}
    let allow = 0
    let deny = 0
    let error = 0
    for (const e of entries) {
      const row = e.row || {}
      const status = String(row?.result?.status || '').toLowerCase()
      if (status === 'allow') allow += 1
      else if (status === 'deny') deny += 1
      else error += 1
      inc(severity, String(row?.severity || 'unknown').toLowerCase())
    }

    sendJson(res, 200, {
      status: 'success',
      data: {
        window_hours: hours,
        total: entries.length,
        allow,
        deny,
        error,
        severity,
        parse_errors: parseErrors,
        source: AUDIT_LOG_PATH
      }
    })
  })
}

function proxyToUds(req, res) {
  if (req.url === '/api/healthz') {
    const hreq = http.request(
      {
        socketPath: SOCKET_PATH,
        path: '/healthz',
        method: 'GET'
      },
      (hres) => {
        let data = ''
        hres.on('data', (c) => {
          data += c
        })
        hres.on('end', () => {
          sendJson(res, hres.statusCode || 502, {
            status: hres.statusCode === 200 ? 'success' : 'fail',
            message: data.trim() || 'gateway health'
          })
        })
      }
    )
    hreq.on('error', (err) => sendJson(res, 502, { status: 'fail', message: err.message }))
    hreq.end()
    return
  }

  const targetPath = req.url.replace(/^\/api/, '') || '/'
  const upstream = http.request(
    {
      socketPath: SOCKET_PATH,
      path: targetPath,
      method: req.method,
      headers: {
        'Content-Type': req.headers['content-type'] || 'application/json'
      }
    },
    (upstreamRes) => {
      res.writeHead(upstreamRes.statusCode || 502, {
        'Content-Type': upstreamRes.headers['content-type'] || 'application/json; charset=utf-8'
      })
      upstreamRes.pipe(res)
    }
  )

  upstream.on('error', (err) => {
    sendJson(res, 502, { status: 'fail', message: err.message })
  })

  req.pipe(upstream)
}

const tlsOptions = {
  cert: fs.readFileSync(TLS_CERT_PATH),
  key: fs.readFileSync(TLS_KEY_PATH)
}

const server = https.createServer(tlsOptions, async (req, res) => {
  if (!req.url) {
    res.writeHead(400)
    res.end('bad request')
    return
  }

  if (req.url.startsWith('/auth/')) {
    const handled = await handleAuth(req, res)
    if (handled) return
  }

  if (req.url.startsWith('/api/audit/logs')) {
    if (!requireAuth(req, res)) return
    serveAuditLogs(req, res)
    return
  }

  if (req.url.startsWith('/api/audit/actors')) {
    if (!requireAuth(req, res)) return
    serveAuditActors(req, res)
    return
  }

  if (req.url.startsWith('/api/audit/summary')) {
    if (!requireAuth(req, res)) return
    serveAuditSummary(req, res)
    return
  }

  if (req.url.startsWith('/api/network/logs')) {
    if (!requireAuth(req, res)) return
    serveNetworkLogs(req, res)
    return
  }

  if (req.url.startsWith('/api/')) {
    if (!requireAuth(req, res)) return
    proxyToUds(req, res)
    return
  }

  serveStatic(req, res)
})

server.on('upgrade', (req, socket, head) => {
  if (!req.url || !req.url.startsWith('/api/')) {
    socket.write('HTTP/1.1 404 Not Found\r\n\r\n')
    socket.destroy()
    return
  }
  if (!isAuthenticated(req)) {
    socket.write('HTTP/1.1 401 Unauthorized\r\n\r\n')
    socket.destroy()
    return
  }

  const targetPath = req.url.replace(/^\/api/, '') || '/'
  const upstream = http.request({
    socketPath: SOCKET_PATH,
    path: targetPath,
    method: 'GET',
    headers: req.headers
  })

  upstream.on('upgrade', (upRes, upSocket, upHead) => {
    const statusLine = `HTTP/1.1 ${upRes.statusCode || 101} ${upRes.statusMessage || 'Switching Protocols'}\r\n`
    socket.write(statusLine)
    for (const [k, v] of Object.entries(upRes.headers)) {
      if (Array.isArray(v)) {
        for (const item of v) socket.write(`${k}: ${item}\r\n`)
      } else if (v !== undefined) {
        socket.write(`${k}: ${v}\r\n`)
      }
    }
    socket.write('\r\n')

    if (head && head.length) upSocket.write(head)
    if (upHead && upHead.length) socket.write(upHead)

    socket.pipe(upSocket)
    upSocket.pipe(socket)
  })

  upstream.on('response', () => {
    socket.write('HTTP/1.1 400 Bad Request\r\n\r\nwebsocket upgrade required')
    socket.destroy()
  })

  upstream.on('error', (err) => {
    if (!socket.destroyed) {
      socket.write(`HTTP/1.1 502 Bad Gateway\r\n\r\n${err.message}`)
      socket.destroy()
    }
  })

  upstream.end()
})

server.listen(PORT, HOST, () => {
  console.log(`raind-webui listening on https://${HOST}:${PORT}`)
  console.log(`raind-webui tls cert: ${TLS_CERT_PATH}`)
  console.log(`raind-webui tls key: ${TLS_KEY_PATH}`)
  console.log(`raind-webui proxy socket: ${SOCKET_PATH}`)
  ensureDb()
    .then(() => {
      console.log(`raind-webui auth db ready: ${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}`)
    })
    .catch((err) => {
      console.warn(`raind-webui auth db init failed: ${err.message}`)
    })
})
