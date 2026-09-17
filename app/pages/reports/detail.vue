<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <template v-if="d">
      <!-- 报告头卡 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="title" :style="{ color: colors.textPrimary }">{{ d.title }}</text>
        <text class="sub" :style="{ color: colors.textSecondary }">{{ d.community_name }} · {{ d.period }} · 生成于 {{ d.created_at }}</text>
        <view
          class="status-chip"
          :style="{
            backgroundColor: d.status === 'approved' ? '#E8F7EE' : '#FDF3E5',
            color: d.status === 'approved' ? colors.success : colors.warning
          }"
        >
          <text class="status-chip-text">{{ statusLabel(d.status) }}</text>
        </view>
        <view v-if="d.reject_reason != ''" class="reject-bar" :style="{ backgroundColor: '#FEF0EF' }">
          <text class="reject-text" :style="{ color: colors.danger }">最近驳回原因：{{ d.reject_reason }}</text>
        </view>
      </view>

      <!-- 汇总统计 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">本月汇总</text>
        <view class="stats-grid">
          <view v-for="s in statsItems" :key="s.label" class="stats-cell">
            <text class="stats-value" :style="{ color: s.danger ? colors.danger : colors.textPrimary }">{{ s.value }}</text>
            <text class="stats-label" :style="{ color: colors.textSecondary }">{{ s.label }}</text>
          </view>
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
            <view class="person-main">
              <text class="person-name" :style="{ color: colors.textRegular }">{{ user.name }}</text>
              <text v-if="user.signed && user.signed_at" class="person-time" :style="{ color: colors.textSecondary }">{{ user.signed_at }}</text>
            </view>
            <image
              v-if="user.signed && user.signature_url"
              class="sign-img"
              :src="user.signature_url"
              mode="aspectFit"
              @click="previewImage(user.signature_url)"
            />
            <text v-else class="person-state" :style="{ color: user.signed ? colors.success : colors.textSecondary }">{{ user.signed ? '已签' : '待签' }}</text>
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
    <view v-else-if="errorMsg != ''" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>
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

    <!-- 手写签名板（未配置签名时签字现场手写；勾选「保存」则写入签章资产下次直接用） -->
    <SignaturePad ref="pad" :show-save-option="true" @save="onPadSave" />
  </view>
</template>

<script lang="ts">
import { apiReportDetail, apiSignStep, apiUploadLocal, apiUpdateProfile, openReportPdf, type ReportDetail, type ReportSignReq } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { Colors } from '@/utils/theme'
import AppDialog from '@/components/AppDialog.vue'
import SignaturePad from '@/components/SignaturePad.vue'

export default {
  components: { AppDialog, SignaturePad },
  data() { return { colors: Colors, d: null as ReportDetail | null, busy: false, reportId: '', errorMsg: '', rejectDlgShow: false, rejectReason: '' } },
  computed: {
    currentStep(): any { return this.d?.review_steps?.[this.d.review_step] },
    canSign(): boolean {
      const uid = String(useAuthStore().userInfo?.id || '')
      const step = this.currentStep
      return !!this.d && this.d.status === 'pending_review' && uid !== '' && !!step && step.candidate_ids.includes(uid) && !step.signed?.some((x: any) => x.user_id === uid)
    },
    statsItems(): Array<{ label: string; value: string | number; danger?: boolean }> {
      const s = (this.d?.stats ?? {}) as Record<string, any>
      const num = (k: string) => (typeof s[k] == 'number' ? s[k] : 0)
      return [
        { label: '巡检任务', value: num('task_total') },
        { label: '已完成', value: num('task_done') },
        { label: '已逾期', value: num('task_overdue'), danger: num('task_overdue') > 0 },
        { label: '应巡点位', value: num('should_points') },
        { label: '实巡点位', value: num('done_points') },
        { label: '覆盖率', value: (typeof s.coverage_rate == 'number' ? s.coverage_rate : 0) + '%' },
        { label: '异常打卡', value: num('abnormal_count'), danger: num('abnormal_count') > 0 },
        { label: '存疑打卡', value: num('suspect_count'), danger: num('suspect_count') > 0 }
      ]
    }
  },
  onLoad(query: any) { this.reportId = String(query?.id || ''); this.load() },
  methods: {
    async load() {
      if (!this.reportId) return
      this.errorMsg = ''
      try {
        this.d = await apiReportDetail(this.reportId)
      } catch (e: any) {
        this.errorMsg = e?.message || '报告加载失败，请稍后重试'
      }
    },
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
    previewImage(url: string | null | undefined) {
      if (url == null || url == '') return
      uni.previewImage({ urls: [url] })
    },
    submit(req: ReportSignReq, message: string) { if (!this.d) return; this.busy = true; apiSignStep(this.d.id, this.d.review_step, req).then(async () => { uni.showToast({ title: message, icon: 'none' }); await this.load() }).catch((e: Error) => { uni.showToast({ title: e.message || '操作失败', icon: 'none' }) }).finally(() => { this.busy = false }) },
    approve() {
      // 未配置手写签名 → 现场弹签名板（可勾选保存复用），不再要求先去个人中心设置
      const sig = useAuthStore().userInfo?.signature_url
      if (sig == null || sig == '') {
        const pad: any = this.$refs.pad
        pad.open()
        return
      }
      this.submit({ action: 'approve' }, '签署已提交')
    },
    /** 签名板确认：上传 PNG（scene=signature）→ 勾选保存则写入签章资产 → 携一次性签名完成签字 */
    onPadSave(filePath: string, saveForLater: boolean) {
      const pad: any = this.$refs.pad
      apiUploadLocal(filePath, 'signature')
        .then((up) => {
          const save = saveForLater
            ? (() => {
                const u = useAuthStore().userInfo
                return apiUpdateProfile(u != null ? u.name : '', u != null ? u.phone : '', up.file_id).then(() => useAuthStore().fetchProfile())
              })()
            : Promise.resolve()
          return save.then(() => up.file_id)
        })
        .then((fileId) => {
          pad.finish(true)
          this.submit({ action: 'approve', signature_file_id: fileId }, '签署已提交')
        })
        .catch((e: Error) => {
          pad.finish(false)
          uni.showToast({ title: e.message || '签名上传失败', icon: 'none' })
        })
    },
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
.reject-bar { margin-top: 18rpx; border-radius: 12rpx; padding: 16rpx 20rpx; }
.reject-text { font-size: 26rpx; }
.sec-title { display: block; font-size: 30rpx; font-weight: 600; margin-bottom: 16rpx; }
.stats-grid { flex-direction: row; flex-wrap: wrap; }
.stats-cell { width: 25%; align-items: center; padding: 14rpx 0; }
.stats-value { font-size: 36rpx; font-weight: 600; }
.stats-label { font-size: 22rpx; margin-top: 6rpx; }
.step { padding: 20rpx 0; border-bottom-width: 1px; border-bottom-style: solid; }
.step:last-child { border-bottom-width: 0; }
.step-head { flex-direction: row; align-items: center; }
.step-no { width: 40rpx; height: 40rpx; border-radius: 50%; align-items: center; justify-content: center; margin-right: 16rpx; }
.step-no-text { font-size: 24rpx; font-weight: 600; }
.step-name { flex: 1; font-size: 30rpx; font-weight: 600; }
.step-mode { font-size: 24rpx; }
.person { flex-direction: row; align-items: center; justify-content: space-between; padding: 10rpx 0 10rpx 56rpx; }
.person-main { flex: 1; }
.person-name { display: block; font-size: 28rpx; }
.person-time { display: block; font-size: 22rpx; margin-top: 4rpx; }
.person-state { font-size: 26rpx; }
.sign-img { width: 96rpx; height: 56rpx; }
.muted { display: block; font-size: 24rpx; padding: 8rpx 0 0 56rpx; }
.btn-outline { border-width: 2rpx; border-style: solid; border-radius: 999rpx; height: 88rpx; align-items: center; justify-content: center; margin-bottom: 24rpx; }
.btn-outline-text { font-size: 30rpx; font-weight: 600; }
.actions { flex-direction: row; }
.btn-primary { flex: 1; height: 88rpx; border-radius: 999rpx; align-items: center; justify-content: center; margin-right: 16rpx; }
.btn-primary-text { font-size: 30rpx; font-weight: 600; }
.btn-danger { flex: 1; height: 88rpx; border-radius: 999rpx; border-width: 2rpx; border-style: solid; align-items: center; justify-content: center; }
.btn-danger-text { font-size: 30rpx; font-weight: 600; }
.loading { text-align: center; padding-top: 160rpx; font-size: 28rpx; }
.empty { align-items: center; padding-top: 160rpx; }
.empty-title { font-size: 28rpx; }
.empty-retry { font-size: 28rpx; margin-top: 16rpx; }
</style>
