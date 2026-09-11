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

        <!-- 处置方式：默认上报待处理；现场已处理需拍处置照片留痕（设备类异常的新标签维保已在该项内拍标签完成，不再单列） -->
        <view class="disp-row">
          <text
            class="disp-opt"
            :style="dispOf(item) == 'report_pending'
              ? { color: colors.white, backgroundColor: colors.danger }
              : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
            @click="$emit('update-disposition', { item: item, value: 'report_pending' })"
          >上报待处理</text>
          <text
            class="disp-opt"
            :style="dispOf(item) == 'on_site_resolved'
              ? { color: colors.white, backgroundColor: colors.success }
              : { color: colors.textSecondary, backgroundColor: colors.bgPage }"
            @click="$emit('update-disposition', { item: item, value: 'on_site_resolved' })"
          >现场已处理</text>
        </view>
        <block v-if="dispOf(item) == 'on_site_resolved'">
          <view class="res-photos">
            <image
              v-for="(p, pi) in resPhotos(item)"
              :key="pi"
              :src="p"
              class="res-thumb"
              mode="aspectFill"
              @click="previewRes(item, pi)"
            />
            <view class="res-add" :style="{ borderColor: colors.success }" @click="$emit('resolution-photo', { item: item })">
              <text class="res-add-text" :style="{ color: colors.success }">+ 拍处置照片</text>
            </view>
          </view>
          <text class="res-hint" :style="{ color: colors.textSecondary }">必拍至少 1 张处置后的照片（至多 3 张），作为已处理凭证</text>
        </block>
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
  emits: ['preview', 'image-error', 'retake', 'update-note', 'update-disposition', 'resolution-photo', 'confirm'],
  methods: {
    retakeIssue(item: WizardItemSnap): string {
      if (item.status == 'todo') return '还没拍'
      if (item.status == 'failed') return item.quality_issue != '' ? item.quality_issue : '识别失败，请重拍'
      return item.quality_issue != '' ? item.quality_issue : '照片不合格'
    },
    onNoteInput(item: WizardItemSnap, event: any) {
      const value = event != null && event.detail != null ? String(event.detail.value) : ''
      this.$emit('update-note', { item, value })
    },
    /** 处置方式（未选 = 默认上报待处理） */
    dispOf(item: WizardItemSnap): string {
      return item.disposition != null && item.disposition != '' ? item.disposition : 'report_pending'
    },
    resPhotos(item: WizardItemSnap): string[] {
      return item.res_photos != null ? item.res_photos : []
    },
    /** 处置照片预览（纯 UI 行为，组件内直接预览即可） */
    previewRes(item: WizardItemSnap, idx: number) {
      const urls = this.resPhotos(item)
      if (urls.length == 0) return
      uni.previewImage({ urls: urls, current: idx })
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

/* 处置方式选择与处置照片 */
.disp-row {
  flex-direction: row;
  margin-top: 24rpx;
}

.disp-opt {
  font-size: 30rpx;
  font-weight: 600;
  padding: 16rpx 36rpx;
  border-radius: 12rpx;
  margin-right: 24rpx;
}

.res-photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 24rpx;
}

.res-thumb {
  width: 160rpx;
  height: 160rpx;
  border-radius: 16rpx;
  margin-right: 16rpx;
}

.res-add {
  width: 240rpx;
  height: 160rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: dashed;
  align-items: center;
  justify-content: center;
}

.res-add-text {
  font-size: 26rpx;
  font-weight: 600;
}

.res-hint {
  font-size: 24rpx;
  margin-top: 12rpx;
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
