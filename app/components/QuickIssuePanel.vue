<template>
  <view>
    <view class="banner" :style="{ backgroundColor: colors.danger }">
      <text class="banner-text" :style="{ color: colors.white }">
        {{ mode == 'retake' ? '⚠ 这几项要重新拍' : '⚠ 发现 ' + items.length + ' 项异常' }}
      </text>
    </view>

    <template v-if="mode == 'retake'">
      <view
        v-for="(item, index) in items"
        :key="item.name + ':' + index"
        class="card retake-card"
        :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }"
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
          <text class="retake-name" :style="{ color: colors.textPrimary }">{{ item.name }}</text>
          <text class="retake-issue" :style="{ color: colors.danger }">{{ retakeIssue(item) }}</text>
        </view>
        <view
          hover-class="hover-dim"
          class="retake-btn"
          :style="{ backgroundColor: item.status == 'recognizing' ? colors.info : colors.primary }"
          @click="$emit('retake', item)"
        >
          <text class="retake-btn-text" :style="{ color: colors.white }">{{ item.status == 'recognizing' ? '检查中' : '重拍' }}</text>
        </view>
      </view>
    </template>

    <template v-else>
      <view
        v-for="(item, index) in items"
        :key="item.name + ':' + index"
        class="card"
        :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }"
      >
        <text class="abn-name" :style="{ color: colors.textPrimary }">{{ item.name }}</text>
        <textarea
          v-if="aiEditable"
          class="abn-note"
          :style="{ borderColor: colors.border, color: colors.textPrimary, backgroundColor: colors.bgPage }"
          :value="item.note"
          placeholder="补充说明（可不填）"
          :maxlength="200"
          @input="onNoteInput(item, $event)"
        />
        <text v-else class="abn-reason" :style="{ color: colors.danger }">{{ item.note != '' ? item.note : 'AI 判断该项异常' }}</text>
      </view>
    </template>

    <view
      hover-class="hover-dim"
      class="btn-big"
      :style="{ backgroundColor: mode == 'retake' ? colors.success : colors.danger }"
      @click="$emit('confirm')"
    >
      <text class="btn-big-text" :style="{ color: colors.white }">{{ mode == 'retake' ? '重新提交本点位' : '确认，去下一处' }}</text>
    </view>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import type { WizardItemSnap } from '@/utils/checkinWizard'

export default {
  props: {
    mode: { type: String as () => 'retake' | 'abnormal', required: true },
    items: { type: Array as () => WizardItemSnap[], default: () => [] },
    aiEditable: { type: Boolean, default: false },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: ['preview', 'image-error', 'retake', 'update-note', 'confirm'],
  methods: {
    retakeIssue(item: WizardItemSnap): string {
      if (item.status == 'todo') return '还没拍'
      if (item.status == 'failed') return item.quality_issue != '' ? item.quality_issue : '识别失败，请重拍'
      return item.quality_issue != '' ? item.quality_issue : '照片不合格'
    },
    onNoteInput(item: WizardItemSnap, event: any) {
      const value = event != null && event.detail != null ? String(event.detail.value) : ''
      this.$emit('update-note', { item, value })
    }
  }
}
</script>

<style scoped>
.banner {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
  align-items: center;
}

.banner-text {
  font-size: 48rpx;
  font-weight: 700;
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

.retake-btn {
  width: 160rpx;
  height: 96rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-left: 24rpx;
}

.retake-btn-text {
  font-size: 36rpx;
  font-weight: 700;
}

.abn-name {
  font-size: 40rpx;
  font-weight: 700;
}

.abn-reason {
  font-size: 34rpx;
  margin-top: 12rpx;
  line-height: 48rpx;
}

.abn-note {
  width: 100%;
  height: 160rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: solid;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 16rpx;
}

.btn-big {
  width: 100%;
  height: 140rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 24rpx;
}

.btn-big-text {
  font-size: 44rpx;
  font-weight: 700;
}
</style>
