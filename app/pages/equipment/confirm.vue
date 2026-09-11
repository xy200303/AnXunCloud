<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无待确认的维保登记"
      empty-sub="巡检员现场登记后会出现在这里"
      :colors="colors"
      @retry="reload"
    >
      <!-- pending 列表：AI 存疑（review）后端已置顶，行头红条加强提示 -->
      <template #default>
      <view class="content">
        <view
          v-for="m in list"
          :key="m.id"
          class="card"
          :style="{ backgroundColor: colors.bgCard, borderLeftColor: m.ai_verdict == 'review' ? colors.danger : colors.bgCard }"
        >
          <view class="card-head" @click="toggleExpand(m.id)">
            <view class="card-title-row">
              <!-- 多选框 -->
              <view
                class="check-circle"
                :style="{ borderColor: selected[m.id] ? colors.primary : colors.border, backgroundColor: selected[m.id] ? colors.primary : colors.bgCard }"
                @click.stop="toggleSelect(m.id)"
              >
                <text v-if="selected[m.id]" class="check-mark" :style="{ color: colors.white }">✓</text>
              </view>
              <text class="card-title" :style="{ color: colors.textPrimary }">{{ m.equipment_name }}</text>
            </view>
            <text v-if="m.ai_verdict == 'review'" class="card-status" :style="{ color: colors.danger }">AI 存疑</text>
            <text v-else-if="m.ai_verdict == 'pass'" class="card-status" :style="{ color: colors.success }">AI 通过</text>
            <text v-else class="card-status" :style="{ color: colors.textSecondary }">未预检</text>
          </view>
          <text class="card-sub" :style="{ color: colors.textSecondary }">编号：{{ m.equipment_code }}<text v-if="m.point_name != null && m.point_name != ''"> · {{ m.point_name }}</text></text>
          <text class="card-sub" :style="{ color: colors.textSecondary }">
            {{ maintTypeText(m.maintenance_type) }} · {{ m.maintenance_date }} · {{ m.operator_name }} 经办 · {{ m.created_by_name }} 登记
          </text>

          <!-- 展开：登记信息 + 照片 -->
          <view v-if="expanded[m.id]" class="expand">
            <text v-if="m.vendor != null && m.vendor != ''" class="info-line" :style="{ color: colors.textRegular }">维保单位：{{ m.vendor }}</text>
            <text v-if="m.maintenance_type == 'ledger_fix'" class="info-line" :style="{ color: colors.textRegular }">
              补录日期：出厂 {{ m.fix_manufacture_date != '' ? m.fix_manufacture_date : '--' }} / 维保 {{ m.fix_last_maintenance_date != '' ? m.fix_last_maintenance_date : '--' }}
            </text>
            <text v-if="m.note != ''" class="info-line" :style="{ color: colors.textRegular }">备注：{{ m.note }}</text>
            <text v-if="m.ai_verdict == 'review' && m.ai_reason != null && m.ai_reason != ''" class="info-line" :style="{ color: colors.danger }">AI 说明：{{ m.ai_reason }}</text>
            <text class="info-line" :style="{ color: colors.textSecondary }">登记时间：{{ m.created_at }}</text>
            <view class="photos">
              <image
                v-for="(p, pi) in m.photos"
                :key="p.file_id"
                class="photo"
                :src="toAbsUrl(p.url)"
                mode="aspectFill"
                lazy-load
                @click="preview(m, pi)"
              />
              <text v-if="m.photos.length == 0" class="info-line" :style="{ color: colors.textSecondary }">无照片</text>
            </view>
            <!-- 单条操作 -->
            <view class="row-actions">
              <view class="btn-half" :style="{ borderColor: colors.danger }" @click="onRejectTap(m)">
                <text class="btn-half-text" :style="{ color: colors.danger }">驳回</text>
              </view>
              <view class="btn-half btn-half-solid" :style="{ backgroundColor: colors.success }" @click="onPass(m)">
                <text class="btn-half-text" :style="{ color: colors.white }">通过</text>
              </view>
            </view>
          </view>
        </view>
      </view>
      </template>
      <template #footer>
        <AppListFooter :loading-more="loadingMore" :no-more="noMore" :visible="list.length > 0" :colors="colors" />
      </template>
    </AppListShell>

    <!-- 底部批量通过栏 -->
    <view v-if="selectedCount > 0" class="footer-bar" :style="{ backgroundColor: colors.bgCard, borderTopColor: colors.border }">
      <view class="btn-primary" :style="{ backgroundColor: acting ? colors.info : colors.success }" @click="onBatchPass">
        <text class="btn-primary-text" :style="{ color: colors.white }">批量通过（{{ selectedCount }}）</text>
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
          placeholder="如：照片为旧标签，请重新拍摄"
          :maxlength="200"
        />
        <view class="dialog-actions">
          <text class="dialog-btn" :style="{ color: colors.textSecondary }" @click="rejecting = false">取消</text>
          <text class="dialog-btn" :style="{ color: colors.danger }" @click="onRejectConfirm">确认驳回</text>
        </view>
      </view>
    </view>

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
import { Colors, ColorTokens } from '@/utils/theme'
import {
  apiMaintenancePending, apiMaintenanceConfirm, apiMaintenanceReject, apiDictOptions,
  MaintenanceItem, DictOption
} from '@/services/api'
import { toAbsUrl } from '@/utils/url'
import AppListShell from '@/components/AppListShell.vue'
import AppListFooter from '@/components/AppListFooter.vue'
import AppDialog from '@/components/AppDialog.vue'

const PAGE_SIZE = 20

type ConfirmData = {
  colors: ColorTokens
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: MaintenanceItem[]
  /** 勾选状态（id → true） */
  selected: Record<string, boolean>
  /** 展开状态（id → true） */
  expanded: Record<string, boolean>
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
  components: { AppListShell, AppListFooter, AppDialog },
  data(): ConfirmData {
    return {
      colors: Colors,
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as MaintenanceItem[],
      selected: {},
      expanded: {},
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
      this.selected[id] = !this.selected[id]
    },
    toggleExpand(id: string) {
      this.expanded[id] = !this.expanded[id]
    },
    reload() {
      this.page = 1
      this.selected = {}
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
          uni.showToast({ title: '已确认 ' + r.confirmed + ' 条', icon: 'none' })
          this.reload()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
        })
    },
    onPass(m: MaintenanceItem) {
      this.doConfirm([m.id])
    },
    onBatchPass() {
      const ids = this.list.filter((m) => this.selected[m.id]).map((m) => m.id)
      this.doConfirm(ids)
    },
    onRejectTap(m: MaintenanceItem) {
      this.rejectTarget = m
      this.rejectReason = ''
      this.rejecting = true
    },
    onRejectConfirm() {
      if (this.rejectTarget == null || this.acting) return
      const reason = this.rejectReason.trim()
      if (reason == '') {
        uni.showToast({ title: '请填写驳回原因', icon: 'none' })
        return
      }
      const id = this.rejectTarget.id
      this.acting = true
      apiMaintenanceReject(id, reason)
        .then(() => {
          uni.showToast({ title: '已驳回，已通知登记人', icon: 'none' })
          this.rejecting = false
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
  border-left-width: 8rpx;
  border-left-style: solid;
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

.check-circle {
  width: 40rpx;
  height: 40rpx;
  border-radius: 20rpx;
  border-width: 2rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
  margin-right: 16rpx;
}

.check-mark {
  font-size: 26rpx;
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

.expand {
  margin-top: 16rpx;
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

.row-actions {
  flex-direction: row;
  margin-top: 24rpx;
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

/* 驳回原因对话框 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 99;
}

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
