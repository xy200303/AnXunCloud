<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
      <!-- 小区 -->
      <text class="label" :style="{ color: colors.textRegular }">小区</text>
      <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }" @click="communityShow = true">
        <text :style="{ color: communityId == '' ? colors.textSecondary : colors.textPrimary }">{{ communityText }}</text>
        <text :style="{ color: colors.textSecondary }">›</text>
      </view>

      <!-- 月份 -->
      <text class="label" :style="{ color: colors.textRegular }">报告月份</text>
      <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }" @click="monthShow = true">
        <text :style="{ color: period == '' ? colors.textSecondary : colors.textPrimary }">{{ period == '' ? '请选择月份' : periodText }}</text>
        <text :style="{ color: colors.textSecondary }">›</text>
      </view>

      <!-- 报告类型 -->
      <text class="label" :style="{ color: colors.textRegular }">报告类型</text>
      <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }" @click="typeShow = true">
        <text :style="{ color: colors.textPrimary }">{{ patrolTypeText }}</text>
        <text :style="{ color: colors.textSecondary }">›</text>
      </view>

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

      <text class="tip" :style="{ color: colors.textSecondary }">巡检员由当期任务自动圈定；主管和经理默认按小区汇报线岗位圈定，也可点击下面名单调整。</text>

      <text class="label" :style="{ color: colors.textRegular }">审核路径</text>
      <view class="signer-row" hover-class="hover-dim" @click="openCandidates('supervisor')">
        <text class="signer-label" :style="{ color: colors.textRegular }">安全主管</text>
        <text class="signer-value" :style="{ color: supervisorIds.length ? colors.textPrimary : colors.textSecondary }">{{ signerDisplay(supervisorIds) }} ›</text>
      </view>
      <view class="signer-row" hover-class="hover-dim" @click="openCandidates('manager')">
        <text class="signer-label" :style="{ color: colors.textRegular }">物业经理</text>
        <text class="signer-value" :style="{ color: managerIds.length ? colors.textPrimary : colors.textSecondary }">{{ signerDisplay(managerIds) }} ›</text>
      </view>
      <text class="tip" :style="{ color: colors.textSecondary }">同一级选择多人时，任意一人签字即可；清空表示跳过该级。候选人必须是当前公司启用用户。</text>
    </view>

    <view class="btn-big" hover-class="hover-dim" :style="{ backgroundColor: canSubmit ? colors.primary : colors.border }" @click="submit">
      <text class="btn-big-text" :style="{ color: canSubmit ? colors.white : colors.textSecondary }">{{ submitting ? '生成中…' : '生成报告' }}</text>
    </view>

    <!-- 小区选择 -->
    <view v-if="communityShow" class="mask" :style="{ backgroundColor: colors.mask }" @click="communityShow = false">
      <view class="sheet" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <view v-for="c in communities" :key="c.id" class="sheet-item" hover-class="hover-dim" @click="pickCommunity(c.id)">
          <text :style="{ color: c.id == communityId ? colors.primary : colors.textPrimary }">{{ c.name }}</text>
        </view>
        <view class="sheet-item" hover-class="hover-dim" @click="communityShow = false">
          <text :style="{ color: colors.textSecondary }">取消</text>
        </view>
      </view>
    </view>

    <!-- 月份选择 -->
    <view v-if="monthShow" class="mask" :style="{ backgroundColor: colors.mask }" @click="monthShow = false">
      <view class="sheet" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <view v-for="m in monthOptions" :key="m.value" class="sheet-item" hover-class="hover-dim" @click="pickMonth(m.value)">
          <text :style="{ color: m.value == period ? colors.primary : colors.textPrimary }">{{ m.text }}</text>
        </view>
        <view class="sheet-item" hover-class="hover-dim" @click="monthShow = false">
          <text :style="{ color: colors.textSecondary }">取消</text>
        </view>
      </view>
    </view>

    <!-- 类型选择 -->
    <view v-if="typeShow" class="mask" :style="{ backgroundColor: colors.mask }" @click="typeShow = false">
      <view class="sheet" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <view class="sheet-item" hover-class="hover-dim" @click="pickType('')">
          <text :style="{ color: patrolType == '' ? colors.primary : colors.textPrimary }">综合（全部巡查类型）</text>
        </view>
        <view v-for="t in typeOptions" :key="t.value" class="sheet-item" hover-class="hover-dim" @click="pickType(t.value)">
          <text :style="{ color: t.value == patrolType ? colors.primary : colors.textPrimary }">{{ t.label }}</text>
        </view>
        <view class="sheet-item" hover-class="hover-dim" @click="typeShow = false">
          <text :style="{ color: colors.textSecondary }">取消</text>
        </view>
      </view>
    </view>

    <!-- 审核人选择 -->
    <view v-if="candidateShow" class="mask" :style="{ backgroundColor: colors.mask }" @click="candidateShow = false">
      <view class="sheet" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <view class="candidate-header">
          <text class="candidate-title" :style="{ color: colors.textPrimary }">{{ candidateRole == 'supervisor' ? '选择安全主管' : '选择物业经理' }}</text>
          <text class="candidate-clear" :style="{ color: colors.danger }" @click="clearCandidates">清空</text>
        </view>
        <view v-if="candidateLoading" class="candidate-empty"><text :style="{ color: colors.textSecondary }">加载中…</text></view>
        <view v-else-if="candidateUsers.length == 0" class="candidate-empty"><text :style="{ color: colors.textSecondary }">暂无当前公司启用用户</text></view>
        <view v-else v-for="u in candidateUsers" :key="u.id" class="candidate-item" hover-class="hover-dim" @click="toggleCandidate(u.id)">
          <text :style="{ color: selectedCandidateIds.indexOf(u.id) >= 0 ? colors.primary : colors.textPrimary }">{{ selectedCandidateIds.indexOf(u.id) >= 0 ? '✓ ' : '○ ' }}{{ u.name }}</text>
          <text v-if="!u.has_signature" class="candidate-warn" :style="{ color: colors.warning }">未配置签名</text>
        </view>
        <view class="sheet-item" hover-class="hover-dim" @click="candidateShow = false"><text :style="{ color: colors.primary }">完成</text></view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { apiCommunityTree, apiDictOptions, apiReportGenerate, apiReportSignCandidates, DictOption, ReportSignCandidate } from '@/services/api'

export default {
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
      candidateUsers: [] as ReportSignCandidate[],
      supervisorIds: [] as string[],
      managerIds: [] as string[],
      candidateRole: 'supervisor' as 'supervisor' | 'manager',
      candidateShow: false,
      candidateLoading: false,
      candidatesLoaded: false,
      detailModes: [
        { value: 'full', label: '全部点位' },
        { value: 'abnormal', label: '仅异常点位' }
      ],
      communityShow: false,
      monthShow: false,
      typeShow: false,
      submitting: false
    }
  },
  computed: {
    selectedCandidateIds(): string[] {
      return this.candidateRole == 'supervisor' ? this.supervisorIds : this.managerIds
    },
    communityText(): string {
      if (this.communityId == '') return '请选择小区'
      const c = this.communities.find((x) => x.id == this.communityId)
      return c != null ? c.name : '请选择小区'
    },
    monthOptions(): Array<{ value: string; text: string }> {
      // 最近 13 个月（当月在前）
      const out = []
      const now = new Date()
      for (let i = 0; i < 13; i++) {
        const d = new Date(now.getFullYear(), now.getMonth() - i, 1)
        const v = d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0')
        out.push({ value: v, text: d.getFullYear() + ' 年 ' + (d.getMonth() + 1) + ' 月' })
      }
      return out
    },
    periodText(): string {
      const m = this.monthOptions.find((x) => x.value == this.period)
      return m != null ? m.text : this.period
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
    noop() {},
    pickCommunity(id: string) {
      this.communityId = id
      this.communityShow = false
      this.loadCandidates()
    },
    pickMonth(v: string) {
      this.period = v
      this.monthShow = false
    },
    pickType(v: string) {
      this.patrolType = v
      this.typeShow = false
      this.loadCandidates()
    },
    async loadCandidates() {
      this.candidatesLoaded = false
      this.candidateUsers = []
      this.supervisorIds = []
      this.managerIds = []
      if (!this.communityId) return
      this.candidateLoading = true
      try {
        const d = await apiReportSignCandidates(this.communityId, this.patrolType || undefined)
        this.candidateUsers = d.users
        this.supervisorIds = d.default_supervisor_ids
        this.managerIds = d.default_manager_ids
        this.candidatesLoaded = true
      } catch {
        this.candidateUsers = []
        this.supervisorIds = []
        this.managerIds = []
      } finally {
        this.candidateLoading = false
      }
    },
    openCandidates(role: 'supervisor' | 'manager') {
      if (!this.communityId) {
        uni.showToast({ title: '请先选择小区', icon: 'none' })
        return
      }
      this.candidateRole = role
      this.candidateShow = true
      if (this.candidateUsers.length == 0) this.loadCandidates()
    },
    toggleCandidate(id: string) {
      const target = this.candidateRole == 'supervisor' ? this.supervisorIds : this.managerIds
      const index = target.indexOf(id)
      if (index >= 0) target.splice(index, 1)
      else target.push(id)
    },
    clearCandidates() {
      if (this.candidateRole == 'supervisor') this.supervisorIds = []
      else this.managerIds = []
    },
    signerNames(ids: string[]): string {
      return ids.map((id) => {
        const user = this.candidateUsers.find((item) => item.id == id)
        return user != null ? user.name : ''
      }).filter((name) => name != '').join('、')
    },
    signerDisplay(ids: string[]): string {
      if (!this.candidatesLoaded && this.candidateLoading) return '加载中…'
      if (!this.candidatesLoaded) return '暂未加载'
      return this.signerNames(ids) || '该级跳过'
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
        payload.supervisor_ids = this.supervisorIds
        payload.manager_ids = this.managerIds
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

.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 99;
  justify-content: flex-end;
}

.sheet {
  border-radius: 24rpx 24rpx 0 0;
  padding: 16rpx 0 48rpx;
  max-height: 70vh;
  overflow-y: auto;
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
