<template>
  <view class="page bg-page" >
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block bg-border" ></view>
    </view>

    <!-- 裸进兜底（无设备参数/编辑参数）：引导先选设备，防深链空表单 -->
    <view v-else-if="bareEntry" class="content">
      <view class="item-card bg-card bare-card">
        <uni-icons type="gear" size="48" color="#2B5AED" />
        <text class="bare-title text-main">维保登记需要先选择设备</text>
        <text class="bare-sub text-secondary">从设备台账里挑一台要维保的设备，再拍新标签提交</text>
        <button plain="true" class="btn-primary btn-big bare-btn" hover-class="hover-dim" @click="goPickEquipment">去选择设备</button>
      </view>
    </view>

    <view v-else class="content">
      <!-- 与打卡向导同一套视觉语言：大卡片 + 大拍照块 + 大按钮（巡检员零学习成本） -->
      <view class="item-card bg-card" >
        <text class="item-name text-main" >{{ eqName }}</text>
        <text class="item-hint text-secondary" >编号：{{ eqCode }}<text v-if="detail != null && detail.next_due_date != ''"> · 到期日 {{ detail.next_due_date }}</text></text>

        <!-- 到期状态横幅（与向导台账有效期项同口径） -->
        <view v-if="dueText != ''" class="equip-banner bg-page" >
          <text class="equip-state" :style="{ color: dueColor }">{{ dueText }}</text>
        </view>

        <!-- 空态：大号虚线拍照块（同向导拍照项） -->
        <view
          v-if="photos.length == 0"
          hover-class="hover-dim"
          class="shot-empty border-brand"
          
          @click="takePhoto"
        >
          <view class="cam-icon border-brand" >
            <view class="cam-lens border-brand" ></view>
          </view>
          <text class="shot-empty-text text-brand" >已维保？点这里拍新标签</text>
        </view>

        <!-- 已拍：大图预览（同向导）+ 缩略图行 + 提示 -->
        <block v-else>
          <view class="shot-preview bg-page"  @click="preview(0)">
            <image v-if="!imgError" :src="photos[0]" class="shot-img" mode="aspectFill" @error="imgError = true" />
            <view v-else class="shot-img shot-img-fallback">
              <text class="shot-img-fallback-text">照片加载失败，可重新拍</text>
            </view>
          </view>
          <view v-if="photos.length >= 1" class="equip-thumbs">
            <view v-for="(p, pi) in photos" :key="pi" class="equip-thumb-wrap">
              <image
                :src="p"
                class="equip-thumb"
                mode="aspectFill"
                @click="preview(pi)"
                @longpress="removePhoto(pi)"
              />
              <view class="equip-thumb-del bg-danger"  @click.stop="removePhoto(pi)">
                <text class="equip-thumb-del-text text-white" >×</text>
              </view>
            </view>
          </view>
          <text class="equip-photo-hint text-secondary" >
            已拍 {{ photos.length }} 张新标签（点缩略图右上角 × 删除，长按也可），提交时系统自动核对，拿不准转经理确认
          </text>
          <button v-if="photos.length < 3" plain="true" hover-class="hover-dim" class="btn-outline reshot" @click="takePhoto">
            <text class="btn-outline-text">再拍一张</text>
          </button>
        </block>

        <!-- 提交（同向导大按钮） -->
        <button plain="true" hover-class="hover-dim" class="btn-big" :class="(photos.length == 0 || submitting ? 'btn-disabled' : 'btn-primary')" @click="submit">
          <text class="btn-big-text">{{ submitting ? 'AI 核对中…' : editId != '' ? '重新提交' : '提交维保' }}</text>
        </button>

        <text class="foot-note text-secondary" >标签磨损无法辨认？请联系经理在电脑端处理</text>
      </view>
      <view class="bottom-space"></view>
    </view>

    <!-- 提交结果弹窗（AppDialog：圆形状态图标 + 标题 + 说明 + 胶囊按钮；结果必须被看到，确认后才退出） -->
    <AppDialog
      :visible="resultDlg.show"
      :kind="resultDlg.kind == 'fail' ? 'danger' : resultDlg.kind == 'pending' ? 'primary' : 'success'"
      :title="resultDlg.title"
      :content="resultDlg.content"
      @update:visible="resultDlg.show = $event"
      @confirm="onResultConfirm"
    />
    <!-- 删除照片确认（自绘弹窗，与结果弹窗同口径） -->
    <AppDialog
      :visible="delDlg.show"
      kind="danger"
      title="删除照片"
      content="确定删除这张照片吗？"
      confirm-text="删除"
      cancel-text="取消"
      @update:visible="delDlg.show = $event"
      @confirm="onDelPhotoConfirm"
    />
  </view>
</template>

<script lang="ts">

import { apiEquipmentDetail, apiEquipmentRegister, apiMaintenanceUpdate, apiMaintenanceMine, apiUploadLocal, EquipmentDetail, CODE_QUALITY_FAIL } from '@/services/api'
import { compressForUpload } from '@/utils/image'
import { toAbsUrl } from '@/utils/url'
import { MAINTAIN_EDIT_KEY } from '@/utils/storage'
import { toastErr } from '@/utils/ui'
import AppDialog from '@/components/AppDialog.vue'

type MaintainEditDraft = {
  id: string
  equipment_id: string
  name: string
  code: string
  photos: Array<{ file_id: string; url: string }>
}

/** 今日 0 点（本地时区），到期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

type MaintainData = {
  equipmentId: string
  /** 编辑模式：待确认记录 id（「我的提交 → 修改照片」跳入）；空 = 新登记 */
  editId: string
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
  /** 提交结果弹窗（AppDialog）：kind 决定图标/颜色（ok=success / pending=primary / fail=danger）；成功类确认后退出，失败类留在原地 */
  resultDlg: { show: boolean; kind: 'ok' | 'pending' | 'fail'; title: string; content: string }
  /** 删除照片确认弹窗：idx 为待删照片下标 */
  delDlg: { show: boolean; idx: number }
}

export default {
  components: { AppDialog },
  data(): MaintainData {
    return {
      equipmentId: '',
      editId: '',
      detail: null,
      eqName: '',
      eqCode: '',
      loading: true,
      photos: [] as string[],
      fileIds: [] as string[],
      imgError: false,
      submitting: false,
      uploading: false,
      resultDlg: { show: false, kind: 'ok', title: '', content: '' },
      delDlg: { show: false, idx: -1 }
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
    /** 裸进判定：无 equipment_id（新登记缺设备）且无 maintenance_id（非编辑模式） */
    bareEntry(): boolean {
      return this.equipmentId == '' && this.editId == ''
    },
    dueColor(): string {
      const d = this.detail
      if (d == null || d.next_due_date == '') return '#909399'
      const days = Math.round((new Date(d.next_due_date.replace(/-/g, '/')).getTime() - todayZero()) / 86400000)
      if (days < 0) return '#D54941'
      if (days <= 30) return '#ED7B2F'
      return '#2BA471'
    }
  },
  onLoad(options: any) {
    this.equipmentId = options && options.equipment_id ? String(options.equipment_id) : ''
    if (options && options.name) {
      // 路由参数是 URL 编码的（中文设备名会带 %XX），显示前先解码；解码失败兜底原文
      const raw = String(options.name)
      try {
        this.eqName = decodeURIComponent(raw)
      } catch {
        this.eqName = raw
      }
    }
    if (options && options.code) this.eqCode = String(options.code)
    // 编辑模式（我的提交 → 修改照片）：预填已上传照片，提交走 PUT 修改
    if (options && options.maintenance_id) {
      this.editId = String(options.maintenance_id)
      uni.setNavigationBarTitle({ title: '修改维保照片' })
      this.prefillDraft()
    }
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
  onUnload() {
    // 离开编辑模式（提交成功返回/中途取消）：清除编辑草稿，防残留串到下次登记
    if (this.editId != '') uni.removeStorageSync(MAINTAIN_EDIT_KEY)
  },
  methods: {
    /** 裸进兜底：去设备台账选择模式挑设备（选择后 redirectTo 回本页带参数） */
    goPickEquipment() {
      uni.redirectTo({ url: '/pages/equipment/index?mode=pick' })
    },
    /** 编辑模式预填：优先 mine.vue 写入的草稿；缺失（如页面刷新）时拉我的提交列表兜底 */
    prefillDraft() {
      const draft = uni.getStorageSync(MAINTAIN_EDIT_KEY) as MaintainEditDraft | ''
      if (draft != null && typeof draft == 'object' && draft.id == this.editId && Array.isArray(draft.photos)) {
        this.photos = draft.photos.map((p) => toAbsUrl(p.url))
        this.fileIds = draft.photos.map((p) => p.file_id)
        return
      }
      apiMaintenanceMine(1, 100)
        .then((p) => {
          const m = p.list.find((it) => it.id == this.editId)
          if (m != null) {
            this.photos = m.photos.map((ph) => toAbsUrl(ph.url))
            this.fileIds = m.photos.map((ph) => ph.file_id)
          }
        })
        .catch(() => {})
    },
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
              toastErr(e)
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
      this.delDlg = { show: true, idx: idx }
    },
    onDelPhotoConfirm() {
      const idx = this.delDlg.idx
      if (idx >= 0) {
        this.photos.splice(idx, 1)
        this.fileIds.splice(idx, 1)
      }
      this.delDlg.idx = -1
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
      if (this.editId != '') {
        // 编辑模式：修改待确认记录（后端重走 AI 核验；经理已处理则 409 提示）
        apiMaintenanceUpdate(this.editId, { file_ids: this.fileIds })
          .then((r) => {
            uni.hideLoading()
            uni.removeStorageSync(MAINTAIN_EDIT_KEY) // 提交成功清除编辑草稿（onUnload 兜底再清一次）
            const autoOk = r.confirm_status == 'confirmed'
            this.openResult(autoOk ? 'ok' : 'pending', autoOk ? '维保已生效' : '修改已提交', autoOk ? '系统核对通过，台账已更新' : '已更新照片，经理确认后生效')
          })
          .catch((e: any) => {
            uni.hideLoading()
            this.submitting = false
            if (e != null && e.code == CODE_QUALITY_FAIL) {
              this.openResult('fail', '照片不合格，请重新拍摄', e.message)
              return
            }
            this.openResult('fail', '提交失败', e != null ? e.message : '网络异常，请重试')
          })
        return
      }
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
        .catch((e: any) => {
          uni.hideLoading()
          this.submitting = false
          // 失败留在原地可重试；43107=AI 判照片明显不合格（质量/不像标签），引导重拍
          if (e != null && e.code == CODE_QUALITY_FAIL) {
            this.openResult('fail', '照片不合格，请重新拍摄', e.message)
            return
          }
          this.openResult('fail', '提交失败', e != null ? e.message : '网络异常，请重试')
        })
    },
    /** 打开结果弹窗：ok=绿勾（已生效）/pending=蓝点（待经理确认）/fail=红叉（留在原地可重试） */
    openResult(kind: 'ok' | 'pending' | 'fail', title: string, content: string) {
      this.resultDlg = { show: true, kind, title, content }
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

.equip-thumb-wrap {
  position: relative;
  margin-right: 16rpx;
  margin-bottom: 12rpx;
}

.equip-thumb-wrap .equip-thumb {
  margin-right: 0;
}

.equip-thumb-del {
  position: absolute;
  top: -12rpx;
  right: -12rpx;
  width: 44rpx;
  height: 44rpx;
  border-radius: 22rpx;
  align-items: center;
  justify-content: center;
}

.equip-thumb-del-text {
  font-size: 30rpx;
  font-weight: 700;
  line-height: 44rpx;
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
  border-radius: 20rpx;
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

.bare-card {
  align-items: center;
  padding-top: 96rpx;
  padding-bottom: 64rpx;
}

.bare-title {
  font-size: 36rpx;
  font-weight: 700;
  margin-top: 32rpx;
}

.bare-sub {
  font-size: 28rpx;
  margin-top: 16rpx;
  text-align: center;
  line-height: 44rpx;
}

.bare-btn {
  margin-top: 48rpx;
}

.bottom-space {
  height: 64rpx;
}
</style>
