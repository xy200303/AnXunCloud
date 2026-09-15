<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <template v-if="d">
      <!-- 报告头卡 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="title" :style="{ color: colors.textPrimary }">{{ d.title }}</text>
        <text class="sub" :style="{ color: colors.textSecondary }">{{ d.community_name }} · {{ d.period }}</text>
        <view
          class="status-chip"
          :style="{
            backgroundColor: d.status === 'approved' ? '#E8F7EE' : '#FDF3E5',
            color: d.status === 'approved' ? colors.success : colors.warning
          }"
        >
          <text class="status-chip-text">{{ statusLabel(d.status) }}</text>
        </view>
      </view>

      <!-- 审核链 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">审核链</text>
        <view v-for="(step, index) in d.review_steps" :key="step.slot" class="step" :style="{ borderBottomColor: colors.border }">
          <view class="step-head">
            <view class="step-no" :style="{ backgroundColor: stepBg(index) }">
              <text class="step-no-text" :style="{ color: stepColor(index) }">{{ index + 1 }}</text>
            </view>
            <text class="step-name" :style="{ color: colors.textPrimary }">{{ step.name }}</text>
            <text class="step-mode" :style="{ color: colors.textSecondary }">{{ step.mode === 'all' ? '全部签署' : '任一签署' }}</text>
          </view>
          <view v-for="user in step.users" :key="user.user_id" class="person">
            <text class="person-name" :style="{ color: colors.textRegular }">{{ user.name }}</text>
            <text class="person-state" :style="{ color: user.signed ? colors.success : colors.textSecondary }">{{ user.signed ? '已签' : '待签' }}</text>
          </view>
          <text v-if="!step.users?.length" class="muted" :style="{ color: colors.textSecondary }">无候选人，已跳过</text>
        </view>
      </view>

      <!-- 查看完整报告 -->
      <view class="btn-outline" :style="{ borderColor: colors.primary, backgroundColor: colors.bgCard }" @click="openPdf">
        <text class="btn-outline-text" :style="{ color: colors.primary }">查看完整报告 PDF</text>
      </view>

      <!-- 审核操作（当前环节候选人才显示） -->
      <view v-if="canSign" class="actions">
        <view class="btn-primary" :style="{ backgroundColor: colors.primary, opacity: busy ? 0.6 : 1 }" @click="approve">
          <text class="btn-primary-text" :style="{ color: colors.white }">签署通过</text>
        </view>
        <view class="btn-danger" :style="{ borderColor: colors.danger, opacity: busy ? 0.6 : 1 }" @click="reject">
          <text class="btn-danger-text" :style="{ color: colors.danger }">驳回</text>
        </view>
      </view>
    </template>
    <view v-else class="loading" :style="{ color: colors.textSecondary }">加载中…</view>

    <!-- 驳回原因弹窗（自绘，替代原生 showModal editable；确认带回输入值，空值不提交） -->
    <AppDialog
      :visible="rejectDlgShow"
      kind="danger"
      title="驳回报告"
      :editable="true"
      placeholder="请输入驳回原因"
      :default-value="rejectReason"
      confirm-text="驳回"
      cancel-text="取消"
      @update:visible="rejectDlgShow = $event"
      @confirm="onRejectConfirm"
    />
  </view>
</template>

<script lang="ts">
import { apiReportDetail, apiSignStep, openReportPdf, type ReportDetail, type ReportSignReq } from '@/services/api'
import { Colors } from '@/utils/theme'
import AppDialog from '@/components/AppDialog.vue'

export default {
  components: { AppDialog },
  data() { return { colors: Colors, d: null as ReportDetail | null, busy: false, reportId: '', rejectDlgShow: false, rejectReason: '' } },
  computed: {
    currentStep(): any { return this.d?.review_steps?.[this.d.review_step] },
    canSign(): boolean {
      const uid = String(uni.getStorageSync('user_id') || '')
      const step = this.currentStep
      return !!this.d && this.d.status === 'pending_review' && !!step && step.candidate_ids.includes(uid) && !step.signed?.some((x: any) => x.user_id === uid)
    }
  },
  onLoad(query: any) { this.reportId = String(query?.id || ''); this.load() },
  methods: {
    async load() { if (this.reportId) this.d = await apiReportDetail(this.reportId) },
    statusLabel(status: string) { return status === 'approved' ? '已通过' : '待审核' },
    stepColor(index: number): string {
      if (!this.d) return Colors.textSecondary
      if (index < this.d.review_step || this.d.status === 'approved') return Colors.success
      if (index === this.d.review_step) return Colors.primary
      return Colors.textSecondary
    },
    stepBg(index: number): string {
      if (!this.d) return Colors.bgPage
      if (index < this.d.review_step || this.d.status === 'approved') return '#E8F7EE'
      if (index === this.d.review_step) return Colors.primaryLight
      return Colors.bgPage
    },
    async submit(req: ReportSignReq, message: string) { if (!this.d) return; this.busy = true; try { await apiSignStep(this.d.id, this.d.review_step, req); uni.showToast({ title: message, icon: 'none' }); await this.load() } finally { this.busy = false } },
    approve() { this.submit({ action: 'approve' }, '签署已提交') },
    reject() { this.rejectReason = ''; this.rejectDlgShow = true },
    onRejectConfirm(reason: string) {
      this.rejectReason = reason
      const v = (reason || '').trim()
      // 空值不触发提交：提示并保持弹窗打开（update:visible 先置关，此处再开回）
      if (v == '') { uni.showToast({ title: '请填写驳回原因', icon: 'none' }); this.rejectDlgShow = true; return }
      this.submit({ action: 'reject', reason: v }, '已驳回')
    },
    // 完整 PDF：ticket → pdf.js 内嵌渲染（App 端）/ 下载后系统打开（其他端），见 api.openReportPdf
    openPdf() { if (this.reportId != '') openReportPdf(this.reportId) }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 24rpx; }
.card { padding: 28rpx; border-radius: 20rpx; margin-bottom: 24rpx; }
.title { display: block; font-size: 34rpx; font-weight: 600; line-height: 1.4; }
.sub { display: block; margin-top: 10rpx; font-size: 26rpx; }
.status-chip { display: inline-flex; margin-top: 18rpx; padding: 6rpx 20rpx; border-radius: 999rpx; }
.status-chip-text { font-size: 24rpx; }
.sec-title { display: block; font-size: 30rpx; font-weight: 600; margin-bottom: 8rpx; }
.step { padding: 20rpx 0; border-bottom-width: 1px; border-bottom-style: solid; }
.step:last-child { border-bottom-width: 0; }
.step-head { flex-direction: row; align-items: center; }
.step-no { width: 40rpx; height: 40rpx; border-radius: 50%; align-items: center; justify-content: center; margin-right: 16rpx; }
.step-no-text { font-size: 24rpx; font-weight: 600; }
.step-name { flex: 1; font-size: 30rpx; font-weight: 600; }
.step-mode { font-size: 24rpx; }
.person { flex-direction: row; justify-content: space-between; padding: 10rpx 0 10rpx 56rpx; }
.person-name { font-size: 28rpx; }
.person-state { font-size: 26rpx; }
.muted { display: block; font-size: 24rpx; padding: 8rpx 0 0 56rpx; }
.btn-outline { border-width: 2rpx; border-style: solid; border-radius: 999rpx; height: 88rpx; align-items: center; justify-content: center; margin-bottom: 24rpx; }
.btn-outline-text { font-size: 30rpx; font-weight: 600; }
.actions { flex-direction: row; }
.btn-primary { flex: 1; height: 88rpx; border-radius: 999rpx; align-items: center; justify-content: center; margin-right: 16rpx; }
.btn-primary-text { font-size: 30rpx; font-weight: 600; }
.btn-danger { flex: 1; height: 88rpx; border-radius: 999rpx; border-width: 2rpx; border-style: solid; align-items: center; justify-content: center; }
.btn-danger-text { font-size: 30rpx; font-weight: 600; }
.loading { text-align: center; padding-top: 160rpx; font-size: 28rpx; }
</style>
