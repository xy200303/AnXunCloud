<template>
  <div class="app-container" v-loading="loading">
    <template v-if="detail">
      <!-- 任务头 -->
      <div class="table-card task-head">
        <div class="task-head-main">
          <h2 class="page-title task-title">
            {{ detail.task.community_name }} · {{ detail.task.plan_name }} · {{ detail.task.inspector_name }} · {{ detail.task.task_date }}
          </h2>
          <el-tag :type="headStatusType" size="small">{{ headStatusLabel }}</el-tag>
          <el-tag size="small" effect="plain">{{ patrolTypeLabel(detail.task.patrol_type) }}</el-tag>
        </div>
        <div class="task-head-progress">
          <el-progress :percentage="detail.task.progress" :stroke-width="12" />
          <span class="progress-text">进度 {{ detail.task.done_points }}/{{ detail.task.total_points }}</span>
        </div>
        <div class="text-secondary">
          执行时段 {{ detail.task.time_window }}
          <template v-if="detail.task.started_at"> · 开始 {{ detail.task.started_at.slice(11, 16) }}</template>
          <template v-if="detail.task.finished_at"> · 完成 {{ detail.task.finished_at.slice(11, 16) }}</template>
        </div>
      </div>

      <!-- 点位时间线（按路线顺序，六类状态图标+颜色双编码；大规模任务分页加载） -->
      <div class="table-card" v-loading="timelineLoading">
        <h3 class="card-title">点位时间线</h3>
        <el-timeline class="point-timeline">
          <el-timeline-item
            v-for="p in detail.points"
            :key="p.point_id"
            :type="pointVisual(p).type"
            :icon="pointVisual(p).icon"
            :hollow="p.status === 'pending'"
            size="large"
          >
            <div class="point-row">
              <span class="point-time">{{ p.checkin ? p.checkin.checkin_time.slice(11, 16) : '--:--' }}</span>
              <span class="point-name">{{ p.point_name }}</span>
              <span class="text-secondary">{{ p.building_name }}</span>

              <template v-if="p.checkin">
                <span class="text-secondary">{{ checkinTypeLabel(p.checkin.checkin_type) }}</span>
                <span class="text-secondary">距点位 {{ p.checkin.distance_to_point }}m</span>
                <el-tooltip v-if="p.checkin.is_suspect" :content="p.checkin.suspect_reason" placement="top">
                  <el-tag type="warning" size="small">疑似作弊</el-tag>
                </el-tooltip>
                <el-tag v-if="p.checkin.result === 'abnormal'" type="danger" size="small">异常</el-tag>
                <el-tag v-if="p.checkin.checkin_type === 'offline'" type="info" size="small">补传</el-tag>
                <el-button
                  v-if="p.checkin.photos?.length"
                  link
                  type="primary"
                  @click="openPhotos(p)"
                >照片（{{ p.checkin.photos.length }}）</el-button>
                <span v-if="p.checkin.remark" class="text-secondary">备注：{{ p.checkin.remark }}</span>
              </template>
              <template v-else-if="p.status === 'doing'">
                <el-tag type="warning" size="small">巡检中 {{ p.item_done }}/{{ p.item_total }} 项</el-tag>
                <el-tag v-if="p.item_recognizing" type="primary" size="small" effect="plain">AI 识别中 {{ p.item_recognizing }}</el-tag>
                <el-tag v-if="p.item_failed" type="danger" size="small" effect="plain">待重拍 {{ p.item_failed }}</el-tag>
              </template>
              <el-tag v-else type="info" size="small">未打卡</el-tag>
            </div>
          </el-timeline-item>
        </el-timeline>

        <div class="timeline-pager" v-if="detail.points_total > detail.points_size">
          <el-pagination
            background
            layout="prev, pager, next, total"
            :total="detail.points_total"
            :page-size="detail.points_size"
            :current-page="pointsPage"
            @current-change="onPageChange"
          />
        </div>

        <!-- 任务汇总条（后端全量聚合，不随分页变化） -->
        <div class="task-summary">
          任务汇总：已检 {{ detail.stats.done }} · 正常 {{ detail.stats.normal }} · 异常 {{ detail.stats.abnormal }} ·
          疑似作弊 {{ detail.stats.suspect }} · 巡检中 {{ detail.stats.doing }} · 未打卡 {{ detail.stats.pending }}
        </div>
      </div>
    </template>

    <!-- 异常：任务不存在/无权限 -->
    <el-result v-else-if="loadError" icon="error" title="任务不存在或无权限查看" class="error-result">
      <template #extra>
        <el-button type="primary" @click="$router.push('/inspection/tasks')">返回任务监控</el-button>
      </template>
    </el-result>

    <!-- 照片查看器 -->
    <el-dialog v-model="photoDialogVisible" title="打卡照片" width="680px">
      <photo-viewer v-if="currentPhotos" :photos="currentPhotos.photos" :meta="currentPhotos.meta" />
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import {
  CircleCheck, CircleClose, Warning, Remove, Clock, Upload, Loading
} from '@element-plus/icons-vue'
import { getTaskDetail } from '@/api/task'
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { TaskDetail, TaskPointDetail, CheckinPhoto } from '@/api/biz-types'
import { patrolTypeLabel } from '@/api/biz-types'

const route = useRoute()
const loading = ref(false)
const loadError = ref(false)
const detail = ref<TaskDetail | null>(null)
const pointsPage = ref(1)
const timelineLoading = ref(false)

async function fetchDetail(page: number) {
  const d = await getTaskDetail(String(route.params.id), page)
  detail.value = d
  pointsPage.value = d.points_page
}

onMounted(async () => {
  loading.value = true
  try {
    await fetchDetail(1)
  } catch {
    loadError.value = true
  } finally {
    loading.value = false
  }
})

async function onPageChange(page: number) {
  timelineLoading.value = true
  try {
    await fetchDetail(page)
  } finally {
    timelineLoading.value = false
  }
}

const headStatusLabel = computed(() => {
  const s = detail.value?.task.status
  return ({ pending: '待开始', doing: '进行中', done: '已完成', overdue: '已逾期' } as Record<string, string>)[s || ''] || s
})

const headStatusType = computed(() => {
  const s = detail.value?.task.status
  return ({ pending: 'info', doing: 'primary', done: 'success', overdue: 'danger' } as Record<string, any>)[s || ''] || 'info'
})

// 点位状态视觉：正常/异常/疑似作弊/离线补传/巡检中/未打卡
function pointVisual(p: TaskPointDetail): { type: string; icon: any } {
  if (!p.checkin) {
    if (p.status === 'doing') return { type: 'warning', icon: Loading }
    return { type: 'info', icon: Clock }
  }
  if (p.checkin.result === 'abnormal') return { type: 'danger', icon: CircleClose }
  if (p.checkin.is_suspect) return { type: 'warning', icon: Warning }
  if (p.checkin.checkin_type === 'offline') return { type: 'info', icon: Upload }
  return { type: 'success', icon: CircleCheck }
}

function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传' }[t] || t
}

// ===== 照片查看 =====
const photoDialogVisible = ref(false)
const currentPhotos = ref<{ photos: CheckinPhoto[]; meta: { time: string; distance: number; coords: string } } | null>(null)

function openPhotos(p: TaskPointDetail) {
  const c = p.checkin!
  currentPhotos.value = {
    photos: c.photos,
    meta: {
      time: c.checkin_time,
      distance: c.distance_to_point,
      coords: `${c.longitude.toFixed(6)}, ${c.latitude.toFixed(6)}`
    }
  }
  photoDialogVisible.value = true
}
</script>

<style scoped lang="scss">
.task-head {
  margin-bottom: $spacing-lg;

  .task-head-main {
    display: flex;
    align-items: center;
    gap: $spacing-md;

    .task-title {
      margin: 0;
    }
  }

  .task-head-progress {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    margin: $spacing-md 0 $spacing-sm;

    .el-progress {
      flex: 1;
    }

    .progress-text {
      color: $color-text-regular;
      font-size: $font-size-aux;
      white-space: nowrap;
    }
  }
}

.point-timeline {
  margin-top: $spacing-lg;

  .point-row {
    display: flex;
    align-items: center;
    gap: $spacing-md;
    flex-wrap: wrap;

    .point-time {
      font-weight: 600;
      color: $color-text-primary;
      width: 48px;
    }

    .point-name {
      color: $color-text-primary;
      font-weight: 600;
    }
  }
}

.timeline-pager {
  display: flex;
  justify-content: flex-end;
  margin-top: $spacing-md;
}

.task-summary {
  margin-top: $spacing-lg;
  padding-top: $spacing-lg;
  border-top: 1px solid $color-border;
  color: $color-text-regular;
  font-size: $font-size-body;
}

.error-result {
  background: $color-bg-card;
  border-radius: $radius-card;
}
</style>
