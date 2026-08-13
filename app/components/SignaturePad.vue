<template>
  <!-- 手写签名板：canvas 自绘（旧版 canvas API，四端兼容），导出白底 PNG 临时路径 -->
  <view v-if="show" class="pad-mask" :style="{ backgroundColor: colors.mask }" @touchmove.stop.prevent="noop">
    <view class="pad-panel" :style="{ backgroundColor: colors.bgCard, borderRadius: radiusSheet }">
      <view class="pad-head">
        <text class="pad-title" :style="{ color: colors.textPrimary }">手写签名</text>
        <text class="pad-tip" :style="{ color: colors.textSecondary }">请在下方区域手写，保存后为白底 PNG</text>
      </view>

      <view class="pad-canvas-wrap" :style="{ borderColor: colors.border }">
        <canvas
          id="signPad"
          canvas-id="signPad"
          class="pad-canvas"
          :disable-scroll="true"
          @touchstart="onTouchStart"
          @touchmove="onTouchMove"
          @touchend="onTouchEnd"
        ></canvas>
        <view v-if="empty" class="pad-placeholder">
          <text class="pad-placeholder-text" :style="{ color: colors.textSecondary }">请在此区域手写签名</text>
        </view>
      </view>

      <!-- 签字流程内弹出时显示「保存下次使用」勾选；个人中心配置入口必保存不显示 -->
      <view v-if="showSaveOption" class="pad-save" @click="saveForLater = !saveForLater">
        <view
          class="pad-checkbox"
          :style="{
            borderColor: saveForLater ? colors.primary : colors.border,
            backgroundColor: saveForLater ? colors.primary : colors.white
          }"
        >
          <text v-if="saveForLater" class="pad-check" :style="{ color: colors.white }">✓</text>
        </view>
        <text class="pad-save-text" :style="{ color: colors.textRegular }">保存为我的签名，下次签字直接使用</text>
      </view>

      <view class="pad-actions">
        <view class="pad-btn" :style="{ borderColor: colors.border }" @click="onClear">
          <text class="pad-btn-text" :style="{ color: colors.textRegular }">清除</text>
        </view>
        <view class="pad-btn" :style="{ borderColor: colors.border }" @click="onCancel">
          <text class="pad-btn-text" :style="{ color: colors.textRegular }">取消</text>
        </view>
        <view
          class="pad-btn pad-btn-primary"
          :style="{ backgroundColor: colors.primary, opacity: saving ? 0.6 : 1 }"
          @click="onConfirm"
        >
          <text class="pad-btn-text" :style="{ color: colors.white }">{{ saving ? '保存中…' : '确认' }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
/**
 * 用法：父组件 <SignaturePad ref="pad" :show-save-option="true" @save="onPadSave" />
 * - this.$refs.pad.open() 打开；手写后点「确认」emit('save', tempFilePath, saveForLater)，面板保持打开（保存中态）；
 * - 父组件上传完成后调 this.$refs.pad.finish(true) 关闭并重置，失败调 finish(false) 留在面板可重试；
 * - 坐标取 touch.touches[0].x/y（相对画布），ctx.draw(true) 增量保留笔迹。
 */
import { Colors, ColorTokens, Radius } from '@/utils/theme'

type PadData = {
  colors: ColorTokens
  radiusSheet: string
  show: boolean
  empty: boolean
  saving: boolean
  saveForLater: boolean
}

export default {
  name: 'SignaturePad',
  props: {
    showSaveOption: {
      type: Boolean,
      default: false
    }
  },
  data(): PadData {
    return {
      colors: Colors,
      radiusSheet: Radius.sheet,
      show: false,
      empty: true,
      saving: false,
      saveForLater: true
    }
  },
  methods: {
    noop() {},
    open() {
      this.show = true
      this.empty = true
      this.saving = false
      this.$nextTick(() => {
        // 等弹层渲染完成再取画布上下文与尺寸
        this.ctx = uni.createCanvasContext('signPad', this)
        this.ctx.setStrokeStyle('#141414')
        this.ctx.setLineWidth(3)
        this.ctx.setLineCap('round')
        this.ctx.setLineJoin('round')
        this.drawing = false
        uni
          .createSelectorQuery()
          .in(this)
          .select('.pad-canvas')
          .boundingClientRect((rect: any) => {
            if (rect != null) {
              this.canvasW = rect.width
              this.canvasH = rect.height
            }
          })
          .exec()
      })
    },
    /** 父组件上传结果回执：true 关闭并重置；false 留在面板（可重试/取消） */
    finish(ok: boolean) {
      this.saving = false
      if (ok) {
        this.show = false
        this.empty = true
        this.saveForLater = true
        this.drawing = false
      }
    },
    onTouchStart(e: any) {
      if (this.ctx == null || this.saving) return
      const t = e.touches[0]
      this.drawing = true
      this.lastX = t.x
      this.lastY = t.y
      // 画一个点，保证单点触碰也算笔迹
      this.ctx.moveTo(t.x, t.y)
      this.ctx.lineTo(t.x + 0.1, t.y + 0.1)
      this.ctx.stroke()
      this.ctx.draw(true)
      this.empty = false
    },
    onTouchMove(e: any) {
      if (this.ctx == null || !this.drawing || this.saving) return
      const t = e.touches[0]
      this.ctx.moveTo(this.lastX, this.lastY)
      this.ctx.lineTo(t.x, t.y)
      this.ctx.stroke()
      this.ctx.draw(true)
      this.lastX = t.x
      this.lastY = t.y
    },
    onTouchEnd() {
      this.drawing = false
    },
    onClear() {
      if (this.ctx == null || this.saving) return
      this.ctx.clearRect(0, 0, this.canvasW || 750, this.canvasH || 400)
      this.ctx.draw()
      this.empty = true
    },
    onCancel() {
      if (this.saving) return
      this.show = false
      this.$emit('cancel')
    },
    onConfirm() {
      if (this.saving) return
      if (this.empty) {
        uni.showToast({ title: '请先手写签名', icon: 'none' })
        return
      }
      this.saving = true
      uni.canvasToTempFilePath(
        {
          canvasId: 'signPad',
          fileType: 'png',
          success: (res: any) => {
            this.$emit('save', res.tempFilePath, this.saveForLater)
          },
          fail: () => {
            this.saving = false
            uni.showToast({ title: '签名生成失败，请重试', icon: 'none' })
          }
        },
        this
      )
    }
  }
}
</script>

<style scoped>
.pad-mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 999;
  align-items: center;
  justify-content: center;
  padding: 48rpx;
}

.pad-panel {
  width: 100%;
  padding: 32rpx;
}

.pad-head {
  margin-bottom: 24rpx;
}

.pad-title {
  font-size: 34rpx;
  font-weight: 600;
}

.pad-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.pad-canvas-wrap {
  position: relative;
  width: 100%;
  height: 400rpx;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 16rpx;
  overflow: hidden;
}

.pad-canvas {
  width: 100%;
  height: 400rpx;
  background-color: #ffffff;
}

.pad-placeholder {
  position: absolute;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  align-items: center;
  justify-content: center;
  pointer-events: none;
}

.pad-placeholder-text {
  font-size: 28rpx;
}

.pad-save {
  flex-direction: row;
  align-items: center;
  margin-top: 24rpx;
  min-height: 64rpx;
}

.pad-checkbox {
  width: 36rpx;
  height: 36rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 8rpx;
  align-items: center;
  justify-content: center;
  margin-right: 16rpx;
}

.pad-check {
  font-size: 26rpx;
  line-height: 36rpx;
}

.pad-save-text {
  font-size: 26rpx;
}

.pad-actions {
  flex-direction: row;
  justify-content: space-between;
  margin-top: 24rpx;
}

.pad-btn {
  width: 30%;
  height: 88rpx;
  border-radius: 16rpx;
  border-width: 1rpx;
  border-style: solid;
  border-color: transparent;
  align-items: center;
  justify-content: center;
}

.pad-btn-primary {
  border-width: 0;
}

.pad-btn-text {
  font-size: 30rpx;
}
</style>
