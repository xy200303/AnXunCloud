<template>
  <div class="app-container">
    <!-- 搜索区：时间筛选默认近 7 天 -->
    <div class="filter-card">
      <el-form :model="query" inline>
        <el-form-item label="小区">
          <el-select v-model="query.community_id" placeholder="全部小区" clearable style="width: 140px">
            <el-option v-for="c in communities" :key="c.id" :label="c.name" :value="c.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="巡检员">
          <el-select v-model="query.inspector_id" placeholder="全部" clearable filterable style="width: 120px">
            <el-option v-for="u in inspectors" :key="u.id" :label="u.name" :value="u.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="结果">
          <el-select v-model="query.result" placeholder="全部" clearable style="width: 110px">
            <el-option label="正常" value="normal" />
            <el-option label="异常" value="abnormal" />
          </el-select>
        </el-form-item>
        <el-form-item label="疑似作弊">
          <el-checkbox v-model="onlySuspect" label="仅看疑似" />
        </el-form-item>
        <el-form-item label="打卡时间">
          <el-date-picker
            v-model="timeRange"
            type="datetimerange"
            range-separator="至"
            start-placeholder="开始"
            end-placeholder="结束"
            value-format="YYYY-MM-DD HH:mm:ss"
            style="width: 340px"
          />
        </el-form-item>
        <el-form-item>
          <el-button type="primary" :icon="Search" @click="handleSearch">查询</el-button>
          <el-button :icon="Refresh" @click="handleReset">重置</el-button>
        </el-form-item>
      </el-form>
    </div>

    <!-- 表格（点击行或「详情」查看） -->
    <div class="table-card">
      <el-table
        v-loading="loading"
        :data="list"
        stripe
        style="width: 100%"
        row-class-name="clickable-row"
        @row-click="openDetail"
      >
        <el-table-column prop="checkin_time" label="打卡时间" width="160" />
        <el-table-column prop="inspector_name" label="巡检员" width="100" />
        <el-table-column prop="community_name" label="小区" min-width="120" />
        <el-table-column prop="point_name" label="点位" min-width="120" show-overflow-tooltip />
        <el-table-column label="方式" width="100" align="center">
          <template #default="{ row }">{{ checkinTypeLabel(row.checkin_type) }}</template>
        </el-table-column>
        <el-table-column prop="distance_to_point" label="距点位" width="90" align="right">
          <template #default="{ row }">{{ row.distance_to_point }}m</template>
        </el-table-column>
        <el-table-column label="结果" width="110" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.is_suspect" type="warning" size="small">疑似作弊</el-tag>
            <el-tag v-else-if="row.result === 'abnormal'" type="danger" size="small">异常</el-tag>
            <el-tag v-else type="success" size="small">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="photo_count" label="照片数" width="80" align="right" />
        <el-table-column label="操作" width="90">
          <template #default="{ row }">
            <el-button link type="primary" @click.stop="openDetail(row)">详情</el-button>
          </template>
        </el-table-column>
        <template #empty>
          <el-empty description="该条件下暂无打卡记录，试试扩大时间范围" />
        </template>
      </el-table>

      <div class="pagination-wrap">
        <el-pagination
          v-model:current-page="query.page"
          v-model:page-size="query.page_size"
          :total="total"
          :page-sizes="[10, 20, 50, 100]"
          layout="total, sizes, prev, pager, next"
          @change="fetchList"
        />
      </div>
    </div>

    <!-- 打卡详情抽屉：分区展示，后续扩展新内容加区块即可 -->
    <el-drawer v-model="detailVisible" title="打卡详情" size="620px">
      <div v-loading="detailLoading" class="detail-body">
        <template v-if="detail && currentRow">
          <!-- 概要 -->
          <div class="detail-header">
            <div class="detail-point">
              <span class="point-name">{{ currentRow.point_name }}</span>
              <span class="text-secondary">{{ currentRow.community_name }}</span>
            </div>
            <el-tag v-if="currentRow.is_suspect" type="warning">疑似作弊</el-tag>
            <el-tag v-else-if="currentRow.result === 'abnormal'" type="danger">异常</el-tag>
            <el-tag v-else type="success">正常</el-tag>
          </div>

          <el-descriptions :column="2" border size="small">
            <el-descriptions-item label="打卡时间">{{ currentRow.checkin_time }}</el-descriptions-item>
            <el-descriptions-item label="巡检员">{{ currentRow.inspector_name }}</el-descriptions-item>
            <el-descriptions-item label="打卡方式">{{ checkinTypeLabel(currentRow.checkin_type) }}</el-descriptions-item>
            <el-descriptions-item label="照片数">{{ currentRow.photo_count }} 张</el-descriptions-item>
          </el-descriptions>

          <!-- 定位与防作弊校验 -->
          <div class="section-title">定位与校验</div>
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
            <el-descriptions-item v-if="detail.work_order_no" label="关联工单">
              <el-link type="danger" @click="goWorkOrder(detail.work_order_no!)">
                {{ detail.work_order_no }}
              </el-link>
            </el-descriptions-item>
          </el-descriptions>

          <!-- 现场照片 -->
          <div class="section-title">现场照片（水印缩略图，点击查看原图）</div>
          <photo-viewer :photos="detail.photos || []" :meta="photoMeta(detail)" />
        </template>
      </div>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { Search, Refresh } from '@element-plus/icons-vue'
import { listCheckins, getCheckin, type CheckinQuery } from '@/api/checkin'
import { listCommunities } from '@/api/community'
import { listUsers } from '@/api/user'
import PhotoViewer from '@/components/PhotoViewer.vue'
import type { CheckinItem, CheckinDetail, CommunityItem } from '@/api/biz-types'
import type { UserItem } from '@/api/types'

const router = useRouter()
const route = useRoute()
const loading = ref(false)
const list = ref<CheckinItem[]>([])
const total = ref(0)
const communities = ref<CommunityItem[]>([])
const inspectors = ref<UserItem[]>([])
const onlySuspect = ref(false)

function checkinTypeLabel(t: string) {
  return { qrcode: '扫码', fence: '围栏', offline: '离线补传' }[t] || t
}

// 时间筛选默认近 7 天
function defaultRange(): [string, string] {
  const end = new Date()
  const start = new Date(Date.now() - 6 * 86400000)
  const fmt = (d: Date, endOfDay: boolean) =>
    `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')} ${endOfDay ? '23:59:59' : '00:00:00'}`
  return [fmt(start, false), fmt(end, true)]
}

const timeRange = ref<[string, string] | null>(defaultRange())
const query = reactive<CheckinQuery>({ page: 1, page_size: 20, community_id: undefined, inspector_id: undefined, result: '' })

async function fetchList() {
  loading.value = true
  try {
    const data = await listCheckins({
      ...query,
      result: query.result || undefined,
      is_suspect: onlySuspect.value || undefined,
      start_time: timeRange.value?.[0],
      end_time: timeRange.value?.[1]
    })
    list.value = data.list
    total.value = data.total
  } finally {
    loading.value = false
  }
}

function handleSearch() {
  query.page = 1
  fetchList()
}

function handleReset() {
  query.community_id = undefined
  query.inspector_id = undefined
  query.result = ''
  onlySuspect.value = false
  timeRange.value = defaultRange()
  handleSearch()
}

onMounted(async () => {
  // 支持从绩效报表带筛选进入（疑似作弊下钻）
  if (route.query.inspector_id) query.inspector_id = String(route.query.inspector_id)
  if (route.query.is_suspect) onlySuspect.value = true
  fetchList()
  const [cData, uData] = await Promise.all([
    listCommunities({ page: 1, page_size: 100, status: 1 }),
    listUsers({ page: 1, page_size: 100, status: 1 })
  ])
  communities.value = cData.list
  inspectors.value = uData.list
})

// ===== 详情抽屉 =====
const detailVisible = ref(false)
const detailLoading = ref(false)
const detail = ref<CheckinDetail | null>(null)
const currentRow = ref<CheckinItem | null>(null)

async function openDetail(row: CheckinItem) {
  currentRow.value = row
  detail.value = null
  detailVisible.value = true
  detailLoading.value = true
  try {
    detail.value = await getCheckin(row.id)
  } finally {
    detailLoading.value = false
  }
}

function photoMeta(d: CheckinDetail) {
  return {
    time: d.checkin_time,
    distance: d.distance_to_point,
    coords: `${d.longitude.toFixed(6)}, ${d.latitude.toFixed(6)}`
  }
}

function goWorkOrder(orderNo: string) {
  detailVisible.value = false
  router.push({ path: '/workorders/list', query: { order_no: orderNo } })
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

.section-title {
  font-weight: 600;
  color: $color-text-primary;
  margin: $spacing-xl 0 $spacing-md;
}

.danger-text {
  color: $color-danger;
}
</style>

<style lang="scss">
// 行可点击提示（非 scoped，作用于 el-table 生成的行）
.clickable-row {
  cursor: pointer;
}
</style>
