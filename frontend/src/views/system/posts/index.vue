<template>
  <div class="app-container">
    <PostManager ref="managerRef" perm-prefix="system:post" :api="postApi" />
  </div>
</template>

<script setup lang="ts">
// 岗位管理（系统管理 /system/posts）：作用于当前租户上下文，租户上下文由请求拦截器统一携带
import { onActivated, ref } from 'vue'
import PostManager from '@/components/PostManager.vue'
import {
  listPosts, createPost, updatePost, deletePost,
  listPostDutyBindings, savePostDutyBindings
} from '@/api/post'

const postApi = {
  list: listPosts,
  create: createPost,
  update: updatePost,
  remove: deletePost,
  listDuty: listPostDutyBindings,
  saveDuty: savePostDutyBindings
}

// keep-alive 缓存页：再次激活（非首次）时刷新岗位/职责数据（onActivated 不穿透子组件，经 expose 转发）
const managerRef = ref<{ reload: () => void } | null>(null)
let activated = false
onActivated(() => {
  if (!activated) {
    activated = true
    return
  }
  managerRef.value?.reload()
})
</script>
