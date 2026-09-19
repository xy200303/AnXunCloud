<template>
  <view class="page bg-page" >
    <AppSegmentTabs :items="tabs" :value="tab" @change="switchTab" />

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      :empty-title="emptyTitle"
      :empty-sub="emptySub"
     
      @retry="load"
    >

    <!-- 报告列表 -->
    <template #default>
    <view class="content">
      <view
        v-for="r in list"
        :key="r.id"
         hover-class="hover-dim" class="card bg-card"
        
        @click="goDetail(r.id)"
      >
        <view  hover-class="hover-dim" class="card-head">
          <text  hover-class="hover-dim" class="card-title text-main" >{{ r.title }}</text>
        </view>
        <view  hover-class="hover-dim" class="card-row">
          <text  hover-class="hover-dim" class="card-node bg-brand-light" :style="{ color: nodeColorOf(r.status) }">{{ nodeTextOf(r.status) }}</text>
          <text v-if="r.status == 'pending_review'" hover-class="hover-dim" class="card-progress text-secondary" >待当前审核步骤处理</text>
        </view>
        <text class="card-signers text-secondary" >
          审核路径：{{ (r.review_steps || []).map((x: any) => x.name).join(' → ') || '无需审核' }}
        </text>
        <text  hover-class="hover-dim" class="card-time text-secondary" >生成时间 {{ r.created_at }}</text>
      </view>
    </view>
    </template>
    </AppListShell>

    <view v-if="menuOpen" class="plus-mask" @click="menuOpen = false">
      <view class="plus-menu" @click.stop>
        <view v-if="canGenerate" hover-class="hover-dim" class="plus-item" @click="goGenerate">
          <text hover-class="hover-dim" class="plus-item-icon text-white" >＋</text>
          <text hover-class="hover-dim" class="plus-item-text text-white" >生成报告</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiReports, ReportListItem } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import AppListShell from '@/components/AppListShell.vue'
import AppSegmentTabs from '@/components/AppSegmentTabs.vue'

type TabKey = 'pending' | 'doing' | 'done' | 'all'

type PendingData = {
  tab: TabKey
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: ReportListItem[]
  menuOpen: boolean
  lastLoadedAt: number
}

const TABS: { value: TabKey; label: string }[] = [
  { value: 'pending', label: '等待签字' },
  { value: 'doing', label: '进行中' },
  { value: 'done', label: '已完成' },
  { value: 'all', label: '全部' }
]

/** 当前签字节点文案（对齐报告状态机） */
function nodeTextOf(status: string): string {
  if (status == 'pending_review') return '待审核'
  if (status == 'approved') return '已归档'
  if (status == 'voided') return '已作废'
  return status
}

function nodeColorOf(status: string): string {
  if (status == 'approved') return '#2BA471'
  if (status == 'voided') return '#86909C'
  return '#2B5AED'
}

export default {
  components: { AppListShell, AppSegmentTabs },
  data(): PendingData {
    return {
      tab: 'pending',
      loading: true,
      loaded: false,
      errorMsg: '',
      list: [] as ReportListItem[],
      menuOpen: false,
      lastLoadedAt: 0
    }
  },
  computed: {
    tabs(): { value: TabKey; label: string }[] {
      return TABS
    },
    /** 管理端「生成报告」入口显隐（须 report:generate 权限） */
    canGenerate(): boolean {
      return useAuthStore().hasPerm('report:generate')
    },
    emptyTitle(): string {
      if (this.tab == 'pending') return '暂时没有要你签字的报告'
      if (this.tab == 'doing') return '暂时没有进行中的报告'
      if (this.tab == 'all') return '数据权限内还没有报告'
      return '还没有已完成的报告'
    },
    emptySub(): string {
      if (this.tab == 'pending') return '月度报告到达你的签字节点时会出现在这里'
      if (this.tab == 'doing') return '你已签字、仍在审批流程中的报告会显示在这里'
      if (this.tab == 'all') return '你数据权限内的全部报告（含在途）都列在这里'
      return '已归档的月度报告会保留在这里，可随时查看完整内容'
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 签字返回后刷新（待我签列表会剔除已签报告，已完成列表能看到归档结果）
    if (this.loaded && Date.now() - this.lastLoadedAt > 20000) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  onNavigationBarButtonTap() {
    if (!this.canGenerate) {
      uni.showToast({ title: '暂无生成报告权限', icon: 'none' })
      return
    }
    // 菜单只有「生成报告」一项，不再经过遮罩菜单两步操作
    this.goGenerate()
  },
  methods: {
    goGenerate() {
      this.menuOpen = false
      uni.navigateTo({ url: '/pages/reports/generate' })
    },
    switchTab(key: TabKey) {
      if (this.tab == key) return
      this.tab = key
      this.loaded = false
      this.list = []
      this.load()
    },
    load() {
      this.loading = !this.loaded
      // 等待签字：pending_mine=1；进行中：signed_mine=doing（我签过未归档）；已完成：status=approved；全部：不过滤（数据权限收口在服务端）
      let req: Promise<any>
      if (this.tab == 'pending') {
        req = apiReports(1, 50, true)
      } else if (this.tab == 'doing') {
        req = apiReports(1, 50, false, '', 'doing')
      } else if (this.tab == 'all') {
        req = apiReports(1, 50, false)
      } else {
        req = apiReports(1, 50, false, 'approved')
      }
      req
        .then((res) => {
          this.loading = false
          this.loaded = true
          this.list = res.list
          this.lastLoadedAt = Date.now()
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goDetail(id: string) {
      this.lastLoadedAt = 0
      uni.navigateTo({ url: '/pages/reports/detail?id=' + encodeURIComponent(id) })
    },
    nodeTextOf,
    nodeColorOf
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  align-items: center;
}

.card-title {
  font-size: 32rpx;
  font-weight: 600;
  flex: 1;
  line-height: 44rpx;
}

.card-row {
  flex-direction: row;
  align-items: center;
  margin-top: 20rpx;
}

.card-node {
  font-size: 24rpx;
  padding: 6rpx 16rpx;
  border-radius: 12rpx; /* Radius.tag */
}

.card-progress {
  font-size: 24rpx;
  margin-left: 16rpx;
}

.card-time {
  font-size: 24rpx;
  margin-top: 16rpx;
}

.card-signers {
  display: block;
  font-size: 24rpx;
  line-height: 1.5;
  margin-top: 12rpx;
}

.plus-mask {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 998;
}

.plus-menu {
  position: absolute;
  top: 8rpx;
  right: 24rpx;
  min-width: 280rpx;
  background-color: #4c4c4c;
  border-radius: 16rpx;
  overflow: hidden;
}

.plus-item {
  display: flex;
  flex-direction: row;
  align-items: center;
  padding: 28rpx 36rpx;
}

.plus-item-icon {
  font-size: 34rpx;
  margin-right: 20rpx;
  width: 40rpx;
  text-align: center;
}

.plus-item-text {
  font-size: 30rpx;
}
</style>
