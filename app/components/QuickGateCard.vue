<template>
  <!-- 提交卡 gate（方案 §六）：全流程唯一会阻塞的地方。
       逐项实时进度（每项一行"上传中…/识别中…"，落定勾掉一行）；未落定项给「跳过识别 ›」（转人工复核）；
       主按钮在底部操作栏：未落定=置灰「等待处理（还剩 N 项）」，全部落定=「提交本点位」。 -->
  <view class="card gate-card bg-card" :style="{ boxShadow: shadow }">
    <text class="gate-title text-main" >本点位 {{ itemCount }} 项</text>
    <view class="gate-summary">
      <text class="gate-summary-item text-success" >✓已落定 {{ stats.done }}</text>
      <text class="gate-summary-item text-brand" >⏳处理中 {{ stats.processing }}</text>
      <text class="gate-summary-item text-danger" >⚠异常 {{ stats.abnormal }}</text>
      <text v-if="stats.escaped > 0" class="gate-summary-item text-warning" >⊘无法检查 {{ stats.escaped }}</text>
    </view>

    <!-- 逐项实时进度：落定一行勾掉一行 -->
    <view v-for="r in rows" :key="r.key" class="gate-row border-default" >
      <text class="gate-row-icon">⏳</text>
      <text class="gate-row-name text-main" >{{ r.name }}</text>
      <text class="gate-row-stage"  :class="(r.pending ? 'text-warning' : 'text-brand')">{{ r.stage }}</text>
      <text
        v-if="r.canSkip"
        hover-class="hover-dim"
        class="gate-row-act text-brand"
        
        @click="$emit('skip', r.item)"
      >跳过识别 ›</text>
      <text
        v-else-if="r.pending"
        hover-class="hover-dim"
        class="gate-row-act text-warning"
        
        @click="$emit('retry', r.item)"
      >重试 ›</text>
    </view>

    <text v-if="rows.length == 0" class="gate-ok text-success" >✓ 全部落定，可以提交</text>
    <view v-if="submitError != ''" class="gate-error bg-danger" >
      <text class="gate-error-text text-white" >上次提交失败（{{ submitError }}），请点下方按钮重试</text>
    </view>
  </view>
</template>

<script lang="ts">
import type { WizardItemSnap } from '@/utils/checkinWizard'

type GateStats = { done: number; processing: number; abnormal: number; escaped: number }
/** 逐项进度行：stage=上传中…/排队中…/识别中…/待补传；canSkip=可跳过识别（转人工复核）；pending=待补传可点重试 */
type GateRow = { key: string; name: string; stage: string; canSkip: boolean; pending: boolean; item: WizardItemSnap }

export default {
  props: {
    itemCount: { type: Number, default: 0 },
    stats: { type: Object as () => GateStats, required: true },
    /** 未落定项实时进度行（落定即从列表勾掉） */
    rows: { type: Array as () => GateRow[], default: () => [] },
    /** 上次提交失败原因（空 = 不显示红色状态条） */
    submitError: { type: String, default: '' },
    shadow: { type: String, default: '' }
  },
  emits: ['skip', 'retry']
}
</script>

<style scoped>
.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.gate-card {
  align-items: center;
}

.gate-title {
  font-size: 48rpx;
  font-weight: 700;
}

.gate-summary {
  flex-direction: row;
  align-items: center;
  justify-content: space-around;
  width: 100%;
  margin-top: 24rpx;
}

.gate-summary-item {
  font-size: 32rpx;
  font-weight: 700;
}

.gate-row {
  width: 100%;
  min-height: 96rpx;
  flex-direction: row;
  align-items: center;
  border-top-width: 1rpx;
  border-top-style: solid;
  margin-top: 16rpx;
  padding-top: 8rpx;
}

.gate-row-icon {
  font-size: 30rpx;
}

.gate-row-name {
  font-size: 32rpx;
  font-weight: 600;
  flex: 1;
  margin-left: 12rpx;
}

.gate-row-stage {
  font-size: 28rpx;
  margin-left: 16rpx;
}

.gate-row-act {
  font-size: 28rpx;
  font-weight: 600;
  padding: 12rpx 0 12rpx 24rpx;
}

.gate-ok {
  font-size: 30rpx;
  font-weight: 600;
  margin-top: 24rpx;
}

.gate-error {
  width: 100%;
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-top: 24rpx;
  align-items: center;
}

.gate-error-text {
  font-size: 28rpx;
  font-weight: 600;
  text-align: center;
}
</style>
