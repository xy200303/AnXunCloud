<template>
  <!-- 维保登记：照片必传，ledger_fix 可随单补录出厂/最近维保日期，提交后进经理确认 -->
  <el-dialog v-model="visible" title="维保登记" width="560px" :close-on-click-modal="false">
    <el-alert
      v-if="equipment"
      :title="`${equipment.name}（${equipment.code}）`"
      :closable="false"
      class="register-target"
    />
    <el-form ref="formRef" :model="form" :rules="rules" label-width="96px">
      <el-form-item label="维保类型" prop="maintenance_type">
        <el-select v-model="form.maintenance_type" style="width: 100%">
          <el-option v-for="d in maintTypeOptions" :key="d.value" :label="d.label" :value="d.value" />
        </el-select>
      </el-form-item>
      <el-form-item label="标签缺失">
        <el-switch v-model="form.label_missing" active-text="钢印磨损/铭牌缺失" />
        <div class="text-secondary">勾选后日期免填；照片仍必传（拍设备本体作证）；经理确认后该设备退出自动到期判定</div>
      </el-form-item>
      <!-- 台账补录：允许随单提交出厂/最近维保日期，经理确认后一并回写台账 -->
      <template v-if="form.maintenance_type === 'ledger_fix' && !form.label_missing">
        <el-form-item label="出厂日期">
          <el-date-picker v-model="form.manufacture_date" type="date" value-format="YYYY-MM-DD" placeholder="瓶体钢印日期" style="width: 100%" />
        </el-form-item>
        <el-form-item label="最近维保日">
          <el-date-picker v-model="form.last_maintenance_date" type="date" value-format="YYYY-MM-DD" placeholder="维修贴纸日期" style="width: 100%" />
        </el-form-item>
      </template>
      <el-form-item label="维保日期" prop="maintenance_date">
        <el-date-picker v-model="form.maintenance_date" type="date" value-format="YYYY-MM-DD" style="width: 100%" />
      </el-form-item>
      <el-form-item label="维保单位">
        <el-input v-model="form.vendor" placeholder="选填" maxlength="128" />
      </el-form-item>
      <el-form-item label="经办人">
        <el-input v-model="form.operator_name" placeholder="默认当前用户" maxlength="64" />
      </el-form-item>
      <el-form-item label="备注">
        <el-input v-model="form.note" placeholder="选填" maxlength="255" />
      </el-form-item>
      <el-form-item label="标签照片" prop="photos">
        <div class="photo-upload">
          <div v-for="(p, i) in form.photos" :key="p.file_id" class="photo-item">
            <el-image :src="p.url" fit="cover" class="photo-thumb" :preview-src-list="form.photos.map((x) => x.url)" :initial-index="i" preview-teleported />
            <el-button link type="danger" :icon="Delete" @click="form.photos.splice(i, 1)" />
          </div>
          <el-upload
            v-if="form.photos.length < 9"
            :show-file-list="false"
            accept="image/*"
            :http-request="handlePhotoUpload"
          >
            <el-button :icon="Plus" :loading="photoUploading">上传照片</el-button>
          </el-upload>
        </div>
        <div class="text-secondary">必传至少 1 张新维修标签照片；提交后进经理确认，确认后台账才更新</div>
      </el-form-item>
    </el-form>
    <template #footer>
      <el-button @click="visible = false">取消</el-button>
      <el-button type="primary" :loading="registering" @click="handleRegister">提交登记</el-button>
    </template>
  </el-dialog>
</template>

<script setup lang="ts">
import { reactive, ref, watch } from 'vue'
import { ElMessage, type FormInstance, type FormRules, type UploadRequestOptions } from 'element-plus'
import { Plus, Delete } from '@element-plus/icons-vue'
import { registerMaintenance, type EquipmentItem, type MaintenanceType } from '@/api/equipment'
import { uploadImage, withFileToken } from '@/api/upload'
import { useDictOptions } from '@/composables/useDictOptions'

const props = defineProps<{ equipment: EquipmentItem | null }>()
const emit = defineEmits<{ saved: [] }>()
const visible = defineModel<boolean>('visible', { required: true })

const { options: maintTypeOptions } = useDictOptions('equipment_maint_type')

const registering = ref(false)
const photoUploading = ref(false)
const formRef = ref<FormInstance>()

const form = reactive({
  maintenance_type: 'repair' as MaintenanceType,
  label_missing: false,
  maintenance_date: '',
  manufacture_date: '',
  last_maintenance_date: '',
  vendor: '',
  operator_name: '',
  note: '',
  photos: [] as { file_id: string; url: string }[]
})

const rules: FormRules = {
  maintenance_type: [{ required: true, message: '请选择维保类型', trigger: 'change' }],
  maintenance_date: [{ required: true, message: '请选择维保日期', trigger: 'change' }],
  photos: [
    {
      validator: (_r, _v, cb) =>
        form.photos.length === 0 ? cb(new Error('请至少上传 1 张新维修标签照片')) : cb(),
      trigger: 'change'
    }
  ]
}

// 打开时重置表单为默认值（维保日期默认本地今天）
watch(visible, (v) => {
  if (!v) return
  formRef.value?.clearValidate()
  Object.assign(form, {
    maintenance_type: 'repair',
    label_missing: false,
    maintenance_date: new Date().toLocaleDateString('sv-SE'), // 本地今天，YYYY-MM-DD
    manufacture_date: '', last_maintenance_date: '',
    vendor: '', operator_name: '', note: '', photos: []
  })
})

async function handlePhotoUpload(opt: UploadRequestOptions) {
  photoUploading.value = true
  try {
    // 管理端维保登记照片上传（scene=equipment，仅图片）；登记接口校验文件归属本人
    const res = await uploadImage(opt.file, 'equipment')
    form.photos.push({ file_id: res.file_id, url: withFileToken(res.url) })
    formRef.value?.validateField('photos')
  } catch {
    // 拦截器已提示
  } finally {
    photoUploading.value = false
  }
}

async function handleRegister() {
  await formRef.value?.validate()
  if (!props.equipment) return
  registering.value = true
  try {
    await registerMaintenance({
      equipment_id: props.equipment.id,
      maintenance_type: form.maintenance_type,
      maintenance_date: form.maintenance_date || undefined,
      manufacture_date: form.maintenance_type === 'ledger_fix' ? form.manufacture_date || undefined : undefined,
      last_maintenance_date: form.maintenance_type === 'ledger_fix' ? form.last_maintenance_date || undefined : undefined,
      vendor: form.vendor || undefined,
      operator_name: form.operator_name || undefined,
      note: form.note || undefined,
      file_ids: form.photos.map((p) => p.file_id),
      label_missing: form.label_missing || undefined
    })
    ElMessage.success('登记已提交，待经理确认后生效')
    visible.value = false
    emit('saved')
  } finally {
    registering.value = false
  }
}
</script>

<style scoped lang="scss">
.register-target {
  margin-bottom: $spacing-lg;
}

.photo-upload {
  display: flex;
  flex-wrap: wrap;
  gap: $spacing-sm;
  align-items: center;
}

.photo-item {
  display: flex;
  align-items: center;
  gap: 2px;
}

.photo-thumb {
  width: 64px;
  height: 64px;
  border-radius: $radius-small;
}
</style>
