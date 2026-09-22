import type { RiskLevel } from './risk'

export type MitigationStatus = 'pending' | 'pending_review' | 'approved' | 'rejected'
export type MeasureType = 'cleaning' | 'line_change'

export const mitigationLabels: Record<MitigationStatus, string> = {
  pending: '待处理',
  pending_review: '待复核',
  approved: '已缓解',
  rejected: '已拒绝',
}

export const measureTypeLabels: Record<MeasureType, string> = {
  cleaning: '清洗',
  line_change: '换线',
}

export interface MitigationMeasure {
  id: number
  route_id: number
  route_version: number
  allergen: string
  source_step_code: string
  target_step_code: string
  measure_type: MeasureType
  completed_on: string
  evidence_note: string
  raw_score: number
  risk_level: RiskLevel
  mitigation_status: MitigationStatus
  created_by: number
  reviewed_by?: number
  review_reason: string
  reviewed_at?: string
  created_at: string
  updated_at: string
}

export interface MitigationInfo {
  measure_id: number
  measure_type: MeasureType
  status: MitigationStatus
  completed_on: string
  evidence_note: string
}
