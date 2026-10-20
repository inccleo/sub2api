import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { listPlatformIds } from '@/constants/platformCatalog'

describe('Composite channel platform options', () => {
  it('includes the CN concrete providers for pricing and model mapping', () => {
    const source = readFileSync(resolve('src/views/admin/ChannelsView.vue'), 'utf8')
    expect(source).toContain('const platformOrder = computed<GroupPlatform[]>(() => listPlatformIds())')
    expect(source).toContain('const compositePlatforms = platformOrder')

    expect(listPlatformIds()).toEqual(expect.arrayContaining([
      'kimi', 'zhipu', 'deepseek', 'minimax', 'opencode_go', 'typesafe'
    ]))
  })
})
