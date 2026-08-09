<template>
  <!-- 按图标名渲染已注册的菜单图标；未知名称不渲染，避免控制台告警 -->
  <component :is="comp" v-if="comp" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { menuIconMap } from '@/utils/menuIcons'

const props = defineProps<{ name?: string }>()

const comp = computed(() => (props.name ? menuIconMap[props.name] : undefined))
</script>
