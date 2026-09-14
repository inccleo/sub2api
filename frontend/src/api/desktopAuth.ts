import { apiClient } from './client'

export interface DesktopAuthorizeRequest {
  client_id: string
  redirect_uri: string
  response_type: string
  scope: string
  state: string
  code_challenge: string
  code_challenge_method: string
}

export interface DesktopAuthorizationDetails {
  client_name: string
  redirect_uri: string
  scope: string
  email: string
  is_admin: boolean
}

export async function getDesktopAuthorization(params: DesktopAuthorizeRequest) {
  const { data } = await apiClient.get<DesktopAuthorizationDetails>('/auth/desktop/authorize', { params })
  return data
}

export async function decideDesktopAuthorization(request: DesktopAuthorizeRequest, decision: 'approve' | 'deny') {
  const { data } = await apiClient.post<{ redirect_uri: string }>('/auth/desktop/authorize', { ...request, decision })
  // Only the registered native callback is allowed, even if an API response is malformed.
  const callback = new URL(data.redirect_uri)
  if (callback.protocol !== 'sub2api:' || callback.host !== 'oauth' || callback.pathname !== '/callback' || callback.username || callback.password || callback.hash || callback.searchParams.get('state') !== request.state) {
    throw new Error('Invalid desktop callback')
  }
  return data.redirect_uri
}
