<template>
  <div class="assistant-layout">
    <aside class="conversation-panel">
      <el-button type="primary" :icon="Plus" @click="newConversation">新建会话</el-button>
      <div v-loading="loadingConversations" class="conversation-list">
        <button v-for="item in conversations" :key="item.id" class="conversation-item" :class="{ active: item.id === conversationId }" @click="selectConversation(item.id)">
          <span>{{ item.title }}</span>
          <time>{{ item.updated_at }}</time>
        </button>
        <el-empty v-if="!conversations.length" description="暂无会话" :image-size="70" />
      </div>
    </aside>
    <main class="chat-panel">
      <header class="chat-header">
        <div class="chat-title"><strong>AI 助手</strong><el-radio-group v-model="mode" @change="changeMode"><el-radio-button value="assistant">对话</el-radio-button><el-radio-button value="agent">Agent</el-radio-button></el-radio-group></div>
        <div class="chat-actions"><el-button v-if="mode === 'agent' && conversationId" @click="auditVisible = true">调用记录（{{ agentCalls.length }}）</el-button><el-tag :type="status.available ? 'success' : 'info'">{{ status.available ? '已连接' : '未配置' }}</el-tag></div>
      </header>
      <el-alert v-if="!status.available" type="warning" :closable="false" :title="status.reason">
        <el-button link type="primary" @click="router.push('/system/ai-config')">前往 AI 配置</el-button>
      </el-alert>
      <div ref="messageContainer" class="message-container">
        <el-empty v-if="!visibleMessages.length" description="输入问题开始对话" />
        <div v-for="message in visibleMessages" :key="message.id" class="message-row" :class="message.role">
          <div class="message-bubble" :class="{ failed: message.status === 'failed' }">
            <template v-if="message.status === 'failed'">{{ message.error_message || '调用失败' }}</template>
            <template v-else>{{ message.content }}</template>
          </div>
          <div v-if="message.status === 'ready' && message.role === 'assistant'" class="message-meta">{{ message.provider_code }} / {{ message.model_id }} · {{ message.prompt_tokens + message.completion_tokens }} tokens</div>
        </div>
        <div v-if="sending" class="message-row assistant"><div class="message-bubble">正在等待模型响应…</div></div>
      </div>
      <div class="chat-input">
        <el-input v-model="draft" type="textarea" :rows="3" maxlength="4000" placeholder="输入消息，Ctrl + Enter 发送" :disabled="!status.available || sending" @keydown.ctrl.enter.prevent="send" />
        <div class="input-actions"><el-button type="primary" :loading="sending" :disabled="!status.available || !draft.trim()" @click="send">发送</el-button></div>
      </div>
    </main>
    <el-drawer v-model="auditVisible" title="Agent 工具调用记录" size="480px">
      <el-empty v-if="!agentCalls.length" description="本次会话尚未调用工具" />
      <el-timeline v-else>
        <el-timeline-item v-for="call in agentCalls" :key="call.id" :timestamp="call.created_at" :type="call.status === 'success' ? 'success' : 'danger'">
          <div class="audit-title"><strong>{{ call.tool_code }}</strong><el-tag :type="call.status === 'success' ? 'success' : 'danger'">{{ call.status === 'success' ? '成功' : '失败' }}</el-tag></div>
          <el-input :model-value="formatAudit(call.arguments)" type="textarea" :rows="4" readonly class="audit-data" />
          <el-alert v-if="call.error_message" :title="call.error_message" type="error" :closable="false" />
          <el-input v-else :model-value="formatAudit(call.result)" type="textarea" :rows="6" readonly class="audit-data" />
        </el-timeline-item>
      </el-timeline>
    </el-drawer>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { Plus } from '@element-plus/icons-vue'
import { getAssistantStatus, listAssistantConversations, listAssistantMessages, sendAssistantMessage, type AssistantConversation, type AssistantMessage } from '@/api/assistant'
import { getAgentStatus, listAgentCalls, sendAgentMessage, type AgentCall } from '@/api/agent'

const router = useRouter()
const mode = ref<'assistant' | 'agent'>('assistant')
const assistantStatus = ref({ available: false, reason: '' })
const agentStatus = ref({ available: false, reason: '' })
const status = computed(() => mode.value === 'agent' ? agentStatus.value : assistantStatus.value)
const conversations = ref<AssistantConversation[]>([])
const messages = ref<AssistantMessage[]>([])
const visibleMessages = computed(() => messages.value.filter((message) => message.content || message.status === 'failed'))
const agentCalls = ref<AgentCall[]>([])
const auditVisible = ref(false)
const conversationId = ref('')
const draft = ref('')
const sending = ref(false)
const loadingConversations = ref(false)
const messageContainer = ref<HTMLElement>()
async function scrollBottom() {
  await nextTick()
  if (messageContainer.value) messageContainer.value.scrollTop = messageContainer.value.scrollHeight
}
async function loadConversations() {
  loadingConversations.value = true
  try { conversations.value = await listAssistantConversations() } finally { loadingConversations.value = false }
}
async function selectConversation(id: string) {
  conversationId.value = id
  messages.value = await listAssistantMessages(id)
  agentCalls.value = mode.value === 'agent' ? await listAgentCalls(id) : []
  scrollBottom()
}
function newConversation() {
  conversationId.value = ''
  messages.value = []
  agentCalls.value = []
  auditVisible.value = false
  draft.value = ''
}
function changeMode() {
  newConversation()
}
async function send() {
  const content = draft.value.trim()
  if (!content || sending.value || !status.value.available) return
  sending.value = true
  try {
    const sendMessage = mode.value === 'agent' ? sendAgentMessage : sendAssistantMessage
    const result = await sendMessage(conversationId.value, content)
    conversationId.value = result.conversation_id
    messages.value = await listAssistantMessages(conversationId.value)
    if (mode.value === 'agent') agentCalls.value = await listAgentCalls(conversationId.value)
    draft.value = ''
    await loadConversations()
    await scrollBottom()
  } finally {
    sending.value = false
  }
}
function formatAudit(value: Record<string, unknown>) {
  return JSON.stringify(value, null, 2)
}
onMounted(async () => {
  ;[assistantStatus.value, agentStatus.value, conversations.value] = await Promise.all([getAssistantStatus(), getAgentStatus(), listAssistantConversations()])
  if (conversations.value.length) await selectConversation(conversations.value[0].id)
})
</script>

<style scoped>
.assistant-layout { display: flex; gap: 16px; height: calc(100vh - 154px); min-height: 560px; }
.conversation-panel { width: 250px; padding: 16px; background: #fff; border-radius: 8px; }
.conversation-panel > .el-button { width: 100%; margin-bottom: 16px; }
.conversation-list { height: calc(100% - 52px); overflow-y: auto; }
.conversation-item { display: flex; flex-direction: column; gap: 6px; width: 100%; padding: 12px; margin-bottom: 6px; text-align: left; border: 0; border-radius: 6px; background: transparent; cursor: pointer; }
.conversation-item:hover, .conversation-item.active { background: #eef3ff; color: #2b5aed; }
.conversation-item span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.conversation-item time { font-size: 12px; color: #909399; }
.chat-panel { display: flex; flex: 1; flex-direction: column; min-width: 0; background: #fff; border-radius: 8px; overflow: hidden; }
.chat-header { display: flex; align-items: center; justify-content: space-between; padding: 16px 20px; border-bottom: 1px solid #e4e7ed; }
.chat-header strong { font-size: 18px; }
.chat-title, .chat-actions, .audit-title { display: flex; align-items: center; gap: 16px; }
.audit-title { justify-content: space-between; margin-bottom: 12px; }
.audit-data { margin-bottom: 12px; }
.message-container { flex: 1; padding: 24px; overflow-y: auto; background: #f7f8fa; }
.message-row { display: flex; flex-direction: column; align-items: flex-start; margin-bottom: 18px; }
.message-row.user { align-items: flex-end; }
.message-bubble { max-width: 76%; padding: 12px 16px; border: 1px solid #e4e7ed; border-radius: 8px; background: #fff; line-height: 1.7; white-space: pre-wrap; overflow-wrap: anywhere; }
.user .message-bubble { border-color: #2b5aed; background: #2b5aed; color: #fff; }
.message-bubble.failed { border-color: #f56c6c; color: #f56c6c; }
.message-meta { margin-top: 6px; font-size: 12px; color: #909399; }
.chat-input { padding: 16px 20px; border-top: 1px solid #e4e7ed; }
.input-actions { display: flex; justify-content: flex-end; margin-top: 12px; }
</style>
