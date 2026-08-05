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

    <!-- 表格（行可点击展开） -->
    <div class="table-card">
      <el-table v-loading="loading" :data="list" stripe style="width: 100%" row-key="id" @expand-change="handleExpand">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div v-loading="expandLoading[row.id]" class="expand-pane">
              <template v-if="expandCache[row.id]">
                <div class="expand-grid">
                  <div class="expand-photos">
                    <div class="expand-label">照片（水印缩略图，点击查看原图）</div>
                    <photo-viewer
                      :photos="expandCache[row.id].photos || []"
                      :meta="photoMeta(expandCache[row.id])"
                    />
                  </div>
                  <div class="expand-info">
                    <el-descriptions :column="1" border size="small">
                      <el-descriptions-item label="坐标">
                        {{ expandCache[row.id].longitude.toFixed(6) }}, {{ expandCache[row.id].latitude.toFixed(6) }}
                      </el-descriptions-item>
                      <el-descriptions-item label="距点位">{{ expandCache[row.id].distance_to_point }}m</el-descriptions-item>
                      <el-descriptions-item label="服务端时间">{{ expandCache[row.id].checkin_time }}</el-descriptions-item>
                      <el-descriptions-item label="客户端时间">{{ expandCache[row.id].client_time }}</el-descriptions-item>
                      <el-descriptions-item v-if="expandCache[row.id].suspect_reason" label="疑似原因">
                        <span class="danger-text">{{ expandCache[row.id].suspect_reason }}</span>
                      </el-descriptions-item>
                      <el-descriptions-item v-if="expandCache[row.id].remark" label="备注">
                        {{ expandCache[row.id].remark }}
                      </el-descriptions-item>
                      <el-descriptions-item v-if="expandCache[row.id].work_order_no" label="关联工单">
                        <el-link type="danger" @click="goWorkOrder(expandCache[row.id].work_order_no!)">
                          {{ expandCache[row.id].work_order_no }}
                        </el-link>
                      </el-descriptions-item>
                    </el-descriptions>
                  </div>
                </div>
              </template>
            </div>
          </template>
        </el-table-column>
        <el-table-column prop="checkin_time" label="打卡时间" width="160" />
        <el-table-column prop="inspector_name" label="巡检员" width="100" />
        <el-table-column prop="community_name" label="小区" min-width="120" />
        <el-table-column prop="point_name" label="点位" min-width="120" show-overflow-tooltip />
        <el-table-column label="方式" width="100" align="center">
          <template #default="{ row }">
            {{ { qrcode: '扫码', fence: '围栏', offline: '离线补传' }[row.checkin_type as string] || row.checkin_type }}
          </template>
        </el-table-column>
        <el-table-column prop="distance_to_point" label="距点位" width="90" align="right">
          <template #default="{ row }">{{ row.distance_to_point }}m</template>
        </el-table-column>
        <el-table-column label="结果" width="110" align="center">
          <template #default="{ row }">
            <!-- 图标+文字双编码 -->
            <el-tag v-if="row.is_suspect" type="warning" size="small">疑似作弊</el-tag>
            <el-tag v-else-if="row.result === 'abnormal'" type="danger" size="small">异常</el-tag>
            <el-tag v-else type="success" size="small">正常</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="photo_count" label="照片数" width="80" align="right" />
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

// ===== 行展开：按需拉取明细并缓存 =====
const expandCache = reactive<Record<string, CheckinDetail>>({})
const expandLoading = reactive<Record<string, boolean>>({})

async function handleExpand(row: CheckinItem, expanded: CheckinItem[]) {
  if (!expanded.includes(row) || expandCache[row.id]) return
  expandLoading[row.id] = true
  try {
    expandCache[row.id] = await getCheckin(row.id)
  } finally {
    expandLoading[row.id] = false
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
  router.push({ path: '/workorders/list', query: { order_no: orderNo } })
}
</script>

<style scoped lang="scss">
.expand-pane {
  padding: $spacing-md $spacing-xl;
  min-height: 80px;
}

.expand-grid {
  display: flex;
  gap: $spacing-xl;
  align-items: flex-start;
}

.expand-photos {
  flex: 1;
  min-width: 0;

  .expand-label {
    font-size: $font-size-aux;
    color: $color-text-secondary;
    margin-bottom: $spacing-sm;
  }
}

.expand-info {
  width: 380px;
  flex-shrink: 0;
}

.danger-text {
  color: $color-danger;
}
</style>
