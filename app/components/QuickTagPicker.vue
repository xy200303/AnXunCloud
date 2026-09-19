<template>
  <!-- 观察点多选半屏（方案 §八；§19 官方化：checklist = uni-data-checkbox multiple mode=list，
       勾选色对齐 $uni-error 保异常语义）：底部滑出勾选异常；withNote=true 时为「有异常」半屏（观察点+备注+确认）。
       对外 props/事件不变（toggle 仍按单个 tag 抛出，由 change 差集换算） -->
  <AppBottomSheet :visible="visible" @close="$emit('update:visible', false)">
    <view class="tp">
      <text class="tp-title text-main">{{ title }}</text>
      <text v-if="tags.length > 0" class="tp-sub text-secondary">默认全部正常，有异常的观察点勾上</text>
      <scroll-view scroll-y enhanced :show-scrollbar="false" class="tp-list" :style="{ height: listHeight }">
        <uni-data-checkbox
          v-if="tags.length > 0"
          multiple
          mode="list"
          :localdata="checkItems"
          :modelValue="selected"
          selectedColor="#D54941"
          selectedTextColor="#D54941"
          @change="onChecksChange"
        />
        <view v-else class="tp-empty">
          <text class="tp-empty-text text-secondary">该项没有配置观察点</text>
        </view>
      </scroll-view>
      <textarea
        v-if="withNote"
        class="tp-note border-default text-main bg-page"
        :value="note"
        placeholder="说说哪里不对劲（可不填）"
        :maxlength="200"
        @input="onNote"
      />
      <button
        plain="true"
        hover-class="hover-dim"
        class="tp-confirm"
        :class="(withNote ? 'btn-danger' : 'btn-primary')"
        @click="$emit('confirm')"
      >
        <text class="tp-confirm-text">{{ confirmText }}</text>
      </button>
    </view>
  </AppBottomSheet>
</template>

<script lang="ts">
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
  },
  emits: ['update:visible', 'toggle', 'update:note', 'confirm'],
  computed: {
    /** string[] 观察点 → uni-data-checkbox localdata 形态（text/value 同名） */
    checkItems(): Array<{ text: string; value: string }> {
      return this.tags.map((t) => ({ text: t, value: t }))
    },
    /** 列表定高（scroll-view 必须显式高度才有原生滚动，max-height 会被内容撑开导致滚动卡顿/失效）：
     *  每行约 104rpx + 边框，超过 480rpx 时内部滚动 */
    listHeight(): string {
      const h = this.tags.length * 106
      return (h > 480 ? 480 : Math.max(h, 106)) + 'rpx'
    }
  },
  methods: {
    /** uni-data-checkbox 整体值变化 → 与当前勾选求差集，按单 tag 逐个抛 toggle（父级接口不变） */
    onChecksChange(e: { detail: { value: string[] } }) {
      const next = e != null && e.detail != null && Array.isArray(e.detail.value) ? e.detail.value : []
      const cur = this.selected
      next.filter((t) => cur.indexOf(t) < 0).forEach((t) => this.$emit('toggle', t))
      cur.filter((t) => next.indexOf(t) < 0).forEach((t) => this.$emit('toggle', t))
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
