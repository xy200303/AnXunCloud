<template>
  <!-- 状态条 ResultBar（方案 §4.4；§18.1 官方化）：识别中=uni-notice-bar 蓝/灰条，落定态=uni-tag
       （异常/复核=warning，不合格/失败=error，正常=success），颜色语义不变。
       纯只读文案，不带任何按钮（重拍走照片槽「重新拍照」，跳过识别走底部操作栏/清算页）。无状态时整行不占位。 -->
  <view v-if="line != null" class="rb">
    <uni-notice-bar
      v-if="line.kind == 'notice'"
      :text="line.text"
      :single="true"
      :scrollable="false"
      :show-icon="false"
      :color="line.fg"
      :background-color="line.bg"
    />
    <uni-tag
      v-else
      :text="line.text"
      :type="line.tagType"
      :custom-style="'width:100%;text-align:center;padding:18rpx 24rpx;border-radius:16rpx;font-size:30rpx;font-weight:600;display:flex;justify-content:center'"
    />
  </view>
</template>

<script lang="ts">

type BarLine = { text: string; kind: 'notice' | 'tag'; bg: string; fg: string; tagType: 'success' | 'warning' | 'error' }

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
  },
  computed: {
    line(): BarLine | null {
      if (this.status == 'recognizing') {
        // 排队/上传中：灰条；识别中：蓝条（不计时、不催促）
        if (!this.hasJob) return { text: '照片处理中…', kind: 'notice', bg: '#E5E6EB', fg: '#4E5969', tagType: 'success' }
        return { text: 'AI 识别中…', kind: 'notice', bg: '#EAEFFF', fg: '#2B5AED', tagType: 'success' }
      }
      if (this.status == 'failed') {
        return { text: '识别失败', kind: 'tag', bg: '', fg: '', tagType: 'error' }
      }
      if (this.status == 'done') {
        if (!this.qualityPass) {
          return { text: '照片不合格：' + (this.qualityIssue != '' ? this.qualityIssue : '请重新拍照'), kind: 'tag', bg: '', fg: '', tagType: 'error' }
        }
        if (this.verdict == 'abnormal') {
          let r = this.reason
          if (r.length > 30) r = r.slice(0, 30) + '…'
          return { text: '⚠ AI 发现异常' + (r != '' ? '：' + r : ''), kind: 'tag', bg: '', fg: '', tagType: 'warning' }
        }
        if (this.verdict == 'review') {
          return { text: '需人工复核', kind: 'tag', bg: '', fg: '', tagType: 'warning' }
        }
        if (this.verdict == 'pass') {
          return { text: '✓ AI 检查正常', kind: 'tag', bg: '', fg: '', tagType: 'success' }
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
</style>
