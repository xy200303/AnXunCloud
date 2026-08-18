<template>
  <div class="app-container">
    <el-alert
      type="warning"
      :closable="false"
      show-icon
      title="模板仅用于开通租户时复制，修改不影响已开通租户"
      style="margin-bottom: 16px"
    />
    <PostManager perm-prefix="platform:post" is-template :api="postTemplateApi" />
  </div>
</template>

<script setup lang="ts">
// 岗位模板库（平台管理 /platform/post-templates，仅超管）：平台模板行，仅作开通租户的初始拷贝源
import PostManager from '@/components/PostManager.vue'
import {
  listPostTemplates, createPostTemplate, updatePostTemplate, deletePostTemplate,
  listPostTemplateDutyBindings, savePostTemplateDutyBindings
} from '@/api/post'

const postTemplateApi = {
  list: listPostTemplates,
  create: createPostTemplate,
  update: updatePostTemplate,
  remove: deletePostTemplate,
  listDuty: listPostTemplateDutyBindings,
  saveDuty: savePostTemplateDutyBindings
}
</script>
