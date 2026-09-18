<template>
  <!-- 打卡向导底部操作栏（文档流底部，随整页一起滚动，不固定）：主操作区通栏居中（双按钮并排均分），「‹ 上一项」为主按钮正下方居中灰字。
       状态-按钮对照见《打卡向导交互重构设计方案》第五节（唯一真相）；凭证步不渲染本栏（由页面控制 v-if）。 -->
  <view class="wbb" :style="{ backgroundColor: colors.bgCard, borderTopColor: colors.border }">
    <view class="wbb-main">
      <template v-if="secondaryText != ''">
        <view hover-class="hover-dim" class="wbb-btn wbb-half" :style="btnStyle(primaryKind)" @click="onPrimary">
          <text class="wbb-btn-text" :style="btnTextStyle(primaryKind)">{{ primaryText }}</text>
        </view>
        <view hover-class="hover-dim" class="wbb-btn wbb-half" :style="btnStyle(secondaryKind)" @click="onSecondary">
          <text class="wbb-btn-text" :style="btnTextStyle(secondaryKind)">{{ secondaryText }}</text>
        </view>
      </template>
      <view v-else hover-class="hover-dim" class="wbb-btn wbb-full" :style="btnStyle(primaryKind)" @click="onPrimary">
        <text class="wbb-btn-text" :style="btnTextStyle(primaryKind)">{{ primaryText }}</text>
      </view>
    </view>
    <view v-if="prevVisible" hover-class="hover-dim" class="wbb-prev" @click="onPrev">
      <text class="wbb-prev-text" :style="{ color: colors.textSecondary }">‹ 上一项</text>
    </view>
    <!-- 档位切换（逐项步显示）：AI 档="AI 不好使？改用人工填写"；手动档="改回 AI 自动识别"。双向可切，草稿在云端，互切不丢进度 -->
    <view v-if="manualVisible" hover-class="hover-dim" class="wbb-prev" @click="$emit('manual')">
      <text class="wbb-prev-text" :style="{ color: colors.textSecondary }">{{ manualText }}</text>
    </view>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'

/** 按钮种类：primary/success/danger 实心；disabled 置灰不可点 */
type BtnKind = 'primary' | 'success' | 'danger' | 'disabled'

export default {
  props: {
    primaryText: { type: String, default: '' },
    primaryKind: { type: String as () => BtnKind, default: 'primary' },
    /** 非空时主操作区为双按钮并排均分（左 primary 右 secondary） */
    secondaryText: { type: String, default: '' },
    secondaryKind: { type: String as () => BtnKind, default: 'danger' },
    prevVisible: { type: Boolean, default: false },
    /** 逐项步底栏档位切换链 */
    manualVisible: { type: Boolean, default: false },
    manualText: { type: String, default: 'AI 不好使？改用人工填写' },
    colors: { type: Object as () => ColorTokens, required: true }
  },
  emits: ['primary', 'secondary', 'prev', 'manual'],
  methods: {
    btnBg(kind: BtnKind): string {
      if (kind == 'success') return this.colors.success
      if (kind == 'danger') return this.colors.danger
      if (kind == 'disabled') return this.colors.border
      return this.colors.primary
    },
    btnStyle(kind: BtnKind): Record<string, string> {
      return { backgroundColor: this.btnBg(kind) }
    },
    btnTextStyle(kind: BtnKind): Record<string, string> {
      return { color: kind == 'disabled' ? this.colors.textSecondary : this.colors.white }
    },
    onPrimary() {
      if (this.primaryKind == 'disabled') return
      this.$emit('primary')
    },
    onSecondary() {
      if (this.secondaryKind == 'disabled') return
      this.$emit('secondary')
    },
    onPrev() {
      this.$emit('prev')
    }
  }
}
</script>

<style scoped>
.wbb {
  width: 100%;
  border-top-width: 1rpx;
  border-top-style: solid;
  margin-top: 24rpx;
  padding: 20rpx 24rpx calc(20rpx + env(safe-area-inset-bottom));
}

.wbb-main {
  flex-direction: row;
  align-items: center;
}

.wbb-btn {
  height: 112rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.wbb-full {
  width: 100%;
}

.wbb-half {
  flex: 1;
}

.wbb-half + .wbb-half {
  margin-left: 24rpx;
}

.wbb-btn-text {
  font-size: 40rpx;
  font-weight: 700;
}

/* 「‹ 上一项」：主按钮正下方居中灰字，位置永远不变 */
.wbb-prev {
  align-items: center;
  justify-content: center;
  padding-top: 16rpx;
}

.wbb-prev-text {
  font-size: 30rpx;
  padding: 12rpx 48rpx;
}
</style>
