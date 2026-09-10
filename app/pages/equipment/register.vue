<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <view v-else class="content">
      <!-- 设备信息回显 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="eq-name" :style="{ color: colors.textPrimary }">{{ eqName }}</text>
        <text class="eq-sub" :style="{ color: colors.textSecondary }">编号：{{ eqCode }}</text>
        <text v-if="detail != null && detail.next_due_date != ''" class="eq-sub" :style="{ color: colors.textSecondary }">
          当前到期日：{{ detail.next_due_date }}
        </text>
      </view>

      <!-- 登记表单 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">维保登记</text>

        <text class="field-label" :style="{ color: colors.textRegular }">维保类型</text>
        <picker mode="selector" :range="typeNames" :value="typeIndex" @change="onTypePick">
          <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
            <text class="picker-text" :style="{ color: colors.textPrimary }">{{ typeNames[typeIndex] || '请选择' }}</text>
            <text class="picker-arrow" :style="{ color: colors.textSecondary }">></text>
          </view>
        </picker>

        <view class="label-missing-row" @click="toggleLabelMissing">
          <text class="field-label" :style="{ color: labelMissing ? colors.danger : colors.textRegular }">{{ labelMissing ? '✓ 标签缺失/无法辨认（钢印磨损）' : '标签缺失/无法辨认（钢印磨损）' }}</text>
        </view>
        <text v-if="labelMissing" class="missing-tip" :style="{ color: colors.warning }">日期免填；照片请拍设备本体作证；经理确认后该设备退出自动到期判定</text>

        <!-- 台账补录：设备缺日期时才显示；确认后一并回写台账 -->
        <block v-if="maintenanceType == 'ledger_fix' && !labelMissing">
          <text class="field-label" :style="{ color: colors.textRegular }">出厂日期（瓶体钢印）</text>
          <picker mode="date" :value="manufactureDate" :end="today" @change="onManufacturePick">
            <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
              <text class="picker-text" :style="{ color: manufactureDate != '' ? colors.textPrimary : colors.textSecondary }">{{ manufactureDate != '' ? manufactureDate : '选择出厂日期' }}</text>
              <text class="picker-arrow" :style="{ color: colors.textSecondary }">></text>
            </view>
          </picker>
          <text class="field-label" :style="{ color: colors.textRegular }">维修日期（贴纸）</text>
          <picker mode="date" :value="lastMaintenanceDate" :end="today" @change="onLastMaintPick">
            <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
              <text class="picker-text" :style="{ color: lastMaintenanceDate != '' ? colors.textPrimary : colors.textSecondary }">{{ lastMaintenanceDate != '' ? lastMaintenanceDate : '选择维修日期' }}</text>
              <text class="picker-arrow" :style="{ color: colors.textSecondary }">></text>
            </view>
          </picker>
        </block>

        <text class="field-label" :style="{ color: colors.textRegular }">维保日期</text>
        <picker mode="date" :value="maintenanceDate" :end="today" @change="onDatePick">
          <view class="picker-box" hover-class="hover-dim" :style="{ borderColor: colors.border }">
            <text class="picker-text" :style="{ color: colors.textPrimary }">{{ maintenanceDate }}</text>
            <text class="picker-arrow" :style="{ color: colors.textSecondary }">></text>
          </view>
        </picker>

        <text class="field-label" :style="{ color: colors.textRegular }">维保单位（选填）</text>
        <input v-model="vendor" class="text-input" :style="{ borderColor: colors.border, color: colors.textPrimary }" placeholder="维保单位名称" :maxlength="64" />

        <text class="field-label" :style="{ color: colors.textRegular }">备注（选填）</text>
        <textarea v-model="note" class="note-input" :style="{ borderColor: colors.border, color: colors.textPrimary }" placeholder="补充说明" :maxlength="200" />
      </view>

      <!-- 标签照片（必传） -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">新维修标签照片</text>
        <text class="photo-tip" :style="{ color: colors.textSecondary }">{{ labelMissing ? '标签缺失：拍设备本体作证，至少 1 张' : '拍换粉/维修后的新标签，至少 1 张' }}；提交后经理确认才生效</text>
        <view class="photos">
          <image
            v-for="(p, pi) in photos"
            :key="pi"
            class="photo"
            :src="p"
            mode="aspectFill"
            @click="preview(pi)"
            @longpress="removePhoto(pi)"
          />
          <view v-if="photos.length < 9" class="photo-add" :style="{ borderColor: colors.border }" @click="takePhoto">
            <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
          </view>
        </view>
      </view>

      <!-- 提交 -->
      <view class="btn-primary" :style="{ backgroundColor: submitting ? colors.info : colors.primary }" @click="submit">
        <text class="btn-primary-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : '提交登记' }}</text>
      </view>
      <view class="bottom-space"></view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiEquipmentDetail, apiEquipmentRegister, apiDictOptions, apiUploadLocal, EquipmentDetail, DictOption } from '@/services/api'
import { compressForUpload } from '@/utils/image'

type RegisterData = {
  colors: ColorTokens
  equipmentId: string
  detail: EquipmentDetail | null
  /** 详情拉取失败时的回显兜底（路由参数带入） */
  eqName: string
  eqCode: string
  loading: boolean
  typeOptions: DictOption[]
  maintenanceType: string
  /** 标签缺失/无法辨认：日期免填，确认后设备退出自动判定 */
  labelMissing: boolean
  maintenanceDate: string
  manufactureDate: string
  lastMaintenanceDate: string
  vendor: string
  note: string
  /** 已上传照片的服务端 URL 列表（与 fileIds 一一对应） */
  photos: string[]
  fileIds: string[]
  submitting: boolean
  uploading: boolean
  today: string
}

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n
}

function todayStr(): string {
  const d = new Date()
  return d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate())
}

export default {
  data(): RegisterData {
    return {
      colors: Colors,
      equipmentId: '',
      detail: null,
      eqName: '',
      eqCode: '',
      loading: true,
      typeOptions: [] as DictOption[],
      maintenanceType: 'repair',
      labelMissing: false,
      maintenanceDate: todayStr(),
      manufactureDate: '',
      lastMaintenanceDate: '',
      vendor: '',
      note: '',
      photos: [] as string[],
      fileIds: [] as string[],
      submitting: false,
      uploading: false,
      today: todayStr()
    }
  },
  computed: {
    /** 维保类型选项：台账补录（ledger_fix）仅设备缺日期时显示 */
    visibleTypes(): DictOption[] {
      const d = this.detail
      const lackDates = d == null || d.manufacture_date == '' || d.last_maintenance_date == ''
      if (lackDates) return this.typeOptions
      return this.typeOptions.filter((o) => o.value != 'ledger_fix')
    },
    typeNames(): string[] {
      return this.visibleTypes.map((o) => o.label)
    },
    typeIndex(): number {
      const i = this.visibleTypes.findIndex((o) => o.value == this.maintenanceType)
      return i >= 0 ? i : 0
    }
  },
  onLoad(options: any) {
    this.equipmentId = options && options.equipment_id ? String(options.equipment_id) : ''
    if (options && options.name) this.eqName = String(options.name)
    if (options && options.code) this.eqCode = String(options.code)
    apiDictOptions('equipment_maint_type')
      .then((opts) => {
        this.typeOptions = opts
      })
      .catch(() => {})
    if (this.equipmentId != '') {
      apiEquipmentDetail(this.equipmentId)
        .then((d) => {
          this.detail = d
          this.eqName = d.name
          this.eqCode = d.code
          this.loading = false
        })
        .catch(() => {
          // 详情拉取失败不阻断登记（编号/名称用路由参数兜底）
          this.loading = false
        })
    } else {
      this.loading = false
    }
  },
  methods: {
    toggleLabelMissing() {
      this.labelMissing = !this.labelMissing
      if (this.labelMissing) {
        this.manufactureDate = ''
        this.lastMaintenanceDate = ''
      }
    },
    onTypePick(e: any) {
      const o = this.visibleTypes[e.detail.value]
      if (o != null) this.maintenanceType = o.value
    },
    onDatePick(e: any) {
      this.maintenanceDate = e.detail.value
    },
    onManufacturePick(e: any) {
      this.manufactureDate = e.detail.value
    },
    onLastMaintPick(e: any) {
      this.lastMaintenanceDate = e.detail.value
    },
    /** 拍照（仅相机防相册作弊）→ 压缩 → 上传换 file_id（scene=checkin：登记照片同属检查类照片） */
    takePhoto() {
      if (this.uploading || this.submitting) return
      uni.chooseImage({
        count: 1,
        sourceType: ['camera'],
        success: (res) => {
          const path = (res.tempFilePaths || [])[0]
          if (path == null) return
          this.uploading = true
          uni.showLoading({ title: '上传中…', mask: true })
          compressForUpload(path)
            .then((p) => apiUploadLocal(p, 'checkin'))
            .then((up) => {
              this.photos.push(up.url)
              this.fileIds.push(up.file_id)
            })
            .catch((e: Error) => {
              uni.showToast({ title: e.message, icon: 'none' })
            })
            .finally(() => {
              this.uploading = false
              uni.hideLoading()
            })
        }
      })
    },
    preview(idx: number) {
      uni.previewImage({ urls: this.photos, current: idx })
    },
    removePhoto(idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) {
            this.photos.splice(idx, 1)
            this.fileIds.splice(idx, 1)
          }
        }
      })
    },
    submit() {
      if (this.submitting || this.uploading) return
      if (this.equipmentId == '') {
        uni.showToast({ title: '缺少设备参数', icon: 'none' })
        return
      }
      if (this.fileIds.length == 0) {
        uni.showToast({ title: '请至少拍 1 张新维修标签照片', icon: 'none' })
        return
      }
      this.submitting = true
      apiEquipmentRegister({
        equipment_id: this.equipmentId,
        maintenance_type: this.maintenanceType,
        maintenance_date: this.maintenanceDate,
        vendor: this.vendor.trim() != '' ? this.vendor.trim() : undefined,
        note: this.note.trim() != '' ? this.note.trim() : undefined,
        file_ids: this.fileIds,
        label_missing: this.labelMissing || undefined,
        manufacture_date: this.maintenanceType == 'ledger_fix' && this.manufactureDate != '' ? this.manufactureDate : undefined,
        last_maintenance_date: this.maintenanceType == 'ledger_fix' && this.lastMaintenanceDate != '' ? this.lastMaintenanceDate : undefined
      })
        .then(() => {
          uni.showToast({ title: '登记已提交，待经理确认', icon: 'none' })
          setTimeout(() => {
            uni.navigateBack()
          }, 800)
        })
        .catch((e: Error) => {
          this.submitting = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.skeleton {
  padding: 24rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  opacity: 0.4;
}

.content {
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.eq-name {
  font-size: 34rpx;
  font-weight: 600;
}

.eq-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
}

.field-label {
  font-size: 26rpx;
  margin-top: 24rpx;
  margin-bottom: 12rpx;
}

.picker-box {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.picker-text {
  font-size: 28rpx;
}

.picker-arrow {
  font-size: 26rpx;
}

.text-input {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  font-size: 28rpx;
}

.note-input {
  width: 100%;
  height: 144rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 16rpx;
  font-size: 28rpx;
  box-sizing: border-box;
}

.label-missing-row {
  margin-top: 24rpx;
}

.missing-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.photo-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 16rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.photo-add {
  width: 160rpx;
  height: 160rpx;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: dashed;
  align-items: center;
  justify-content: center;
  margin-bottom: 16rpx;
}

.photo-add-text {
  font-size: 26rpx;
}

.btn-primary {
  height: 104rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 8rpx;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

.bottom-space {
  height: 64rpx;
}
</style>
