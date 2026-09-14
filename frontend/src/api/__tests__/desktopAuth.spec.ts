import { beforeEach, describe, expect, it, vi } from 'vitest'
import { apiClient } from '../client'
import { decideDesktopAuthorization } from '../desktopAuth'

vi.mock('../client', () => ({ apiClient: { post: vi.fn() } }))
const params = { client_id: 'sub2api-desktop', redirect_uri: 'sub2api://oauth/callback', response_type: 'code', scope: 'account', state: 's'.repeat(43), code_challenge: 'c'.repeat(43), code_challenge_method: 'S256' }
beforeEach(() => vi.clearAllMocks())
describe('desktop callback allowlist', () => {
  it.each([
    'https://evil.example/callback', 'javascript:alert(1)', 'sub2api://oauth@evil.example/callback',
    'sub2api://oauth/callback/extra', 'sub2api://oauth:80/callback', 'sub2api://oauth/callback#fragment',
    'sub2api://oauth/callback?state=wrong'
  ])('rejects %s', async url => {
    vi.mocked(apiClient.post).mockResolvedValue({ data: { redirect_uri: url } })
    await expect(decideDesktopAuthorization(params, 'approve')).rejects.toThrow()
  })
  it('accepts the registered native callback with matching state', async () => {
    const url = `sub2api://oauth/callback?code=code&state=${params.state}`
    vi.mocked(apiClient.post).mockResolvedValue({ data: { redirect_uri: url } })
    await expect(decideDesktopAuthorization(params, 'approve')).resolves.toBe(url)
  })
})
