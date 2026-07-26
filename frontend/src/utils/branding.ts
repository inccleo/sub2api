import { sanitizeUrl } from '@/utils/url'

const DEFAULT_FAVICON_URL = '/logo.svg'

export function updateFavicon(iconUrl: string = DEFAULT_FAVICON_URL): void {
  const sanitizedIconUrl = sanitizeUrl(iconUrl, {
    allowRelative: true,
    allowDataUrl: true,
  })
  if (!sanitizedIconUrl) {
    return
  }

  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = sanitizedIconUrl.endsWith('.svg') ? 'image/svg+xml' : 'image/x-icon'
  link.href = sanitizedIconUrl
}
