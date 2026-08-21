<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 状态 tab（角标为 status_counts 聚合计数） -->
    <view class="tabs" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <view v-for="t in tabs" :key="t.value" class="tab" @click="switchStatus(t.value)">
        <view class="tab-label">
          <text class="tab-text" :style="{ color: status == t.value ? colors.primary : colors.textRegular }">{{ t.label }}</text>
          <text
            v-if="countOf(t.value) > 0"
            class="tab-count"
            :style="status == t.value
              ? { backgroundColor: colors.primary, color: colors.white }
              : { backgroundColor: colors.bgPage, color: colors.textSecondary }"
          >{{ countOf(t.value) }}</text>
        </view>
        <view class="tab-line" :style="{ backgroundColor: status == t.value ? colors.primary : 'transparent' }"></view>
      </view>
    </view>

    <!-- 骨架屏 -->
    <view v-if="loading && list.length == 0" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">该状态下暂无工单</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">巡检异常会自动生成工单</text>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="reload">重试</text>
    </view>

    <!-- 工单列表 -->
    <view v-else class="content">
      <view
        v-for="o in list"
        :key="o.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="goDetail(o.id)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ o.title }}</text>
          <text class="card-status" :style="{ color: o.status_color }">{{ o.status_text }}</text>
        </view>
        <text class="card-no" :style="{ color: colors.textSecondary }">单号：{{ o.order_no }}</text>
        <text class="card-sub" :style="{ color: colors.textSecondary }">
          {{ o.community_name }}<text v-if="o.point_name != ''"> · {{ o.point_name }}</text>
        </text>
        <view class="card-foot">
          <view class="foot-tags">
            <text class="tag" :style="{ color: o.priority_color, borderColor: o.priority_color }">{{ o.priority_text }}</text>
            <text v-if="o.assignee_name != null && o.assignee_name != ''" class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">
              处理人：{{ o.assignee_name }}
            </text>
          </view>
          <text class="card-time" :style="{ color: colors.textSecondary }">{{ o.created_at }}</text>
        </view>
      </view>

      <!-- 加载更多状态 -->
      <view class="loadmore">
        <text v-if="loadingMore" class="loadmore-text" :style="{ color: colors.textSecondary }">加载中…</text>
        <text v-else-if="noMore" class="loadmore-text" :style="{ color: colors.textSecondary }">没有更多了</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiManageOrders, ManageOrderItem } from '@/services/api'

const PAGE_SIZE = 20

/** 列表项视图模型：文案/颜色在数据层预计算 */
type OrderView = ManageOrderItem & {
  status_text: string
  status_color: string
  priority_text: string
  priority_color: string
}

type ListData = {
  colors: ColorTokens
  status: string
  tabs: Array<{ label: string; value: string }>
  counts: Record<string, number>
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: OrderView[]
}

/** tab 档位 → 实际下传的 status（逗号多值；已关闭 = 已闭环+已作废） */
const TAB_QUERY: Record<string, string> = {
  reported: 'reported',
  pending_dispatch: 'pending_dispatch',
  processing: 'processing',
  pending_confirm: 'pending_confirm',
  closed: 'closed,closed_invalid'
}

/** 工单状态文案（P2 六态，对齐后端 model.Order* 枚举） */
function statusTextOf(s: string): string {
  if (s == 'reported') return '待受理'
  if (s == 'pending_dispatch') return '待派单'
  if (s == 'processing') return '处理中'
  if (s == 'pending_confirm') return '待验收'
  if (s == 'closed') return '已闭环'
  if (s == 'closed_invalid') return '已作废'
  return s
}

function statusColorOf(s: string): string {
  if (s == 'reported') return Colors.info
  if (s == 'pending_dispatch') return Colors.warning
  if (s == 'processing') return Colors.primary
  if (s == 'pending_confirm') return Colors.warning
  if (s == 'closed') return Colors.success
  if (s == 'closed_invalid') return Colors.danger
  return Colors.info
}

function priorityTextOf(p: string): string {
  if (p == 'urgent') return '紧急'
  if (p == 'high') return '高'
  if (p == 'low') return '低'
  return '普通'
}

function priorityColorOf(p: string): string {
  if (p == 'urgent') return Colors.danger
  if (p == 'high') return Colors.warning
  return Colors.textSecondary
}

function toOrderView(o: ManageOrderItem): OrderView {
  return Object.assign({}, o, {
    status_text: statusTextOf(o.status),
    status_color: statusColorOf(o.status),
    priority_text: priorityTextOf(o.priority),
    priority_color: priorityColorOf(o.priority)
  })
}

export default {
  data(): ListData {
    return {
      colors: Colors,
      status: 'pending_dispatch',
      tabs: [
        { label: '待受理', value: 'reported' },
        { label: '待派单', value: 'pending_dispatch' },
        { label: '处理中', value: 'processing' },
        { label: '待验收', value: 'pending_confirm' },
        { label: '已关闭', value: 'closed' }
      ],
      counts: {},
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as OrderView[]
    }
  },
  computed: {
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    }
  },
  onLoad() {
    this.reload()
  },
  onShow() {
    // 详情页派单/验收返回后刷新
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    switchStatus(v: string) {
      if (this.status == v) return
      this.status = v
      this.reload()
    },
    countOf(v: string): number {
      return this.counts[v] ?? 0
    },
    reload() {
      this.page = 1
      this.fetchPage(false)
    },
    loadMore() {
      if (this.loading || this.loadingMore || this.noMore || !this.loaded) return
      this.page += 1
      this.fetchPage(true)
    },
    fetchPage(append: boolean) {
      if (append) {
        this.loadingMore = true
      } else {
        this.loading = true
      }
      apiManageOrders(this.page, PAGE_SIZE, TAB_QUERY[this.status] ?? this.status)
        .then((res) => {
          this.counts = res.status_counts
          const views: OrderView[] = []
          res.list.forEach((o: ManageOrderItem) => {
            views.push(toOrderView(o))
          })
          this.total = res.total
          this.list = append ? this.list.concat(views) : views
          this.loading = false
          this.loadingMore = false
          this.loaded = true
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          this.loadingMore = false
          if (append) this.page -= 1
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded || append) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: '/pages/admin/workorder-detail?id=' + encodeURIComponent(id) })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.tabs {
  flex-direction: row;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.tab {
  flex: 1;
  align-items: center;
  padding-top: 24rpx;
}

.tab-label {
  flex-direction: row;
  align-items: center;
}

.tab-text {
  font-size: 30rpx;
  font-weight: 600;
}

.tab-count {
  font-size: 20rpx;
  border-radius: 16rpx;
  padding: 2rpx 10rpx;
  margin-left: 8rpx;
}

.tab-line {
  width: 64rpx;
  height: 6rpx;
  border-radius: 3rpx;
  margin-top: 16rpx;
}

.skeleton {
  padding: 24rpx;
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
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-sub {
  font-size: 26rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
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

.card-title {
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-no {
  font-size: 24rpx;
  margin-top: 8rpx;
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

.foot-tags {
  flex-direction: row;
  flex: 1;
  flex-wrap: wrap;
}

.tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
  margin-right: 16rpx;
}

.card-time {
  font-size: 24rpx;
}

.loadmore {
  align-items: center;
  padding: 16rpx 0 32rpx;
}

.loadmore-text {
  font-size: 24rpx;
}
</style>
