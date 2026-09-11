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

      <!-- 拍新维修标签（必传 1-3 张） -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">拍新维修标签</text>
        <text class="photo-tip" :style="{ color: colors.textSecondary }">拍换粉/维修后的新标签，至少 1 张；系统自动核对，核对通过立即生效，拿不准转经理确认</text>
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
          <view v-if="photos.length < 3" class="photo-add" :style="{ borderColor: colors.border }" @click="takePhoto">
            <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
          </view>
        </view>
        <text class="photo-tip" :style="{ color: colors.textSecondary }">标签磨损无法辨认？请联系经理在电脑端处理</text>
      </view>

      <!-- 提交 -->
      <view class="btn-primary" :style="{ backgroundColor: submitting ? colors.info : colors.primary }" @click="submit">
        <text class="btn-primary-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : '提交维保' }}</text>
      </view>
      <view class="bottom-space"></view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiEquipmentDetail, apiEquipmentRegister, apiUploadLocal, EquipmentDetail } from '@/services/api'
import { compressForUpload } from '@/utils/image'

type MaintainData = {
  colors: ColorTokens
  equipmentId: string
  detail: EquipmentDetail | null
  /** 详情拉取失败时的回显兜底（路由参数带入） */
  eqName: string
  eqCode: string
  loading: boolean
  /** 已上传照片的服务端 URL 列表（与 fileIds 一一对应） */
  photos: string[]
  fileIds: string[]
  submitting: boolean
  uploading: boolean
}

export default {
  data(): MaintainData {
    return {
      colors: Colors,
      equipmentId: '',
      detail: null,
      eqName: '',
      eqCode: '',
      loading: true,
      photos: [] as string[],
      fileIds: [] as string[],
      submitting: false,
      uploading: false
    }
  },
  onLoad(options: any) {
    this.equipmentId = options && options.equipment_id ? String(options.equipment_id) : ''
    if (options && options.name) this.eqName = String(options.name)
    if (options && options.code) this.eqCode = String(options.code)
    if (this.equipmentId != '') {
      apiEquipmentDetail(this.equipmentId)
        .then((d) => {
          this.detail = d
          this.eqName = d.name
          this.eqCode = d.code
          this.loading = false
        })
        .catch(() => {
          // 详情拉取失败不阻断（编号/名称用路由参数兜底）
          this.loading = false
        })
    } else {
      this.loading = false
    }
  },
  methods: {
    /** 拍照（仅相机防相册作弊）→ 压缩 → 上传换 file_id（scene=checkin：维保照片同属检查类照片） */
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
      // 最小载荷：拍新标签即登记（默认维修类、当天）；后端同步 AI 核对，可信直接生效回写台账
      apiEquipmentRegister({
        equipment_id: this.equipmentId,
        maintenance_type: 'repair',
        file_ids: this.fileIds
      })
        .then((r) => {
          // 已生效（AI 核对通过）与待确认（存疑转经理）分别反馈
          if (r.confirm_status == 'confirmed' || r.confirm_mode == 'ai') {
            uni.showToast({ title: '系统核对通过，维保已生效', icon: 'none' })
          } else {
            uni.showToast({ title: '已提交，待经理确认', icon: 'none' })
          }
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
