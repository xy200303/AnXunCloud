<template>
  <div v-if="visible" class="tenant-switcher">
    <span class="label">当前公司</span>
    <el-select :model-value="selectValue" size="small" style="width: 200px" @change="handleChange">
      <el-option label="默认租户" :value="DEFAULT_VALUE" />
      <el-option v-for="t in options" :key="t.id" :label="t.name" :value="t.id" />
    </el-select>
  </div>
</template>

<script setup lang="ts">
// 全局租户切换器（顶栏）：仅超管可见；私有化单租户（总数 ≤ 1）自动隐藏
// 切换后整页刷新：上下文已持久化到 localStorage，刷新后全站（系统管理 + 业务模块）请求统一带 X-Tenant-Id 头生效
// el-select 将空串视为未选择并显示 placeholder，默认租户选项用哨兵值 __default__ 占位展示
import { computed, onMounted, ref } from 'vue'
import { listTenants, type TenantItem } from '@/api/tenant'
import { useTenantStore } from '@/store/tenant'
import { useUserStore } from '@/store/user'

const DEFAULT_VALUE = '__default__'

const userStore = useUserStore()
const tenantStore = useTenantStore()

const options = ref<TenantItem[]>([])
const total = ref(0)

const visible = computed(() => userStore.isSuperAdmin && total.value > 1)
// store 空串（默认租户）映射为哨兵值，保证选中态正确显示「默认租户」
const selectValue = computed(() => tenantStore.id || DEFAULT_VALUE)

function handleChange(id: string) {
  if (id === DEFAULT_VALUE) {
    tenantStore.reset()
  } else {
    const target = options.value.find((t) => t.id === id)
    tenantStore.switchTo(id, target?.name || '')
  }
  // 全量刷新，确保各页面/store 数据按新租户上下文重新拉取
  window.location.reload()
}

onMounted(async () => {
  // tenant:list 仅超管有权限，非超管不发起请求
  if (!userStore.isSuperAdmin) return
  try {
    const data = await listTenants({ page: 1, page_size: 100 })
    total.value = data.total
    // 默认租户走「缺省」选项（value 为哨兵值），候选里剔除避免重复
    options.value = data.list.filter((t) => t.code !== 'default')
    // 单租户或缓存的上下文已失效（租户被删）：回落默认租户
    if (data.total <= 1 || (tenantStore.id && !options.value.some((t) => t.id === tenantStore.id))) {
      tenantStore.reset()
    }
  } catch {
    // 拉取失败保持现状，错误提示由请求拦截器统一弹出
  }
})
</script>

<style scoped lang="scss">
.tenant-switcher {
  display: flex;
  align-items: center;
  gap: $spacing-sm;

  .label {
    font-size: $font-size-body;
    color: $color-text-secondary;
    white-space: nowrap;
  }
}
</style>
