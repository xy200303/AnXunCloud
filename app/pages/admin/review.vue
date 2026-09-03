<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 审核状态 tab -->
    <view class="tabs" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <view v-for="t in tabs" :key="t.value" class="tab" @click="switchStatus(t.value)">
        <text class="tab-text" :style="{ color: status == t.value ? colors.primary : colors.textRegular }">{{ t.label }}</text>
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
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ emptyTitle }}</text>
      <text class="empty-sub" :style="{ color: colors.textSecondary }">下拉可刷新</text>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded && list.length == 0" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="reload">重试</text>
    </view>

    <!-- 记录列表 -->
    <view v-else class="content">
      <view
        v-for="r in list"
        :key="r.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="openDetail(r)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ r.point_name }}</text>
          <text class="card-status" :style="{ color: r.result == 'abnormal' ? colors.danger : colors.success }">
            {{ r.result == 'abnormal' ? '异常' : '正常' }}
          </text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">{{ r.community_name }} · {{ r.inspector_name }}</text>
        <view class="card-foot">
          <view class="foot-tags">
            <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ typeTextOf(r.checkin_type) }}</text>
            <text v-if="r.is_suspect" class="tag" :style="{ color: colors.warning, borderColor: colors.warning }">疑似作弊</text>
          </view>
          <text class="card-time" :style="{ color: colors.textSecondary }">{{ r.checkin_time }}</text>
        </view>
      </view>

      <!-- 加载更多状态 -->
      <view class="loadmore">
        <text v-if="loadingMore" class="loadmore-text" :style="{ color: colors.textSecondary }">加载中…</text>
        <text v-else-if="noMore" class="loadmore-text" :style="{ color: colors.textSecondary }">没有更多了</text>
      </view>
    </view>

    <!-- 详情弹层 -->
    <view v-if="detail != null" class="mask" :style="{ backgroundColor: colors.mask }" @click="closeDetail">
      <view class="sheet" :style="{ backgroundColor: colors.bgPage, borderTopLeftRadius: '32rpx', borderTopRightRadius: '32rpx' }" @click.stop="">
        <scroll-view scroll-y class="sheet-scroll">
          <view class="sheet-head">
            <text class="sheet-title" :style="{ color: colors.textPrimary }">{{ detail.point_name }}</text>
            <text class="sheet-close" :style="{ color: colors.textSecondary }" @click="closeDetail">×</text>
          </view>

          <!-- 基本信息 -->
          <view class="card" :style="{ backgroundColor: colors.bgCard }">
            <text class="info-line" :style="{ color: colors.textRegular }">小区：{{ detail.community_name }}</text>
            <text class="info-line" :style="{ color: colors.textRegular }">巡检员：{{ detail.inspector_name }}</text>
            <text class="info-line" :style="{ color: colors.textRegular }">打卡时间：{{ detail.checkin_time }}</text>
            <text class="info-line" :style="{ color: colors.textRegular }">打卡方式：{{ typeTextOf(detail.checkin_type) }}</text>
            <text v-if="detail.distance_to_point != null" class="info-line" :style="{ color: colors.textRegular }">
              距点位：{{ detail.distance_to_point }} m
            </text>
            <text class="info-line" :style="{ color: detail.result == 'abnormal' ? colors.danger : colors.success }">
              结果：{{ detail.result == 'abnormal' ? '异常' : '正常' }}
            </text>
            <text v-if="detail.is_suspect" class="info-line" :style="{ color: colors.warning }">
              疑似作弊：{{ detail.suspect_reason != '' ? detail.suspect_reason : '系统标记' }}
            </text>
            <text v-if="detail.remark != ''" class="info-line" :style="{ color: colors.textRegular }">备注：{{ detail.remark }}</text>
          </view>

          <!-- AI 结论（有才显示） -->
          <view v-if="detail.ai_verdict != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
            <text class="sec-title" :style="{ color: colors.textPrimary }">AI 审核结论</text>
            <text class="info-line" :style="{ color: colors.textRegular }">结论：{{ detail.ai_verdict }}</text>
            <text v-if="detail.ai_reason != ''" class="info-line" :style="{ color: colors.textSecondary }">{{ detail.ai_reason }}</text>
          </view>

          <!-- 检查项逐项结果 -->
          <view v-if="detail.check_items.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
            <text class="sec-title" :style="{ color: colors.textPrimary }">检查项</text>
            <view v-for="(it, idx) in detail.check_items" :key="idx" class="check-item">
              <view class="check-item-head">
                <text class="check-item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
                <text class="check-item-result" :style="{ color: it.pass ? colors.success : colors.danger }">{{ it.pass ? '正常' : '异常' }}</text>
              </view>
              <text v-if="it.note != ''" class="check-item-note" :style="{ color: colors.textSecondary }">{{ it.note }}</text>
              <view v-if="itemPhotoUrls(it).length > 0" class="photos">
                <image
                  v-for="(u, pi) in itemPhotoUrls(it)"
                  :key="pi"
                  class="photo"
                  :src="u"
                  mode="aspectFill"
                  @click="preview(itemPhotoUrls(it), pi)"
                />
              </view>
            </view>
          </view>

          <!-- 现场照片墙 -->
          <view v-if="photoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
            <text class="sec-title" :style="{ color: colors.textPrimary }">现场照片</text>
            <view class="photos">
              <image
                v-for="(u, pi) in photoUrls"
                :key="pi"
                class="photo"
                :src="u"
                mode="aspectFill"
                @click="preview(photoUrls, pi)"
              />
            </view>
          </view>

          <!-- 已审核信息 -->
          <view v-if="detail.audit_status != 'pending'" class="card" :style="{ backgroundColor: colors.bgCard }">
            <text class="sec-title" :style="{ color: colors.textPrimary }">审核结果</text>
            <text class="info-line" :style="{ color: detail.audit_status == 'passed' ? colors.success : colors.danger }">
              {{ detail.audit_status == 'passed' ? '已通过' : '已驳回' }}<text v-if="detail.audit_at != null"> · {{ detail.audit_at }}</text>
            </text>
            <text v-if="detail.audit_remark != ''" class="info-line" :style="{ color: colors.textSecondary }">意见：{{ detail.audit_remark }}</text>
          </view>
        </scroll-view>

        <!-- 待审核操作 -->
        <view v-if="detail.audit_status == 'pending'" class="sheet-actions" :style="{ backgroundColor: colors.bgCard, borderTopColor: colors.border }">
          <view class="btn-half" :style="{ borderColor: colors.danger }" @click="onRejectTap">
            <text class="btn-half-text" :style="{ color: colors.danger }">驳回</text>
          </view>
          <view class="btn-half btn-half-solid" :style="{ backgroundColor: colors.success }" @click="onPass">
            <text class="btn-half-text" :style="{ color: colors.white }">通过</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 驳回原因弹层 -->
    <view v-if="rejecting" class="mask mask-center" :style="{ backgroundColor: colors.mask }" @click="rejecting = false">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }" @click.stop="">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">驳回原因（必填）</text>
        <textarea
          v-model="rejectReason"
          class="dialog-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="请填写驳回原因"
          :maxlength="200"
        />
        <view class="dialog-actions">
          <text class="dialog-btn" :style="{ color: colors.textSecondary }" @click="rejecting = false">取消</text>
          <text class="dialog-btn" :style="{ color: colors.danger }" @click="onRejectConfirm">确认驳回</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiReviewRecords, apiReviewPass, apiReviewReject, ReviewRecord } from '@/services/api'
import { toAbsUrl } from '@/utils/url'

const PAGE_SIZE = 20

type ReviewData = {
  colors: ColorTokens
  status: string
  tabs: Array<{ label: string; value: string }>
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: ReviewRecord[]
  detail: ReviewRecord | null
  rejecting: boolean
  rejectReason: string
  acting: boolean
  /** 消息深链带入的记录 ID：首屏加载后自动打开该记录详情 */
  focusId: string
}

function typeTextOf(t: string): string {
  if (t == 'qrcode') return '扫码'
  if (t == 'nfc') return 'NFC'
  if (t == 'offline') return '离线'
  return '围栏'
}

/** 照片展示：优先水印图，其次原图，统一转绝对地址 */
function photoUrl(p: { url: string; watermarked_url: string }): string {
  return toAbsUrl(p.watermarked_url != '' ? p.watermarked_url : p.url)
}

export default {
  data(): ReviewData {
    return {
      colors: Colors,
      status: 'pending',
      tabs: [
        { label: '待审核', value: 'pending' },
        { label: '已通过', value: 'passed' },
        { label: '已驳回', value: 'rejected' }
      ],
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as ReviewRecord[],
      detail: null,
      rejecting: false,
      rejectReason: '',
      acting: false,
      focusId: ''
    }
  },
  computed: {
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    },
    emptyTitle(): string {
      if (this.status == 'passed') return '暂无已通过记录'
      if (this.status == 'rejected') return '暂无已驳回记录'
      return '暂无待审核打卡'
    },
    /** 详情整单照片绝对地址列表 */
    photoUrls(): string[] {
      if (this.detail == null) return []
      return (this.detail.photos ?? []).map(photoUrl)
    }
  },
  onLoad(options: any) {
    this.focusId = options && options.id ? String(options.id) : ''
    this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    typeTextOf,
    switchStatus(v: string) {
      if (this.status == v) return
      this.status = v
      this.closeDetail()
      this.reload()
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
      apiReviewRecords(this.page, PAGE_SIZE, this.status)
        .then((res) => {
          this.total = res.total
          this.list = append ? this.list.concat(res.list) : res.list
          this.loading = false
          this.loadingMore = false
          this.loaded = true
          uni.stopPullDownRefresh()
          if (!append) this.openFocus()
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
    /** 消息深链：按 id 精确查该记录（不带状态过滤，任何审核态都能开）并直接弹出详情 */
    openFocus() {
      if (this.focusId == '') return
      const id = this.focusId
      this.focusId = ''
      apiReviewRecords(1, 1, '', id)
        .then((res) => {
          if (res.list.length == 0) {
            uni.showToast({ title: '该记录不在你的审核范围内', icon: 'none' })
            return
          }
          this.openDetail(res.list[0])
        })
        .catch(() => {})
    },
    openDetail(r: ReviewRecord) {
      this.detail = r
    },
    closeDetail() {
      this.detail = null
      this.rejecting = false
      this.rejectReason = ''
    },
    /** 检查项照片：后端已按文件 ID 解析 URL，旧数据再回退存储路径。 */
    itemPhotoUrls(it: { photos: string[]; photo_urls?: string[] }): string[] {
      if ((it.photo_urls ?? []).length > 0) return (it.photo_urls ?? []).map(toAbsUrl)
      return (it.photos ?? []).map((key) => toAbsUrl('/uploads/' + key))
    },
    preview(urls: string[], idx: number) {
      uni.previewImage({ urls: urls, current: urls[idx] })
    },
    onPass() {
      if (this.detail == null || this.acting) return
      const id = this.detail.id
      uni.showModal({
        title: '审核通过',
        content: '确认该打卡记录审核通过？',
        confirmText: '通过',
        success: (res) => {
          if (!res.confirm) return
          this.acting = true
          apiReviewPass(id)
            .then(() => {
              uni.showToast({ title: '已通过', icon: 'none' })
              this.closeDetail()
              this.reload()
            })
            .catch((e: Error) => {
              uni.showToast({ title: e.message, icon: 'none' })
            })
            .finally(() => {
              this.acting = false
            })
        }
      })
    },
    onRejectTap() {
      this.rejectReason = ''
      this.rejecting = true
    },
    onRejectConfirm() {
      if (this.detail == null || this.acting) return
      const reason = this.rejectReason.trim()
      if (reason == '') {
        uni.showToast({ title: '请填写驳回原因', icon: 'none' })
        return
      }
      const id = this.detail.id
      this.acting = true
      apiReviewReject(id, reason)
        .then(() => {
          uni.showToast({ title: '已驳回', icon: 'none' })
          this.closeDetail()
          this.reload()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
        })
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

/* 详情弹层 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  justify-content: flex-end;
  z-index: 99;
}

.sheet {
  height: 80%;
}

.sheet-scroll {
  flex: 1;
  min-height: 0;
  padding: 24rpx;
}

.sheet-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 8rpx 8rpx 16rpx;
}

.sheet-title {
  font-size: 34rpx;
  font-weight: 600;
  flex: 1;
}

.sheet-close {
  font-size: 48rpx;
  padding: 0 16rpx;
  line-height: 48rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.check-item {
  margin-top: 16rpx;
}

.check-item-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.check-item-name {
  font-size: 28rpx;
  flex: 1;
}

.check-item-result {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.check-item-note {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.sheet-actions {
  flex-direction: row;
  padding: 24rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
}

.btn-half {
  flex: 1;
  height: 88rpx;
  border-radius: 20rpx; /* Radius.button */
  border-width: 2rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
}

.btn-half-solid {
  border-width: 0;
  margin-right: 0;
}

.btn-half-text {
  font-size: 30rpx;
  font-weight: 600;
}

/* 驳回原因对话框 */
.mask-center {
  justify-content: center;
  align-items: center;
}

.dialog {
  width: 600rpx;
  border-radius: 24rpx;
  padding: 32rpx;
}

.dialog-title {
  font-size: 32rpx;
  font-weight: 600;
}

.dialog-input {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 24rpx;
}

.dialog-actions {
  flex-direction: row;
  justify-content: flex-end;
  margin-top: 24rpx;
}

.dialog-btn {
  font-size: 30rpx;
  padding: 8rpx 24rpx;
}
</style>
