<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="!loaded" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <!-- 工单详情 -->
    <view v-else-if="order != null" class="content">
      <!-- 头部：标题 + 状态/优先级 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ order.title }}</text>
          <text class="card-status" :style="{ color: statusColor }">{{ statusText }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">单号：{{ order.order_no }}</text>
        <view class="head-tags">
          <text class="tag" :style="{ color: priorityColor, borderColor: priorityColor }">{{ priorityText }}</text>
          <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ myRoleText }}</text>
        </view>
      </view>

      <!-- 基本信息 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">基本信息</text>
        <view class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">小区</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ order.community_name }}</text>
        </view>
        <view v-if="order.point_name != ''" class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">点位</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ order.point_name }}</text>
        </view>
        <view class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">上报人</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ order.reporter_name }}</text>
        </view>
        <view class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">处理人</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ order.assignee_name != '' ? order.assignee_name : '待派单' }}</text>
        </view>
        <view class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">上报时间</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ order.created_at }}</text>
        </view>
        <view v-if="order.description != ''" class="desc">
          <text class="row-label" :style="{ color: colors.textSecondary }">异常描述</text>
          <text class="desc-text" :style="{ color: colors.textRegular }">{{ order.description }}</text>
        </view>
      </view>

      <!-- 异常项快照 -->
      <view v-if="order.items.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">异常项</text>
        <view v-for="(it, idx) in order.items" :key="idx" class="snap-item">
          <text class="snap-name" :style="{ color: colors.textRegular }">{{ it.name }}</text>
          <text v-if="it.remark != ''" class="snap-remark" :style="{ color: colors.textSecondary }">{{ it.remark }}</text>
          <view v-if="it.before_photo_urls.length > 0" class="photos">
            <image
              v-for="(u, pi) in it.before_photo_urls"
              :key="pi"
              class="photo"
              :src="u"
              mode="aspectFill"
              @click="preview(it.before_photo_urls, pi)"
            />
          </view>
          <view v-if="it.after_photo_urls.length > 0">
            <text class="snap-label" :style="{ color: colors.textSecondary }">整改后</text>
            <view class="photos">
              <image
                v-for="(u, pi) in it.after_photo_urls"
                :key="pi"
                class="photo"
                :src="u"
                mode="aspectFill"
                @click="preview(it.after_photo_urls, pi)"
              />
            </view>
          </view>
        </view>
      </view>

      <!-- 上报照片（before） -->
      <view v-if="beforePhotoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">现场照片</text>
        <view class="photos">
          <image
            v-for="(u, pi) in beforePhotoUrls"
            :key="pi"
            class="photo"
            :src="u"
            mode="aspectFill"
            @click="preview(beforePhotoUrls, pi)"
          />
        </view>
      </view>

      <!-- 处理结果（完工后展示） -->
      <view v-if="order.fix_remark != '' || fixPhotoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">处理结果</text>
        <text v-if="order.fix_remark != ''" class="desc-text" :style="{ color: colors.textRegular }">{{ order.fix_remark }}</text>
        <view v-if="fixPhotoUrls.length > 0" class="photos">
          <image
            v-for="(u, pi) in fixPhotoUrls"
            :key="pi"
            class="photo"
            :src="u"
            mode="aspectFill"
            @click="preview(fixPhotoUrls, pi)"
          />
        </view>
        <text v-if="order.finished_at != ''" class="card-sub" :style="{ color: colors.textSecondary }">完工时间：{{ order.finished_at }}</text>
      </view>

      <!-- 审核意见（审核后展示） -->
      <view v-if="order.review_remark != null && order.review_remark != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">审核意见</text>
        <text
          class="desc-text"
          :style="{ color: order.status == 'rejected' ? colors.danger : colors.textRegular }"
        >{{ order.review_remark }}</text>
      </view>

      <!-- 操作区：接单 / 完工表单（仅指派给我的工单） -->
      <view v-if="canAccept" class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="btn-primary" :style="{ backgroundColor: acting ? colors.info : colors.primary }" @click="doAccept">
          <text class="btn-primary-text" :style="{ color: colors.white }">{{ acting ? '处理中…' : '接单' }}</text>
        </view>
      </view>

      <view v-if="canFinish" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">完工反馈</text>
        <text class="sec-tip" :style="{ color: colors.textSecondary }">维修照片（必传，最多 6 张，长按照片可删除）</text>
        <view class="photos">
          <image
            v-for="(ph, pi) in fixPhotos"
            :key="pi"
            class="photo"
            :src="ph"
            mode="aspectFill"
            @longpress="removeFixPhoto(pi)"
          />
          <view
            v-if="fixPhotos.length < 6"
            class="photo-add"
            :style="{ borderColor: colors.border }"
            @click="takeFixPhotos"
          >
            <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
          </view>
        </view>
        <textarea
          v-model="fixRemark"
          class="remark"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="完工说明（必填）"
          :maxlength="500"
        />
        <view class="btn-primary" :style="{ backgroundColor: acting ? colors.info : colors.primary }" @click="doFinish">
          <text class="btn-primary-text" :style="{ color: colors.white }">{{ acting ? '提交中…' : '提交完工' }}</text>
        </view>
      </view>

      <!-- 流转记录 -->
      <view v-if="order.logs.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">流转记录</text>
        <view v-for="(l, idx) in order.logs" :key="idx" class="log">
          <view class="log-dot" :style="{ backgroundColor: colors.primary }"></view>
          <view class="log-body">
            <view class="log-head">
              <text class="log-action" :style="{ color: colors.textRegular }">{{ logActionText(l.action) }}</text>
              <text class="log-time" :style="{ color: colors.textSecondary }">{{ l.created_at }}</text>
            </view>
            <text class="log-detail" :style="{ color: colors.textSecondary }">{{ l.operator_name }}：{{ l.detail }}</text>
          </view>
        </view>
      </view>

      <view class="bottom-space"></view>
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
import {
  apiOrderDetail,
  apiAcceptOrder,
  apiFinishOrder,
  apiUploadLocal,
  WorkOrderDetail
} from '@/services/api'
import { burnWatermark } from '@/utils/watermark'
import { useAuthStore } from '@/stores/auth'

type DetailData = {
  colors: ColorTokens
  orderId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  order: WorkOrderDetail | null
  /** 我的角色：reporter=我上报的 / assignee=指派给我（由详情字段与当前用户比对得出） */
  myRole: string
  myName: string
  fixPhotos: string[]
  fixRemark: string
  acting: boolean
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

/** 取照片可预览 URL：优先水印图 */
function photoUrl(p: { url: string; watermarked_url: string }): string {
  return p.watermarked_url != '' ? p.watermarked_url : p.url
}

export default {
  data(): DetailData {
    return {
      colors: Colors,
      orderId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      order: null,
      myRole: '',
      myName: '',
      fixPhotos: [] as string[],
      fixRemark: '',
      acting: false,
      canvasW: 0,
      canvasH: 0
    }
  },
  computed: {
    statusText(): string {
      const s = this.order != null ? this.order.status : ''
      if (s == 'pending') return '待派单'
      if (s == 'assigned') return '待接单'
      if (s == 'processing') return '处理中'
      if (s == 'review') return '待审核'
      if (s == 'closed') return '已完成'
      if (s == 'rejected') return '已驳回'
      return s
    },
    statusColor(): string {
      const s = this.order != null ? this.order.status : ''
      if (s == 'pending') return Colors.info
      if (s == 'assigned') return Colors.warning
      if (s == 'processing') return Colors.primary
      if (s == 'review') return Colors.warning
      if (s == 'closed') return Colors.success
      if (s == 'rejected') return Colors.danger
      return Colors.info
    },
    priorityText(): string {
      const p = this.order != null ? this.order.priority : ''
      if (p == 'urgent') return '紧急'
      if (p == 'high') return '高'
      if (p == 'low') return '低'
      return '普通'
    },
    priorityColor(): string {
      const p = this.order != null ? this.order.priority : ''
      if (p == 'urgent') return Colors.danger
      if (p == 'high') return Colors.warning
      return Colors.textSecondary
    },
    myRoleText(): string {
      return this.myRole == 'assignee' ? '指派给我' : '我上报的'
    },
    /** 上报照片 URL 列表 */
    beforePhotoUrls(): string[] {
      if (this.order == null) return []
      return this.order.photos.map(photoUrl)
    },
    /** 维修照片 URL 列表 */
    fixPhotoUrls(): string[] {
      if (this.order == null) return []
      return this.order.fix_photos.map(photoUrl)
    },
    /** 指派给我 + 待接单 → 可接单 */
    canAccept(): boolean {
      return this.myRole == 'assignee' && this.order != null && this.order.status == 'assigned'
    },
    /** 指派给我 + 处理中 → 可提交完工 */
    canFinish(): boolean {
      return this.myRole == 'assignee' && this.order != null && this.order.status == 'processing'
    }
  },
  onLoad(options: any) {
    this.orderId = options && options.id ? String(options.id) : ''
    const u = useAuthStore().userInfo
    this.myName = u != null ? u.name : ''
    this.load()
  },
  methods: {
    load() {
      if (this.orderId == '') {
        this.loading = false
        this.errorMsg = '缺少工单参数'
        return
      }
      this.loading = true
      apiOrderDetail(this.orderId)
        .then((res) => {
          this.order = res
          // 客户端比对当前用户得出我的角色（详情接口不下发 my_role）
          const u = useAuthStore().userInfo
          const uid = u != null ? u.id : ''
          this.myRole = res.assignee_id != null && res.assignee_id == uid && res.reporter_id != uid
            ? 'assignee'
            : 'reporter'
          this.loading = false
          this.loaded = true
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
        })
    },
    /** 照片预览 */
    preview(urls: string[], idx: number) {
      uni.previewImage({ urls: urls, current: urls[idx] })
    },
    logActionText(a: string): string {
      if (a == 'create') return '创建工单'
      if (a == 'assign') return '派单'
      if (a == 'accept') return '接单'
      if (a == 'finish') return '提交完工'
      if (a == 'review_pass') return '审核通过'
      if (a == 'review_reject') return '审核驳回'
      if (a == 'close') return '关闭'
      return a
    },
    /** 接单 */
    doAccept() {
      if (this.acting || this.order == null) return
      this.acting = true
      apiAcceptOrder(this.order.id)
        .then(() => {
          this.acting = false
          uni.showToast({ title: '已接单', icon: 'success' })
          this.load()
        })
        .catch((e: Error) => {
          this.acting = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 拍维修照片（仅相机）→ 水印烧录 → 入列表 */
    takeFixPhotos() {
      const remain = 6 - this.fixPhotos.length
      if (remain <= 0) return
      uni.chooseImage({
        count: remain,
        sourceType: ['camera'],
        success: (res) => {
          const paths = (res.tempFilePaths || []) as string[]
          if (paths.length == 0) return
          uni.showLoading({ title: '处理中…', mask: true })
          const lines = this.wmLines()
          let chain: Promise<void> = Promise.resolve()
          paths.forEach((p) => {
            chain = chain
              .then(() => burnWatermark(p, lines, 'wmCanvas', this))
              .then((burned) => {
                this.fixPhotos.push(burned)
              })
          })
          chain
            .then(() => uni.hideLoading())
            .catch(() => uni.hideLoading())
        }
      })
    },
    removeFixPhoto(idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) this.fixPhotos.splice(idx, 1)
        }
      })
    },
    /** 水印行：时间+单号 / 维修人 */
    wmLines(): string[] {
      const lines: string[] = []
      const no = this.order != null ? this.order.order_no : ''
      lines.push(fmtDateTime(new Date()) + ' ' + no)
      if (this.myName != '') {
        lines.push('维修人：' + this.myName)
      }
      return lines
    },
    /** 提交完工：上传照片换 file_key → finish */
    doFinish() {
      if (this.acting || this.order == null) return
      if (this.fixPhotos.length == 0) {
        uni.showToast({ title: '请至少拍 1 张维修照片', icon: 'none' })
        return
      }
      if (this.fixRemark.trim() == '') {
        uni.showToast({ title: '请填写完工说明', icon: 'none' })
        return
      }
      this.acting = true
      uni.showLoading({ title: '上传中…', mask: true })
      const keys: string[] = []
      let chain: Promise<void> = Promise.resolve()
      this.fixPhotos.forEach((p) => {
        chain = chain
          .then(() => apiUploadLocal(p, 'workorder'))
          .then((r) => {
            keys.push(r.file_key)
          })
      })
      chain
        .then(() => {
          uni.showLoading({ title: '提交中…', mask: true })
          return apiFinishOrder((this.order as WorkOrderDetail).id, {
            fix_remark: this.fixRemark.trim(),
            fix_photos: keys.map((k) => ({ file_key: k }))
          })
        })
        .then(() => {
          uni.hideLoading()
          this.acting = false
          uni.showToast({ title: '已提交完工，待审核', icon: 'none' })
          this.fixPhotos = []
          this.fixRemark = ''
          this.load()
        })
        .catch((e: Error) => {
          uni.hideLoading()
          this.acting = false
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

.skeleton {
  padding-top: 8rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.sk-short {
  height: 96rpx;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 36rpx;
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.head-tags {
  flex-direction: row;
  margin-top: 16rpx;
}

.tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
  margin-right: 16rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
}

.sec-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.row {
  flex-direction: row;
  margin-top: 16rpx;
}

.row-label {
  font-size: 28rpx;
  width: 140rpx;
}

.row-value {
  font-size: 28rpx;
  flex: 1;
}

.desc {
  margin-top: 16rpx;
}

.desc-text {
  font-size: 28rpx;
  margin-top: 8rpx;
  line-height: 40rpx;
}

.snap-item {
  margin-top: 24rpx;
}

.snap-name {
  font-size: 30rpx;
}

.snap-remark {
  font-size: 26rpx;
  margin-top: 4rpx;
}

.snap-label {
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

.remark {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 16rpx;
  box-sizing: border-box;
}

.btn-primary {
  height: 104rpx;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  margin-top: 24rpx;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

.log {
  flex-direction: row;
  margin-top: 24rpx;
}

.log-dot {
  width: 16rpx;
  height: 16rpx;
  border-radius: 8rpx;
  margin-top: 10rpx;
  margin-right: 16rpx;
}

.log-body {
  flex: 1;
}

.log-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.log-action {
  font-size: 28rpx;
  font-weight: 600;
}

.log-time {
  font-size: 24rpx;
}

.log-detail {
  font-size: 26rpx;
  margin-top: 4rpx;
}

.bottom-space {
  height: 64rpx;
}
</style>
