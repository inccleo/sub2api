// Copy into a Tauri v2 app. Use native HTTP for postJSON and an encrypted/native
// credential store for saveRefreshToken. Never put refresh tokens in localStorage.
export interface DesktopSession {
  access_token: string
  refresh_token: string
  expires_in: number
  token_type: string
  user: { id: number; email: string; role: string }
}

interface Options {
  serverURL: string
  openUrl(url: string): Promise<unknown>
  postJSON(url: string, body: Record<string, string>): Promise<unknown>
  saveRefreshToken(token: string): Promise<void>
}

const clientID = 'sub2api-desktop'
const redirectURI = 'sub2api://oauth/callback'
const base64url = (bytes: Uint8Array) => btoa(String.fromCharCode(...bytes)).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '')
const random = () => base64url(crypto.getRandomValues(new Uint8Array(32)))

export function createDesktopLogin(options: Options) {
  const server = new URL(options.serverURL)
  if (server.protocol !== 'https:' && !(server.protocol === 'http:' && ['127.0.0.1', 'localhost', '[::1]'].includes(server.hostname))) {
    throw new Error('Use HTTPS, or a local development server')
  }
  if (server.username || server.password || server.search || server.hash || server.pathname !== '/') {
    throw new Error('serverURL must be a bare origin')
  }
  let pending: { state: string; verifier: string; expiresAt: number } | null = null
  let exchanging = false

  async function start() {
    if (exchanging) throw new Error('A sign-in exchange is already in progress')
    const attempt = { state: random(), verifier: random(), expiresAt: Date.now() + 10 * 60_000 }
    pending = attempt
    const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(attempt.verifier))
    if (pending !== attempt) return
    const url = new URL('/oauth/authorize', server)
    url.search = new URLSearchParams({
      client_id: clientID,
      redirect_uri: redirectURI,
      response_type: 'code',
      scope: 'account',
      state: attempt.state,
      code_challenge: base64url(new Uint8Array(digest)),
      code_challenge_method: 'S256'
    }).toString()
    try {
      await options.openUrl(url.toString())
    } catch (err) {
      if (pending === attempt) pending = null
      throw err
    }
  }

  async function handleCallback(raw: string): Promise<DesktopSession | null> {
    const callback = new URL(raw)
    if (callback.protocol !== 'sub2api:' || callback.host !== 'oauth' || callback.pathname !== '/callback' || callback.username || callback.password || callback.hash) return null
    if (!pending || exchanging) return null // Cold start or duplicate delivery: start sign-in again.
    if (Date.now() >= pending.expiresAt) { pending = null; throw new Error('Sign-in expired; start again') }
    const params = callback.searchParams
    if (params.getAll('state').length !== 1 || params.get('state') !== pending.state) return null
    const attempt = pending
    pending = null // Consume locally before any asynchronous work.
    if (params.get('error') === 'access_denied') throw new Error('Authorization denied')
    if (params.has('error') || params.getAll('code').length !== 1 || !/^[A-Za-z0-9_-]{43}$/.test(params.get('code') || '')) {
      throw new Error('Invalid authorization callback; start again')
    }
    exchanging = true
    try {
      const result = await options.postJSON(new URL('/api/v1/auth/desktop/token', server).toString(), {
        grant_type: 'authorization_code',
        client_id: clientID,
        redirect_uri: redirectURI,
        code: params.get('code')!,
        code_verifier: attempt.verifier
      }) as { code?: number; data?: DesktopSession }
      if (result.code !== 0 || !result.data?.access_token || !result.data.refresh_token || !result.data.expires_in) {
        throw new Error('Sign-in failed; start again')
      }
      await options.saveRefreshToken(result.data.refresh_token)
      return result.data // Keep the access token in memory.
    } finally {
      exchanging = false
    }
  }

  return { start, handleCallback, cancel: () => { pending = null } }
}
