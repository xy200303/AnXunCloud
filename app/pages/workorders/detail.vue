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
          <text v-if="myRoleText != ''" class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ myRoleText }}</text>
          <text v-if="slaOverdue" class="tag" :style="{ color: colors.danger, borderColor: colors.danger }">已超时</text>
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
          <text class="row-label" :style="{ color: colors.textSecondary }">来源</text>
          <text class="row-value" :style="{ color: colors.textRegular }">{{ sourceText }}</text>
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
        <view v-if="order.sla_deadline != ''" class="row">
          <text class="row-label" :style="{ color: colors.textSecondary }">期望完成</text>
          <text class="row-value" :style="{ color: slaOverdue ? colors.danger : colors.textRegular }">{{ order.sla_deadline }}</text>
        </view>
        <view v-if="order.description != ''" class="desc">
          <text class="row-label" :style="{ color: colors.textSecondary }">问题描述</text>
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

      <!-- 上报照片 -->
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

      <!-- 驳回/退回原因（受理驳回作废 或 验收退回返工时展示） -->
      <view v-if="showRejectReason" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.danger }">{{ order.status == 'closed_invalid' ? '作废原因' : '退回原因' }}</text>
        <text class="desc-text" :style="{ color: colors.textRegular }">{{ order.reject_reason }}</text>
      </view>

      <!-- 处理结果（完工后展示） -->
      <view v-if="order.finish_note != '' || finishPhotoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">处理结果</text>
        <text v-if="order.finish_note != ''" class="desc-text" :style="{ color: colors.textRegular }">{{ order.finish_note }}</text>
        <view v-if="finishPhotoUrls.length > 0" class="photos">
          <image
            v-for="(u, pi) in finishPhotoUrls"
            :key="pi"
            class="photo"
            :src="u"
            mode="aspectFill"
            @click="preview(finishPhotoUrls, pi)"
          />
        </view>
        <text v-if="order.finish_at != ''" class="card-sub" :style="{ color: colors.textSecondary }">完工时间：{{ order.finish_at }}</text>
      </view>

      <!-- 验收意见（验收后展示） -->
      <view v-if="order.confirm_note != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">验收意见</text>
        <text
          class="desc-text"
          :style="{ color: order.status == 'closed' ? colors.textRegular : colors.danger }"
        >{{ order.confirm_note }}</text>
        <text v-if="order.confirm_at != ''" class="card-sub" :style="{ color: colors.textSecondary }">
          验收人：{{ order.confirm_by_name }} · {{ order.confirm_at }}
        </text>
      </view>

      <!-- 操作区：抢单（工单池） -->
      <view v-if="canGrab" class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="btn-primary" :style="{ backgroundColor: acting ? colors.info : colors.primary }" @click="doGrab">
          <text class="btn-primary-text" :style="{ color: colors.white }">{{ acting ? '处理中…' : '立即抢单' }}</text>
        </view>
      </view>

      <!-- 操作区：完工表单（派给我 + 处理中） -->
      <view v-if="canFinish" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">完工反馈</text>
        <text class="sec-tip" :style="{ color: colors.textSecondary }">完工照片（必传，最多 6 张，长按照片可删除）</text>
        <view class="photos">
          <image
            v-for="(ph, pi) in finishPhotos"
            :key="pi"
            class="photo"
            :src="ph"
            mode="aspectFill"
            @longpress="removeFinishPhoto(pi)"
          />
          <view
            v-if="finishPhotos.length < 6"
            class="photo-add"
            :style="{ borderColor: colors.border }"
            @click="takeFinishPhotos"
          >
            <text class="photo-add-text" :style="{ color: colors.textSecondary }">+拍照</text>
          </view>
        </view>
        <textarea
          v-model="finishNote"
          class="remark"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="完工说明（必填）"
          :maxlength="500"
        />
        <view class="btn-primary" :style="{ backgroundColor: acting ? colors.info : colors.primary }" @click="doFinish">
          <text class="btn-primary-text" :style="{ color: colors.white }">{{ acting ? '提交中…' : '提交完工' }}</text>
        </view>
      </view>

      <!-- 操作区：验收（我上报的 + 待验收） -->
      <view v-if="canConfirm" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">验收</text>
        <text class="sec-tip" :style="{ color: colors.textSecondary }">请核对处理结果：通过后工单闭环，不通过则退回处理人返工</text>
        <view class="confirm-actions">
          <view class="btn-half btn-outline" :style="{ borderColor: colors.danger }" @click="openReject">
            <text class="btn-half-text" :style="{ color: colors.danger }">验收退回</text>
          </view>
          <view
            class="btn-half"
            :style="{ backgroundColor: acting ? colors.info : colors.success }"
            @click="doConfirmPass"
          >
            <text class="btn-half-text" :style="{ color: colors.white }">{{ acting ? '提交中…' : '验收通过' }}</text>
          </view>
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
            <text class="log-detail" :style="{ color: colors.textSecondary }">{{ l.operator_name }}<text v-if="l.detail != ''">：{{ l.detail }}</text></text>
          </view>
        </view>
      </view>

      <view class="bottom-space"></view>
    </view>

    <!-- 验收退回原因弹层 -->
    <view v-if="rejecting" class="mask mask-center" :style="{ backgroundColor: colors.mask }" @click="rejecting = false">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }" @click.stop="">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">退回原因（必填）</text>
        <textarea
          v-model="rejectReason"
          class="dialog-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="请填写退回原因，将通知处理人重新处理"
          :maxlength="200"
        />
        <view class="dialog-actions">
          <text class="dialog-btn" :style="{ color: colors.textSecondary }" @click="rejecting = false">取消</text>
          <text class="dialog-btn" :style="{ color: colors.danger }" @click="doConfirmReject">确认退回</text>
        </view>
      </view>
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
  apiGrabOrder,
  apiFinishOrder,
  apiConfirmOrder,
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
  /** 当前用户 ID（角色/操作权限在客户端比对得出） */
  myId: string
  myName: string
  finishPhotos: string[]
  finishNote: string
  rejecting: boolean
  rejectReason: string
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
      myId: '',
      myName: '',
      finishPhotos: [] as string[],
      finishNote: '',
      rejecting: false,
      rejectReason: '',
      acting: false,
      canvasW: 0,
      canvasH: 0
    }
  },
  computed: {
    statusText(): string {
      const s = this.order != null ? this.order.status : ''
      if (s == 'reported') return '待受理'
      if (s == 'pending_dispatch') return '待派单'
      if (s == 'processing') return '处理中'
      if (s == 'pending_confirm') return '待验收'
      if (s == 'closed') return '已闭环'
      if (s == 'closed_invalid') return '已作废'
      return s
    },
    statusColor(): string {
      const s = this.order != null ? this.order.status : ''
      if (s == 'reported') return Colors.info
      if (s == 'pending_dispatch') return Colors.warning
      if (s == 'processing') return Colors.primary
      if (s == 'pending_confirm') return Colors.warning
      if (s == 'closed') return Colors.success
      if (s == 'closed_invalid') return Colors.danger
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
    /** 来源文案（对齐后端 Source* 枚举） */
    sourceText(): string {
      const s = this.order != null ? this.order.source : ''
      if (s == 'inspection') return '巡检异常转单'
      if (s == 'frontdesk') return '前台代录'
      return '主动上报'
    },
    /** 我的角色标签：我既是上报人又是处理人时优先展示「派给我」 */
    myRoleText(): string {
      if (this.order == null) return ''
      if (this.order.assignee_id != null && this.order.assignee_id == this.myId) return '派给我'
      if (this.order.reporter_id == this.myId) return '我上报的'
      return ''
    },
    /** 未闭环且已超 SLA 期望完成时间 */
    slaOverdue(): boolean {
      if (this.order == null) return false
      return this.order.sla_overdue && this.order.status != 'closed' && this.order.status != 'closed_invalid'
    },
    /** 上报照片 URL 列表 */
    beforePhotoUrls(): string[] {
      if (this.order == null) return []
      return this.order.photos.map(photoUrl)
    },
    /** 完工照片 URL 列表 */
    finishPhotoUrls(): string[] {
      if (this.order == null) return []
      return this.order.finish_photos.map(photoUrl)
    },
    /** 展示驳回/退回原因：作废（受理驳回）或处理中（验收退回返工） */
    showRejectReason(): boolean {
      if (this.order == null || this.order.reject_reason == '') return false
      return this.order.status == 'closed_invalid' || this.order.status == 'processing'
    },
    /** 工单池待派单工单（非我上报、未指派）→ 可抢单；抢单资格由后端可见性判定兜底 */
    canGrab(): boolean {
      return (
        this.order != null &&
        this.order.status == 'pending_dispatch' &&
        this.order.reporter_id != this.myId &&
        this.order.assignee_id == null
      )
    },
    /** 派给我 + 处理中 → 可提交完工 */
    canFinish(): boolean {
      return (
        this.order != null &&
        this.order.status == 'processing' &&
        this.order.assignee_id != null &&
        this.order.assignee_id == this.myId
      )
    },
    /** 我上报的 + 待验收 → 可验收 */
    canConfirm(): boolean {
      return (
        this.order != null &&
        this.order.status == 'pending_confirm' &&
        this.order.reporter_id == this.myId
      )
    }
  },
  onLoad(options: any) {
    this.orderId = options && options.id ? String(options.id) : ''
    const u = useAuthStore().userInfo
    this.myId = u != null ? u.id : ''
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
    /** 流转动作文案（对齐后端 model.Action* 枚举） */
    logActionText(a: string): string {
      if (a == 'create') return '上报工单'
      if (a == 'triage_pass') return '受理通过'
      if (a == 'triage_reject') return '受理驳回'
      if (a == 'dispatch') return '派单'
      if (a == 'grab') return '抢单'
      if (a == 'finish') return '完工提交'
      if (a == 'confirm_pass') return '验收通过'
      if (a == 'confirm_reject') return '验收退回'
      return a
    },
    /** 抢单（工单池） */
    doGrab() {
      if (this.acting || this.order == null) return
      uni.showModal({
        title: '抢单',
        content: '抢单后该工单将由你负责处理，确认抢单？',
        confirmText: '抢单',
        success: (r) => {
          if (!r.confirm || this.order == null) return
          this.acting = true
          apiGrabOrder(this.order.id)
            .then(() => {
              this.acting = false
              uni.showToast({ title: '抢单成功，请及时处理', icon: 'none' })
              this.load()
            })
            .catch((e: Error) => {
              this.acting = false
              uni.showToast({ title: e.message, icon: 'none' })
            })
        }
      })
    },
    /** 拍完工照片（仅相机）→ 水印烧录 → 入列表 */
    takeFinishPhotos() {
      const remain = 6 - this.finishPhotos.length
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
                this.finishPhotos.push(burned)
              })
          })
          chain
            .then(() => uni.hideLoading())
            .catch(() => uni.hideLoading())
        }
      })
    },
    removeFinishPhoto(idx: number) {
      uni.showModal({
        title: '删除照片',
        content: '确定删除这张照片吗？',
        success: (r) => {
          if (r.confirm) this.finishPhotos.splice(idx, 1)
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
      if (this.finishPhotos.length == 0) {
        uni.showToast({ title: '请至少拍 1 张完工照片', icon: 'none' })
        return
      }
      if (this.finishNote.trim() == '') {
        uni.showToast({ title: '请填写完工说明', icon: 'none' })
        return
      }
      this.acting = true
      uni.showLoading({ title: '上传中…', mask: true })
      const keys: string[] = []
      let chain: Promise<void> = Promise.resolve()
      this.finishPhotos.forEach((p) => {
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
            fix_remark: this.finishNote.trim(),
            fix_photos: keys.map((k) => ({ file_key: k }))
          })
        })
        .then(() => {
          uni.hideLoading()
          this.acting = false
          uni.showToast({ title: '已提交完工，待验收', icon: 'none' })
          this.finishPhotos = []
          this.finishNote = ''
          this.load()
        })
        .catch((e: Error) => {
          uni.hideLoading()
          this.acting = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    /** 验收通过（二次确认） */
    doConfirmPass() {
      if (this.acting || this.order == null) return
      uni.showModal({
        title: '验收通过',
        content: '确认处理结果验收通过？工单将闭环。',
        confirmText: '通过',
        success: (r) => {
          if (!r.confirm || this.order == null) return
          this.acting = true
          apiConfirmOrder((this.order as WorkOrderDetail).id, 'pass', '')
            .then(() => {
              this.acting = false
              uni.showToast({ title: '验收通过，工单已闭环', icon: 'none' })
              this.load()
            })
            .catch((e: Error) => {
              this.acting = false
              uni.showToast({ title: e.message, icon: 'none' })
            })
        }
      })
    },
    openReject() {
      this.rejectReason = ''
      this.rejecting = true
    },
    /** 验收退回（原因必填） */
    doConfirmReject() {
      if (this.acting || this.order == null) return
      const reason = this.rejectReason.trim()
      if (reason == '') {
        uni.showToast({ title: '请填写退回原因', icon: 'none' })
        return
      }
      this.acting = true
      apiConfirmOrder(this.order.id, 'reject', reason)
        .then(() => {
          this.acting = false
          this.rejecting = false
          uni.showToast({ title: '已退回处理人返工', icon: 'none' })
          this.load()
        })
        .catch((e: Error) => {
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

.confirm-actions {
  flex-direction: row;
  margin-top: 24rpx;
}

.btn-half {
  flex: 1;
  height: 88rpx;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
}

.btn-half:last-child {
  margin-right: 0;
}

.btn-outline {
  border-width: 2rpx;
  border-style: solid;
}

.btn-half-text {
  font-size: 30rpx;
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

/* 退回原因对话框 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 99;
}

.mask-center {
  justify-content: center;
  align-items: center;
}

.dialog {
  width: 600rpx;
  border-radius: 24rpx;
  padding: 32rpx;
}

.dialog-title {
  font-size: 32rpx;
  font-weight: 600;
}

.dialog-input {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 16rpx;
  font-size: 28rpx;
  margin-top: 24rpx;
}

.dialog-actions {
  flex-direction: row;
  justify-content: flex-end;
  margin-top: 24rpx;
}

.dialog-btn {
  font-size: 30rpx;
  padding: 8rpx 24rpx;
}
</style>
