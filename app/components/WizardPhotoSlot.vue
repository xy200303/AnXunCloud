<template>
  <!-- 照片槽 PhotoSlot（六态，方案 §4.3）：
       空·必拍=灰底占位"还没有照片"（不可点，拍照只走底栏大按钮）；空·选拍=列表行"📷 随手拍一张（可选） ›"；
       后台处理中=本地实拍图+右上角状态点；已拍=16:9 大图+通栏描边「重新拍照」（一图一位，重拍=替换）；
       待补传=大图+橙色状态条（可点立即重试）；已逃生=灰占位+异常类型文字（点导航条「?」改报）。
       variant=multi 为多图变体（仅台账新标签/手动档多图/处置佐证，至多 max 张）。 -->
  <view class="slot">
    <!-- 已逃生（可撤销：选错回到待拍） -->
    <view v-if="escaped" class="slot-escaped" :style="{ backgroundColor: colors.bgPage }">
      <text class="slot-escaped-text" :style="{ color: colors.textRegular }">{{ escapeText }}</text>
      <text class="slot-escaped-hint" :style="{ color: colors.textSecondary }">点导航条右上角 ? 可改报</text>
      <view hover-class="hover-dim" class="slot-undo" :style="{ borderColor: colors.primary }" @click="$emit('escape-undo')">
        <text class="slot-undo-text" :style="{ color: colors.primary }">撤销上报，重新拍照</text>
      </view>
    </view>

    <template v-else>
      <!-- 待补传：大图 + 橙色状态条（点条=立即重试） -->
      <template v-if="pendingLocal != ''">
        <view class="slot-img-wrap" @click="onPreview(0)">
          <image
            v-if="photos.length > 0 && !imgError"
            :src="photos[0]"
            class="slot-img"
            mode="aspectFill"
            @error="$emit('image-error')"
          />
          <view v-else class="slot-img slot-img-fallback">
            <text class="slot-img-fallback-text">照片已保留，待补传</text>
          </view>
        </view>
        <view hover-class="hover-dim" class="slot-pending" :style="{ backgroundColor: colors.warning }" @click="$emit('retry-upload')">
          <text class="slot-pending-text" :style="{ color: colors.white }">照片还没传上去，联网自动补传（点我立即重试）</text>
        </view>
        <view hover-class="hover-dim" class="slot-retake" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
          <text class="slot-retake-text" :style="{ color: colors.primary }">重新拍照</text>
        </view>
      </template>

      <!-- 空态 -->
      <template v-else-if="photos.length == 0">
        <view v-if="required" class="slot-empty" :style="{ backgroundColor: colors.bgPage }">
          <text class="slot-empty-text" :style="{ color: colors.textSecondary }">还没有照片</text>
          <text class="slot-empty-hint" :style="{ color: colors.textSecondary }">点底部「📷 拍照片」按钮拍摄</text>
        </view>
        <view v-else hover-class="hover-dim" class="slot-opt-row" :style="{ borderColor: colors.border }" @click="$emit('take-photo')">
          <text class="slot-opt-text" :style="{ color: colors.primary }">📷 {{ emptyAddText }}</text>
          <text class="slot-opt-arrow" :style="{ color: colors.textSecondary }">›</text>
        </view>
      </template>

      <!-- 已拍 / 后台处理中 -->
      <template v-else>
        <view class="slot-img-wrap" @click="onPreview(0)">
          <image
            v-if="!imgError"
            :src="photos[0]"
            class="slot-img"
            mode="aspectFill"
            lazy-load
            @error="$emit('image-error')"
          />
          <view v-else class="slot-img slot-img-fallback">
            <text class="slot-img-fallback-text">照片加载失败</text>
          </view>
          <!-- 后台处理中：右上角小型状态点（灰转圈），不阻塞任何操作 -->
          <view v-if="status == 'recognizing'" class="slot-busy" :style="{ backgroundColor: colors.bgCard }">
            <view class="slot-busy-dot" :style="{ borderTopColor: colors.info }"></view>
          </view>
        </view>
        <!-- 单图：一图一位，重拍=替换 -->
        <view
          v-if="variant == 'single' && status != 'recognizing'"
          hover-class="hover-dim"
          class="slot-retake"
          :style="{ borderColor: colors.primary }"
          @click="$emit('take-photo')"
        >
          <text class="slot-retake-text" :style="{ color: colors.primary }">重新拍照</text>
        </view>
        <!-- 多图变体：缩略图行 + 补拍列表行 -->
        <block v-if="variant == 'multi'">
          <view v-if="photos.length > 1" class="slot-thumbs">
            <image
              v-for="(p, pi) in photos"
              :key="pi"
              :src="p"
              class="slot-thumb"
              mode="aspectFill"
              @click="onPreview(pi)"
              @error="$emit('image-error')"
            />
          </view>
          <view
            v-if="photos.length < max"
            hover-class="hover-dim"
            class="slot-opt-row"
            :style="{ borderColor: colors.border }"
            @click="$emit('take-photo')"
          >
            <text class="slot-opt-text" :style="{ color: colors.primary }">📷 {{ addText }}（{{ photos.length }}/{{ max }}）</text>
            <text class="slot-opt-arrow" :style="{ color: colors.textSecondary }">›</text>
          </view>
        </block>
      </template>
    </template>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'

export default {
  props: {
    /** 照片展示地址（本地临时路径或服务端 URL） */
    photos: { type: Array as () => string[], default: () => [] },
    imgError: { type: Boolean, default: false },
    /** todo/recognizing/done/failed；recognizing 时右上角显示状态点 */
    status: { type: String, default: 'todo' },
    /** 非空 = 待补传态（值为本地压缩图路径） */
    pendingLocal: { type: String, default: '' },
    escaped: { type: Boolean, default: false },
    escapeText: { type: String, default: '' },
    /** true=必拍（空态灰占位不可点）；false=选拍（空态列表行可点调相机） */
    required: { type: Boolean, default: true },
    /** single=一图一位（重拍=替换）；multi=多图（至多 max 张） */
    variant: { type: String as () => 'single' | 'multi', default: 'single' },
    max: { type: Number, default: 3 },
    /** 空·选拍列表行文案 */
    emptyAddText: { type: String, default: '随手拍一张（可选）' },
    /** 多图变体补拍行文案 */
    addText: { type: String, default: '再拍一张' },
    colors: { type: Object as () => ColorTokens, required: true }
  },
  emits: ['take-photo', 'preview', 'retry-upload', 'image-error', 'escape-undo'],
  methods: {
    onPreview(idx: number) {
      if (this.photos.length == 0 || this.imgError) return
      this.$emit('preview', idx)
    }
  }
}
</script>

<style scoped>
.slot {
  width: 100%;
  margin-top: 32rpx;
}

/* 空·必拍：灰底占位，不可点 */
.slot-empty {
  width: 100%;
  height: 380rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.slot-empty-text {
  font-size: 36rpx;
  font-weight: 600;
}

.slot-empty-hint {
  font-size: 26rpx;
  margin-top: 12rpx;
}

/* 空·选拍：微信列表行（左文右 ›） */
.slot-opt-row {
  width: 100%;
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 20rpx;
  padding: 0 28rpx;
}

.slot-opt-text {
  font-size: 34rpx;
  font-weight: 600;
}

.slot-opt-arrow {
  font-size: 44rpx;
}

.slot-img-wrap {
  width: 100%;
  height: 422rpx;
  border-radius: 20rpx;
  overflow: hidden;
  position: relative;
}

.slot-img {
  width: 100%;
  height: 422rpx;
}

.slot-img-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f2f3f5;
}

.slot-img-fallback-text {
  font-size: 26rpx;
  color: #9ca3af;
}

/* 后台处理中状态点（右上角，灰转圈） */
.slot-busy {
  position: absolute;
  top: 16rpx;
  right: 16rpx;
  width: 48rpx;
  height: 48rpx;
  border-radius: 24rpx;
  align-items: center;
  justify-content: center;
}

.slot-busy-dot {
  width: 28rpx;
  height: 28rpx;
  border-radius: 14rpx;
  border-width: 4rpx;
  border-style: solid;
  border-color: rgba(0, 0, 0, 0.12);
  animation: slot-spin 0.9s linear infinite;
}

@keyframes slot-spin {
  from {
    transform: rotate(0deg);
  }
  to {
    transform: rotate(360deg);
  }
}

/* 待补传橙色状态条（可点=立即重试） */
.slot-pending {
  width: 100%;
  border-radius: 16rpx;
  padding: 18rpx 24rpx;
  margin-top: 16rpx;
  align-items: center;
}

.slot-pending-text {
  font-size: 28rpx;
  font-weight: 600;
}

/* 已拍态：照片下方通栏描边「重新拍照」 */
.slot-retake {
  width: 100%;
  height: 104rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 16rpx;
}

.slot-retake-text {
  font-size: 36rpx;
  font-weight: 600;
}

/* 已逃生：灰占位 + 异常类型文字 */
.slot-escaped {
  width: 100%;
  height: 380rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  padding: 0 48rpx;
}

.slot-escaped-text {
  font-size: 34rpx;
  font-weight: 600;
  text-align: center;
  line-height: 48rpx;
}

.slot-escaped-hint {
  font-size: 26rpx;
  margin-top: 16rpx;
}

/* 逃生撤销：描边文字按钮（选错回到待拍） */
.slot-undo {
  margin-top: 24rpx;
  height: 80rpx;
  padding: 0 40rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  align-items: center;
  justify-content: center;
}

.slot-undo-text {
  font-size: 30rpx;
  font-weight: 600;
}

/* 多图变体缩略图行 */
.slot-thumbs {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.slot-thumb {
  width: 120rpx;
  height: 120rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
}
</style>
