import { webcrypto } from 'node:crypto'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createDesktopLogin } from '../../../../examples/tauri/desktop-auth'

beforeEach(() => vi.stubGlobal('crypto', webcrypto))
afterEach(() => vi.unstubAllGlobals())

async function fixture() {
  const options = {
    serverURL: 'https://test.example', openUrl: vi.fn().mockResolvedValue(undefined),
    postJSON: vi.fn().mockResolvedValue({ code: 0, data: { access_token: 'access', refresh_token: 'refresh', expires_in: 900 } }),
    saveRefreshToken: vi.fn().mockResolvedValue(undefined)
  }
  const login = createDesktopLogin(options)
  await login.start()
  const url = new URL(options.openUrl.mock.calls[0][0])
  const callback = `sub2api://oauth/callback?code=${'c'.repeat(43)}&state=${url.searchParams.get('state')}`
  return { login, options, url, callback }
}

describe('Tauri login example', () => {
  it('keeps verifier out of the browser, verifies PKCE, and consumes duplicate callbacks once', async () => {
    const { login, options, url, callback } = await fixture()
    expect(url.searchParams.has('code_verifier')).toBe(false)
    const sessions = await Promise.all([login.handleCallback(callback), login.handleCallback(callback)])
    expect(sessions.filter(Boolean)).toHaveLength(1)
    expect(options.postJSON).toHaveBeenCalledOnce()
    expect(options.saveRefreshToken).toHaveBeenCalledWith('refresh')
    const body = options.postJSON.mock.calls[0][1]
    const sum = await webcrypto.subtle.digest('SHA-256', new TextEncoder().encode(body.code_verifier))
    expect(Buffer.from(sum).toString('base64url')).toBe(url.searchParams.get('code_challenge'))
  })

  it('ignores foreign URI/state without losing the valid pending attempt', async () => {
    const { login, options, callback } = await fixture()
    expect(await login.handleCallback(callback.replace('sub2api:', 'https:'))).toBeNull()
    expect(await login.handleCallback(callback.replace('state=', 'state=wrong'))).toBeNull()
    expect(options.postJSON).not.toHaveBeenCalled()
    expect(await login.handleCallback(callback)).not.toBeNull()
  })

  it('handles denial without exchanging and rejects a cold-start callback', async () => {
    const { login, options, callback, url } = await fixture()
    await expect(login.handleCallback(`sub2api://oauth/callback?error=access_denied&state=${url.searchParams.get('state')}`)).rejects.toThrow('denied')
    expect(options.postJSON).not.toHaveBeenCalled()
    expect(await login.handleCallback(callback)).toBeNull()
    expect(await createDesktopLogin(options).handleCallback(callback)).toBeNull()
  })

  it('cancels previous transactions when a new sign-in begins', async () => {
    const { login, options, callback } = await fixture()
    await login.start()
    expect(await login.handleCallback(callback)).toBeNull()
    expect(options.postJSON).not.toHaveBeenCalled()
  })
})
