<template>
  <!-- 观察点多选半屏（方案 §八）：底部滑出 checklist 勾选异常；withNote=true 时为「有异常」半屏（观察点+备注+确认） -->
  <AppBottomSheet :visible="visible" @close="$emit('update:visible', false)">
    <view class="tp">
      <text class="tp-title" :style="{ color: colors.textPrimary }">{{ title }}</text>
      <text v-if="tags.length > 0" class="tp-sub" :style="{ color: colors.textSecondary }">默认全部正常，有异常的观察点勾上</text>
      <scroll-view scroll-y class="tp-list">
        <view
          v-for="(t, ti) in tags"
          :key="ti"
          hover-class="hover-dim"
          class="tp-row"
          :style="{ borderBottomColor: colors.border }"
          @click="$emit('toggle', t)"
        >
          <text class="tp-row-text" :style="{ color: isSel(t) ? colors.danger : colors.textPrimary }">{{ t }}</text>
          <text class="tp-row-state" :style="{ color: isSel(t) ? colors.danger : colors.textSecondary }">{{ isSel(t) ? '✓ 异常' : '正常' }}</text>
        </view>
        <view v-if="tags.length == 0" class="tp-empty">
          <text class="tp-empty-text" :style="{ color: colors.textSecondary }">该项没有配置观察点</text>
        </view>
      </scroll-view>
      <textarea
        v-if="withNote"
        class="tp-note"
        :value="note"
        :style="{ borderColor: colors.border, color: colors.textPrimary, backgroundColor: colors.bgPage }"
        placeholder="说说哪里不对劲（可不填）"
        :maxlength="200"
        @input="onNote"
      />
      <view
        hover-class="hover-dim"
        class="tp-confirm"
        :style="{ backgroundColor: withNote ? colors.danger : colors.primary }"
        @click="$emit('confirm')"
      >
        <text class="tp-confirm-text" :style="{ color: colors.white }">{{ confirmText }}</text>
      </view>
    </view>
  </AppBottomSheet>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import AppBottomSheet from '@/components/AppBottomSheet.vue'

export default {
  components: { AppBottomSheet },
  props: {
    visible: { type: Boolean, default: false },
    title: { type: String, default: '观察点' },
    /** 全部观察点 */
    tags: { type: Array as () => string[], default: () => [] },
    /** 已勾选为异常的观察点 */
    selected: { type: Array as () => string[], default: () => [] },
    /** true=「有异常」半屏（附加备注输入，确认按钮红色） */
    withNote: { type: Boolean, default: false },
    note: { type: String, default: '' },
    confirmText: { type: String, default: '完成' },
    colors: { type: Object as () => ColorTokens, required: true }
  },
  emits: ['update:visible', 'toggle', 'update:note', 'confirm'],
  methods: {
    isSel(t: string): boolean {
      return this.selected.indexOf(t) >= 0
    },
    onNote(e: any) {
      const v = e != null && e.detail != null ? String(e.detail.value) : ''
      this.$emit('update:note', v)
    }
  }
}
</script>

<style scoped>
.tp {
  padding: 32rpx 32rpx 24rpx;
}

.tp-title {
  font-size: 40rpx;
  font-weight: 700;
  text-align: center;
}

.tp-sub {
  font-size: 26rpx;
  text-align: center;
  margin-top: 12rpx;
}

.tp-list {
  margin-top: 24rpx;
  max-height: 480rpx;
}

.tp-row {
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.tp-row-text {
  font-size: 34rpx;
  font-weight: 600;
  flex: 1;
}

.tp-row-state {
  font-size: 30rpx;
  font-weight: 600;
  margin-left: 24rpx;
}

.tp-empty {
  align-items: center;
  padding: 48rpx 0;
}

.tp-empty-text {
  font-size: 28rpx;
}

.tp-note {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 24rpx;
}

.tp-confirm {
  width: 100%;
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 24rpx;
}

.tp-confirm-text {
  font-size: 40rpx;
  font-weight: 700;
}
</style>
