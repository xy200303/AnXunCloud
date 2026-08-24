<template>
  <!-- ECharts 容器：负责初始化、resize、空态；option 由父组件按图表规范组装 -->
  <div class="echart-wrap" :style="{ height }">
    <div v-show="!isEmpty" ref="el" class="echart-canvas" />
    <el-empty v-if="isEmpty" :description="emptyText" :image-size="72" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import echarts from '@/utils/echarts'
import type { EChartsCoreOption } from 'echarts/core'

const props = withDefaults(
  defineProps<{
    option: EChartsCoreOption | null
    height?: string
    // 无数据标记：由父组件判断（如图表数据为空数组）
    empty?: boolean
    emptyText?: string
  }>(),
  { height: '280px', empty: false, emptyText: '暂无数据' }
)

const el = ref<HTMLElement>()
const isEmpty = computed(() => props.empty || !props.option)

let chart: ReturnType<typeof echarts.init> | null = null
let observer: ResizeObserver | null = null
let renderFrame = 0
let resizeFrame = 0

function render() {
  if (renderFrame) cancelAnimationFrame(renderFrame)
  renderFrame = requestAnimationFrame(() => {
    renderFrame = 0
    if (!el.value || !props.option || isEmpty.value) return
    if (!chart) chart = echarts.init(el.value)
    chart.setOption(props.option, { notMerge: true, lazyUpdate: true })
  })
}

function resize() {
  if (resizeFrame) cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(() => {
    resizeFrame = 0
    chart?.resize()
  })
}

onMounted(() => {
  render()
  // 容器尺寸变化（侧边栏折叠、窗口缩放）时自适应
  observer = new ResizeObserver(resize)
  if (el.value) observer.observe(el.value)
})

watch(() => props.option, render)

onBeforeUnmount(() => {
  observer?.disconnect()
  if (renderFrame) cancelAnimationFrame(renderFrame)
  if (resizeFrame) cancelAnimationFrame(resizeFrame)
  chart?.dispose()
  chart = null
})
</script>

<style scoped lang="scss">
.echart-wrap {
  width: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
}

.echart-canvas {
  width: 100%;
  height: 100%;
}
</style>
