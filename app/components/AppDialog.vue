<template>
  <!-- 通用确认/提示弹窗（自绘，替代 uni.showModal）：与维保提交结果弹窗同口径，
       圆形状态图标 + 标题 + 说明 + 胶囊按钮；editable 时带输入框（驳回原因场景） -->
  <view v-if="visible" class="dlg-mask" :style="{ backgroundColor: colors.mask }" @click="onMaskTap">
    <view class="dlg-card" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
      <view v-if="kind != ''" class="dlg-icon" :style="{ backgroundColor: kindColor }">
        <text class="dlg-icon-text" :style="{ color: colors.white }">{{ kindIcon }}</text>
      </view>
      <text class="dlg-title" :style="{ color: colors.textPrimary }">{{ title }}</text>
      <text v-if="content != ''" class="dlg-content" :style="{ color: colors.textRegular }">{{ content }}</text>
      <input
        v-if="editable"
        class="dlg-input"
        :style="{ borderColor: colors.border, color: colors.textPrimary, backgroundColor: colors.bgPage }"
        v-model="inputValue"
        :placeholder="placeholder"
        :placeholder-style="'color:' + colors.textSecondary"
      />
      <!-- cancelText 为空：单按钮整宽胶囊；非空：左取消（灰底）右确认（实色） -->
      <view v-if="cancelText == ''" hover-class="hover-dim" class="dlg-btn" :style="{ backgroundColor: kindColor }" @click="onConfirm">
        <text class="dlg-btn-text" :style="{ color: colors.white }">{{ confirmText }}</text>
      </view>
      <view v-else class="dlg-btns">
        <view hover-class="hover-dim" class="dlg-btn dlg-btn-half" :style="{ backgroundColor: colors.bgPage }" @click="onCancel">
          <text class="dlg-btn-text" :style="{ color: colors.textRegular }">{{ cancelText }}</text>
        </view>
        <view hover-class="hover-dim" class="dlg-btn dlg-btn-half" :style="{ backgroundColor: kindColor }" @click="onConfirm">
          <text class="dlg-btn-text" :style="{ color: colors.white }">{{ confirmText }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'

type DialogData = {
  colors: ColorTokens
  /** editable 输入值（每次打开回填 defaultValue） */
  inputValue: string
}

export default {
  props: {
    visible: { type: Boolean, default: false },
    /** ''=无图标；success=✓ / warning=! / danger=✕ / primary=● */
    kind: { type: String, default: '' },
    title: { type: String, default: '' },
    content: { type: String, default: '' },
    confirmText: { type: String, default: '知道了' },
    cancelText: { type: String, default: '' },
    editable: { type: Boolean, default: false },
    placeholder: { type: String, default: '' },
    defaultValue: { type: String, default: '' },
    maskClosable: { type: Boolean, default: false }
  },
  emits: ['confirm', 'cancel', 'update:visible'],
  data(): DialogData {
    return {
      colors: Colors,
      inputValue: ''
    }
  },
  computed: {
    kindColor(): string {
      if (this.kind == 'success') return this.colors.success
      if (this.kind == 'warning') return this.colors.warning
      if (this.kind == 'danger') return this.colors.danger
      return this.colors.primary
    },
    kindIcon(): string {
      if (this.kind == 'success') return '✓'
      if (this.kind == 'danger') return '✕'
      if (this.kind == 'warning') return '!'
      return '●'
    }
  },
  watch: {
    // 每次打开回填默认值，避免串用上一次输入
    visible(v: boolean) {
      if (v) this.inputValue = this.defaultValue
    }
  },
  methods: {
    noop() {},
    onMaskTap() {
      if (this.maskClosable) this.onCancel()
    },
    onConfirm() {
      this.$emit('update:visible', false)
      this.$emit('confirm', this.editable ? this.inputValue : undefined)
    },
    onCancel() {
      this.$emit('update:visible', false)
      this.$emit('cancel')
    }
  }
}
</script>

<style scoped>
.dlg-mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 1000;
  justify-content: center;
  align-items: center;
  animation: dlg-fade-in 180ms ease-out;
}

.dlg-card {
  width: 600rpx;
  border-radius: 28rpx;
  padding: 48rpx 40rpx 40rpx;
  align-items: center;
}

.dlg-icon {
  width: 112rpx;
  height: 112rpx;
  border-radius: 56rpx;
  align-items: center;
  justify-content: center;
}

.dlg-icon-text {
  font-size: 56rpx;
  font-weight: 700;
}

.dlg-title {
  font-size: 36rpx;
  font-weight: 700;
  margin-top: 24rpx;
  text-align: center;
}

.dlg-content {
  font-size: 28rpx;
  margin-top: 16rpx;
  line-height: 44rpx;
  text-align: center;
  /* content 支持 \n 多行 */
  white-space: pre-line;
}

.dlg-input {
  width: 100%;
  height: 88rpx;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 0 24rpx;
  margin-top: 24rpx;
  font-size: 28rpx;
  box-sizing: border-box;
}

.dlg-btn {
  width: 100%;
  height: 96rpx;
  border-radius: 48rpx;
  align-items: center;
  justify-content: center;
  margin-top: 40rpx;
}

.dlg-btn-text {
  font-size: 34rpx;
  font-weight: 600;
}

.dlg-btns {
  flex-direction: row;
  width: 100%;
}

.dlg-btn-half {
  flex: 1;
  width: auto;
}

.dlg-btn-half + .dlg-btn-half {
  margin-left: 24rpx;
}

@keyframes dlg-fade-in {
  from { opacity: 0; }
  to { opacity: 1; }
}
</style>
