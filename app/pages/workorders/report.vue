<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 所属项目：单项目自动选定；多项目可切换 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">所属项目</text>
      <view v-if="communities.length == 0" class="comm-empty">
        <text class="comm-empty-text" :style="{ color: colors.danger }">你暂未加入任何项目编制，无法上报，请联系主管</text>
      </view>
      <view v-else class="chips-inner">
        <view
          v-for="c in communities"
          :key="c.id"
          class="chip"
          :style="communityId == c.id
            ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
            : { backgroundColor: colors.bgCard, borderColor: colors.border }"
          @click="selectCommunity(c.id)"
        >
          <text
            class="chip-text"
            :style="{ color: communityId == c.id ? colors.primary : colors.textSecondary }"
          >{{ c.name }}</text>
        </view>
      </view>
    </view>

    <!-- 关联点位：选填 -->
    <view v-if="communities.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">关联点位</text>
      <text class="sec-tip" :style="{ color: colors.textSecondary }">选填，问题所属的巡检点位</text>
      <picker :range="pointOptions" range-key="name" :value="pointIndex" @change="onPointChange">
        <view class="point-picker" :style="{ borderColor: colors.border }">
          <text
            class="point-picker-text"
            :style="{ color: pointIndex > 0 ? colors.textPrimary : colors.textSecondary }"
          >{{ pointOptions.length > 0 ? pointOptions[pointIndex].name : '加载中…' }}</text>
        </view>
      </picker>
    </view>

    <!-- 问题信息 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">问题信息</text>
      <input
        v-model="title"
        class="title-input"
        :style="{ borderColor: colors.border, color: colors.textPrimary }"
        placeholder="标题（必填，如：3栋电梯异响）"
        :maxlength="60"
      />
      <textarea
        v-model="description"
        class="desc-input"
        :style="{ borderColor: colors.border, color: colors.textPrimary }"
        placeholder="问题描述（必填）：位置、现场情况、影响范围等"
        :maxlength="500"
      />
    </view>

    <!-- 现场照片 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">现场照片</text>
      <text class="sec-tip" :style="{ color: colors.textSecondary }">选填，最多 6 张，长按照片可删除</text>
      <view class="photos">
        <image
          v-for="(ph, pi) in photos"
          :key="pi"
          class="photo"
          :src="ph"
          mode="aspectFill"
          @longpress="removePhoto(pi)"
        />
        <view
          v-if="photos.length < 6"
          class="photo-add"
          :style="{ borderColor: colors.border }"
          @click="takePhotos"
        >
          <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
        </view>
      </view>
    </view>

    <!-- 优先级 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="sec-title" :style="{ color: colors.textPrimary }">优先级</text>
      <view class="chips-inner">
        <view
          v-for="p in priorities"
          :key="p.value"
          class="chip"
          :style="priority == p.value
            ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
            : { backgroundColor: colors.bgCard, borderColor: colors.border }"
          @click="priority = p.value"
        >
          <text
            class="chip-text"
            :style="{ color: priority == p.value ? colors.primary : colors.textSecondary }"
          >{{ p.label }}</text>
        </view>
      </view>
    </view>

    <!-- 提交 -->
    <view class="btn-primary" :style="{ backgroundColor: submitting ? colors.info : colors.primary }" @click="submit">
      <text class="btn-primary-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : '提交上报' }}</text>
    </view>

    <!-- 水印烧录用隐藏 canvas（屏外，尺寸由数据驱动） -->
    <canvas
      canvas-id="wmCanvas"
      :style="{ width: canvasW + 'px', height: canvasH + 'px', position: 'fixed', left: '-9999px', top: '-9999px' }"
    ></canvas>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiReportOrder, apiUploadLocal, apiCommunityTree, apiListPoints, PointOption } from '@/services/api'
import { burnWatermark } from '@/utils/watermark'
import { useAuthStore } from '@/stores/auth'

type ReportData = {
  colors: ColorTokens
  /** 我的在职项目（profile projects + 名称解析） */
  communities: Array<{ id: string; name: string }>
  communityId: string
  /** 关联点位选项：index 0 恒为「不关联点位」 */
  pointOptions: PointOption[]
  pointIndex: number
  title: string
  description: string
  photos: string[]
  priority: string
  priorities: Array<{ label: string; value: string }>
  submitting: boolean
  canvasW: number
  canvasH: number
}

function pad2(n: number): string {
  return n < 10 ? '0' + n : '' + n
}

/** YYYY-MM-DD HH:mm:ss（与后端 timefmt.Layout 一致，本地时区） */
function fmtDateTime(d: Date): string {
  return (
    d.getFullYear() + '-' + pad2(d.getMonth() + 1) + '-' + pad2(d.getDate()) +
    ' ' + pad2(d.getHours()) + ':' + pad2(d.getMinutes()) + ':' + pad2(d.getSeconds())
  )
}

export default {
  data(): ReportData {
    return {
      colors: Colors,
      communities: [] as Array<{ id: string; name: string }>,
      communityId: '',
      pointOptions: [{ id: '', name: '不关联点位' }] as PointOption[],
      pointIndex: 0,
      title: '',
      description: '',
      photos: [] as string[],
      priority: 'normal',
      priorities: [
        { label: '普通', value: 'normal' },
        { label: '低', value: 'low' },
        { label: '高', value: 'high' },
        { label: '紧急', value: 'urgent' }
      ],
      submitting: false,
      canvasW: 0,
      canvasH: 0
    }
  },
  onLoad() {
    this.loadCommunities()
  },
  methods: {
    /**
     * 加载我的在职项目：id 取自 profile projects；
     * 名称优先走 /communities/tree（无权限的一线账号会 403，静默回退为占位名）。
     */
    loadCommunities() {
      const u = useAuthStore().userInfo
      const projects = u != null ? u.projects : []
      const fallback = projects.map((p, i) => ({
        id: p.id,
        name: p.name != null && p.name != '' ? p.name : (projects.length > 1 ? '项目 ' + (i + 1) : '我的项目')
      }))
      apiCommunityTree()
        .then((tree) => {
          const nameOf: Record<string, string> = {}
          tree.forEach((c) => {
            nameOf[c.id] = c.name
          })
          this.communities = fallback.map((c) => ({
            id: c.id,
            name: nameOf[c.id] != null && nameOf[c.id] != '' ? nameOf[c.id] : c.name
          }))
        })
        .catch((_e) => {
          this.communities = fallback
        })
        .finally(() => {
          if (this.communityId == '' && this.communities.length > 0) {
            this.communityId = this.communities[0].id
            this.loadPoints()
          }
        })
    },
    /** 切换项目：重置并重新加载点位选项 */
    selectCommunity(id: string) {
      if (this.communityId == id) return
      this.communityId = id
      this.loadPoints()
    },
    /** 加载当前项目的启用点位（index 0 恒为「不关联点位」） */
    loadPoints() {
      this.pointOptions = [{ id: '', name: '不关联点位' }]
      this.pointIndex = 0
      if (this.communityId == '') return
      apiListPoints(this.communityId)
        .then((list) => {
          this.pointOptions = [{ id: '', name: '不关联点位' }].concat(list)
        })
        .catch((_e) => {
          // 点位加载失败不阻断上报，保持「不关联点位」单选项
        })
    },
    onPointChange(e: { detail: { value: number | string } }) {
      this.pointIndex = Number(e.detail.value)
    },
    /** 拍照/选图 → 水印烧录 → 入列表 */
    takePhotos() {
      const remain = 6 - this.photos.length
      if (remain <= 0) return
      uni.chooseImage({
        count: remain,
        sourceType: ['camera', 'album'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          uni.showLoading({ title: '处理中…', mask: true })
          const u = useAuthStore().userInfo
          const lines = [fmtDateTime(new Date()) + ' 问题上报']
          if (u != null && u.name != '') {
            lines.push('上报人：' + u.name)
          }
          let chain: Promise<void> = Promise.resolve()
          paths.forEach((p) => {
            chain = chain
              .then(() => burnWatermark(p, lines, 'wmCanvas', this))
              .then((burned) => {
                this.photos.push(burned)
              })
          })
          chain
            .then(() => uni.hideLoading())
            .catch(() => uni.hideLoading())
        }
      })
    },
    removePhoto(idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) this.photos.splice(idx, 1)
        }
      })
    },
    /** 提交：先逐张上传照片换 file_key，再调上报接口 */
    submit() {
      if (this.submitting) return
      if (this.communityId == '') {
        uni.showToast({ title: '请选择所属项目', icon: 'none' })
        return
      }
      if (this.title.trim() == '') {
        uni.showToast({ title: '请填写问题标题', icon: 'none' })
        return
      }
      if (this.description.trim() == '') {
        uni.showToast({ title: '请填写问题描述', icon: 'none' })
        return
      }
      this.submitting = true
      uni.showLoading({ title: '上传中…', mask: true })
      const keys: string[] = []
      let chain: Promise<void> = Promise.resolve()
      this.photos.forEach((p) => {
        chain = chain
          .then(() => apiUploadLocal(p, 'workorder'))
          .then((r) => {
            keys.push(r.file_key)
          })
      })
      chain
        .then(() => {
          uni.showLoading({ title: '提交中…', mask: true })
          const pointId = this.pointIndex > 0 ? this.pointOptions[this.pointIndex].id : ''
          return apiReportOrder({
            community_id: this.communityId,
            point_id: pointId != '' ? pointId : undefined,
            title: this.title.trim(),
            description: this.description.trim(),
            photos: keys.map((k) => ({ file_key: k })),
            priority: this.priority
          })
        })
        .then(() => {
          uni.hideLoading()
          this.submitting = false
          uni.showToast({ title: '上报成功', icon: 'success' })
          setTimeout(() => {
            uni.navigateBack()
          }, 800)
        })
        .catch((e: Error) => {
          uni.hideLoading()
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
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
}

.sec-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.chips-inner {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.chip {
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  padding: 8rpx 24rpx;
  margin-right: 16rpx;
  margin-bottom: 8rpx;
}

.chip-text {
  font-size: 26rpx;
}

.comm-empty {
  margin-top: 16rpx;
}

.comm-empty-text {
  font-size: 26rpx;
}

.title-input {
  width: 100%;
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 0 24rpx;
  font-size: 28rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
}

.point-picker {
  width: 100%;
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 0 24rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
  justify-content: center;
}

.point-picker-text {
  font-size: 28rpx;
}

.desc-input {
  width: 100%;
  height: 200rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
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
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  margin-top: 8rpx;
  margin-bottom: 48rpx;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}
</style>
