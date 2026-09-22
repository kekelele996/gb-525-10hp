export type MitigationStatus = 'pending_review' | 'approved' | 'rejected' | 'invalidated'

export const mitigationLabels: Record<MitigationStatus, string> = {
  pending_review: '待复核',
  approved: '已通过',
  rejected: '已拒绝',
  invalidated: '已失效·待处理',
}

export type MeasureType = 'cleaning' | 'line_change'

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
  completed_at: string
  evidence_note: string
  mitigation_status: MitigationStatus
  created_by: number
  reviewed_by?: number
  review_reason: string
  reviewed_at?: string
  invalidated_at?: string
  created_at: string
}

export interface MitigationInfo {
  measure_id: number
  measure_type: MeasureType
  completed_at: string
  evidence_note: string
  route_version: number
  reviewed_at?: string
}
