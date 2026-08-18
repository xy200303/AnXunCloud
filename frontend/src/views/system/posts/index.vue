<template>
  <div class="app-container">
    <PostManager perm-prefix="system:post" :api="postApi" />
  </div>
</template>

<script setup lang="ts">
// 岗位管理（系统管理 /system/posts）：作用于当前租户上下文，租户上下文由请求拦截器统一携带
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
</script>
