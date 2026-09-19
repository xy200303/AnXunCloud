<template>
  <view class="page bg-page" >
    <view class="card bg-card" :style="{ boxShadow: shadow }">
      <!-- 小区 -->
      <text class="label text-regular" >小区</text>
      <uni-data-select v-model="communityId" :localdata="communityItems" placeholder="请选择小区" :clear="false" @change="onCommunityChange" />

      <!-- 月份 -->
      <text class="label text-regular" >报告月份</text>
      <picker mode="date" fields="month" :value="period" :start="monthStart" :end="monthEnd" @change="onMonthPick">
        <view class="picker-box border-default" hover-class="hover-dim" >
          <text  :class="(period == '' ? 'text-secondary' : 'text-main')">{{ period == '' ? '请选择月份' : periodText }}</text>
          <text  class="text-secondary">›</text>
        </view>
      </picker>

      <!-- 报告类型 -->
      <text class="label text-regular" >报告类型</text>
      <uni-data-select v-model="patrolType" :localdata="typeItems" placeholder="综合（全部巡查类型）" :clear="false" @change="onTypeChange" />

      <!-- 明细范围 -->
      <text class="label text-regular" >明细范围</text>
      <!-- 明细范围 chip：官方 uni-tag（选中实心品牌色 / 未选灰底描边），点击语义不变 -->
      <view class="mode-row">
        <uni-tag
          v-for="m in detailModes"
          :key="m.value"
          :text="m.label"
          :inverted="true"
          :custom-style="detailMode == m.value ? 'background-color:#2B5AED;border-color:#2B5AED;color:#FFFFFF;margin-right:20rpx' : 'background-color:#F5F6F8;border-color:#E5E6EB;color:#4E5969;margin-right:20rpx'"
          @click="detailMode = m.value"
        />
      </view>
      <text class="tip text-secondary" >点位量大时选「仅异常点位」，报告页数更少（汇总统计不受影响）</text>

      <text class="tip text-secondary" >审核链由后台配置，生成时可调整各步骤候选人。</text>

      <text class="label text-regular" >审核路径</text>
      <view v-for="step in reviewSteps" :key="step.slot" class="signer-row" hover-class="hover-dim" @click="openCandidates(step.slot)">
        <text class="signer-label text-regular" >{{ step.name }}</text>
        <text class="signer-value" :style="{ color: selectedIds(step.slot).length ? '#1F2329' : '#86909C' }">{{ signerDisplay(selectedIds(step.slot), step.slot) }} ›</text>
      </view>
      <view v-if="candidateLoading && reviewSteps.length == 0" class="signer-row">
        <text class="signer-label text-secondary" >审核链加载中…</text>
      </view>
      <view v-else-if="candidateError != ''" class="signer-row" hover-class="hover-dim" @click="loadCandidates">
        <text class="signer-label text-danger" >{{ candidateError }}</text>
      </view>
      <view v-else-if="candidatesLoaded && reviewSteps.length == 0" class="signer-row">
        <text class="signer-label text-secondary" >该小区未配置审核链，生成后直接归档</text>
      </view>
      <text class="tip text-secondary" >空候选步骤会自动跳过；任一/全部签署规则由审核链配置决定。</text>
    </view>

    <button plain="true" class="btn-big" hover-class="hover-dim" :class="(canSubmit ? 'btn-primary' : 'btn-disabled')" @click="submit">
      <text class="btn-big-text">{{ submitting ? '生成中…' : '生成报告' }}</text>
    </button>

    <AppSelectionSheet
      :visible="candidateShow"
      :title="'选择' + (activeStep?.name || '审核人')"
      :items="candidateItems"
      :selected-ids="selectedCandidateIds"
      :loading="candidateLoading"
      :error="candidateError"
      empty-text="暂无该审核级别候选人"
      :mask-color="'rgba(0, 0, 0, 0.45)'"
      :background-color="'#FFFFFF'"
     
      @close="candidateShow = false"
      @clear="clearCandidates"
      @retry="loadCandidates"
      @toggle="toggleCandidate"
    />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'
import { ShadowCard } from '@/utils/theme'
import { apiCommunityTree, apiDictOptions, apiReportGenerate, apiReportSignCandidates, DictOption, ReportSignCandidate } from '@/services/api'
import AppSelectionSheet from '@/components/AppSelectionSheet.vue'

export default {
  components: { AppSelectionSheet },
  data() {
    return {
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
    candidateItems(): Array<{ id: string; name: string; warning?: string }> {
      return this.candidateUsers.map((user) => ({
        id: user.id,
        name: user.name,
        warning: user.has_signature ? '' : '未配置签名'
      }))
    },
    activeStep(): any { return this.reviewSteps.find((step) => step.slot === this.candidateRole) },
    /** 小区下拉（uni-data-select localdata 形态；value=小区 id） */
    communityItems(): Array<{ text: string; value: string }> {
      return this.communities.map((community) => ({ text: community.name, value: community.id }))
    },
    /** 报告类型下拉（首项=综合全部，value=''） */
    typeItems(): Array<{ text: string; value: string }> {
      return [{ text: '综合（全部巡查类型）', value: '' }].concat(this.typeOptions.map((option) => ({ text: option.label, value: option.value })))
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
      .catch((e: Error) => {
        // 小区树加载失败会导致选择器为空、提交按钮永远置灰且无原因——必须显性报错
        uni.showToast({ title: e.message || '小区列表加载失败，请退出重进', icon: 'none' })
      })
    apiDictOptions('patrol_type')
      .then((opts) => {
        this.typeOptions = opts
      })
      .catch((e: Error) => {
        uni.showToast({ title: e.message || '巡查类型加载失败', icon: 'none' })
      })
  },
  methods: {
    onCommunityChange() {
      this.loadCandidates()
    },
    onMonthPick(event: any) {
      this.period = String(event.detail.value)
      this.loadCandidates() // 巡检员确认环节候选人按月份解析，换月份必须重载
    },
    onTypeChange() {
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
        const d = await apiReportSignCandidates(this.communityId, this.patrolType || undefined, this.period || undefined)
        if (requestId != this.candidateRequestId) return
        this.reviewSteps = d.steps
        this.selected = Object.fromEntries(d.steps.map((step) => [step.slot, [...step.default_candidate_ids]]))
        this.candidatesLoaded = true
      } catch {
        if (requestId != this.candidateRequestId) return
        this.reviewSteps = []
        this.selected = {}
        this.candidateError = '审核链加载失败，点我重试'
      } finally {
        if (requestId == this.candidateRequestId) this.candidateLoading = false
      }
    },
    openCandidates(role: string) {
      if (!this.communityId) {
        uni.showToast({ title: '请先选择小区', icon: 'none' })
        return
      }
      if (this.candidatesLoaded) {
        const step = this.reviewSteps.find((s) => s.slot == role)
        if (step != null && step.users.length == 0) {
          uni.showToast({ title: role == 'report_inspector' ? '该月没有任务巡检员，生成时此环节自动跳过' : '该环节暂无候选人', icon: 'none' })
          return
        }
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
      const names = this.signerNames(ids, role)
      if (names != '') return names
      // 巡检员确认环节：候选人 = 当月有任务的巡检员（后端按 period 解析）；当月无任务则生成时该级自动跳过
      if (role == 'report_inspector') return '当月无任务巡检员，该级自动跳过'
      return '该级跳过'
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

</style>
