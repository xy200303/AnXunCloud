<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <!-- 小区 -->
      <text class="label" :style="{ color: colors.textRegular }">小区</text>
      <picker mode="selector" :range="communityNames" :value="communityIndex" @change="onCommunityPick">
        <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
          <text :style="{ color: communityId == '' ? colors.textSecondary : colors.textPrimary }">{{ communityText }}</text>
          <text :style="{ color: colors.textSecondary }">›</text>
        </view>
      </picker>

      <!-- 月份 -->
      <text class="label" :style="{ color: colors.textRegular }">报告月份</text>
      <picker mode="date" fields="month" :value="period" :start="monthStart" :end="monthEnd" @change="onMonthPick">
        <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
          <text :style="{ color: period == '' ? colors.textSecondary : colors.textPrimary }">{{ period == '' ? '请选择月份' : periodText }}</text>
          <text :style="{ color: colors.textSecondary }">›</text>
        </view>
      </picker>

      <!-- 报告类型 -->
      <text class="label" :style="{ color: colors.textRegular }">报告类型</text>
      <picker mode="selector" :range="typeNames" :value="typeIndex" @change="onTypePick">
        <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
          <text :style="{ color: colors.textPrimary }">{{ patrolTypeText }}</text>
          <text :style="{ color: colors.textSecondary }">›</text>
        </view>
      </picker>

      <!-- 明细范围 -->
      <text class="label" :style="{ color: colors.textRegular }">明细范围</text>
      <view class="mode-row">
        <view
          v-for="m in detailModes"
          :key="m.value"
          class="mode-chip"
          hover-class="hover-dim"
          :style="{
            backgroundColor: detailMode == m.value ? colors.primary : colors.bgPage,
            borderColor: detailMode == m.value ? colors.primary : colors.border
          }"
          @click="detailMode = m.value"
        >
          <text class="mode-chip-text" :style="{ color: detailMode == m.value ? colors.white : colors.textRegular }">{{ m.label }}</text>
        </view>
      </view>
      <text class="tip" :style="{ color: colors.textSecondary }">点位量大时选「仅异常点位」，报告页数更少（汇总统计不受影响）</text>

      <text class="tip" :style="{ color: colors.textSecondary }">审核链由后台配置，生成时可调整各步骤候选人。</text>

      <text class="label" :style="{ color: colors.textRegular }">审核路径</text>
      <view v-for="step in reviewSteps" :key="step.slot" class="signer-row" hover-class="hover-dim" @click="openCandidates(step.slot)">
        <text class="signer-label" :style="{ color: colors.textRegular }">{{ step.name }}</text>
        <text class="signer-value" :style="{ color: selectedIds(step.slot).length ? colors.textPrimary : colors.textSecondary }">{{ signerDisplay(selectedIds(step.slot), step.slot) }} ›</text>
      </view>
      <text class="tip" :style="{ color: colors.textSecondary }">空候选步骤会自动跳过；任一/全部签署规则由审核链配置决定。</text>
    </view>

    <view class="btn-big" hover-class="hover-dim" :style="{ backgroundColor: canSubmit ? colors.primary : colors.border }" @click="submit">
      <text class="btn-big-text" :style="{ color: canSubmit ? colors.white : colors.textSecondary }">{{ submitting ? '生成中…' : '生成报告' }}</text>
    </view>

    <!-- 审核人选择 -->
    <AppBottomSheet :visible="candidateShow" :mask-color="colors.mask" :background-color="colors.bgCard" @close="candidateShow = false">
        <view class="candidate-header">
        <text class="candidate-title" :style="{ color: colors.textPrimary }">选择{{ activeStep?.name || '审核人' }}</text>
          <text class="candidate-clear" :style="{ color: colors.danger }" @click="clearCandidates">清空</text>
        </view>
        <view v-if="candidateLoading" class="candidate-empty"><text :style="{ color: colors.textSecondary }">加载中…</text></view>
        <view v-else-if="candidateError != ''" class="candidate-empty" hover-class="hover-dim" @click="loadCandidates">
          <text :style="{ color: colors.danger }">{{ candidateError }}</text>
        </view>
        <view v-else-if="candidateUsers.length == 0" class="candidate-empty"><text :style="{ color: colors.textSecondary }">暂无该审核级别候选人</text></view>
        <scroll-view v-else scroll-y class="candidate-scroll" :show-scrollbar="false">
        <view v-for="u in candidateUsers" :key="u.id" class="candidate-item" hover-class="hover-dim" @click="toggleCandidate(u.id)">
            <text :style="{ color: selectedCandidateIds.indexOf(u.id) >= 0 ? colors.primary : colors.textPrimary }">{{ selectedCandidateIds.indexOf(u.id) >= 0 ? '✓ ' : '○ ' }}{{ u.name }}</text>
            <text v-if="!u.has_signature" class="candidate-warn" :style="{ color: colors.warning }">未配置签名</text>
          </view>
        </scroll-view>
        <view class="sheet-item" hover-class="hover-dim" @click="candidateShow = false"><text :style="{ color: colors.primary }">完成</text></view>
    </AppBottomSheet>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { apiCommunityTree, apiDictOptions, apiReportGenerate, apiReportSignCandidates, DictOption, ReportSignCandidate } from '@/services/api'
import AppBottomSheet from '@/components/AppBottomSheet.vue'

export default {
  components: { AppBottomSheet },
  data() {
    return {
      colors: Colors,
      shadow: ShadowCard,
      communities: [] as Array<{ id: string; name: string }>,
      communityId: '',
      period: '',
      patrolType: '',
      typeOptions: [] as DictOption[],
      detailMode: 'full',
      reviewSteps: [] as Array<{ slot: string; name: string; mode: 'any' | 'all'; users: ReportSignCandidate[]; default_candidate_ids: string[] }>,
      selected: {} as Record<string, string[]>,
      candidateRole: '',
      candidateShow: false,
      candidateLoading: false,
      candidateError: '',
      candidatesLoaded: false,
      candidateRequestId: 0,
      detailModes: [
        { value: 'full', label: '全部点位' },
        { value: 'abnormal', label: '仅异常点位' }
      ],
      submitting: false
    }
  },
  computed: {
    selectedCandidateIds(): string[] { return this.selected[this.candidateRole] || [] },
    candidateUsers(): ReportSignCandidate[] {
      return this.reviewSteps.find((step) => step.slot === this.candidateRole)?.users || []
    },
    activeStep(): any { return this.reviewSteps.find((step) => step.slot === this.candidateRole) },
    communityText(): string {
      if (this.communityId == '') return '请选择小区'
      const c = this.communities.find((x) => x.id == this.communityId)
      return c != null ? c.name : '请选择小区'
    },
    communityNames(): string[] {
      return this.communities.map((community) => community.name)
    },
    communityIndex(): number {
      const index = this.communities.findIndex((community) => community.id == this.communityId)
      return index >= 0 ? index : 0
    },
    typeNames(): string[] {
      return ['综合（全部巡查类型）'].concat(this.typeOptions.map((option) => option.label))
    },
    typeIndex(): number {
      if (this.patrolType == '') return 0
      const index = this.typeOptions.findIndex((option) => option.value == this.patrolType)
      return index >= 0 ? index + 1 : 0
    },
    monthStart(): string {
      const now = new Date()
      const d = new Date(now.getFullYear(), now.getMonth() - 12, 1)
      return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-01'
    },
    monthEnd(): string {
      const now = new Date()
      return now.getFullYear() + '-' + String(now.getMonth() + 1).padStart(2, '0') + '-01'
    },
    periodText(): string {
      const parts = this.period.split('-')
      return parts.length == 2 ? parts[0] + ' 年 ' + Number(parts[1]) + ' 月' : this.period
    },
    patrolTypeText(): string {
      if (this.patrolType == '') return '综合（全部巡查类型）'
      const t = this.typeOptions.find((x) => x.value == this.patrolType)
      return t != null ? t.label : this.patrolType
    },
    canSubmit(): boolean {
      return this.communityId != '' && this.period != '' && !this.submitting
    }
  },
  onLoad() {
    // 默认上个月（物业月报口径）
    const now = new Date()
    const d = new Date(now.getFullYear(), now.getMonth() - 1, 1)
    this.period = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0')
    apiCommunityTree()
      .then((tree) => {
        this.communities = tree.map((n) => ({ id: n.id, name: n.name }))
        if (this.communities.length == 1) {
          this.communityId = this.communities[0].id
          this.loadCandidates()
        }
      })
      .catch(() => {})
    apiDictOptions('patrol_type')
      .then((opts) => {
        this.typeOptions = opts
      })
      .catch(() => {})
  },
  methods: {
    onCommunityPick(event: any) {
      const index = Number(event.detail.value)
      const community = this.communities[index]
      if (community == null || community.id == this.communityId) return
      this.communityId = community.id
      this.loadCandidates()
    },
    onMonthPick(event: any) {
      this.period = String(event.detail.value)
    },
    onTypePick(event: any) {
      const index = Number(event.detail.value)
      const option = this.typeOptions[index - 1]
      const nextType = index <= 0 || option == null ? '' : option.value
      if (nextType == this.patrolType) return
      this.patrolType = nextType
      this.loadCandidates()
    },
    async loadCandidates() {
      const requestId = this.candidateRequestId + 1
      this.candidateRequestId = requestId
      this.candidatesLoaded = false
      this.candidateError = ''
      this.reviewSteps = []
      this.selected = {}
      if (!this.communityId) return
      this.candidateLoading = true
      try {
        const d = await apiReportSignCandidates(this.communityId, this.patrolType || undefined)
        if (requestId != this.candidateRequestId) return
        this.reviewSteps = d.steps
        this.selected = Object.fromEntries(d.steps.map((step) => [step.slot, [...step.default_candidate_ids]]))
        this.candidatesLoaded = true
      } catch {
        if (requestId != this.candidateRequestId) return
        this.reviewSteps = []
        this.selected = {}
        this.candidateError = '加载失败，点击重试'
      } finally {
        if (requestId == this.candidateRequestId) this.candidateLoading = false
      }
    },
    openCandidates(role: string) {
      if (!this.communityId) {
        uni.showToast({ title: '请先选择小区', icon: 'none' })
        return
      }
      this.candidateRole = role
      this.candidateShow = true
      if (!this.candidatesLoaded) this.loadCandidates()
    },
    toggleCandidate(id: string) {
      const target = this.selected[this.candidateRole] || (this.selected[this.candidateRole] = [])
      const index = target.indexOf(id)
      if (index >= 0) target.splice(index, 1)
      else target.push(id)
    },
    clearCandidates() {
      this.selected[this.candidateRole] = []
    },
    signerNames(ids: string[], role: string): string {
      const users = this.reviewSteps.find((step) => step.slot === role)?.users || []
      return ids.map((id) => {
        const user = users.find((item) => item.id == id)
        return user != null ? user.name : ''
      }).filter((name) => name != '').join('、')
    },
    signerDisplay(ids: string[], role: string): string {
      if (!this.candidatesLoaded && this.candidateLoading) return '加载中…'
      if (!this.candidatesLoaded) return '暂未加载'
      return this.signerNames(ids, role) || '该级跳过'
    },
    submit() {
      if (!this.canSubmit) return
      this.submitting = true
      const payload: Parameters<typeof apiReportGenerate>[0] = {
        community_id: this.communityId,
        period: this.period,
        patrol_type: this.patrolType == '' ? undefined : this.patrolType,
        detail_mode: this.detailMode
      }
      if (this.candidatesLoaded) {
        payload.sign_steps = this.reviewSteps.filter((step) => step.users.length > 0).map((step) => ({ slot: step.slot, candidate_ids: this.selected[step.slot] || [] }))
      }
      apiReportGenerate(payload)
        .then((res) => {
          this.submitting = false
          uni.showToast({ title: res.regenerated ? '已重新生成' : '已生成', icon: 'success' })
          setTimeout(() => {
            uni.navigateBack({ fail: () => uni.switchTab({ url: '/pages/reports/pending' }) })
          }, 600)
        })
        .catch((e: Error) => {
          this.submitting = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.card {
  border-radius: 24rpx;
  padding: 28rpx;
  margin-bottom: 32rpx;
}

.label {
  font-size: 26rpx;
  margin-top: 20rpx;
  margin-bottom: 12rpx;
}

.label:first-child {
  margin-top: 0;
}

.picker-box {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 22rpx 24rpx;
  font-size: 30rpx;
}

.mode-row {
  flex-direction: row;
}

.mode-chip {
  border-width: 1rpx;
  border-style: solid;
  border-radius: 999rpx;
  padding: 14rpx 32rpx;
  margin-right: 20rpx;
}

.mode-chip-text {
  font-size: 28rpx;
}

.tip {
  display: block;
  font-size: 24rpx;
  margin-top: 20rpx;
  line-height: 1.6;
}

.signer-row {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  border-width: 1rpx;
  border-style: solid;
  border-color: #e5e7eb;
  border-radius: 16rpx;
  padding: 22rpx 24rpx;
  margin-top: 12rpx;
}

.signer-label,
.signer-value {
  font-size: 28rpx;
}

.signer-value {
  flex: 1;
  text-align: right;
  margin-left: 20rpx;
}

.btn-big {
  height: 104rpx;
  border-radius: 52rpx;
  align-items: center;
  justify-content: center;
}

.btn-big-text {
  font-size: 34rpx;
  font-weight: 600;
}

.candidate-scroll {
  height: 52vh;
  max-height: 720rpx;
}

.sheet-item {
  padding: 28rpx 32rpx;
  align-items: center;
}

.candidate-header {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 24rpx 32rpx;
}

.candidate-title,
.candidate-clear {
  font-size: 30rpx;
  font-weight: 600;
}

.candidate-item {
  flex-direction: row;
  justify-content: space-between;
  padding: 28rpx 32rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
  border-top-color: #f0f0f0;
}

.candidate-item text {
  font-size: 30rpx;
}

.candidate-warn {
  font-size: 24rpx !important;
}

.candidate-empty {
  align-items: center;
  padding: 42rpx 32rpx;
}

.sheet-item text {
  font-size: 30rpx;
}
</style>
