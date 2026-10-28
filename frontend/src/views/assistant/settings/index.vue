<template>
  <div class="app-container">
    <div class="table-card">
      <el-tabs v-model="tab">
        <el-tab-pane label="工具目录" name="tools">
          <el-table v-loading="loading" :data="tools">
            <el-table-column prop="title" label="工具" min-width="160" />
            <el-table-column prop="name" label="编码" min-width="180" />
            <el-table-column prop="description" label="用途" min-width="300" show-overflow-tooltip />
            <el-table-column label="类型" width="100"><template #default="{ row }"><el-tag :type="row.read_only ? 'success' : 'warning'">{{ row.read_only ? '只读' : '写入' }}</el-tag></template></el-table-column>
          </el-table>
        </el-tab-pane>
        <el-tab-pane label="MCP 接入" name="tokens">
          <div class="table-toolbar"><div class="table-toolbar-left"><el-button v-perms="'assistant:settings'" type="primary" :icon="Plus" @click="issueVisible = true">签发 token</el-button></div><el-button :icon="RefreshRight" circle @click="loadTokens" /></div>
          <el-alert v-if="issued" type="success" :closable="false" title="token 仅显示一次，请立即保存">
            <div class="secret-row"><el-input v-model="issued.token" readonly /><el-button @click="copyToken">复制</el-button><el-button @click="issued = null">完成</el-button></div>
          </el-alert>
          <el-table v-loading="loading" :data="tokens">
            <el-table-column prop="name" label="名称" min-width="140" />
            <el-table-column prop="prefix" label="掩码" min-width="120"><template #default="{ row }">{{ row.prefix }}…</template></el-table-column>
            <el-table-column label="工具范围" min-width="180"><template #default="{ row }">{{ row.scopes.join('、') }}</template></el-table-column>
            <el-table-column prop="expires_at" label="到期时间" width="170"><template #default="{ row }">{{ row.expires_at || '长期' }}</template></el-table-column>
            <el-table-column prop="last_used_at" label="最近使用" width="170"><template #default="{ row }">{{ row.last_used_at || '未使用' }}</template></el-table-column>
            <el-table-column label="状态" width="90"><template #default="{ row }"><el-tag :type="row.revoked_at ? 'info' : 'success'">{{ row.revoked_at ? '已撤销' : '有效' }}</el-tag></template></el-table-column>
            <el-table-column label="操作" width="100" fixed="right"><template #default="{ row }"><el-button v-if="!row.revoked_at" v-perms="'assistant:settings'" link type="danger" @click="revoke(row)">撤销</el-button></template></el-table-column>
            <template #empty><el-empty description="尚未签发 MCP token" /></template>
          </el-table>
        </el-tab-pane>
      </el-tabs>
    </div>
    <el-dialog v-model="issueVisible" title="签发 MCP token" width="500px" :close-on-click-modal="false">
      <el-form :model="form" label-width="90px">
        <el-form-item label="名称" required><el-input v-model="form.name" maxlength="128" /></el-form-item>
        <el-form-item label="工具范围" required><el-checkbox-group v-model="form.scopes"><el-checkbox v-for="tool in tools" :key="tool.name" :value="tool.name">{{ tool.title }}</el-checkbox></el-checkbox-group></el-form-item>
        <el-form-item label="有效天数" required><el-input-number v-model="form.days" :min="1" :max="365" controls-position="right" /></el-form-item>
      </el-form>
      <template #footer><el-button @click="issueVisible = false">取消</el-button><el-button type="primary" :loading="issuing" :disabled="!form.name || !form.scopes.length" @click="issue">签发</el-button></template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Plus, RefreshRight } from '@element-plus/icons-vue'
import { listMCPTools, listMCPTokens, issueMCPToken, revokeMCPToken, type MCPTool, type MCPToken, type MCPTokenIssue } from '@/api/mcp'

const tab = ref('tools')
const tools = ref<MCPTool[]>([])
const tokens = ref<MCPToken[]>([])
const issued = ref<MCPTokenIssue | null>(null)
const loading = ref(false)
const issueVisible = ref(false)
const issuing = ref(false)
const form = ref({ name: '', scopes: [] as string[], days: 30 })
async function loadTokens() {
  loading.value = true
  try { tokens.value = await listMCPTokens() } finally { loading.value = false }
}
async function issue() {
  issuing.value = true
  try {
    issued.value = await issueMCPToken(form.value)
    issueVisible.value = false
    form.value.name = ''
    await loadTokens()
    tab.value = 'tokens'
  } finally { issuing.value = false }
}
async function revoke(token: MCPToken) {
  await ElMessageBox.confirm(`撤销「${token.name}」后，使用该 token 的调用会立即失败。`, '撤销 token', { type: 'warning' })
  await revokeMCPToken(token.id)
  ElMessage.success('token 已撤销')
  await loadTokens()
}
async function copyToken() {
  if (!issued.value) return
  await navigator.clipboard.writeText(issued.value.token)
  ElMessage.success('已复制')
}
onMounted(async () => {
  ;[tools.value, tokens.value] = await Promise.all([listMCPTools(), listMCPTokens()])
})
</script>

<style scoped>
.secret-row { display: flex; gap: 8px; margin-top: 12px; }
</style>
