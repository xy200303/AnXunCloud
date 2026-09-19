<template>
  <view class="page bg-page" >
    <!-- 状态筛选 chips（客户端过滤：待维保列表一次拉全，上限 200 台） -->
    <view class="filter-bar bg-card border-default" >
      <uni-segmented-control
        :values="dueChips.map((c) => c.label)"
        :current="dueTabIdx"
        style-type="button"
        active-color="#2B5AED"
        @clickItem="onDueTab"
      />
    </view>

    <!-- 我的提交入口：提交后设备从待办消失（有 pending 流水），这里给登记人查看/修改的固定入口 -->
    <view hover-class="hover-dim" class="mine-entry bg-card border-default"  @click="goMine">
      <text class="mine-entry-text text-main" >我的提交记录</text>
      <text class="mine-entry-arrow text-secondary" >查看 / 修改 ›</text>
    </view>

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="filtered.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无待维保设备"
      empty-sub="临期/逾期设备会出现在这里"
     
      @retry="load"
    >
      <template #default>
      <view class="content">
        <view
          v-for="e in filtered"
          :key="e.id"
          hover-class="hover-dim"
          class="card bg-card"
          
          @click="goDetail(e)"
        >
          <view class="card-head">
            <view class="card-title-row">
              <uni-badge :is-dot="true" :custom-style="{ backgroundColor: dueColorOf(e.due_state), marginRight: '12rpx' }" />
              <text class="card-title text-main" >{{ e.name }}</text>
            </view>
            <text class="card-status" :style="{ color: dueColorOf(e.due_state) }">{{ dueTextOf(e) }}</text>
          </view>
          <text class="card-sub text-secondary" >编号：{{ e.code }}</text>
          <view class="card-foot">
            <uni-tag :text="e.type_label != '' ? e.type_label : e.type" :inverted="true" size="small" :custom-style="'color:#86909C;border-color:#E5E6EB;margin-right:16rpx'" />
            <text class="card-loc text-secondary" >{{ locationText(e) }}</text>
          </view>
        </view>
      </view>
      </template>
    </AppListShell>
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiEquipmentDue, EquipmentListItem, EquipmentDueState } from '@/services/api'
import AppListShell from '@/components/AppListShell.vue'
import { useAuthStore } from '@/stores/auth'

type PageData = {
  dueFilter: string
  /** 'pick' = 选设备模式（今日任务维保待办卡片跳入）：点卡片直达维保拍照页 */
  mode: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: EquipmentListItem[]
  lastLoadedAt: number
}

/** 状态灯颜色：与台账页同口径（warning 黄 / overdue、scrap 红 / 其他灰） */
function dueColorOf(s: EquipmentDueState): string {
  if (s == 'normal') return '#2BA471'
  if (s == 'warning') return '#ED7B2F'
  if (s == 'overdue' || s == 'scrap') return '#D54941'
  return '#909399'
}

/** 今日 0 点（本地时区），逾期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

export default {
  components: { AppListShell },
  data(): PageData {
    return {
      dueFilter: '',
      mode: '',
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
    /** 分段器当前下标（dueChips 下标 ↔ dueFilter 值换算） */
    dueTabIdx(): number {
      const i = this.dueChips.findIndex((c) => c.value == this.dueFilter)
      return i >= 0 ? i : 0
    },
    filtered(): EquipmentListItem[] {
      if (this.dueFilter == '') return this.list
      return this.list.filter((e) => e.due_state == this.dueFilter)
    }
  },
  onLoad(options: any) {
    // 今日任务「维保待办」卡片带筛选跳入（due_state=overdue/warning）
    if (options && options.due_state) this.dueFilter = String(options.due_state)
    // 选设备模式：点卡片直达维保拍照页
    if (options && options.mode == 'pick') {
      this.mode = 'pick'
      uni.setNavigationBarTitle({ title: '选择维保设备' })
    }
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
    onDueTab(e: { currentIndex: number }) {
      const c = this.dueChips[e.currentIndex]
      if (c != null) this.dueFilter = c.value
    },
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
    goDetail(e: EquipmentListItem) {
      this.lastLoadedAt = 0
      if (this.mode == 'pick') {
        // 登记维保须 equipment:maintenance 权限（只有台账查看权限的用户前置拦截，不走进拍照流程才被 403）
        if (!useAuthStore().hasPerm('equipment:maintenance')) {
          uni.showToast({ title: '你没有维保登记权限', icon: 'none' })
          return
        }
        // 选设备模式：直达维保拍照页（name/code 作详情拉取失败时的回显兜底）
        uni.navigateTo({
          url:
            '/pages/equipment/maintain?equipment_id=' + encodeURIComponent(e.id) +
            '&name=' + encodeURIComponent(e.name) +
            '&code=' + encodeURIComponent(e.code)
        })
        return
      }
      uni.navigateTo({ url: '/pages/equipment/detail?id=' + encodeURIComponent(e.id) })
    },
    goMine() {
      uni.navigateTo({ url: '/pages/equipment/mine' })
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

/* 我的提交入口条 */
.mine-entry {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 24rpx 32rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.mine-entry-text {
  font-size: 28rpx;
  font-weight: 600;
}

.mine-entry-arrow {
  font-size: 26rpx;
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


.card-loc {
  font-size: 24rpx;
  flex: 1;
  text-align: right;
}
</style>
