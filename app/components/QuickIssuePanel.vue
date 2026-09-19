<template>
  <!-- 补拍清算清单页（方案 §六）：无红横幅，逐项卡片（缩略图+名称+原因+操作），主按钮在底部操作栏 -->
  <view>
    <view class="head">
      <text class="head-title text-main" >还有 {{ items.length }} 项要处理</text>
      <text class="head-sub text-secondary" >逐项重新拍照，完成后点底部按钮重新提交</text>
    </view>
    <view
      v-for="(item, index) in items"
      :key="item.name + ':' + index"
      class="card retake-card bg-card"
      :style="{ boxShadow: shadow }"
    >
      <image
        v-if="item.photos.length > 0 && !item.img_error"
        :src="item.photos[0]"
        class="retake-thumb"
        mode="aspectFill"
        lazy-load
        @click="$emit('preview', item)"
        @error="$emit('image-error', item)"
      />
      <view class="retake-texts">
        <text class="retake-name text-main" >{{ item.name }}</text>
        <text class="retake-issue text-danger" >{{ retakeIssue(item) }}</text>
      </view>
      <view class="retake-actions">
        <button
          v-if="item.status != 'recognizing'"
          plain="true"
          hover-class="hover-dim"
          class="retake-btn btn-primary"
          @click="$emit('retake', item)"
        >
          <text class="retake-btn-text">重拍</text>
        </button>
        <text v-else class="retake-waiting text-secondary" >识别中…</text>
        <!-- 识别失败/超时（基础设施故障）才给手动确认逃生；质量不合格仍须重拍 -->
        <button
          v-if="item.status == 'failed'"
          plain="true"
          hover-class="hover-dim"
          class="retake-manual btn-outline"
          @click="$emit('manual-confirm', item)"
        >
          <text class="retake-manual-text">跳过识别</text>
        </button>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import type { WizardItemSnap } from '@/utils/checkinWizard'

export default {
  props: {
    items: { type: Array as () => WizardItemSnap[], default: () => [] },
    shadow: { type: String, default: '' }
  },
  emits: ['preview', 'image-error', 'retake', 'manual-confirm'],
  methods: {
    retakeIssue(item: WizardItemSnap): string {
      if (item.status == 'todo') return '还没拍'
      if (item.status == 'failed') return item.quality_issue != '' ? item.quality_issue : '识别失败，可重拍或跳过识别'
      return item.quality_issue != '' ? item.quality_issue : '照片不合格'
    }
  }
}
</script>

<style scoped>
.head {
  align-items: center;
  margin-bottom: 24rpx;
}

.head-title {
  font-size: 48rpx;
  font-weight: 700;
}

.head-sub {
  font-size: 30rpx;
  margin-top: 12rpx;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.retake-card {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.retake-thumb {
  width: 120rpx;
  height: 120rpx;
  border-radius: 16rpx;
  margin-right: 24rpx;
}

.retake-texts {
  flex: 1;
}

.retake-name {
  font-size: 40rpx;
  font-weight: 700;
}

.retake-issue {
  font-size: 30rpx;
  margin-top: 8rpx;
}

.retake-actions {
  align-items: center;
  margin-left: 24rpx;
}

.retake-btn {
  width: 160rpx;
  height: 96rpx;
  border-radius: 20rpx;
}

.retake-btn-text {
  font-size: 36rpx;
  font-weight: 700;
}

.retake-waiting {
  font-size: 30rpx;
  padding: 20rpx 0;
}

/* 跳过识别：描边次级按钮（不再是小字链接） */
.retake-manual {
  margin-top: 16rpx;
  height: 72rpx;
  padding: 0 24rpx;
  border-radius: 16rpx;
}

.retake-manual-text {
  font-size: 28rpx;
  font-weight: 600;
}
</style>
