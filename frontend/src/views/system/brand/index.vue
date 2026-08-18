<template>
  <div class="app-container">

    <div class="table-card">
      <div class="card-header">
        <span class="card-title">企业品牌</span>
      </div>

      <el-alert
        title="留空保存即清除该项覆盖，回落平台默认值"
        type="info"
        :closable="false"
        show-icon
        style="margin-top: 16px"
      />

      <el-form v-loading="loading" label-width="110px" style="max-width: 640px; margin-top: 16px">
        <el-form-item v-for="item in items" :key="item.key" :label="labelOf(item.key)">
          <div v-if="item.key === 'site.theme_color'" class="theme-field">
            <el-color-picker v-model="form[item.key]" />
            <el-input v-model="form[item.key]" style="width: 140px" :placeholder="platformPlaceholder(item)" clearable />
            <span class="field-remark">{{ platformTip(item) }}</span>
          </div>
          <el-input
            v-else-if="item.key === 'site.slogan' || item.key === 'site.footer_note'"
            v-model="form[item.key]"
            type="textarea"
            :rows="2"
            :placeholder="platformPlaceholder(item)"
            clearable
          />
          <el-input v-else v-model="form[item.key]" :placeholder="platformPlaceholder(item)" clearable />
        </el-form-item>
        <el-form-item>
          <el-button v-perms="'tenant:config'" type="primary" :loading="saving" @click="handleSave">保存配置</el-button>
          <span class="field-remark" style="margin-left: 12px">保存后该租户的品牌展示立即生效</span>
        </el-form-item>
      </el-form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getTenantConfig, saveTenantConfig, type TenantConfigItem } from '@/api/tenant'


// 白名单 key 中文名（与后端 tenantConfigWhitelist 对齐）
const KEY_LABELS: Record<string, string> = {
  'site.company_name': '公司名称',
  'site.slogan': '标语',
  'site.theme_color': '主题色',
  'site.footer_note': '页脚文案',
  'site.contact_phone': '联系电话',
  'site.contact_email': '联系邮箱'
}

function labelOf(key: string) {
  return KEY_LABELS[key] || key
}

const items = ref<TenantConfigItem[]>([])
const form = reactive<Record<string, string>>({})
const loading = ref(false)
const saving = ref(false)

async function fetchConfig() {
  loading.value = true
  try {
    items.value = (await getTenantConfig()) || []
    for (const item of items.value) form[item.key] = item.value
  } finally {
    loading.value = false
  }
}

// 平台默认值占位提示（空 = 平台未配置）
function platformPlaceholder(item: TenantConfigItem) {
  return item.platform ? `平台默认：${item.platform}` : '平台默认未配置，留空不覆盖'
}

function platformTip(item: TenantConfigItem) {
  return item.platform ? `平台默认 ${item.platform}` : '平台默认未配置'
}

async function handleSave() {
  saving.value = true
  try {
    const values: Record<string, string> = {}
    for (const item of items.value) values[item.key] = (form[item.key] || '').trim()
    await saveTenantConfig(values)
    ElMessage.success('已保存')
    fetchConfig()
  } finally {
    saving.value = false
  }
}

onMounted(fetchConfig)

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
