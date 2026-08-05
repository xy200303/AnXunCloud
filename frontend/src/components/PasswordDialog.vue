<template>
  <!-- 修改密码对话框：顶栏下拉与个人中心共用；成功后强制重新登录 -->
  <el-dialog
    :model-value="modelValue"
    title="修改密码"
    width="440px"
    :close-on-click-modal="false"
    @update:model-value="emit('update:modelValue', $event)"
    @closed="resetForm"
  >
    <el-form ref="formRef" :model="form" :rules="rules" label-width="88px">
      <el-form-item label="旧密码" prop="old_password">
        <el-input v-model="form.old_password" type="password" show-password placeholder="请输入旧密码" />
      </el-form-item>
      <el-form-item label="新密码" prop="new_password">
        <el-input v-model="form.new_password" type="password" show-password placeholder="8-32 位，须含字母与数字" />
      </el-form-item>
      <el-form-item label="确认密码" prop="confirm_password">
        <el-input v-model="form.confirm_password" type="password" show-password placeholder="请再次输入新密码" />
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="submitting" @click="handleSubmit">保存修改</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref } from 'vue'
import { ElMessage, type FormInstance, type FormRules } from 'element-plus'
import { updatePassword } from '@/api/user'

const props = defineProps<{ modelValue: boolean }>()
const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  success: []
}>()

const formRef = ref<FormInstance>()
const submitting = ref(false)

const form = reactive({
  old_password: '',
  new_password: '',
  confirm_password: ''
})

// 新密码强度：≥8 位且含字母与数字
const validateNewPassword = (_rule: any, value: string, callback: (e?: Error) => void) => {
  if (!/^(?=.*[a-zA-Z])(?=.*\d).{8,32}$/.test(value || '')) {
    callback(new Error('新密码需 8-32 位，且同时包含字母与数字'))
  } else {
    callback()
  }
}

const validateConfirm = (_rule: any, value: string, callback: (e?: Error) => void) => {
  if (value !== form.new_password) {
    callback(new Error('两次输入的密码不一致'))
  } else {
    callback()
  }
}

const rules: FormRules = {
  old_password: [{ required: true, message: '请输入旧密码', trigger: 'blur' }],
  new_password: [
    { required: true, message: '请输入新密码', trigger: 'blur' },
    { validator: validateNewPassword, trigger: 'blur' }
  ],
  confirm_password: [
    { required: true, message: '请再次输入新密码', trigger: 'blur' },
    { validator: validateConfirm, trigger: 'blur' }
  ]
}

async function handleSubmit() {
  await formRef.value?.validate()
  submitting.value = true
  try {
    await updatePassword({ old_password: form.old_password, new_password: form.new_password })
    ElMessage.success('密码已修改，请使用新密码重新登录')
    emit('update:modelValue', false)
    emit('success')
  } finally {
    submitting.value = false
  }
}

function resetForm() {
  form.old_password = ''
  form.new_password = ''
  form.confirm_password = ''
  formRef.value?.clearValidate()
}
</script>
