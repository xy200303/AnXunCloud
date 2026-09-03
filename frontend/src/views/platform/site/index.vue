<template>
  <div class="app-container">
    <!-- 页面配置 -->
    <div class="table-card">
      <div class="card-header">
        <span class="card-title">页面配置</span>
        <div class="card-header-extra">
          <el-button text type="primary" :icon="View" @click="openSite('/')">查看官网首页</el-button>
          <el-button text type="primary" :icon="Download" @click="openSite('/download')">查看下载页</el-button>
        </div>
      </div>
      <el-form v-loading="configLoading" :model="configForm" label-width="110px" style="max-width: 640px; margin-top: 16px">
        <el-form-item v-for="item in configItems" :key="item.key" :label="item.name">
          <div v-if="item.key === 'site.theme_color'" class="theme-field">
            <el-color-picker v-model="configForm[item.key]" />
            <el-input v-model="configForm[item.key]" style="width: 140px" placeholder="#2b5aed" />
            <span class="field-remark">{{ item.remark }}</span>
          </div>
          <el-input
            v-else-if="item.key === 'site.slogan' || item.key === 'site.footer_note' || item.key === 'site.address'"
            v-model="configForm[item.key]"
            type="textarea"
            :rows="2"
            :placeholder="item.remark"
          />
          <el-input v-else v-model="configForm[item.key]" :placeholder="item.remark" />
        </el-form-item>
        <el-form-item>
          <el-button v-perms="'system:site:update'" type="primary" :loading="saving" @click="handleSaveConfig">保存配置</el-button>
          <span class="field-remark" style="margin-left: 12px">保存后官网立即生效</span>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { View, Download } from '@element-plus/icons-vue'
import { getSiteConfig, saveSiteConfig, type SiteConfigItem } from '@/api/site'

// ========== 页面配置 ==========
// 发布物管理已拆至「平台管理 → 应用发布」（/platform/releases）
const configItems = ref<SiteConfigItem[]>([])
const configForm = reactive<Record<string, string>>({})
const configLoading = ref(false)
const saving = ref(false)

async function fetchConfig() {
  configLoading.value = true
  try {
    const list = await getSiteConfig()
    configItems.value = list || []
    for (const item of configItems.value) configForm[item.key] = item.value
  } finally {
    configLoading.value = false
  }
}

async function handleSaveConfig() {
  saving.value = true
  try {
    await saveSiteConfig({ ...configForm })
    ElMessage.success('已保存，官网约 1 分钟内生效')
  } finally {
    saving.value = false
  }
}

function openSite(path: string) {
  window.open(path, '_blank')
}

onMounted(() => {
  fetchConfig()
})
</script>

<style scoped lang="scss">
.card-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
}
.card-title {
  font-size: $font-size-card-title;
  font-weight: 600;
  color: $color-text-primary;
}
.theme-field {
  display: flex;
  align-items: center;
  gap: 12px;
}
.field-remark {
  font-size: $font-size-aux;
  color: $color-text-secondary;
}
</style>
