<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 顶部操作行：公告入口 + 全部已读 -->
    <view class="topbar">
      <view  hover-class="hover-dim" class="topbar-item" :style="{ backgroundColor: colors.bgCard }" @click="openAnnouncements">
        <text class="topbar-text" :style="{ color: colors.primary }">公告</text>
      </view>
      <view  hover-class="hover-dim" class="topbar-item" :style="{ backgroundColor: colors.bgCard }" @click="markAllRead">
        <text class="topbar-text" :style="{ color: colors.textRegular }">全部已读</text>
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
      <text class="empty-title" :style="{ color: colors.textRegular }">没有新消息，都去巡检啦</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">派单、待签、驳回等提醒会出现在这里</text>
    </view>

    <!-- 消息列表 -->
    <view v-else-if="loaded" class="content">
      <view
        v-for="m in list"
        :key="m.id"
         hover-class="hover-dim" class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="onTap(m)"
      >
        <view class="msg-main">
          <view class="msg-head">
            <view class="type-tag" :style="{ backgroundColor: typeColorOf(m.type) }">
              <text class="type-tag-text" :style="{ color: colors.white }">{{ typeTextOf(m.type) }}</text>
            </view>
            <text class="msg-title" :style="{ color: colors.textPrimary }">{{ m.title }}</text>
            <view v-if="!m.is_read" class="dot" :style="{ backgroundColor: colors.danger }"></view>
          </view>
          <text class="msg-content" :style="{ color: colors.textSecondary }">{{ m.content }}</text>
          <text class="msg-time" :style="{ color: colors.textSecondary }">{{ m.created_at }}</text>
        </view>
      </view>

      <text v-if="finished && list.length > 0" class="list-end" :style="{ color: colors.textSecondary }">没有更多了</text>
      <text v-else-if="loadingMore" class="list-end" :style="{ color: colors.textSecondary }">加载中…</text>
    </view>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="reload">重试</text>
    </view>

    <!-- 公告弹层：已发布公告列表，点击进入公告详情页 -->
    <view v-if="noticeShow" class="mask" :style="{ backgroundColor: colors.mask }" @click="noticeShow = false">
      <view class="notice-panel" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <view class="notice-head">
          <text class="notice-title" :style="{ color: colors.textPrimary }">公告</text>
          <text class="notice-close" :style="{ color: colors.textSecondary }" @click="noticeShow = false">关闭</text>
        </view>
        <scroll-view scroll-y class="notice-scroll">
          <view v-if="noticeLoading" class="notice-empty">
            <text :style="{ color: colors.textSecondary }">加载中…</text>
          </view>
          <view v-else-if="notices.length == 0" class="notice-empty">
            <text :style="{ color: colors.textSecondary }">暂时没有公告</text>
          </view>
          <view v-for="n in notices" :key="n.id"  hover-class="hover-dim" class="notice-item" @click="openNoticeDetail(n.id)">
            <view  hover-class="hover-dim" class="notice-item-head">
              <text  hover-class="hover-dim" class="notice-item-title" :style="{ color: colors.textPrimary }">{{ n.title }}</text>
              <text  hover-class="hover-dim" class="notice-item-time" :style="{ color: colors.textSecondary }">{{ n.publish_at }}</text>
            </view>
            <text  hover-class="hover-dim" class="notice-item-brief" :style="{ color: colors.textSecondary }">{{ n.content }}</text>
          </view>
        </scroll-view>
      </view>
    </view>

    <view class="tabbar-space"></view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiMessages, apiMarkMessageRead, apiAnnouncements, MessageItem, AnnouncementItem } from '@/services/api'
import { useMessageStore } from '@/stores/message'
import { syncBadge } from '@/utils/push'

const PAGE_SIZE = 20

type MessagesData = {
  colors: ColorTokens
  loading: boolean
  loaded: boolean
  loadingMore: boolean
  finished: boolean
  errorMsg: string
  page: number
  total: number
  list: MessageItem[]
  noticeShow: boolean
  noticeLoading: boolean
  notices: AnnouncementItem[]
}

/** 消息类型展示（对齐后端 SysMessage 写入点：report 月报 / checkin_audit 打卡审核 / announcement 公告） */
function typeTextOf(t: string): string {
  if (t == 'report') return '月报'
  if (t == 'checkin_audit') return '审核'
  if (t == 'announcement' || t == 'notice') return '公告'
  return '系统'
}

function typeColorOf(t: string): string {
  if (t == 'report') return Colors.warning
  if (t == 'checkin_audit') return Colors.danger
  if (t == 'announcement' || t == 'notice') return Colors.success
  return Colors.info
}

export default {
  data(): MessagesData {
    return {
      colors: Colors,
      loading: true,
      loaded: false,
      loadingMore: false,
      finished: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as MessageItem[],
      noticeShow: false,
      noticeLoading: false,
      notices: [] as AnnouncementItem[]
    }
  },
  onLoad() {
    this.reload()
  },
  onShow() {
    // 从深链页面返回时刷新（可能已在别处读过/处理过）
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    typeTextOf,
    typeColorOf,
    noop() {},
    reload() {
      this.loading = !this.loaded
      this.page = 1
      this.finished = false
      apiMessages(1, PAGE_SIZE)
        .then((res) => {
          this.loading = false
          this.loaded = true
          this.list = res.list
          this.total = res.total
          this.finished = res.list.length >= res.total
          useMessageStore().setUnread(res.unread_count)
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    loadMore() {
      if (this.loading || this.loadingMore || this.finished || !this.loaded) return
      this.loadingMore = true
      const next = this.page + 1
      apiMessages(next, PAGE_SIZE)
        .then((res) => {
          this.page = next
          this.list = this.list.concat(res.list)
          this.total = res.total
          this.finished = this.list.length >= res.total
          this.loadingMore = false
        })
        .catch((e: Error) => {
          this.loadingMore = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 全部已读 PUT /messages/0/read */
    markAllRead() {
      apiMarkMessageRead(0)
        .then(() => {
          this.list = this.list.map((m) => ({ ...m, is_read: true }))
          useMessageStore().setUnread(0)
          syncBadge() // 图标角标清零（App 端生效）
          uni.showToast({ title: '已全部标记为已读', icon: 'none' })
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 点击单条：标记已读 + 按 biz_id 深链（待签→报告详情；无映射→toast 内容） */
    onTap(m: MessageItem) {
      if (!m.is_read) {
        apiMarkMessageRead(m.id)
          .then(() => {
            m.is_read = true
            const store = useMessageStore()
            store.setUnread(Math.max(0, store.unread - 1))
            syncBadge() // 图标角标跟随最新未读数（App 端生效）
          })
          .catch((_e: any) => {})
      }
      const biz = m.biz_id
      if (m.type == 'report' && biz != null && biz != '') {
        uni.navigateTo({ url: '/pages/reports/detail?id=' + encodeURIComponent(biz) })
        return
      }
      if (m.type == 'announcement') {
        // 公告消息：跳转公告详情页（biz_id = 公告 ID）
        if (biz != null && biz != '') {
          uni.navigateTo({ url: '/pages/messages/notice-detail?id=' + encodeURIComponent(biz) })
          return
        }
        this.openAnnouncements()
        return
      }
      if (m.type == 'checkin_audit') {
        // 巡检员收到「打卡被打回」：直接带他去今日任务补巡（tabBar 页须用 switchTab）
        if (m.title.indexOf('打回') >= 0) {
          uni.switchTab({ url: '/pages/tasks/today' })
          return
        }
        // AI 转人工等管理侧提醒：App 无审核页，展示完整内容
        uni.showModal({ title: m.title, content: m.content, showCancel: false, confirmText: '知道了' })
        return
      }
      // 无跳转映射：直接展示完整内容
      uni.showModal({ title: m.title, content: m.content, showCancel: false, confirmText: '知道了' })
    },

    // ===== 公告 =====
    openAnnouncements() {
      this.noticeShow = true
      if (this.notices.length > 0) return
      this.noticeLoading = true
      apiAnnouncements(1, 50)
        .then((res) => {
          this.notices = res.list
          this.noticeLoading = false
        })
        .catch((e: Error) => {
          this.noticeLoading = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 弹层列表项 → 公告详情页（弹层仅作快速浏览列表） */
    openNoticeDetail(id: string) {
      this.noticeShow = false
      uni.navigateTo({ url: '/pages/messages/notice-detail?id=' + encodeURIComponent(id) })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.topbar {
  flex-direction: row;
  justify-content: space-between;
  margin-bottom: 24rpx;
}

.topbar-item {
  border-radius: 16rpx;
  padding: 16rpx 32rpx;
}

.topbar-text {
  font-size: 26rpx;
}

.skeleton {
  padding-top: 8rpx;
}

.sk-block {
  height: 160rpx;
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

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 28rpx 32rpx;
  margin-bottom: 24rpx;
}

.msg-head {
  flex-direction: row;
  align-items: center;
}

.type-tag {
  border-radius: 8rpx;
  padding: 4rpx 12rpx;
  margin-right: 16rpx;
}

.type-tag-text {
  font-size: 20rpx;
}

.msg-title {
  font-size: 30rpx;
  font-weight: 600;
  flex: 1;
}

.dot {
  width: 16rpx;
  height: 16rpx;
  border-radius: 8rpx;
  margin-left: 16rpx;
}

.msg-content {
  font-size: 26rpx;
  line-height: 38rpx;
  margin-top: 12rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.msg-time {
  font-size: 22rpx;
  margin-top: 12rpx;
}

.list-end {
  font-size: 24rpx;
  text-align: center;
  padding: 24rpx 0;
}

/* 公告弹层 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 998;
  align-items: center;
  justify-content: center;
  padding: 48rpx;
}

.notice-panel {
  width: 100%;
  max-height: 70%;
  border-radius: 24rpx;
  padding: 32rpx;
}

.notice-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16rpx;
}

.notice-title {
  font-size: 34rpx;
  font-weight: 600;
}

.notice-close {
  font-size: 26rpx;
  padding: 8rpx 16rpx;
}

.notice-scroll {
  max-height: 800rpx;
}

.notice-empty {
  align-items: center;
  padding: 64rpx 0;
}

.notice-item {
  padding: 20rpx 0;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
  border-bottom-color: #e5e6eb;
}

.notice-item-head {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.notice-item-title {
  font-size: 28rpx;
  font-weight: 600;
  flex: 1;
}

.notice-item-time {
  font-size: 22rpx;
  margin-left: 16rpx;
}

.notice-item-brief {
  font-size: 24rpx;
  line-height: 34rpx;
  margin-top: 8rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  display: -webkit-box;
  -webkit-line-clamp: 2;
  -webkit-box-orient: vertical;
}

.tabbar-space {
  height: 160rpx;
}
</style>
