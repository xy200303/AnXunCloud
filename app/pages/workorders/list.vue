<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 类型 tab：派给我的 / 我上报的 / 工单池（有待办时红点提示；工单池按 counts.pool 显隐） -->
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
            v-if="tabBadge(t.value) > 0"
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

    <!-- 状态筛选 chips（横向滚动，带数量角标；工单池均为待派单，不展示） -->
    <scroll-view v-if="type != 'pool'" scroll-x class="chips" :show-scrollbar="false">
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
  /** 当前类型下各状态工单数（chip 角标） */
  counts: Record<string, number>
  /** 各 tab 待办数（tab 红点：assigned=待我处理 / reported=待我验收 / pool=可抢） */
  badgeAssigned: number
  badgeReported: number
  badgePool: number
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: OrderView[]
}

/** 工单状态文案（P2 六态，对齐后端 model.Order* 枚举） */
function statusTextOf(s: string): string {
  if (s == 'reported') return '待分诊'
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
    role_text: o.my_role == 'assignee' ? '派给我' : (o.my_role == 'reporter' ? '我上报的' : '可抢单')
  })
}

/** 各 tab 的状态筛选 chips（后端 status 支持逗号多值） */
const STATUS_CHIPS: Record<string, Array<{ label: string; value: string }>> = {
  assigned: [
    { label: '全部状态', value: '' },
    { label: '处理中', value: 'processing' },
    { label: '待验收', value: 'pending_confirm' },
    { label: '已闭环', value: 'closed' },
    { label: '已作废', value: 'closed_invalid' }
  ],
  reported: [
    { label: '全部状态', value: '' },
    { label: '待分诊', value: 'reported' },
    { label: '待派单', value: 'pending_dispatch' },
    { label: '处理中', value: 'processing' },
    { label: '待验收', value: 'pending_confirm' },
    { label: '已闭环', value: 'closed' },
    { label: '已作废', value: 'closed_invalid' }
  ],
  pool: []
}

export default {
  data(): ListData {
    return {
      colors: Colors,
      type: 'assigned',
      status: '',
      counts: {},
      badgeAssigned: 0,
      badgeReported: 0,
      badgePool: 0,
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
    /** tab 列表：工单池仅在可抢数 >0（或正处于该 tab）时显示 */
    tabs(): Array<{ label: string; value: string }> {
      const arr = [
        { label: '派给我的', value: 'assigned' },
        { label: '我上报的', value: 'reported' }
      ]
      if (this.badgePool > 0 || this.type == 'pool') {
        arr.push({ label: '工单池', value: 'pool' })
      }
      return arr
    },
    /** 当前 tab 的状态筛选 chips */
    statusChips(): Array<{ label: string; value: string }> {
      return STATUS_CHIPS[this.type] ?? []
    },
    /** 是否已加载完全部 */
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    },
    /** 空态标题：状态筛选中 > 按 tab 区分 */
    emptyTitle(): string {
      if (this.status != '') return '该状态下暂无工单'
      if (this.type == 'assigned') return '暂无派给你的工单'
      if (this.type == 'reported') return '暂无你上报的工单'
      if (this.type == 'pool') return '工单池暂无可抢工单'
      return '暂无工单'
    },
    emptySub(): string {
      if (this.status != '') return '切换其他状态看看'
      if (this.type == 'assigned') return '派单或抢单后会第一时间通知你'
      if (this.type == 'pool') return '项目开启抢单后，新工单会出现在这里'
      return '可通过首页「问题上报」主动提交问题'
    }
  },
  onLoad() {
    this.reload()
  },
  onShow() {
    // 详情页抢单/完工/验收返回后刷新列表
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    /** 切类型 tab（重置分页与状态筛选） */
    switchType(v: string) {
      if (this.type == v) return
      this.type = v
      this.status = ''
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
    /** tab 红点数量：assigned=待我处理 / reported=待我验收 / pool=可抢 */
    tabBadge(v: string): number {
      if (v == 'assigned') return this.badgeAssigned
      if (v == 'reported') return this.badgeReported
      if (v == 'pool') return this.badgePool
      return 0
    },
    /** chip 角标数量（全部状态不显示角标） */
    chipCount(v: string): number {
      if (v == '') return 0
      let n = 0
      const parts = v.split(',')
      for (let i = 0; i < parts.length; i++) {
        n += this.counts[parts[i]] ?? 0
      }
      return n
    },
    /**
     * 拉取状态计数：assigned/reported 各一次（chip 角标 + tab 红点），
     * 两次响应均带 pool 可抢池数量；失败静默不影响列表。
     */
    fetchCounts() {
      apiMyOrderCounts('assigned')
        .then((d) => {
          this.badgeAssigned = d['processing'] ?? 0
          this.badgePool = d['pool'] ?? 0
          if (this.type == 'assigned') this.counts = d
        })
        .catch((_e) => {})
      apiMyOrderCounts('reported')
        .then((d) => {
          this.badgeReported = d['pending_confirm'] ?? 0
          if (this.type == 'reported') this.counts = d
        })
        .catch((_e) => {})
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
