<template>
  <!-- 状态条 ResultBar（方案 §4.4）：纯只读文案，不带任何按钮（重拍走照片槽「重新拍照」，
       跳过识别走底部操作栏/清算页，避免重复按钮）。无状态时整行不占位。 -->
  <view v-if="line != null" class="rb">
    <view class="rb-bar" :style="{ backgroundColor: line.bg }">
      <text class="rb-text" :style="{ color: line.fg }">{{ line.text }}</text>
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
</style>
