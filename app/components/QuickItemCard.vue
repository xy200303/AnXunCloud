<template>
  <view class="item-card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
    <text class="item-name" :style="{ color: colors.textPrimary }">{{ item.name }}</text>
    <text class="item-hint" :style="{ color: colors.textSecondary }">{{ item.requirement != '' ? item.requirement : (isPhoto ? '拍一张该项的照片' : '这项正常吗？') }}</text>

    <template v-if="isPhoto">
      <view
        v-if="item.status == 'todo' || item.status == 'failed'"
        hover-class="hover-dim"
        class="shot-empty"
        :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }"
        @click="$emit('take-photo')"
      >
        <view class="cam-icon" :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }">
          <view class="cam-lens" :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }"></view>
        </view>
        <text class="shot-empty-text" :style="{ color: item.status == 'failed' ? colors.danger : colors.primary }">
          {{ item.status == 'failed' ? '不合格，点这里重拍' : '点这里拍照' }}
        </text>
      </view>
      <block v-else>
        <view class="shot-preview" :style="{ backgroundColor: colors.bgPage }" @click="$emit('preview-photo')">
          <image
            v-if="item.photos.length > 0 && !item.img_error"
            :src="item.photos[0]"
            class="shot-img"
            mode="aspectFill"
            lazy-load
            @error="$emit('image-error')"
          />
          <view v-else class="shot-img shot-img-fallback">
            <text class="shot-img-fallback-text">照片加载失败，可重新拍</text>
          </view>
        </view>
        <view hover-class="hover-dim" class="btn-big shot-next" :style="{ backgroundColor: colors.success }" @click="$emit('next')">
          <text class="btn-big-text" :style="{ color: colors.white }">下一项</text>
        </view>
        <view hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
          <text class="btn-outline-text" :style="{ color: colors.primary }">重新拍</text>
        </view>
      </block>
      <text
        v-if="item.status != 'recognizing'"
        hover-class="hover-dim"
        class="escape-link"
        :style="{ color: item.exception_type != '' ? colors.danger : colors.warning }"
        @click="$emit('report-missing')"
      >{{ exceptionLabel }}</text>
    </template>

    <template v-else>
      <view hover-class="hover-dim" class="btn-big btn-normal" :style="{ backgroundColor: colors.success }" @click="$emit('manual-ok')">
        <text class="btn-big-text" :style="{ color: colors.white }">✓ 正常</text>
      </view>
      <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.danger }" @click="$emit('manual-abnormal')">
        <text class="btn-big-text" :style="{ color: colors.white }">⚠ 有异常</text>
      </view>
      <block v-if="manualAbnormalOpen">
        <textarea
          class="manual-note"
          :value="manualNote"
          :style="{ borderColor: colors.danger, color: colors.textPrimary, backgroundColor: colors.bgPage }"
          placeholder="说说哪里不对劲（可不填）"
          :maxlength="200"
          @input="$emit('update:manual-note', $event.detail.value)"
        />
        <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.danger }" @click="$emit('confirm-manual-abnormal')">
          <text class="btn-big-text" :style="{ color: colors.white }">确认异常，下一项</text>
        </view>
      </block>
    </template>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import type { WizardItemSnap } from '@/utils/checkinWizard'

export default {
  props: {
    item: { type: Object as () => WizardItemSnap, required: true },
    isPhoto: { type: Boolean, default: true },
    manualAbnormalOpen: { type: Boolean, default: false },
    manualNote: { type: String, default: '' },
    exceptionLabel: { type: String, default: '设备不存在/无法检测，提交异常' },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: [
    'take-photo',
    'preview-photo',
    'image-error',
    'next',
    'report-missing',
    'manual-ok',
    'manual-abnormal',
    'update:manual-note',
    'confirm-manual-abnormal'
  ]
}
</script>

<style scoped>
.item-card {
  border-radius: 24rpx;
  padding: 48rpx 32rpx;
  align-items: center;
  margin-bottom: 24rpx;
}

.item-name {
  font-size: 56rpx;
  font-weight: 700;
}

.item-hint {
  font-size: 34rpx;
  text-align: center;
  margin-top: 16rpx;
  line-height: 48rpx;
}

.shot-empty {
  width: 100%;
  height: 360rpx;
  border-width: 3rpx;
  border-style: dashed;
  border-radius: 24rpx;
  margin-top: 40rpx;
  align-items: center;
  justify-content: center;
}

.cam-icon {
  width: 120rpx;
  height: 96rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.cam-lens {
  width: 40rpx;
  height: 40rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
}

.shot-empty-text {
  font-size: 40rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.shot-preview {
  width: 100%;
  height: 480rpx;
  border-radius: 20rpx;
  margin-top: 40rpx;
  overflow: hidden;
  align-items: center;
  justify-content: center;
}

.shot-img {
  width: 100%;
  height: 480rpx;
}

.shot-img-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f2f3f5;
}

.shot-img-fallback-text {
  font-size: 26rpx;
  color: #9ca3af;
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

.btn-outline {
  height: 112rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-outline-text {
  font-size: 40rpx;
  font-weight: 600;
}

.btn-normal {
  margin-top: 32rpx;
}

.shot-next {
  margin-top: 32rpx;
}

.reshot {
  width: 100%;
  margin-top: 8rpx;
}

.manual-note {
  width: 100%;
  height: 192rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 8rpx;
  margin-bottom: 24rpx;
}

.escape-link {
  display: block;
  font-size: 30rpx;
  font-weight: 700;
  margin: 30rpx 12rpx 8rpx;
  padding: 22rpx 18rpx;
  border: 2rpx solid currentColor;
  border-radius: 14rpx;
  text-align: center;
  background-color: rgba(255, 150, 0, 0.1);
}
</style>
