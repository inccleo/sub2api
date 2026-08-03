/**
 * Admin daily check-in records API
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'

export interface DailyCheckinAdminItem {
  id: number
  user_id: number
  user_email: string
  username: string
  checkin_date: string
  base_reward: number
  bonus_reward: number
  total_reward: number
  streak_count: number
  created_at: string
}

export interface DailyCheckinAdminStats {
  today_checkins: number
  yesterday_checkins: number
  last_7_days_checkins: number
  total_checkins: number
  unique_users: number
  today_reward_total: number
  yesterday_reward_total: number
  last_7_days_reward_total: number
  total_reward: number
  total_bonus_reward: number
  today_bonus_count: number
  today_date: string
  yesterday_date: string
  server_timezone: string
}

export interface ListDailyCheckinsParams {
  page?: number
  page_size?: number
  search?: string
  user_id?: number
  start_date?: string
  end_date?: string
}

export async function list(
  page = 1,
  pageSize = 20,
  filters?: Omit<ListDailyCheckinsParams, 'page' | 'page_size'>,
  options?: { signal?: AbortSignal },
): Promise<PaginatedResponse<DailyCheckinAdminItem>> {
  const { data } = await apiClient.get<PaginatedResponse<DailyCheckinAdminItem>>('/admin/checkins', {
    params: {
      page,
      page_size: pageSize,
      ...filters,
    },
    signal: options?.signal,
  })
  return data
}

export async function getStats(options?: { signal?: AbortSignal }): Promise<DailyCheckinAdminStats> {
  const { data } = await apiClient.get<DailyCheckinAdminStats>('/admin/checkins/stats', {
    signal: options?.signal,
  })
  return data
}

const checkinsAPI = {
  list,
  getStats,
}

export default checkinsAPI
