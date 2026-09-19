<template>
  <view class="page bg-page" >
    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无待确认的维保登记"
      empty-sub="巡检员现场登记后会出现在这里"
     
      @retry="reload"
    >
      <!-- pending 列表：AI 存疑（review）后端已置顶，行头红条加强提示；点卡片开详情弹层（与打卡审核同交互） -->
      <template #default>
      <view class="content">
        <view
          v-for="m in list"
          :key="m.id"
          class="card bg-card"
           :class="(m.ai_verdict == 'review' ? 'border-danger' : 'border-card')"
          hover-class="hover-dim"
          @click="openDetail(m)"
        >
          <view class="card-head">
            <view class="card-title-row">
              <!-- 多选框 -->
              <uni-icons
                class="check-icon"
                :type="selected[m.id] ? 'checkbox-filled' : 'circle'"
                size="22"
                :color="selected[m.id] ? '#2B5AED' : '#E5E6EB'"
                @click.stop="toggleSelect(m.id)"
              />
              <text class="card-title text-main" >{{ m.equipment_name }}</text>
            </view>
            <text v-if="m.ai_verdict == 'review'" class="card-status text-danger" >AI 存疑</text>
            <text v-else-if="m.ai_verdict == 'pass'" class="card-status text-success" >AI 通过</text>
            <text v-else class="card-status text-secondary" >未预检</text>
          </view>
          <text class="card-sub text-secondary" >编号：{{ m.equipment_code }}<text v-if="m.point_name != null && m.point_name != ''"> · {{ m.point_name }}</text></text>
          <text class="card-sub text-secondary" >
            {{ maintTypeText(m.maintenance_type) }} · {{ m.maintenance_date }} · {{ m.operator_name }} 经办 · {{ m.created_by_name }} 登记
          </text>
          <view class="card-foot">
            <text v-if="m.ai_verdict == 'review' && m.ai_reason != null && m.ai_reason != ''" class="card-ai text-danger" >AI 说明：{{ m.ai_reason }}</text>
            <text v-if="m.can_confirm === false" class="card-noauth text-secondary" >待授权人处理</text>
          </view>
        </view>
      </view>
      </template>
      <template #footer>
        <AppListFooter :loading-more="loadingMore" :no-more="noMore" :visible="list.length > 0" />
      </template>
    </AppListShell>

    <!-- 详情弹层（与打卡审核同交互：点开看明细，底部驳回/通过） -->
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
            <text class="sheet-title text-main" >{{ detail.equipment_name }}</text>
            <text class="sheet-close text-secondary"  @click="closeDetail">×</text>
          </view>
          <view class="sheet-body">
            <text class="info-line text-regular" >编号：{{ detail.equipment_code }}<text v-if="detail.point_name != null && detail.point_name != ''"> · {{ detail.point_name }}</text></text>
            <text class="info-line text-regular" >
              {{ maintTypeText(detail.maintenance_type) }} · {{ detail.maintenance_date }} · {{ detail.operator_name }} 经办 · {{ detail.created_by_name }} 登记
            </text>
            <text v-if="detail.vendor != null && detail.vendor != ''" class="info-line text-regular" >维保单位：{{ detail.vendor }}</text>
            <text v-if="detail.maintenance_type == 'ledger_fix'" class="info-line text-regular" >
              补录日期：出厂 {{ detail.fix_manufacture_date != '' ? detail.fix_manufacture_date : '--' }} / 维保 {{ detail.fix_last_maintenance_date != '' ? detail.fix_last_maintenance_date : '--' }}
            </text>
            <text v-if="detail.note != ''" class="info-line text-regular" >备注：{{ detail.note }}</text>
            <text v-if="detail.ai_verdict == 'review' && detail.ai_reason != null && detail.ai_reason != ''" class="info-line text-danger" >AI 说明：{{ detail.ai_reason }}</text>
            <text class="info-line text-secondary" >登记时间：{{ detail.created_at }}</text>
            <view class="photos">
              <image
                v-for="(p, pi) in detail.photos"
                :key="p.file_id"
                class="photo"
                :src="toAbsUrl(p.url)"
                mode="aspectFill"
                lazy-load
                @click="preview(detail, pi)"
              />
              <text v-if="detail.photos.length == 0" class="info-line text-secondary" >无照片</text>
            </view>
          </view>
        </scroll-view>

        <!-- 底部操作：不在当前环节授权名单内的只给说明，不让点了再报错 -->
        <view v-if="detail.can_confirm !== false" class="sheet-actions bg-card border-default" >
          <button plain="true" class="btn-half btn-outline-danger" hover-class="hover-dim" @click="onRejectTap">
            <text class="btn-half-text">驳回</text>
          </button>
          <button plain="true" class="btn-half btn-success" hover-class="hover-dim" @click="onPass">
            <text class="btn-half-text">通过</text>
          </button>
        </view>
        <view v-else class="sheet-actions bg-card border-default" >
          <text class="no-auth-text text-secondary" >当前环节「{{ detail.current_step_name || '确认' }}」· 你不在授权名单内，待授权人处理</text>
        </view>
      </template>
    </AppBottomSheet>

    <!-- 底部批量通过栏 -->
    <view v-if="selectedCount > 0" class="footer-bar bg-card border-default" >
      <button plain="true" class="btn-primary" :class="(acting ? 'btn-disabled' : 'btn-success')" hover-class="hover-dim" @click="onBatchPass">
        <text class="btn-primary-text">批量通过（{{ selectedCount }}）</text>
      </button>
    </view>

    <!-- 驳回原因弹窗（AppDialog editable 多行；确认带回输入值，空值/失败不关窗且保留已填内容） -->
    <AppDialog
      :visible="rejecting"
      kind="danger"
      title="驳回原因（必填）"
      :editable="true"
      :multiline="true"
      placeholder="如：照片为旧标签，请重新拍摄"
      :default-value="rejectReason"
      confirm-text="确认驳回"
      cancel-text="取消"
      @update:visible="rejecting = $event"
      @confirm="onRejectConfirm"
    />

    <!-- 确认通过弹窗（自绘，替代原生 showModal）：单条/批量共用，ids 在打开时暂存 -->
    <AppDialog
      :visible="confirmDlgShow"
      kind="primary"
      title="确认通过"
      :content="'确认通过 ' + confirmIds.length + ' 条维保登记？确认后台账即时生效（回写最近维保日期并重算到期日）。'"
      confirm-text="通过"
      cancel-text="取消"
      @update:visible="confirmDlgShow = $event"
      @confirm="onConfirmOk"
    />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import {
  apiMaintenancePending, apiMaintenanceConfirm, apiMaintenanceReject, apiDictOptions,
  MaintenanceItem, DictOption
} from '@/services/api'
import { toAbsUrl } from '@/utils/url'
import AppListShell from '@/components/AppListShell.vue'
import AppListFooter from '@/components/AppListFooter.vue'
import AppBottomSheet from '@/components/AppBottomSheet.vue'
import AppDialog from '@/components/AppDialog.vue'

const PAGE_SIZE = 20

type ConfirmData = {
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: MaintenanceItem[]
  /** 勾选状态（id → true） */
  selected: Record<string, boolean>
  /** 详情弹层当前记录（null = 关闭） */
  detail: MaintenanceItem | null
  maintTypeOptions: DictOption[]
  rejecting: boolean
  rejectReason: string
  rejectTarget: MaintenanceItem | null
  acting: boolean
  /** 确认通过弹窗：待确认 id 集合（单条/批量共用） */
  confirmDlgShow: boolean
  confirmIds: string[]
}

export default {
  components: { AppListShell, AppListFooter, AppBottomSheet, AppDialog },
  data(): ConfirmData {
    return {
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as MaintenanceItem[],
      selected: {},
      detail: null,
      maintTypeOptions: [] as DictOption[],
      rejecting: false,
      rejectReason: '',
      rejectTarget: null,
      acting: false,
      confirmDlgShow: false,
      confirmIds: [] as string[]
    }
  },
  computed: {
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    },
    selectedCount(): number {
      let n = 0
      Object.keys(this.selected).forEach((k) => {
        if (this.selected[k]) n += 1
      })
      return n
    }
  },
  onLoad() {
    apiDictOptions('equipment_maint_type').then((opts) => {
      this.maintTypeOptions = opts
    }).catch(() => {})
    this.reload()
  },
  onShow() {
    if (this.loaded) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    toAbsUrl,
    maintTypeText(t: string): string {
      const o = this.maintTypeOptions.find((x) => x.value == t)
      return o != null ? o.label : t
    },
    toggleSelect(id: string) {
      const m = this.list.find((x) => x.id == id)
      if (m != null && m.can_confirm === false) {
        uni.showToast({ title: '该记录你不在授权名单内', icon: 'none' })
        return
      }
      this.selected[id] = !this.selected[id]
    },
    /** 打开详情弹层（点卡片；勾选框已 stop 冒泡） */
    openDetail(m: MaintenanceItem) {
      this.detail = m
    },
    closeDetail() {
      this.detail = null
    },
    reload() {
      this.page = 1
      this.selected = {}
      this.detail = null
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
      apiMaintenancePending(this.page, PAGE_SIZE)
        .then((res) => {
          this.total = res.total
          this.list = append ? this.list.concat(res.list) : res.list
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
    preview(m: MaintenanceItem, idx: number) {
      uni.previewImage({ urls: m.photos.map((p) => toAbsUrl(p.url)), current: idx })
    },
    /** 确认（单条/批量共用）：弹窗确认后台账即时生效 */
    doConfirm(ids: string[]) {
      if (this.acting || ids.length == 0) return
      this.confirmIds = ids
      this.confirmDlgShow = true
    },
    onConfirmOk() {
      const ids = this.confirmIds
      if (ids.length == 0) return
      this.acting = true
      apiMaintenanceConfirm(ids)
        .then((r) => {
          const parts: string[] = []
          if (r.confirmed > 0) parts.push('已确认 ' + r.confirmed + ' 条')
          if (r.advanced > 0) parts.push('推进 ' + r.advanced + ' 条待下一环节')
          if (r.forbidden && r.forbidden.length > 0) parts.push(r.forbidden.length + ' 条不在你的授权名单')
          uni.showToast({ title: parts.length > 0 ? parts.join('，') : '无可确认记录', icon: 'none' })
          this.reload()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
        })
    },
    /** 详情弹层「通过」 */
    onPass() {
      if (this.detail == null) return
      this.doConfirm([this.detail.id])
    },
    onBatchPass() {
      const ids = this.list.filter((m) => this.selected[m.id]).map((m) => m.id)
      this.doConfirm(ids)
    },
    /** 详情弹层「驳回」 */
    onRejectTap() {
      if (this.detail == null) return
      this.rejectTarget = this.detail
      this.rejectReason = ''
      this.rejecting = true
    },
    onRejectConfirm(reason: string) {
      if (this.rejectTarget == null || this.acting) return
      this.rejectReason = reason
      const v = (reason || '').trim()
      // 空值不提交：提示并开回弹窗（AppDialog confirm 时已先置关）
      if (v == '') {
        uni.showToast({ title: '请填写驳回原因', icon: 'none' })
        this.rejecting = true
        return
      }
      const id = this.rejectTarget.id
      this.acting = true
      apiMaintenanceReject(id, v)
        .then(() => {
          uni.showToast({ title: '已驳回，已通知登记人', icon: 'none' })
          this.rejecting = false
          this.reload()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
          // 失败开回弹窗：default-value 回填 rejectReason，已填理由不丢
          this.rejecting = true
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
  border-left-width: 8rpx;
  border-left-style: solid;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

/* 多选勾选图标（uni-icons）：放大触控区 */
.check-icon {
  padding: 16rpx;
}

.card-title-row {
  flex-direction: row;
  align-items: center;
  flex: 1;
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
  margin-top: 12rpx;
}

.card-ai {
  font-size: 26rpx;
  line-height: 36rpx;
}

.card-noauth {
  font-size: 24rpx;
  margin-top: 8rpx;
}

/* 详情弹层（与打卡审核同款） */
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

.sheet-body {
  padding: 0 8rpx 24rpx;
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

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
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

.footer-bar {
  padding: 16rpx 24rpx 32rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
}

.btn-primary {
  height: 104rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

/* 驳回原因弹窗（AppDialog 承载，样式见 components/AppDialog.vue） */
</style>
