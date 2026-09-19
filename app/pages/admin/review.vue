<template>
  <view class="page bg-page" >
    <AppSegmentTabs :items="tabs" :value="status" @change="switchStatus" />

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      :empty-title="emptyTitle"
      empty-sub="下拉可刷新"
     
      @retry="reload"
    >

    <!-- 记录列表 -->
    <template #default>
    <view class="content">
      <view
        v-for="r in list"
        :key="r.id"
        class="card bg-card"
        
        @click="openDetail(r)"
      >
        <view class="card-head">
          <text class="card-title text-main" >{{ r.point_name }}</text>
          <text class="card-status"  :class="(r.result == 'abnormal' ? 'text-danger' : 'text-success')">
            {{ r.result == 'abnormal' ? '异常' : '正常' }}
          </text>
        </view>
        <text class="card-sub text-secondary" >{{ r.community_name }} · {{ r.inspector_name }}</text>
        <view class="card-foot">
          <view class="foot-tags">
            <uni-tag :text="typeTextOf(r.checkin_type)" :inverted="true" size="small" :custom-style="'color:#86909C;border-color:#E5E6EB;margin-right:16rpx;margin-bottom:8rpx'" />
            <uni-tag v-if="r.is_suspect" text="疑似作弊" :inverted="true" size="small" :custom-style="'color:#ED7B2F;border-color:#ED7B2F;margin-right:16rpx;margin-bottom:8rpx'" />
            <uni-tag v-if="status == 'pending' && r.can_audit === false" text="待授权人处理" :inverted="true" size="small" :custom-style="'color:#86909C;border-color:#E5E6EB;margin-right:16rpx;margin-bottom:8rpx'" />
          </view>
          <text class="card-time text-secondary" >{{ r.checkin_time }}</text>
        </view>
      </view>

    </view>
    </template>
    <template #footer>
      <AppListFooter :loading-more="loadingMore" :no-more="noMore" :visible="list.length > 0" />
    </template>
    </AppListShell>

    <!-- 详情弹层 -->
    <AppBottomSheet
      :visible="detail != null"
      :mask-color="'rgba(0, 0, 0, 0.45)'"
      :background-color="'#F5F6F8'"
      height="80%"
      @close="closeDetail"
    >
      <template v-if="detail != null">
        <scroll-view scroll-y class="sheet-scroll">
          <view class="sheet-head">
            <text class="sheet-title text-main" >{{ detail.point_name }}</text>
            <text class="sheet-close text-secondary"  @click="closeDetail">×</text>
          </view>

          <!-- 详情主体：与独立详情页共用 CheckinDetailView（ReviewRecord 结构对齐其 props，可直接传入） -->
          <CheckinDetailView :record="detail" />
        </scroll-view>

        <!-- 待审核操作：不在当前环节授权名单内的只给说明，不让点了再报错 -->
        <view v-if="detail.audit_status == 'pending' && detail.can_audit !== false" class="sheet-actions bg-card border-default" >
          <button plain="true" class="btn-half btn-danger" hover-class="hover-dim" @click="onRejectTap">
            <text class="btn-half-text">驳回</text>
          </button>
          <button plain="true" class="btn-half btn-success" hover-class="hover-dim" @click="onPass">
            <text class="btn-half-text">通过</text>
          </button>
        </view>
        <view v-else-if="detail.audit_status == 'pending'" class="sheet-actions bg-card border-default" >
          <text class="no-auth-text text-secondary" >当前环节「{{ detail.current_step_name || '审核' }}」· 你不在授权名单内，待授权人处理</text>
        </view>
      </template>
    </AppBottomSheet>

    <!-- 驳回原因弹层 -->
    <view v-if="rejecting" class="mask mask-center bg-mask"  @click="rejecting = false">
      <view class="dialog bg-card"  @click.stop="">
        <text class="dialog-title text-main" >驳回原因（必填）</text>
        <textarea
          v-model="rejectReason"
          class="dialog-input border-default text-main"
          
          placeholder="请填写驳回原因"
          :maxlength="200"
        />
        <view class="dialog-actions">
          <text class="dialog-btn text-secondary"  @click="rejecting = false">取消</text>
          <text class="dialog-btn text-danger"  @click="onRejectConfirm">确认驳回</text>
        </view>
      </view>
    </view>

    <!-- 审核通过确认（自绘，替代原生 showModal） -->
    <AppDialog
      :visible="passDlgShow"
      kind="success"
      title="审核通过"
      content="确认该打卡记录审核通过？"
      confirm-text="通过"
      cancel-text="取消"
      @update:visible="passDlgShow = $event"
      @confirm="onPassConfirm"
    />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiReviewRecords, apiReviewPass, apiReviewReject, ReviewRecord } from '@/services/api'
import AppBottomSheet from '@/components/AppBottomSheet.vue'
import AppListShell from '@/components/AppListShell.vue'
import AppSegmentTabs from '@/components/AppSegmentTabs.vue'
import AppListFooter from '@/components/AppListFooter.vue'
import AppDialog from '@/components/AppDialog.vue'
import CheckinDetailView from '@/components/CheckinDetailView.vue'

const PAGE_SIZE = 20

type ReviewData = {
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
  /** 审核通过确认弹窗 */
  passDlgShow: boolean
}

function typeTextOf(t: string): string {
  if (t == 'qrcode') return '扫码'
  if (t == 'nfc') return 'NFC'
  if (t == 'offline') return '离线'
  return '围栏'
}

export default {
  components: { AppBottomSheet, AppListShell, AppListFooter, AppSegmentTabs, AppDialog, CheckinDetailView },
  data(): ReviewData {
    return {
      status: 'pending',
      tabs: [
        { label: '待审核', value: 'pending' },
        { label: '已通过', value: 'pass' },
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
      focusId: '',
      passDlgShow: false
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
        .catch((e: Error) => {
          uni.showToast({ title: e.message || '记录加载失败', icon: 'none' })
        })
    },
    openDetail(r: ReviewRecord) {
      this.detail = r
    },
    closeDetail() {
      this.detail = null
      this.rejecting = false
      this.rejectReason = ''
    },
    onPass() {
      if (this.detail == null || this.acting) return
      this.passDlgShow = true
    },
    onPassConfirm() {
      if (this.detail == null || this.acting) return
      const id = this.detail.id
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


.card-time {
  font-size: 24rpx;
}


.sheet-scroll {
  /* 半屏 maxHeight 80vh：扣除头部与底部操作栏，滚动区显式定高（uvue 下 flex:1 不收敛会溢出） */
  height: 56vh;
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

.sheet-actions {
  flex-direction: row;
  padding: 24rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
}

.no-auth-text {
  flex: 1;
  text-align: center;
  font-size: 26rpx;
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
