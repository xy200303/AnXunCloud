<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <view v-if="d" class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="title" :style="{ color: colors.textPrimary }">{{ d.title }}</text>
      <text class="sub" :style="{ color: colors.textSecondary }">{{ d.community_name }} · {{ d.period }}</text>
      <view class="status" :style="{ color: d.status === 'approved' ? colors.success : colors.warning }">{{ statusLabel(d.status) }}</view>
      <view class="section"><text class="section-title">审核链</text></view>
      <view v-for="(step, index) in d.review_steps" :key="step.slot" class="step">
        <view class="step-head"><text :style="{ color: colors.textPrimary }">{{ index + 1 }}. {{ step.name }}</text><text :style="{ color: colors.textSecondary }">{{ step.mode === 'all' ? '全部签署' : '任一签署' }}</text></view>
        <view v-for="user in step.users" :key="user.user_id" class="person"><text :style="{ color: colors.textPrimary }">{{ user.name }}</text><text :style="{ color: user.signed ? colors.success : colors.textSecondary }">{{ user.signed ? '已签' : '待签' }}</text></view>
        <text v-if="!step.users?.length" class="muted">无候选人，已跳过</text>
      </view>
      <view v-if="canSign" class="actions"><button type="primary" :loading="busy" @click="approve">提交审核</button><button class="reject" :loading="busy" @click="reject">驳回</button></view>
      <button class="pdf" @click="openPdf">查看 PDF</button>
    </view>
    <view v-else class="loading">加载中…</view>
  </view>
</template>

<script lang="ts">
import { apiReportDetail, apiSignStep, apiReportPdfTicket, type ReportDetail, type ReportSignReq } from '@/services/api'
import { Colors } from '@/utils/theme'

export default {
  data() { return { colors: Colors, d: null as ReportDetail | null, busy: false, reportId: '' } },
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
    async submit(req: ReportSignReq, message: string) { if (!this.d) return; this.busy = true; try { await apiSignStep(this.d.id, this.d.review_step, req); uni.showToast({ title: message, icon: 'none' }); await this.load() } finally { this.busy = false } },
    approve() { this.submit({ action: 'approve' }, '审核已提交') },
    reject() { uni.showModal({ title: '驳回报告', editable: true, placeholderText: '请输入驳回原因' }).then((res: any) => { if (res.confirm && res.content) this.submit({ action: 'reject', reason: res.content }, '已驳回') }) },
    async openPdf() { const ticket = await apiReportPdfTicket(this.reportId); uni.navigateTo({ url: `/pages/webview/index?url=${encodeURIComponent(ticket)}` }) }
  }
}
</script>

<style scoped>
.page{min-height:100vh;padding:24rpx}.card{padding:28rpx;border-radius:20rpx}.title{display:block;font-size:36rpx;font-weight:600}.sub,.muted{display:block;margin-top:10rpx;font-size:26rpx}.status{margin-top:20rpx;font-size:28rpx}.section{margin-top:34rpx}.section-title{font-size:30rpx;font-weight:600}.step{padding:22rpx 0;border-bottom:1px solid #eee}.step-head,.person{display:flex;justify-content:space-between;padding:10rpx 0}.actions{display:flex;gap:20rpx;margin-top:30rpx}.actions button{flex:1}.reject{background:#fff;color:#d33;border:1px solid #d33}.pdf{margin-top:22rpx}.loading{text-align:center;padding-top:160rpx}
</style>
