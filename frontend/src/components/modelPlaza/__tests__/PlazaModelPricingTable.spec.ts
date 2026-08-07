import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import PlazaModelPricingTable from '../PlazaModelPricingTable.vue'
import type { PlazaModel } from '@/api/modelPlaza'

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function tokenModel(overrides: Partial<PlazaModel> = {}): PlazaModel {
  return {
    name: 'claude-sonnet',
    platform: 'anthropic',
    pricing: {
      billing_mode: 'token',
      input_price: 3e-6,
      output_price: 1.5e-5,
      cache_write_price: 3.75e-6,
      cache_read_price: 3e-7,
      image_input_price: null,
      image_output_price: null,
      per_request_price: null,
      intervals: []
    },
    official_pricing: {
      input_price: 3e-6,
      output_price: 1.5e-5,
      cache_write_price: 3.75e-6,
      cache_write_1h_price: 6e-6,
      cache_read_price: 3e-7
    },
    ...overrides
  }
}

function mountTable(
  models: PlazaModel[],
  rateMultiplier: number,
  userRateMultiplier?: number | null,
  extraProps?: { imageRateIndependent?: boolean; imageRateMultiplier?: number | null }
) {
  return mount(PlazaModelPricingTable, {
    props: { models, rateMultiplier, userRateMultiplier: userRateMultiplier ?? null, ...extraProps }
  })
}

describe('PlazaModelPricingTable', () => {
  it('倍率为 1 时按固定汇率 7 折算美元展示价', () => {
    const wrapper = mountTable([tokenModel()], 1)
    const cells = wrapper.findAll('tbody td')
    expect(cells[1].text()).toBe('$0.4286')
    expect(cells[2].text()).toBe('$2.1429')
    expect(cells[3].text()).toContain('$0.5357')
    expect(cells[3].text()).toContain('$0.0429')
    expect(cells[7].text()).toBe('0.1429x')
    expect(wrapper.text()).toContain('modelPlaza.table.exchangeNote')
  })

  it('倍率 ≠ 1 时折合价格同时应用倍率和汇率,官方价保持原价', () => {
    const wrapper = mountTable([tokenModel()], 0.5)
    const cells = wrapper.findAll('tbody td')
    expect(cells[1].text()).toBe('$0.2143')
    expect(cells[2].text()).toBe('$1.0714')
    expect(cells[4].text()).toBe('$3.00')
    expect(cells[5].text()).toBe('$15.00')
    expect(cells[7].text()).toBe('0.0714x')
  })

  it('用户专属倍率覆盖分组倍率,并划线展示原倍率', () => {
    const wrapper = mountTable([tokenModel()], 1, 0.8)
    const text = wrapper.text()
    expect(text).toContain('$0.3429')
    expect(text).toContain('$1.7143')
    const struck = wrapper.find('td .line-through')
    expect(struck.exists()).toBe(true)
    expect(struck.text()).toBe('0.1429x')
    expect(text).toContain('0.1143x')
  })

  it('模型按官方输出价从高到低排序,无官方价的排最后', () => {
    const expensive = tokenModel({
      name: 'model-expensive',
      official_pricing: {
        input_price: 1e-5,
        output_price: 7.5e-5,
        cache_write_price: null,
        cache_write_1h_price: null,
        cache_read_price: null
      }
    })
    const cheap = tokenModel({
      name: 'model-cheap',
      official_pricing: {
        input_price: 1e-6,
        output_price: 5e-6,
        cache_write_price: null,
        cache_write_1h_price: null,
        cache_read_price: null
      }
    })
    const noOfficial = tokenModel({ name: 'model-no-official', official_pricing: null })

    const wrapper = mountTable([cheap, noOfficial, expensive], 1)
    const names = wrapper.findAll('tbody tr').map((tr) => tr.find('td').text())
    expect(names).toEqual(['model-expensive', 'model-cheap', 'model-no-official'])
  })

  it('官方输出价相同时按模型名降序(新版本号在前)', () => {
    const older = tokenModel({ name: 'gpt-5.5' })
    const newer = tokenModel({ name: 'gpt-5.6-sol' })

    const wrapper = mountTable([older, newer], 1)
    const names = wrapper.findAll('tbody tr').map((tr) => tr.find('td').text())
    expect(names).toEqual(['gpt-5.6-sol', 'gpt-5.5'])
  })

  it('按图片/按次计费的模型沉到末尾,不与 token 模型按官方价混排', () => {
    // 官方输出价 $10,介于下面两个 token 模型之间,但因计费模式不同应排最后
    const image = tokenModel({
      name: 'gpt-image-2',
      pricing: {
        billing_mode: 'image',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: 0.002,
        intervals: []
      },
      official_pricing: {
        input_price: 5e-6,
        output_price: 1e-5,
        cache_write_price: null,
        cache_write_1h_price: null,
        cache_read_price: 1.25e-6
      }
    })
    const pricier = tokenModel({
      name: 'gpt-5.6-terra',
      official_pricing: {
        input_price: 2.5e-6,
        output_price: 1.5e-5,
        cache_write_price: null,
        cache_write_1h_price: null,
        cache_read_price: null
      }
    })
    const cheaper = tokenModel({
      name: 'gpt-5.6-luna',
      official_pricing: {
        input_price: 1e-6,
        output_price: 6e-6,
        cache_write_price: null,
        cache_write_1h_price: null,
        cache_read_price: null
      }
    })

    const wrapper = mountTable([pricier, image, cheaper], 1)
    const names = wrapper.findAll('tbody tr').map((tr) => tr.find('td').text())
    expect(names[0]).toBe('gpt-5.6-terra')
    expect(names[1]).toBe('gpt-5.6-luna')
    // 首列含「按图片计费」徽章文本,只断言模型名
    expect(names[2]).toContain('gpt-image-2')
  })

  it('两级表头:实付区与官方区各拆输入/输出/缓存列', () => {
    const wrapper = mountTable([tokenModel()], 1)
    const text = wrapper.text()
    expect(text).toContain('modelPlaza.table.paidPrice')
    expect(text).toContain('modelPlaza.table.officialPrice')
    // token 行:模型 + 实付 3 列 + 官方 3 列 + 倍率
    expect(wrapper.findAll('tbody td')).toHaveLength(8)
  })

  it('官方价包含 1h 缓存写入价;official_pricing 为 null 时官方三列显示 -', () => {
    const withOfficial = mountTable([tokenModel()], 1)
    expect(withOfficial.text()).toContain('$6.00')
    expect(withOfficial.text()).toContain('(1h')

    const withoutOfficial = mountTable([tokenModel({ official_pricing: null })], 1)
    const cells = withoutOfficial.findAll('tbody td')
    // 官方 输入/输出/缓存 三列均为 -
    expect(cells[4].text().trim()).toBe('-')
    expect(cells[5].text().trim()).toBe('-')
    expect(cells[6].text().trim()).toBe('-')
  })

  it('per_request 模型按单次价 × 倍率展示,官方价列显示 -', () => {
    const model = tokenModel({
      name: 'search-tool',
      pricing: {
        billing_mode: 'per_request',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: 0.04,
        intervals: []
      },
      official_pricing: null
    })
    const wrapper = mountTable([model], 0.5)
    const text = wrapper.text()
    expect(text).toContain('$0.002857')
    expect(text).toContain('modelPlaza.table.perRequest')
    // 单位后缀跟在价格后(按次 → / 次)
    expect(text).toContain('modelPlaza.table.perUnitRequest')
  })

  it('token 模型阶梯定价内联进输入/输出列,按倍率折算', () => {
    const model = tokenModel({
      pricing: {
        billing_mode: 'token',
        input_price: 3e-6,
        output_price: 1.5e-5,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: null,
        intervals: [
          {
            min_tokens: 0,
            max_tokens: 200000,
            tier_label: '',
            input_price: 3e-6,
            output_price: 1.5e-5,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null
          },
          {
            min_tokens: 200000,
            max_tokens: null,
            tier_label: '',
            input_price: 6e-6,
            output_price: 3e-5,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: null
          }
        ]
      }
    })
    const wrapper = mountTable([model], 0.5)
    const text = wrapper.text()
    // 区间标签按 token 数生成
    expect(text).toContain('≤200K')
    expect(text).toContain('>200K')
    expect(text).toContain('$0.2143')
    expect(text).toContain('$0.4286')
    expect(text).toContain('$1.0714')
    expect(text).toContain('$2.1429')
  })

  it('生图独立倍率开启时,按图价格 × 独立倍率,不乘分组倍率;倍率列展示独立倍率', () => {
    const model = tokenModel({
      name: 'gpt-image-2',
      pricing: {
        billing_mode: 'image',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: null,
        intervals: [
          {
            min_tokens: 0,
            max_tokens: null,
            tier_label: '1K',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: 0.02
          }
        ]
      },
      official_pricing: null
    })
    const wrapper = mountTable([model], 0.1, null, {
      imageRateIndependent: true,
      imageRateMultiplier: 1
    })
    const text = wrapper.text()
    // 0.02 × 1(独立倍率) ÷ 7 展示汇率,而非 0.02 × 0.1
    expect(text).toContain('$0.002857')
    // 倍率列展示综合折扣(独立倍率 ÷ 7),而非分组倍率 0.1x
    const rateCell = wrapper.findAll('tbody tr td').at(-1)!
    expect(rateCell.text()).toBe('0.1429x')
  })

  it('生图独立倍率关闭时,按图价格仍乘分组/专属生效倍率', () => {
    const model = tokenModel({
      name: 'gpt-image-2',
      pricing: {
        billing_mode: 'image',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        image_output_price: null,
        per_request_price: 0.2,
        intervals: []
      },
      official_pricing: null
    })
    const wrapper = mountTable([model], 0.1, null, { imageRateIndependent: false })
    const text = wrapper.text()
    // 0.2 × 0.1 ÷ 7
    expect(text).toContain('$0.002857')
    const rateCell = wrapper.findAll('tbody tr td').at(-1)!
    expect(rateCell.text()).toBe('0.0143x')
  })

  it('按图模型主行展示阶梯芯片,不把 image_output_price(每 token)当按次价', () => {
    const model = tokenModel({
      name: 'gpt-image-2',
      pricing: {
        billing_mode: 'image',
        input_price: null,
        output_price: null,
        cache_write_price: null,
        cache_read_price: null,
        image_input_price: null,
        // 每 token 图片输出价:不应被当作按次单价展示
        image_output_price: 3e-5,
        per_request_price: null,
        intervals: [
          {
            min_tokens: 0,
            max_tokens: null,
            tier_label: '1K',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: 0.01
          },
          {
            min_tokens: 0,
            max_tokens: null,
            tier_label: '2K',
            input_price: null,
            output_price: null,
            cache_write_price: null,
            cache_read_price: null,
            per_request_price: 0.02
          }
        ]
      },
      official_pricing: null
    })
    const wrapper = mountTable([model], 0.1)
    const text = wrapper.text()
    expect(text).toContain('modelPlaza.table.perImage')
    // 芯片按倍率和汇率折算，单位后缀内嵌(按图 → / 张)
    expect(text).toContain('1K')
    expect(text).toContain('$0.000143')
    expect(text).toContain('2K')
    expect(text).toContain('$0.000286')
    expect(text).toContain('modelPlaza.table.perUnitImage')
    // 旧 bug:image_output_price × 0.1 = 0.000003 被当按次价
    expect(text).not.toContain('$0.000003')
  })

  it('Composite 分组中相同模型名按具体平台分别展示徽章', () => {
    const anthropic = tokenModel({ name: 'shared-model', platform: 'anthropic' })
    const openai = tokenModel({ name: 'shared-model', platform: 'openai' })
    const wrapper = mount(PlazaModelPricingTable, {
      props: {
        models: [anthropic, openai],
        platform: 'composite',
        rateMultiplier: 1
      }
    })

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows.map((row) => row.find('td').text())).toEqual([
      'shared-modelAnthropic',
      'shared-modelOpenAI'
    ])
    expect(wrapper.text()).toContain('Anthropic')
    expect(wrapper.text()).toContain('OpenAI')
  })
})
