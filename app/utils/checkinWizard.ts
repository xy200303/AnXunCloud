export type CheckItemEnumOption = { value: string; label: string; result: 'normal' | 'abnormal' }
export type CheckItemValueType = 'ok' | 'number' | 'date' | 'text' | 'enum'

export type CheckItemRef = {
  template_id: string
  item_code: string
  equipment_id?: string
}

export type WizardItemSnap = CheckItemRef & {
  name: string
  standard: string
  guide: string
  value_type: CheckItemValueType
  unit: string
  value_min?: number | null
  value_max?: number | null
  enum_options: CheckItemEnumOption[]
  value_num: number | null
  value_date: string
  value_text: string
  note_required: boolean
  ai_enabled: boolean
  builtin_kind: string
  tags: string[]
  abnormal_tags: string[]
  auto_judge?: import('@/services/api').EquipmentAutoJudge | null
  photo_required?: string
  photos: string[]
  file_ids: string[]
  file_id?: string
  job_id: string
  status: 'todo' | 'recognizing' | 'done' | 'failed'
  verdict: string
  reason: string
  reading: string
  exception_type?: string
  quality_pass: boolean
  quality_issue: string
  img_error?: boolean
  pass: boolean
  manual_confirmed?: boolean
  note: string
  pending_local?: string
  pending_mode?: '' | 'ai' | 'escape' | 'manual'
  pending_exception_type?: string
  shoot_lng?: number
  shoot_lat?: number
  shoot_at?: string
}

export type WizardPointSnap = {
  point_id: string
  status: 'doing' | 'submitted'
  scannedNo: string
  nfcCardId: string
  cred_type?: string
  cred_verified_at?: string
  fence_distance?: number
  items: WizardItemSnap[]
}

export function itemRef(item: CheckItemRef): CheckItemRef {
  return {
    template_id: item.template_id || '',
    item_code: item.item_code,
    ...(item.equipment_id ? { equipment_id: item.equipment_id } : {})
  }
}

export function itemKey(item: CheckItemRef): string {
  return [item.template_id || '', item.item_code, item.equipment_id || ''].join(':')
}

export function isManualItem(item: Pick<WizardItemSnap, 'ai_enabled' | 'builtin_kind'>): boolean {
  return !item.ai_enabled && item.builtin_kind === ''
}

export function isValidityItem(item: Pick<WizardItemSnap, 'builtin_kind'>): boolean {
  return item.builtin_kind === 'equipment_validity'
}

export function isSpotItem(item: Pick<WizardItemSnap, 'builtin_kind'>): boolean {
  return item.builtin_kind === 'equipment_date_spot'
}

export function isTypedValueItem(item: Pick<WizardItemSnap, 'value_type'>): boolean {
  return item.value_type !== 'ok'
}
