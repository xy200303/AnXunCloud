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

      <text class="tip" :style="{ color: colors.textSecondary }">签字人按汇报线自动圈定（巡检员 → 安全主管 → 项目经理），无需选择</text>
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
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { apiCommunityTree, apiDictOptions, apiReportGenerate, DictOption } from '@/services/api'

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
        if (this.communities.length == 1) this.communityId = this.communities[0].id
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
    },
    pickMonth(v: string) {
      this.period = v
      this.monthShow = false
    },
    pickType(v: string) {
      this.patrolType = v
      this.typeShow = false
    },
    submit() {
      if (!this.canSubmit) return
      this.submitting = true
      apiReportGenerate({
        community_id: this.communityId,
        period: this.period,
        patrol_type: this.patrolType == '' ? undefined : this.patrolType,
        detail_mode: this.detailMode
      })
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

.sheet-item text {
  font-size: 30rpx;
}
</style>
