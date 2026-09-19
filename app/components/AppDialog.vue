<template>
  <!-- 通用确认/提示弹窗：内部基于 uni-popup（type=center），遮罩/动画交给官方；
       圆形状态图标 + 标题 + 说明 + 胶囊按钮；editable 时带输入框（multiline 升级为多行，
       驳回原因场景）。对外 props/事件接口不变 -->
  <uni-popup
    ref="popup"
    type="center"
    :mask-background-color="'rgba(0, 0, 0, 0.45)'"
    :is-mask-click="maskClosable"
    border-radius="28rpx"
    @maskClick="onMaskTap"
  >
    <view class="dlg-card bg-card" >
      <view v-if="kind != ''" class="dlg-icon" :style="{ backgroundColor: kindColor }">
        <text class="dlg-icon-text text-white" >{{ kindIcon }}</text>
      </view>
      <text class="dlg-title text-main" >{{ title }}</text>
      <text v-if="content != ''" class="dlg-content text-regular" >{{ content }}</text>
      <textarea
        v-if="editable && multiline"
        class="dlg-input dlg-input-multi border-default text-main bg-page"
        v-model="inputValue"
        :placeholder="placeholder"
        :placeholder-style="'color:' + '#86909C'"
        :maxlength="200"
      />
      <input
        v-else-if="editable"
        class="dlg-input border-default text-main bg-page"
        
        v-model="inputValue"
        :placeholder="placeholder"
        :placeholder-style="'color:' + '#86909C'"
      />
      <!-- cancelText 为空：单按钮整宽胶囊；非空：左取消（灰底）右确认（实色） -->
      <button v-if="cancelText == ''" plain="true" hover-class="hover-dim" class="dlg-btn" :class="kindBtnClass" @click="onConfirm">
        <text class="dlg-btn-text">{{ confirmText }}</text>
      </button>
      <view v-else class="dlg-btns">
        <button plain="true" hover-class="hover-dim" class="dlg-btn dlg-btn-half bg-page" @click="onCancel">
          <text class="dlg-btn-text text-regular">{{ cancelText }}</text>
        </button>
        <button plain="true" hover-class="hover-dim" class="dlg-btn dlg-btn-half" :class="kindBtnClass" @click="onConfirm">
          <text class="dlg-btn-text">{{ confirmText }}</text>
        </button>
      </view>
    </view>
  </uni-popup>
</template>

<script lang="ts">


type DialogData = {
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
    /** editable 时用多行 textarea（驳回原因等长文本场景） */
    multiline: { type: Boolean, default: false },
    placeholder: { type: String, default: '' },
    defaultValue: { type: String, default: '' },
    maskClosable: { type: Boolean, default: false }
  },
  emits: ['confirm', 'cancel', 'update:visible'],
  data(): DialogData {
    return {
      inputValue: ''
    }
  },
  computed: {
    /** kind → 全局按钮工具类（确认键；图标圆底色仍用 kindColor） */
    kindBtnClass(): string {
      if (this.kind == 'success') return 'btn-success'
      if (this.kind == 'warning') return 'btn-warning'
      if (this.kind == 'danger') return 'btn-danger'
      return 'btn-primary'
    },
    kindColor(): string {
      if (this.kind == 'success') return '#2BA471'
      if (this.kind == 'warning') return '#ED7B2F'
      if (this.kind == 'danger') return '#D54941'
      return '#2B5AED'
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
      const popup: any = this.$refs.popup
      if (popup != null) {
        if (v) popup.open()
        else popup.close()
      }
      if (v) this.inputValue = this.defaultValue
    }
  },
  mounted() {
    if (this.visible) {
      this.inputValue = this.defaultValue
      const popup: any = this.$refs.popup
      if (popup != null) popup.open()
    }
  },
  methods: {
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

.dlg-input-multi {
  height: 180rpx;
  padding: 20rpx 24rpx;
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
</style>
