<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <view v-else class="content">
      <!-- 与打卡向导同一套视觉语言：大卡片 + 大拍照块 + 大按钮（巡检员零学习成本） -->
      <view class="item-card" :style="{ backgroundColor: colors.bgCard }">
        <text class="item-name" :style="{ color: colors.textPrimary }">{{ eqName }}</text>
        <text class="item-hint" :style="{ color: colors.textSecondary }">编号：{{ eqCode }}<text v-if="detail != null && detail.next_due_date != ''"> · 到期日 {{ detail.next_due_date }}</text></text>

        <!-- 到期状态横幅（与向导台账有效期项同口径） -->
        <view v-if="dueText != ''" class="equip-banner" :style="{ backgroundColor: colors.bgPage }">
          <text class="equip-state" :style="{ color: dueColor }">{{ dueText }}</text>
        </view>

        <!-- 空态：大号虚线拍照块（同向导拍照项） -->
        <view
          v-if="photos.length == 0"
          hover-class="hover-dim"
          class="shot-empty"
          :style="{ borderColor: colors.primary }"
          @click="takePhoto"
        >
          <view class="cam-icon" :style="{ borderColor: colors.primary }">
            <view class="cam-lens" :style="{ borderColor: colors.primary }"></view>
          </view>
          <text class="shot-empty-text" :style="{ color: colors.primary }">已维保？点这里拍新标签</text>
        </view>

        <!-- 已拍：大图预览（同向导）+ 缩略图行 + 提示 -->
        <block v-else>
          <view class="shot-preview" :style="{ backgroundColor: colors.bgPage }" @click="preview(0)">
            <image v-if="!imgError" :src="photos[0]" class="shot-img" mode="aspectFill" @error="imgError = true" />
            <view v-else class="shot-img shot-img-fallback">
              <text class="shot-img-fallback-text">照片加载失败，可重新拍</text>
            </view>
          </view>
          <view v-if="photos.length > 1" class="equip-thumbs">
            <image
              v-for="(p, pi) in photos"
              :key="pi"
              :src="p"
              class="equip-thumb"
              mode="aspectFill"
              @click="preview(pi)"
              @longpress="removePhoto(pi)"
            />
          </view>
          <text class="equip-photo-hint" :style="{ color: colors.textSecondary }">
            已拍 {{ photos.length }} 张新标签（长按缩略图可删除），提交时系统自动核对，拿不准转经理确认
          </text>
          <view v-if="photos.length < 3" hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="takePhoto">
            <text class="btn-outline-text" :style="{ color: colors.primary }">再拍一张</text>
          </view>
        </block>

        <!-- 提交（同向导大按钮） -->
        <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: photos.length == 0 || submitting ? colors.info : colors.primary }" @click="submit">
          <text class="btn-big-text" :style="{ color: colors.white }">{{ submitting ? 'AI 核对中…' : '提交维保' }}</text>
        </view>

        <text class="foot-note" :style="{ color: colors.textSecondary }">标签磨损无法辨认？请联系经理在电脑端处理</text>
      </view>
      <view class="bottom-space"></view>
    </view>

    <!-- 提交结果弹窗（自绘：圆形状态图标 + 标题 + 说明 + 胶囊按钮；系统 showModal 太生硬） -->
    <view v-if="resultDlg.show" class="dlg-mask" :style="{ backgroundColor: colors.mask }">
      <view class="dlg-card" :style="{ backgroundColor: colors.bgCard }">
        <view class="dlg-icon" :style="{ backgroundColor: resultDlg.color }">
          <text class="dlg-icon-text" :style="{ color: colors.white }">{{ resultDlg.icon }}</text>
        </view>
        <text class="dlg-title" :style="{ color: colors.textPrimary }">{{ resultDlg.title }}</text>
        <text class="dlg-content" :style="{ color: colors.textRegular }">{{ resultDlg.content }}</text>
        <view hover-class="hover-dim" class="dlg-btn" :style="{ backgroundColor: resultDlg.color }" @click="onResultConfirm">
          <text class="dlg-btn-text" :style="{ color: colors.white }">知道了</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiEquipmentDetail, apiEquipmentRegister, apiUploadLocal, EquipmentDetail } from '@/services/api'
import { compressForUpload } from '@/utils/image'

/** 今日 0 点（本地时区），到期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

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
  imgError: boolean
  submitting: boolean
  uploading: boolean
  /** 提交结果弹窗（自绘）：kind 决定图标/颜色；成功类确认后退出，失败类留在原地 */
  resultDlg: { show: boolean; kind: 'ok' | 'pending' | 'fail'; icon: string; color: string; title: string; content: string }
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
      imgError: false,
      submitting: false,
      uploading: false,
      resultDlg: { show: false, kind: 'ok', icon: '✓', color: Colors.success, title: '', content: '' }
    }
  },
  computed: {
    /** 到期状态文案/颜色（与 QuickItemCard 台账有效期项同口径；详情缺失时不显示） */
    dueText(): string {
      const d = this.detail
      if (d == null || d.next_due_date == '') return ''
      const days = Math.round((new Date(d.next_due_date.replace(/-/g, '/')).getTime() - todayZero()) / 86400000)
      if (days < 0) return '已逾期 ' + (-days) + ' 天（到期日 ' + d.next_due_date + '）'
      if (days == 0) return '今天到期'
      return days + ' 天后到期（' + d.next_due_date + '）'
    },
    dueColor(): string {
      const d = this.detail
      if (d == null || d.next_due_date == '') return Colors.info
      const days = Math.round((new Date(d.next_due_date.replace(/-/g, '/')).getTime() - todayZero()) / 86400000)
      if (days < 0) return Colors.danger
      if (days <= 30) return Colors.warning
      return Colors.success
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
              this.imgError = false
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
        uni.showToast({ title: '请先拍新维修标签照片', icon: 'none' })
        return
      }
      this.submitting = true
      // 后端同步 AI 核对（最长约 15s）：遮罩 loading 与打卡「AI 检查中」同口径，防重复点击
      uni.showLoading({ title: 'AI 核对中…', mask: true })
      // 最小载荷：拍新标签即登记（默认维修类、当天）；后端同步 AI 核对，结论供经理确认参考
      apiEquipmentRegister({
        equipment_id: this.equipmentId,
        maintenance_type: 'repair',
        file_ids: this.fileIds
      })
        .then((r) => {
          uni.hideLoading()
          // 结果必须被看到：自绘弹窗确认后才退出（toast 会被 navigateBack 吞掉）
          const autoOk = r.confirm_status == 'confirmed' || r.confirm_mode == 'ai'
          this.openResult(autoOk ? 'ok' : 'pending', autoOk ? '维保已生效' : '提交成功', autoOk ? '系统核对通过，台账已更新' : '已提交，经理确认后生效')
        })
        .catch((e: Error) => {
          uni.hideLoading()
          this.submitting = false
          // 失败留在原地可重试（照片已上传，重试不丢）
          this.openResult('fail', '提交失败', e.message)
        })
    },
    /** 打开结果弹窗：ok=绿勾（已生效）/pending=蓝点（待经理确认）/fail=红叉（留在原地可重试） */
    openResult(kind: 'ok' | 'pending' | 'fail', title: string, content: string) {
      const icon = kind == 'fail' ? '✕' : kind == 'pending' ? '●' : '✓'
      const color = kind == 'fail' ? Colors.danger : kind == 'pending' ? Colors.primary : Colors.success
      this.resultDlg = { show: true, kind, icon, color, title, content }
    },
    /** 结果弹窗确认：成功类退出本页；失败类仅关闭，照片保留可重试 */
    onResultConfirm() {
      const kind = this.resultDlg.kind
      this.resultDlg.show = false
      if (kind != 'fail') {
        uni.navigateBack()
      }
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

/* 以下样式与 QuickItemCard.vue 同口径：打卡向导的视觉语言原样复用 */
.item-card {
  border-radius: 24rpx;
  padding: 48rpx 32rpx;
  align-items: center;
  margin-bottom: 24rpx;
}

.item-name {
  font-size: 56rpx;
  font-weight: 700;
}

.item-hint {
  font-size: 34rpx;
  text-align: center;
  margin-top: 16rpx;
  line-height: 48rpx;
}

.equip-banner {
  width: 100%;
  border-radius: 20rpx;
  margin-top: 40rpx;
  padding: 32rpx;
  align-items: center;
}

.equip-state {
  font-size: 40rpx;
  font-weight: 700;
}

.shot-empty {
  width: 100%;
  height: 360rpx;
  border-width: 3rpx;
  border-style: dashed;
  border-radius: 24rpx;
  margin-top: 40rpx;
  align-items: center;
  justify-content: center;
}

.cam-icon {
  width: 120rpx;
  height: 96rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.cam-lens {
  width: 40rpx;
  height: 40rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
}

.shot-empty-text {
  font-size: 40rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.shot-preview {
  width: 100%;
  height: 480rpx;
  border-radius: 20rpx;
  margin-top: 40rpx;
  overflow: hidden;
  align-items: center;
  justify-content: center;
}

.shot-img {
  width: 100%;
  height: 480rpx;
}

.shot-img-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f2f3f5;
}

.shot-img-fallback-text {
  font-size: 26rpx;
  color: #9ca3af;
}

.equip-thumbs {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
  width: 100%;
}

.equip-thumb {
  width: 120rpx;
  height: 120rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
}

.equip-photo-hint {
  font-size: 24rpx;
  margin-top: 12rpx;
  margin-bottom: 16rpx;
  text-align: center;
}

.btn-big {
  width: 100%;
  height: 140rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 32rpx;
}

.btn-big-text {
  font-size: 44rpx;
  font-weight: 700;
}

.btn-outline {
  width: 100%;
  height: 112rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 8rpx;
}

.btn-outline-text {
  font-size: 40rpx;
  font-weight: 600;
}

.foot-note {
  font-size: 24rpx;
  margin-top: 24rpx;
  text-align: center;
}

.bottom-space {
  height: 64rpx;
}

/* 提交结果弹窗（自绘，居中卡片） */
.dlg-mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 999;
  justify-content: center;
  align-items: center;
}

.dlg-card {
  width: 600rpx;
  border-radius: 28rpx;
  padding: 48rpx 40rpx 40rpx;
  align-items: center;
}

.dlg-icon {
  width: 112rpx;
  height: 112rpx;
  border-radius: 56rpx;
  align-items: center;
  justify-content: center;
}

.dlg-icon-text {
  font-size: 56rpx;
  font-weight: 700;
}

.dlg-title {
  font-size: 36rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.dlg-content {
  font-size: 28rpx;
  margin-top: 16rpx;
  line-height: 44rpx;
  text-align: center;
}

.dlg-btn {
  width: 100%;
  height: 96rpx;
  border-radius: 48rpx;
  align-items: center;
  justify-content: center;
  margin-top: 40rpx;
}

.dlg-btn-text {
  font-size: 34rpx;
  font-weight: 600;
}
</style>
