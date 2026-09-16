<template>
  <main class="app-main">
    <router-view v-slot="{ Component }">
      <transition name="fade" mode="out-in">
        <keep-alive :max="10" :include="tagsViewStore.cachedViews">
          <component :is="wrapComponent(Component)" :key="route.path" />
        </keep-alive>
      </transition>
    </router-view>
  </main>
</template>

<script setup lang="ts">
import { defineComponent, h, type Component } from 'vue'
import { useRoute } from 'vue-router'
import { useTagsViewStore } from '@/store/tagsView'

const route = useRoute()
const tagsViewStore = useTagsViewStore()

// keep-alive include 按组件名匹配，而业务页面均为 <script setup> 无显式 name：
// 按路由 name 包一层带 name 的组件，作为 include 匹配与关 tag 驱逐的可靠锚点；
// 无 name 的路由不包装，include 匹配不上即不缓存（安全兜底）
const wrapperCache = new Map<string, Component>()
function wrapComponent(Comp: Component | undefined) {
  const name = typeof route.name === 'string' ? route.name : undefined
  if (!Comp || !name) return Comp
  let wrapper = wrapperCache.get(name)
  if (!wrapper) {
    wrapper = defineComponent({
      name,
      setup: () => () => h(Comp)
    })
    wrapperCache.set(name, wrapper)
  }
  return wrapper
}
</script>

<style scoped lang="scss">
.app-main {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
}

// 页面切换动画：150-300ms，仅 opacity
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
