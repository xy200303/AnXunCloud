<template>
  <!-- 打卡详情抽屉：统一区块结构（标题+分隔线），证据在前结论在后，扩展新内容加区块即可 -->
  <el-drawer :model-value="modelValue" title="打卡详情" size="620px" @update:model-value="emit('update:modelValue', $event)">
    <div v-loading="loading" class="detail-body">
      <template v-if="detail && row">
        <!-- 概要 -->
        <div class="detail-header">
          <div class="detail-point">
            <span class="point-name">{{ row.point_name }}</span>
            <span class="text-secondary">{{ row.community_name }}</span>
          </div>
          <div class="detail-header-tags">
            <el-tag v-if="detail.force_submit || row.force_submit" type="warning" effect="dark">强制提交</el-tag>
            <el-tag v-if="row.is_suspect" type="warning">疑似作弊</el-tag>
            <el-tag v-else-if="row.result === 'abnormal'" type="danger">异常</el-tag>
            <el-tag v-else type="success">正常</el-tag>
          </div>
        </div>

        <div class="detail-section">
          <h4 class="section-title">基本信息</h4>
          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="打卡时间">{{ row.checkin_time }}</el-descriptions-item>
            <el-descriptions-item label="巡检员">{{ row.inspector_name }}</el-descriptions-item>
            <el-descriptions-item label="打卡方式">{{ checkinTypeLabel(row.checkin_type) }}</el-descriptions-item>
            <el-descriptions-item label="照片数">{{ row.photo_count }} 张</el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- 定位与防作弊校验 -->
        <div class="detail-section">
          <h4 class="section-title">定位与校验</h4>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="坐标">
              {{ detail.longitude.toFixed(6) }}, {{ detail.latitude.toFixed(6) }}
            </el-descriptions-item>
            <el-descriptions-item label="距点位">{{ detail.distance_to_point }}m</el-descriptions-item>
            <el-descriptions-item label="服务端时间">{{ detail.checkin_time }}</el-descriptions-item>
            <el-descriptions-item label="客户端时间">{{ detail.client_time }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.suspect_reason" label="疑似原因">
              <span class="danger-text">{{ detail.suspect_reason }}</span>
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.remark" label="备注">{{ detail.remark }}</el-descriptions-item>
          </el-descriptions>
        </div>

        <!-- AI 质量校验（同步判定；不合格时红色提示原因，误判由人工复核兜底） -->
        <div v-if="detail.ai_quality_pass !== null && detail.ai_quality_pass !== undefined" class="detail-section">
          <h4 class="section-title">AI 质量校验</h4>
          <el-alert
            v-if="detail.ai_quality_pass"
            type="success"
            :closable="false"
            show-icon
            title="照片质量校验通过"
          />
          <el-alert
            v-else
            type="error"
            :closable="false"
            show-icon
            :title="`照片质量不合格：${detail.ai_quality_issue || '未说明原因'}`"
          />
        </div>

        <!-- 检查项结果 -->
        <div v-if="detail.check_items?.length" class="detail-section">
          <h4 class="section-title">检查项结果</h4>
          <el-table :data="detail.check_items" border size="small" style="width: 100%">
            <el-table-column label="检查项" min-width="140">
              <template #default="{ row: item }">
                <div>{{ item.name }}</div>
                <div v-if="item.requirement" class="item-requirement">{{ item.requirement }}</div>
                <div v-if="item.ai_hint" class="item-ai-hint">AI 要点：{{ item.ai_hint }}</div>
                <el-tag v-if="item.exception_type === 'device_missing'" type="danger" size="small">项目异常：设备缺失</el-tag>
                <el-tag v-else-if="item.exception_type === 'unable_to_capture'" type="warning" size="small">项目异常：无法拍摄</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="结果" width="80" align="center">
              <template #default="{ row: item }">
                <el-tag :type="item.pass ? 'success' : 'danger'" size="small">
                  {{ item.pass ? '合格' : '不合格' }}
                </el-tag>
              </template>
            </el-table-column>
            <!-- 逐项 AI 初判（辅助参考，最终以人工审核为准，不阻断操作） -->
            <el-table-column label="AI 结论" min-width="130">
              <template #default="{ row: item }">
                <el-tag v-if="item.ai_verdict === 'pass'" type="success" size="small">AI 通过</el-tag>
                <span v-else-if="item.ai_verdict === 'review'" class="ai-suspect-text">
                  AI 存疑：{{ item.ai_reason || '请人工核对' }}
                </span>
                <el-tag v-else-if="item.ai_verdict === 'error'" type="info" size="small">AI 识别失败</el-tag>
                <span v-else class="text-secondary">--</span>
                <div v-if="item.ai_reading" class="ai-reading">读数：{{ item.ai_reading }}</div>
              </template>
            </el-table-column>
            <el-table-column label="照片" min-width="110">
              <template #default="{ row: item }">
                <div v-if="item.photo_urls?.length" class="item-photos">
                  <el-image
                    v-for="(url, i) in item.photo_urls"
                    :key="i"
                    :src="url"
                    :preview-src-list="item.photo_urls"
                    :initial-index="i"
                    fit="cover"
                    preview-teleported
                    class="item-photo-thumb"
                  />
                </div>
                <span v-else class="text-secondary">--</span>
              </template>
            </el-table-column>
            <el-table-column prop="note" label="备注" min-width="100" show-overflow-tooltip>
              <template #default="{ row: item }">{{ item.note || '--' }}</template>
            </el-table-column>
          </el-table>
        </div>

        <!-- 全景照片（记录级） -->
        <div class="detail-section">
          <h4 class="section-title">全景照片（水印缩略图，点击查看原图）</h4>
          <photo-viewer :photos="detail.photos || []" :meta="photoMeta(detail)" />
        </div>

        <!-- 审核信息（结论性内容置底） -->
        <div v-if="detail.audit_status" class="detail-section detail-section-last">
          <h4 class="section-title">审核信息</h4>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="审核状态">
              <el-tag :type="auditStatusTag(detail.audit_status).type" size="small">
                {{ auditStatusTag(detail.audit_status).label }}
              </el-tag>
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.ai_verdict" label="AI 结论">
              <el-tag :type="aiVerdictTag(detail.ai_verdict).type" size="small">
                {{ aiVerdictTag(detail.ai_verdict).label }}
              </el-tag>
              <span v-if="detail.ai_reason" class="ai-reason">{{ detail.ai_reason }}</span>
            </el-descriptions-item>
            <el-descriptions-item v-if="detail.audit_by_name" label="审核人">{{ detail.audit_by_name }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.audit_at" label="审核时间">{{ detail.audit_at }}</el-descriptions-item>
            <el-descriptions-item v-if="detail.audit_remark" label="打回原因">
              <span class="danger-text">{{ detail.audit_remark }}</span>
            </el-descriptions-item>
          </el-descriptions>
        </div>
      </template>
    </div>
  </el-drawer>
</template>

<script setup lang="ts">
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { CheckinItem, CheckinDetail } from '@/api/biz-types'

defineProps<{
  modelValue: boolean
  row: CheckinItem | null
  detail: CheckinDetail | null
  loading: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传', nfc: 'NFC' }[t] || t
}

// 审核状态：auto_pass 默认通过-灰 / pending 待审核-橙 / pass 人工通过-绿 / rejected 已打回-红
function auditStatusTag(s: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      auto_pass: { label: '默认通过', type: 'info' },
      pending: { label: '待审核', type: 'warning' },
      pass: { label: '人工通过', type: 'success' },
      rejected: { label: '已打回', type: 'danger' }
    }[s] || { label: s, type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

// AI 结论：pass 大模型通过 / review 转人工 / error 审核失败
function aiVerdictTag(v: string): { label: string; type: 'info' | 'warning' | 'success' | 'danger' } {
  return (
    {
      pass: { label: '大模型通过', type: 'success' },
      review: { label: '转人工', type: 'warning' },
      error: { label: '审核失败', type: 'danger' }
    }[v] || { label: v, type: 'info' }
  ) as { label: string; type: 'info' | 'warning' | 'success' | 'danger' }
}

function photoMeta(d: CheckinDetail) {
  return {
    time: d.checkin_time,
    distance: d.distance_to_point,
    coords: `${d.longitude.toFixed(6)}, ${d.latitude.toFixed(6)}`
  }
}
</script>

<style scoped lang="scss">
.detail-body {
  min-height: 200px;
}

.detail-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: $spacing-lg;

  .detail-point {
    display: flex;
    align-items: baseline;
    gap: $spacing-md;

    .point-name {
      font-size: $font-size-card-title;
      font-weight: 600;
      color: $color-text-primary;
    }
  }
}

// 统一区块：标题 + 内容 + 底部分隔线
.detail-section {
  padding-bottom: $spacing-lg;
  margin-bottom: $spacing-lg;
  border-bottom: 1px solid $color-border;

  &:last-child,
  &.detail-section-last {
    padding-bottom: 0;
    margin-bottom: 0;
    border-bottom: none;
  }
}

.section-title {
  margin: 0 0 $spacing-md;
  font-size: $font-size-body;
  font-weight: 600;
  color: $color-text-primary;
}

.danger-text {
  color: $color-danger;
}

.ai-reason {
  margin-left: $spacing-sm;
  color: $color-text-secondary;
}

.item-requirement {
  font-size: 12px;
  line-height: 1.4;
  color: $color-text-secondary;
}

// AI 识别要点（内部提示，仅管理端可见）
.item-ai-hint {
  font-size: 12px;
  line-height: 1.4;
  color: $color-text-placeholder;
}

// 逐项 AI 存疑：黄色文案提示，不阻断人工审核
.ai-suspect-text {
  font-size: 12px;
  line-height: 1.4;
  color: $color-warning;
}

.detail-header-tags {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
}

// AI 表计读数（读数类检查项）
.ai-reading {
  margin-top: 2px;
  font-size: 12px;
  line-height: 1.4;
  color: $color-text-secondary;
}

.item-photos {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-xs;

  .item-photo-thumb {
    width: 48px;
    height: 48px;
    border-radius: $radius-small;
    border: 1px solid $color-border;
    cursor: pointer;
  }
}
</style>
