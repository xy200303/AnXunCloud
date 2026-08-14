<template>
  <div class="layout-wrapper" :class="{ collapsed: appStore.sidebarCollapsed }">
    <aside class="sidebar">
      <div class="logo">
        <img class="logo-icon" :src="brandMark" alt="安巡云" />
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
import SidebarItem from './components/SidebarItem.vue'
import Navbar from './components/Navbar.vue'
import TagsView from './components/TagsView.vue'
import AppMain from './components/AppMain.vue'

// 品牌资源走 public 目录，需带上部署子路径（/admin/）
const brandMark = `${import.meta.env.BASE_URL}brand/anxuncloud-mark-reverse.svg`

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
      width: 34px;
      height: 34px;
      flex-shrink: 0;
    }

    .logo-text {
      color: #f7f3eb;
      font-size: 17px;
      font-weight: 600;
      letter-spacing: 5px;
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
