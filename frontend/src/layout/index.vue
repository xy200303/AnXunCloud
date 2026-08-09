<template>
  <div class="layout-wrapper" :class="{ collapsed: appStore.sidebarCollapsed }">
    <aside class="sidebar">
      <div class="logo">
        <el-icon :size="22" class="logo-icon"><OfficeBuilding /></el-icon>
        <span v-show="!appStore.sidebarCollapsed" class="logo-text">安巡云</span>
      </div>
      <el-scrollbar class="menu-scroll">
        <el-menu
          class="sidebar-menu"
          :collapse="appStore.sidebarCollapsed"
          :collapse-transition="false"
          :default-active="activeMenu"
          :unique-opened="true"
          router
        >
          <sidebar-item v-for="menu in permissionStore.menus" :key="menu.id" :item="menu" />
        </el-menu>
      </el-scrollbar>
    </aside>

    <div class="main-area">
      <navbar />
      <tags-view />
      <app-main />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useAppStore } from '@/store/app'
import { usePermissionStore } from '@/store/permission'
import { OfficeBuilding } from '@element-plus/icons-vue'
import SidebarItem from './components/SidebarItem.vue'
import Navbar from './components/Navbar.vue'
import TagsView from './components/TagsView.vue'
import AppMain from './components/AppMain.vue'

const route = useRoute()
const appStore = useAppStore()
const permissionStore = usePermissionStore()

// 当前项高亮
const activeMenu = computed(() => route.path)
</script>

<style scoped lang="scss">
.layout-wrapper {
  display: flex;
  height: 100%;
  min-width: 1280px;
}

.sidebar {
  width: $sidebar-width;
  flex-shrink: 0;
  background-color: $color-sidebar;
  display: flex;
  flex-direction: column;
  transition: width 0.2s;

  .logo {
    height: $navbar-height;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: $spacing-sm;
    color: $color-white;
    overflow: hidden;
    flex-shrink: 0;

    .logo-icon {
      color: $color-primary-hover;
      flex-shrink: 0;
    }

    .logo-text {
      font-size: $font-size-card-title;
      font-weight: 600;
      white-space: nowrap;
    }
  }

  .menu-scroll {
    flex: 1;
  }
}

.collapsed .sidebar {
  width: $sidebar-width-collapsed;
}

.main-area {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  background-color: $color-bg-page;
}
</style>
