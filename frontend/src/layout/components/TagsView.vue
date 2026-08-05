<template>
  <div class="tags-view">
    <el-scrollbar>
      <div class="tags-list">
        <router-link
          v-for="tag in tagsViewStore.tags"
          :key="tag.path"
          :to="tag.path"
          class="tag-item"
          :class="{ active: route.path === tag.path }"
          @contextmenu.prevent="openContextMenu($event, tag.path)"
        >
          <span class="tag-dot" />
          {{ tag.title }}
          <el-icon
            v-if="!tag.affix"
            class="tag-close"
            :size="12"
            @click.prevent.stop="closeTag(tag.path)"
          >
            <Close />
          </el-icon>
        </router-link>
      </div>
    </el-scrollbar>

    <!-- 右键菜单：关闭其他 / 关闭全部 -->
    <teleport to="body">
      <ul
        v-show="contextMenu.visible"
        class="context-menu"
        :style="{ left: contextMenu.x + 'px', top: contextMenu.y + 'px' }"
      >
        <li @click="closeOthers">关闭其他</li>
        <li @click="closeAll">关闭全部</li>
      </ul>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useTagsViewStore } from '@/store/tagsView'

const route = useRoute()
const router = useRouter()
const tagsViewStore = useTagsViewStore()

const contextMenu = reactive({
  visible: false,
  x: 0,
  y: 0,
  targetPath: ''
})

function closeTag(path: string) {
  const nextPath = tagsViewStore.closeTag(path)
  // 关闭当前页时跳到相邻标签
  if (route.path === path) {
    router.push(nextPath)
  }
}

function openContextMenu(e: MouseEvent, path: string) {
  contextMenu.visible = true
  contextMenu.x = e.clientX
  contextMenu.y = e.clientY
  contextMenu.targetPath = path
}

function closeOthers() {
  tagsViewStore.closeOthers(contextMenu.targetPath)
  if (route.path !== contextMenu.targetPath) {
    router.push(contextMenu.targetPath)
  }
  hideContextMenu()
}

function closeAll() {
  tagsViewStore.closeAll()
  router.push('/dashboard')
  hideContextMenu()
}

function hideContextMenu() {
  contextMenu.visible = false
}

onMounted(() => document.addEventListener('click', hideContextMenu))
onUnmounted(() => document.removeEventListener('click', hideContextMenu))
</script>

<style scoped lang="scss">
.tags-view {
  height: $tags-view-height;
  background: $color-bg-card;
  border-bottom: 1px solid $color-border;
  display: flex;
  align-items: center;
  padding: 0 $spacing-lg;
  flex-shrink: 0;
}

.tags-list {
  display: flex;
  align-items: center;
  gap: $spacing-sm;
  height: $tags-view-height;
}

.tag-item {
  display: inline-flex;
  align-items: center;
  gap: $spacing-xs;
  height: 26px;
  padding: 0 $spacing-md;
  border: 1px solid $color-border;
  border-radius: $radius-small;
  font-size: $font-size-aux;
  color: $color-text-regular;
  white-space: nowrap;
  transition: color 0.2s, border-color 0.2s, background-color 0.2s;

  .tag-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: $color-border-dark;
  }

  &:hover {
    color: $color-primary;
    border-color: $color-primary-hover;
  }

  &.active {
    color: $color-primary;
    border-color: $color-primary;
    background: $color-primary-light;

    .tag-dot {
      background: $color-primary;
    }
  }

  .tag-close {
    border-radius: 50%;

    &:hover {
      background: $color-primary;
      color: $color-white;
    }
  }
}

.context-menu {
  position: fixed;
  z-index: 1000;
  margin: 0;
  padding: $spacing-xs 0;
  list-style: none;
  background: $color-bg-card;
  border-radius: $radius-small;
  box-shadow: $shadow-popup;
  font-size: $font-size-aux;

  li {
    padding: $spacing-sm $spacing-lg;
    cursor: pointer;
    color: $color-text-regular;

    &:hover {
      background: $color-primary-light;
      color: $color-primary;
    }
  }
}
</style>
