<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 类型 tab：全部 / 我上报的 / 指派给我（有待接单时红点提示） -->
    <view class="tabs" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <view
        v-for="t in tabs"
        :key="t.value"
        class="tab"
        @click="switchType(t.value)"
      >
        <view class="tab-label">
          <text
            class="tab-text"
            :style="{ color: type == t.value ? colors.primary : colors.textRegular }"
          >{{ t.label }}</text>
          <view
            v-if="t.value == 'assigned' && assignedNew > 0"
            class="tab-dot"
            :style="{ backgroundColor: colors.danger }"
          ></view>
        </view>
        <view
          class="tab-line"
          :style="{ backgroundColor: type == t.value ? colors.primary : 'transparent' }"
        ></view>
      </view>
    </view>

    <!-- 状态筛选 chips（横向滚动，带数量角标） -->
    <scroll-view scroll-x class="chips" :show-scrollbar="false">
      <view class="chips-inner">
        <view
          v-for="c in statusChips"
          :key="c.value"
          class="chip"
          :style="status == c.value
            ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
            : { backgroundColor: colors.bgCard, borderColor: colors.border }"
          @click="switchStatus(c.value)"
        >
          <text
            class="chip-text"
            :style="{ color: status == c.value ? colors.primary : colors.textSecondary }"
          >{{ c.label }}</text>
          <text
            v-if="chipCount(c.value) > 0"
            class="chip-count"
            :style="status == c.value
              ? { backgroundColor: colors.primary, color: colors.white }
              : { backgroundColor: colors.bgPage, color: colors.textSecondary }"
          >{{ chipCount(c.value) }}</text>
        </view>
      </view>
    </scroll-view>

    <!-- 骨架屏 -->
    <view v-if="loading && list.length == 0" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 空态（按当前 tab/状态给引导文案） -->
    <view v-else-if="loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ emptyTitle }}</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">{{ emptySub }}</text>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="reload">重试</text>
    </view>

    <!-- 工单卡片列表 -->
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
            <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ o.role_text }}</text>
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

    <view class="tabbar-space"></view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiMyOrders, apiMyOrderCounts, MyOrderItem } from '@/services/api'

const PAGE_SIZE = 20

/** 列表项视图模型：文案/颜色在数据层预计算 */
type OrderView = MyOrderItem & {
  status_text: string
  status_color: string
  priority_text: string
  priority_color: string
  role_text: string
}

type ListData = {
  colors: ColorTokens
  type: string
  status: string
  tabs: Array<{ label: string; value: string }>
  statusChips: Array<{ label: string; value: string }>
  /** 当前类型下各状态工单数（chip 角标） */
  counts: Record<string, number>
  /** 指派给我且待接单数量（tab 红点） */
  assignedNew: number
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: OrderView[]
}

/** 工单状态文案（对齐后端 model.Order* 枚举） */
function statusTextOf(s: string): string {
  if (s == 'pending') return '待派单'
  if (s == 'assigned') return '待接单'
  if (s == 'processing') return '处理中'
  if (s == 'review') return '待审核'
  if (s == 'closed') return '已完成'
  if (s == 'rejected') return '已驳回'
  return s
}

function statusColorOf(s: string): string {
  if (s == 'pending') return Colors.info
  if (s == 'assigned') return Colors.warning
  if (s == 'processing') return Colors.primary
  if (s == 'review') return Colors.warning
  if (s == 'closed') return Colors.success
  if (s == 'rejected') return Colors.danger
  return Colors.info
}

/** 优先级文案（对齐后端 low/normal/high/urgent） */
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

function toOrderView(o: MyOrderItem): OrderView {
  return Object.assign({}, o, {
    status_text: statusTextOf(o.status),
    status_color: statusColorOf(o.status),
    priority_text: priorityTextOf(o.priority),
    priority_color: priorityColorOf(o.priority),
    role_text: o.my_role == 'assignee' ? '指派给我' : '我上报的'
  })
}

export default {
  data(): ListData {
    return {
      colors: Colors,
      type: '',
      status: '',
      tabs: [
        { label: '全部', value: '' },
        { label: '我上报的', value: 'reported' },
        { label: '指派给我', value: 'assigned' }
      ],
      // 状态精简：待派单+待接单合并为「待处理」（后端 status 支持逗号多值）
      statusChips: [
        { label: '全部状态', value: '' },
        { label: '待处理', value: 'pending,assigned' },
        { label: '处理中', value: 'processing' },
        { label: '待审核', value: 'review' },
        { label: '已完成', value: 'closed' },
        { label: '已驳回', value: 'rejected' }
      ],
      counts: {},
      assignedNew: 0,
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
    /** 是否已加载完全部 */
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    },
    /** 空态标题：状态筛选中 > 按 tab 区分 */
    emptyTitle(): string {
      if (this.status != '') return '该状态下暂无工单'
      if (this.type == 'assigned') return '暂无指派给你的工单'
      if (this.type == 'reported') return '暂无你上报的工单'
      return '暂无工单'
    },
    emptySub(): string {
      if (this.status != '') return '切换其他状态看看'
      if (this.type == 'assigned') return '主管派单后会第一时间通知你'
      return '巡检异常会自动生成工单'
    }
  },
  onLoad() {
    this.reload()
  },
  onShow() {
    // 详情页接单/完工返回后刷新列表
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    /** 切类型 tab（重置分页） */
    switchType(v: string) {
      if (this.type == v) return
      this.type = v
      this.reload()
    },
    /** 切状态筛选（重置分页） */
    switchStatus(v: string) {
      if (this.status == v) return
      this.status = v
      this.reload()
    },
    /** 重新加载第一页 */
    reload() {
      this.page = 1
      this.fetchPage(false)
      this.fetchCounts()
    },
    /** chip 角标数量（待处理 = 待派单+待接单；全部状态不显示角标） */
    chipCount(v: string): number {
      if (v == '') return 0
      let n = 0
      const parts = v.split(',')
      for (let i = 0; i < parts.length; i++) {
        n += this.counts[parts[i]] ?? 0
      }
      return n
    },
    /** 拉取状态计数：当前类型（chip 角标）+ 指派给我待接单（tab 红点）；失败静默不影响列表 */
    fetchCounts() {
      apiMyOrderCounts(this.type)
        .then((d) => {
          this.counts = d
          if (this.type == 'assigned') this.assignedNew = d['assigned'] ?? 0
        })
        .catch((_e) => {})
      if (this.type != 'assigned') {
        apiMyOrderCounts('assigned')
          .then((d) => {
            this.assignedNew = d['assigned'] ?? 0
          })
          .catch((_e) => {})
      }
    },
    /** 上拉加载更多 */
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
      apiMyOrders(this.page, PAGE_SIZE, this.type, this.status)
        .then((res) => {
          const views: OrderView[] = []
          res.list.forEach((o: MyOrderItem) => {
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
          // 加载更多失败回退页码，下拉/首屏失败给错误态或 toast
          if (append) this.page -= 1
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded || append) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goDetail(id: string) {
      uni.navigateTo({ url: '/pages/workorders/detail?id=' + encodeURIComponent(id) })
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

.tab-dot {
  width: 14rpx;
  height: 14rpx;
  border-radius: 7rpx;
  margin-left: 8rpx;
}

.tab-text {
  font-size: 30rpx;
  font-weight: 600;
}

.tab-line {
  width: 64rpx;
  height: 6rpx;
  border-radius: 3rpx;
  margin-top: 16rpx;
}

.chips {
  padding: 16rpx 24rpx 0;
}

/* inline-flex 让内容宽度按 chip 总宽撑开（超出 scroll-view 才可横向滑动）；
   全局 flex 规则会让 flex 子项默认收缩，这里必须 inline-flex + flex-shrink:0 */
.chips-inner {
  display: inline-flex;
  flex-direction: row;
  flex-wrap: nowrap;
}

.chip {
  flex-shrink: 0;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  padding: 8rpx 24rpx;
  margin-right: 16rpx;
}

.chip-text {
  font-size: 26rpx;
  white-space: nowrap;
}

.chip-count {
  font-size: 20rpx;
  border-radius: 16rpx;
  padding: 2rpx 10rpx;
  margin-left: 8rpx;
  white-space: nowrap;
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

.tabbar-space {
  height: 160rpx;
}
</style>
