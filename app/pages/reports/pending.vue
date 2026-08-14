<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 选项卡：待我签字 / 已完成 -->
    <view class="tabs" :style="{ backgroundColor: colors.bgCard }">
      <view
        v-for="t in tabs"
        :key="t.key"
        class="tab"
        @click="switchTab(t.key)"
      >
        <text
          class="tab-text"
          :style="{ color: tab == t.key ? colors.primary : colors.textRegular, fontWeight: tab == t.key ? 600 : 400 }"
        >{{ t.label }}</text>
        <view v-if="tab == t.key" class="tab-line" :style="{ backgroundColor: colors.primary }"></view>
      </view>
    </view>

    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ emptyTitle }}</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">{{ emptySub }}</text>
    </view>

    <!-- 报告列表 -->
    <view v-else-if="loaded" class="content">
      <view
        v-for="r in list"
        :key="r.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="goDetail(r.id)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ r.title }}</text>
        </view>
        <view class="card-row">
          <text class="card-node" :style="{ color: nodeColorOf(r.status), backgroundColor: colors.primaryLight }">{{ nodeTextOf(r.status) }}</text>
          <text v-if="r.status == 'pending_inspector'" class="card-progress" :style="{ color: colors.textSecondary }">
            巡检员已确认 {{ r.inspector_signed_count }}/{{ r.inspector_total }}
          </text>
        </view>
        <text class="card-time" :style="{ color: colors.textSecondary }">生成时间 {{ r.created_at }}</text>
      </view>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiReports, ReportListItem } from '@/services/api'

type TabKey = 'pending' | 'doing' | 'done'

type PendingData = {
  colors: ColorTokens
  tab: TabKey
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: ReportListItem[]
}

const TABS: { key: TabKey; label: string }[] = [
  { key: 'pending', label: '等待签字' },
  { key: 'doing', label: '进行中' },
  { key: 'done', label: '已完成' }
]

/** 当前签字节点文案（对齐报告状态机） */
function nodeTextOf(status: string): string {
  if (status == 'pending_inspector') return '待巡检员确认'
  if (status == 'pending_supervisor') return '待主管审批'
  if (status == 'pending_manager') return '待经理终审'
  if (status == 'approved') return '已归档'
  return status
}

function nodeColorOf(status: string): string {
  if (status == 'approved') return Colors.success
  return Colors.primary
}

export default {
  data(): PendingData {
    return {
      colors: Colors,
      tab: 'pending',
      loading: true,
      loaded: false,
      errorMsg: '',
      list: [] as ReportListItem[]
    }
  },
  computed: {
    tabs(): { key: TabKey; label: string }[] {
      return TABS
    },
    emptyTitle(): string {
      if (this.tab == 'pending') return '暂无待签报告'
      if (this.tab == 'doing') return '暂无进行中报告'
      return '暂无已完成报告'
    },
    emptySub(): string {
      if (this.tab == 'pending') return '月度报告到达你的签字节点时会出现在这里'
      if (this.tab == 'doing') return '你已签字、仍在审批流程中的报告会显示在这里'
      return '已归档的月度报告会保留在这里，可随时查看完整内容'
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 签字返回后刷新（待我签列表会剔除已签报告，已完成列表能看到归档结果）
    if (this.loaded) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    switchTab(key: TabKey) {
      if (this.tab == key) return
      this.tab = key
      this.loaded = false
      this.list = []
      this.load()
    },
    load() {
      this.loading = !this.loaded
      // 等待签字：pending_mine=1；进行中：signed_mine=doing（我签过未归档）；已完成：status=approved
      let req: Promise<any>
      if (this.tab == 'pending') {
        req = apiReports(1, 50, true)
      } else if (this.tab == 'doing') {
        req = apiReports(1, 50, false, '', 'doing')
      } else {
        req = apiReports(1, 50, false, 'approved')
      }
      req
        .then((res) => {
          this.loading = false
          this.loaded = true
          this.list = res.list
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

.tabs {
  flex-direction: row;
  border-radius: 24rpx; /* Radius.card */
  margin-bottom: 24rpx;
  padding: 0 32rpx;
}

.tab {
  flex: 1;
  align-items: center;
  padding-top: 24rpx;
}

.tab-text {
  font-size: 30rpx;
}

.tab-line {
  width: 48rpx;
  height: 6rpx;
  border-radius: 3rpx;
  margin-top: 16rpx;
  margin-bottom: 18rpx;
}

.skeleton {
  padding-top: 8rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.sk-short {
  height: 96rpx;
}

.empty {
  align-items: center;
  padding-top: 160rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-sub {
  font-size: 26rpx;
  text-align: center;
  padding-left: 48rpx;
  padding-right: 48rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
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
</style>
