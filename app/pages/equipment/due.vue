<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 状态筛选 chips（客户端过滤：待维保列表一次拉全，上限 200 台） -->
    <view class="filter-bar" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <AppChipScroller :items="dueChips" :value="dueFilter" :colors="colors" @change="dueFilter = $event" />
    </view>

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="filtered.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无待维保设备"
      empty-sub="临期/逾期设备会出现在这里"
      :colors="colors"
      @retry="load"
    >
      <template #default>
      <view class="content">
        <view
          v-for="e in filtered"
          :key="e.id"
          hover-class="hover-dim"
          class="card"
          :style="{ backgroundColor: colors.bgCard }"
          @click="goDetail(e.id)"
        >
          <view class="card-head">
            <view class="card-title-row">
              <view class="due-dot" :style="{ backgroundColor: dueColorOf(e.due_state) }"></view>
              <text class="card-title" :style="{ color: colors.textPrimary }">{{ e.name }}</text>
            </view>
            <text class="card-status" :style="{ color: dueColorOf(e.due_state) }">{{ dueTextOf(e) }}</text>
          </view>
          <text class="card-sub" :style="{ color: colors.textSecondary }">编号：{{ e.code }}</text>
          <view class="card-foot">
            <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ e.type_label != '' ? e.type_label : e.type }}</text>
            <text class="card-loc" :style="{ color: colors.textSecondary }">{{ locationText(e) }}</text>
          </view>
        </view>
      </view>
      </template>
    </AppListShell>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiEquipmentDue, EquipmentListItem, EquipmentDueState } from '@/services/api'
import AppListShell from '@/components/AppListShell.vue'
import AppChipScroller from '@/components/AppChipScroller.vue'

type PageData = {
  colors: ColorTokens
  dueFilter: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: EquipmentListItem[]
  lastLoadedAt: number
}

/** 状态灯颜色：与台账页同口径（warning 黄 / overdue、scrap 红 / 其他灰） */
function dueColorOf(s: EquipmentDueState): string {
  if (s == 'normal') return Colors.success
  if (s == 'warning') return Colors.warning
  if (s == 'overdue' || s == 'scrap') return Colors.danger
  return Colors.info
}

/** 今日 0 点（本地时区），逾期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

export default {
  components: { AppListShell, AppChipScroller },
  data(): PageData {
    return {
      colors: Colors,
      dueFilter: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      list: [] as EquipmentListItem[],
      lastLoadedAt: 0
    }
  },
  computed: {
    dueChips(): Array<{ value: string; label: string }> {
      return [
        { value: '', label: '全部' },
        { value: 'overdue', label: '已逾期' },
        { value: 'warning', label: '临期' },
        { value: 'scrap', label: '应报废' }
      ]
    },
    filtered(): EquipmentListItem[] {
      if (this.dueFilter == '') return this.list
      return this.list.filter((e) => e.due_state == this.dueFilter)
    }
  },
  onLoad(options: any) {
    // 今日任务「维保待办」卡片带筛选跳入（due_state=overdue/warning）
    if (options && options.due_state) this.dueFilter = String(options.due_state)
    this.load()
  },
  onShow() {
    // 登记/详情返回后刷新（confirmed 流水会回写台账到期日）
    if (this.loaded && Date.now() - this.lastLoadedAt > 10000) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    dueColorOf,
    /** 到期状态文案：逾期红字带天数（与台账页同口径） */
    dueTextOf(e: EquipmentListItem): string {
      if (e.due_state == 'scrap') return '应报废'
      if (e.next_due_date == '') return ''
      const due = new Date(e.next_due_date.replace(/-/g, '/')).getTime()
      const days = Math.round((due - todayZero()) / 86400000)
      if (days < 0) return '已逾期 ' + (-days) + ' 天'
      if (days == 0) return '今天到期'
      return days + ' 天后到期'
    },
    locationText(e: EquipmentListItem): string {
      const parts: string[] = []
      if (e.community_name != '') parts.push(e.community_name)
      if (e.building_name != null && e.building_name != '') parts.push(e.building_name)
      if (e.point_name != null && e.point_name != '') parts.push(e.point_name)
      return parts.length > 0 ? parts.join(' · ') : '未绑定位置'
    },
    load() {
      this.loading = !this.loaded
      apiEquipmentDue()
        .then((list) => {
          this.list = list
          this.loading = false
          this.loaded = true
          this.lastLoadedAt = Date.now()
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          else uni.showToast({ title: e.message, icon: 'none' })
          uni.stopPullDownRefresh()
        })
    },
    goDetail(id: string) {
      this.lastLoadedAt = 0
      uni.navigateTo({ url: '/pages/equipment/detail?id=' + encodeURIComponent(id) })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.filter-bar {
  padding: 16rpx 24rpx 24rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.content {
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title-row {
  flex-direction: row;
  align-items: center;
  flex: 1;
}

/* 红黄绿灰状态灯 */
.due-dot {
  width: 20rpx;
  height: 20rpx;
  border-radius: 10rpx;
  margin-right: 16rpx;
}

.card-title {
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.card-foot {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  margin-top: 24rpx;
}

.tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
}

.card-loc {
  font-size: 24rpx;
  flex: 1;
  text-align: right;
}
</style>
