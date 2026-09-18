<template>
  <!-- 状态条 ResultBar（方案 §4.4）：只读展示后台进度，仅「‹ 上一项」回退查看时可见；
       没有任何状态会拦住巡检员（不合格/失败统一在点位提交时清算）。无状态时整行不占位。 -->
  <view v-if="line != null" class="rb">
    <view class="rb-bar" :style="{ backgroundColor: line.bg }">
      <text class="rb-text" :style="{ color: line.fg }">{{ line.text }}</text>
    </view>
    <view v-if="showActions" class="rb-actions">
      <view hover-class="hover-dim" class="rb-act" :style="{ borderColor: colors.primary }" @click="$emit('retake')">
        <text class="rb-act-text" :style="{ color: colors.primary }">重新拍照</text>
      </view>
      <view v-if="status == 'failed'" hover-class="hover-dim" class="rb-act rb-act-gap" :style="{ borderColor: colors.primary }" @click="$emit('skip')">
        <text class="rb-act-text" :style="{ color: colors.primary }">跳过识别</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'

type BarLine = { text: string; bg: string; fg: string }

export default {
  props: {
    /** todo/recognizing/done/failed */
    status: { type: String, default: 'todo' },
    verdict: { type: String, default: '' },
    reason: { type: String, default: '' },
    qualityPass: { type: Boolean, default: true },
    qualityIssue: { type: String, default: '' },
    /** 是否已建 AI job（recognizing 且无 job = 还在上传/排队） */
    hasJob: { type: Boolean, default: false },
    colors: { type: Object as () => ColorTokens, required: true }
  },
  emits: ['retake', 'skip'],
  computed: {
    line(): BarLine | null {
      if (this.status == 'recognizing') {
        // 排队/上传中：灰条；识别中：蓝条（不计时、不催促）
        if (!this.hasJob) return { text: '照片处理中…', bg: this.colors.border, fg: this.colors.textRegular }
        return { text: 'AI 识别中…', bg: this.colors.primaryLight, fg: this.colors.primary }
      }
      if (this.status == 'failed') {
        return { text: '识别失败', bg: this.colors.danger, fg: this.colors.white }
      }
      if (this.status == 'done') {
        if (!this.qualityPass) {
          return { text: '照片不合格：' + (this.qualityIssue != '' ? this.qualityIssue : '请重新拍照'), bg: this.colors.danger, fg: this.colors.white }
        }
        if (this.verdict == 'abnormal') {
          let r = this.reason
          if (r.length > 30) r = r.slice(0, 30) + '…'
          return { text: '⚠ AI 发现异常' + (r != '' ? '：' + r : ''), bg: this.colors.warning, fg: this.colors.white }
        }
        if (this.verdict == 'review') {
          return { text: '需人工复核', bg: this.colors.warning, fg: this.colors.white }
        }
        if (this.verdict == 'pass') {
          return { text: '✓ AI 检查正常', bg: this.colors.success, fg: this.colors.white }
        }
      }
      return null
    },
    /** 质量不合格 → 「重新拍照」；失败/超时 → 「重新拍照」「跳过识别」 */
    showActions(): boolean {
      if (this.status == 'failed') return true
      return this.status == 'done' && !this.qualityPass
    }
  }
}
</script>

<style scoped>
.rb {
  width: 100%;
  margin-top: 16rpx;
}

.rb-bar {
  width: 100%;
  border-radius: 16rpx;
  padding: 18rpx 24rpx;
  align-items: center;
}

.rb-text {
  font-size: 30rpx;
  font-weight: 600;
  text-align: center;
}

.rb-actions {
  flex-direction: row;
  justify-content: center;
  margin-top: 16rpx;
}

.rb-act {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  align-items: center;
  justify-content: center;
  padding: 0 48rpx;
}

.rb-act-gap {
  margin-left: 24rpx;
}

.rb-act-text {
  font-size: 30rpx;
  font-weight: 600;
}
</style>
