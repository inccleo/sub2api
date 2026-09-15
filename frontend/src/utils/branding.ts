const DEFAULT_FAVICON_URL = '/logo.svg'

export function updateFavicon(): void {
  let link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'icon'
    document.head.appendChild(link)
  }

  link.type = 'image/svg+xml'
  link.href = DEFAULT_FAVICON_URL
}
