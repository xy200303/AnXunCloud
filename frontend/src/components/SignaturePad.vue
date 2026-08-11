<template>
  <!-- 手写签名板：鼠标/触屏手写，输出白底 PNG（月报签字栏用） -->
  <el-dialog
    v-model="visible"
    title="手写签名"
    width="560px"
    :close-on-click-modal="false"
    @closed="handleClosed"
  >
    <div class="pad-wrap">
      <canvas ref="canvasRef" class="pad-canvas" />
      <div v-if="empty" class="pad-placeholder">请在此区域手写签名</div>
    </div>
    <div class="pad-tip text-secondary">支持鼠标与触屏手写；保存后为白底 PNG，将显示在月度报告签字栏</div>
    <el-checkbox v-if="showSaveOption" v-model="saveForLater" class="pad-save-option">
      保存为我的手写签名，下次签字直接使用
    </el-checkbox>
    <template #footer>
      <el-button :disabled="empty" @click="handleClear">清除重写</el-button>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :disabled="empty" :loading="saving" @click="handleSave">保存签名</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { nextTick, ref, watch } from 'vue'
import { ElMessage } from 'element-plus'
import SignaturePad from 'signature_pad'

// showSaveOption：签字流程内弹出时显示「保存下次使用」勾选；个人中心配置入口不需要（必保存）
withDefaults(defineProps<{ showSaveOption?: boolean }>(), { showSaveOption: false })

const emit = defineEmits<{ (e: 'save', file: File, saveForLater: boolean): Promise<boolean> | boolean }>()

const visible = ref(false)
const saving = ref(false)
const empty = ref(true)
const saveForLater = ref(true)
const canvasRef = ref<HTMLCanvasElement>()
let pad: SignaturePad | null = null

function open() {
  visible.value = true
}

// 弹窗动画结束后初始化画布（否则 canvas 尺寸为 0）
watch(visible, async (v) => {
  if (!v) return
  await nextTick()
  requestAnimationFrame(setupPad)
})

function setupPad() {
  const canvas = canvasRef.value
  if (!canvas) return
  // 按容器实际尺寸 + devicePixelRatio 设置画布，保证笔迹清晰不模糊
  const ratio = Math.max(window.devicePixelRatio || 1, 1)
  const rect = canvas.getBoundingClientRect()
  canvas.width = rect.width * ratio
  canvas.height = rect.height * ratio
  canvas.getContext('2d')?.scale(ratio, ratio)
  pad = new SignaturePad(canvas, {
    penColor: 'rgb(20, 20, 20)',
    backgroundColor: 'rgb(255, 255, 255)',
    minWidth: 1.2,
    maxWidth: 3
  })
  pad.addEventListener('endStroke', () => {
    empty.value = pad?.isEmpty() ?? true
  })
  empty.value = true
}

function handleClear() {
  pad?.clear()
  empty.value = true
}

async function handleSave() {
  if (!pad || pad.isEmpty()) return
  saving.value = true
  try {
    const blob = await new Promise<Blob | null>((resolve) => canvasRef.value?.toBlob(resolve, 'image/png'))
    if (!blob) {
      ElMessage.warning('签名生成失败，请重试')
      return
    }
    const file = new File([blob], `signature-${Date.now()}.png`, { type: 'image/png' })
    const ok = await emit('save', file, saveForLater.value)
    if (ok !== false) visible.value = false
  } finally {
    saving.value = false
  }
}

function handleClosed() {
  pad?.off()
  pad = null
  empty.value = true
  saveForLater.value = true
}

defineExpose({ open })
</script>

<style scoped lang="scss">
.pad-wrap {
  position: relative;
  width: 100%;
  height: 220px;
  background: $color-white;
  border: 1px solid $color-border;
  border-radius: $radius-card;
  overflow: hidden;

  .pad-canvas {
    width: 100%;
    height: 100%;
    display: block;
    touch-action: none; // 触屏手写时禁止滚动
    cursor: crosshair;
  }

  .pad-placeholder {
    position: absolute;
    inset: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    color: $color-text-secondary;
    font-size: $font-size-body;
    pointer-events: none;
  }
}

.pad-tip {
  margin-top: $spacing-sm;
  font-size: $font-size-aux;
}

.pad-save-option {
  margin-top: $spacing-sm;
}
</style>
