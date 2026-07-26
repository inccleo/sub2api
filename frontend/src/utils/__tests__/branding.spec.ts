import { beforeEach, describe, expect, it } from 'vitest'
import { updateFavicon } from '@/utils/branding'

describe('updateFavicon', () => {
  beforeEach(() => {
    document.head.innerHTML = '<link rel="icon" href="/logo.svg">'
  })

  it('uses the dedicated square brand mark by default', () => {
    updateFavicon()

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.svg')
    expect(link?.type).toBe('image/svg+xml')
  })

  it('supports a separately configured favicon', () => {
    updateFavicon('https://example.com/custom-icon.png')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.href).toBe('https://example.com/custom-icon.png')
  })

  it('ignores unsafe logo URLs', () => {
    updateFavicon('javascript:alert(1)')

    const link = document.querySelector<HTMLLinkElement>('link[rel="icon"]')
    expect(link?.getAttribute('href')).toBe('/logo.svg')
  })
})
