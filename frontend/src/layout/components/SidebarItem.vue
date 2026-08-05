<template>
  <!-- 递归渲染侧边栏菜单：目录用 sub-menu，菜单项用 menu-item，图标+文字 -->
  <el-sub-menu v-if="item.type === 'dir' && visibleChildren.length" :index="item.path">
    <template #title>
      <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
      <span>{{ item.title }}</span>
    </template>
    <sidebar-item v-for="child in visibleChildren" :key="child.id" :item="child" />
  </el-sub-menu>

  <el-menu-item v-else-if="item.type === 'menu'" :index="item.path">
    <el-icon v-if="item.icon"><component :is="item.icon" /></el-icon>
    <template #title>{{ item.title }}</template>
  </el-menu-item>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { RouteMenu } from '@/api/types'

defineOptions({ name: 'SidebarItem' })

const props = defineProps<{ item: RouteMenu }>()

// 目录下无可见子菜单时不渲染
const visibleChildren = computed(() =>
  (props.item.children || []).filter((c) => c.type === 'dir' || c.type === 'menu')
)
</script>
