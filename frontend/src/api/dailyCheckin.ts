import { apiClient } from './client'

export interface DailyCheckinStatus {
  enabled: boolean
  checked_in_today: boolean
  daily_reward: number
  weekly_bonus: number
  current_streak: number
  days_until_bonus: number
  today_reward?: number
  server_timezone: string
}

export interface DailyCheckinResult {
  reward_amount: number
  base_reward: number
  bonus_reward: number
  new_balance: number
  current_streak: number
  checked_in_at: string
}

export interface DailyCheckinHistoryItem {
  id: number
  checkin_date: string
  base_reward: number
  bonus_reward: number
  total_reward: number
  streak_count: number
  created_at: string
}

export const dailyCheckinAPI = {
  async getStatus(): Promise<DailyCheckinStatus> {
    const { data } = await apiClient.get<DailyCheckinStatus>('/checkin/status')
    return data
  },

  async checkin(): Promise<DailyCheckinResult> {
    const { data } = await apiClient.post<DailyCheckinResult>('/checkin')
    return data
  },

  async getHistory(limit = 30): Promise<DailyCheckinHistoryItem[]> {
    const { data } = await apiClient.get<DailyCheckinHistoryItem[]>('/checkin/history', {
      params: { limit },
    })
    return data
  },
}
