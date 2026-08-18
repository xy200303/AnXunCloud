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

    <template v-else>
      <!-- 基本信息 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ order.title }}</text>
          <text class="card-status" :style="{ color: statusColor }">{{ statusText }}</text>
        </view>
        <text class="info-line" :style="{ color: colors.textSecondary }">单号：{{ order.order_no }}</text>
        <text class="info-line" :style="{ color: colors.textRegular }">
          {{ order.community_name }}<text v-if="order.point_name != ''"> · {{ order.point_name }}</text>
        </text>
        <text class="info-line" :style="{ color: colors.textRegular }">紧急度：{{ priorityText }}</text>
        <text class="info-line" :style="{ color: colors.textRegular }">上报人：{{ order.reporter_name }} · {{ order.created_at }}</text>
        <text class="info-line" :style="{ color: colors.textRegular }">
          处理人：{{ order.assignee_name != null && order.assignee_name != '' ? order.assignee_name : '未指派' }}
        </text>
        <text v-if="order.description != ''" class="info-line" :style="{ color: colors.textRegular }">描述：{{ order.description }}</text>
        <text v-if="order.category != ''" class="info-line" :style="{ color: colors.textRegular }">工单分类：{{ order.category }}</text>
      </view>

      <!-- 异常项快照 -->
      <view v-if="order.items.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">异常项</text>
        <view v-for="(it, idx) in order.items" :key="idx" class="check-item">
          <text class="check-item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
          <text v-if="it.remark != ''" class="check-item-note" :style="{ color: colors.textSecondary }">{{ it.remark }}</text>
          <view v-if="it.before_photo_urls.length > 0" class="photos">
            <image
              v-for="(u, pi) in absUrls(it.before_photo_urls)"
              :key="'b' + pi"
              class="photo"
              :src="u"
              mode="aspectFill"
              @click="preview(absUrls(it.before_photo_urls), pi)"
            />
          </view>
          <view v-if="it.after_photo_urls.length > 0" class="photos">
            <image
              v-for="(u, pi) in absUrls(it.after_photo_urls)"
              :key="'a' + pi"
              class="photo"
              :src="u"
              mode="aspectFill"
              @click="preview(absUrls(it.after_photo_urls), pi)"
            />
          </view>
        </view>
      </view>

      <!-- 上报照片 -->
      <view v-if="reportPhotoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">上报照片</text>
        <view class="photos">
          <image
            v-for="(u, pi) in reportPhotoUrls"
            :key="pi"
            class="photo"
            :src="u"
            mode="aspectFill"
            @click="preview(reportPhotoUrls, pi)"
          />
        </view>
      </view>

      <!-- 驳回/退回原因（分诊驳回作废 或 验收退回返工时展示） -->
      <view v-if="showRejectReason" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.danger }">{{ order.status == 'closed_invalid' ? '作废原因' : '退回原因' }}</text>
        <text class="info-line" :style="{ color: colors.textRegular }">{{ order.reject_reason }}</text>
      </view>

      <!-- 处理结果（完工后展示） -->
      <view v-if="order.finish_note != '' || fixPhotoUrls.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">处理结果</text>
        <text v-if="order.finish_note != ''" class="info-line" :style="{ color: colors.textRegular }">{{ order.finish_note }}</text>
        <text v-if="order.finish_at != ''" class="info-line" :style="{ color: colors.textSecondary }">完工时间：{{ order.finish_at }}</text>
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
      </view>

      <!-- 验收意见（验收后展示） -->
      <view v-if="order.confirm_note != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">验收意见</text>
        <text class="info-line" :style="{ color: colors.textRegular }">{{ order.confirm_note }}</text>
        <text v-if="order.confirm_at != ''" class="info-line" :style="{ color: colors.textSecondary }">
          验收人：{{ order.confirm_by_name }} · {{ order.confirm_at }}
        </text>
      </view>

      <!-- 流转记录 -->
      <view v-if="order.logs.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">流转记录</text>
        <view v-for="(l, idx) in order.logs" :key="idx" class="log-row">
          <text class="log-action" :style="{ color: colors.textPrimary }">{{ logActionText(l.action) }}</text>
          <text class="log-detail" :style="{ color: colors.textSecondary }">{{ l.operator_name }}<text v-if="l.detail != ''"> · {{ l.detail }}</text></text>
          <text class="log-time" :style="{ color: colors.textSecondary }">{{ l.created_at }}</text>
        </view>
      </view>

      <!-- 底部操作：按状态 + 权限显示（分诊 → 派单 → 验收） -->
      <view v-if="canTriage || canDispatch || canConfirmAct" class="bottom-actions" :style="{ backgroundColor: colors.bgCard, borderTopColor: colors.border }">
        <template v-if="canTriage">
          <view class="btn-action btn-action-outline" :style="{ borderColor: colors.danger }" @click="onTriageRejectTap">
            <text class="btn-action-text" :style="{ color: colors.danger }">分诊驳回</text>
          </view>
          <view class="btn-action" :style="{ backgroundColor: colors.primary }" @click="onTriagePass">
            <text class="btn-action-text" :style="{ color: colors.white }">分诊通过</text>
          </view>
        </template>
        <view
          v-if="canDispatch"
          class="btn-action"
          :style="{ backgroundColor: colors.primary }"
          @click="openAssign"
        >
          <text class="btn-action-text" :style="{ color: colors.white }">派单</text>
        </view>
        <template v-if="canConfirmAct">
          <view class="btn-action btn-action-outline" :style="{ borderColor: colors.danger }" @click="onRejectTap">
            <text class="btn-action-text" :style="{ color: colors.danger }">验收退回</text>
          </view>
          <view class="btn-action" :style="{ backgroundColor: colors.success }" @click="onPass">
            <text class="btn-action-text" :style="{ color: colors.white }">验收通过</text>
          </view>
        </template>
      </view>
      <view class="bottom-space"></view>
    </template>

    <!-- 派单选人弹层 -->
    <view v-if="assigning" class="mask" :style="{ backgroundColor: colors.mask }" @click="assigning = false">
      <view class="sheet" :style="{ backgroundColor: colors.bgPage, borderTopLeftRadius: '32rpx', borderTopRightRadius: '32rpx' }" @click.stop="">
        <view class="sheet-head">
          <text class="sheet-title" :style="{ color: colors.textPrimary }">选择处理人</text>
          <text class="sheet-close" :style="{ color: colors.textSecondary }" @click="assigning = false">×</text>
        </view>
        <text v-if="staffTip != ''" class="sheet-tip" :style="{ color: colors.warning }">{{ staffTip }}</text>
        <scroll-view scroll-y class="staff-list">
          <view v-if="staffLoading" class="staff-empty">
            <text class="staff-empty-text" :style="{ color: colors.textSecondary }">加载中…</text>
          </view>
          <view v-else-if="staff.length == 0" class="staff-empty">
            <text class="staff-empty-text" :style="{ color: colors.textSecondary }">暂无可选人员</text>
          </view>
          <view
            v-for="u in staff"
            :key="u.id"
            class="staff-row"
            :style="{ backgroundColor: colors.bgCard, borderColor: assigneeId == u.id ? colors.primary : colors.border }"
            @click="assigneeId = u.id"
          >
            <view class="staff-main">
              <text class="staff-name" :style="{ color: colors.textPrimary }">{{ u.name }}</text>
              <text class="staff-sub" :style="{ color: colors.textSecondary }">{{ u.username }}<text v-if="u.phone != ''"> · {{ u.phone }}</text></text>
            </view>
            <text v-if="assigneeId == u.id" class="staff-check" :style="{ color: colors.primary }">✓</text>
          </view>
        </scroll-view>
        <view class="sheet-foot" :style="{ backgroundColor: colors.bgCard, borderTopColor: colors.border }">
          <input
            v-model="assignRemark"
            class="remark-input"
            :style="{ borderColor: colors.border, color: colors.textPrimary }"
            placeholder="派单备注（选填）"
            :maxlength="100"
          />
          <view class="btn-action" :style="{ backgroundColor: colors.primary }" @click="onAssignConfirm">
            <text class="btn-action-text" :style="{ color: colors.white }">确认派单</text>
          </view>
        </view>
      </view>
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
          <text class="dialog-btn" :style="{ color: colors.danger }" @click="onRejectConfirm">确认退回</text>
        </view>
      </view>
    </view>

    <!-- 分诊驳回原因弹层（驳回后工单作废） -->
    <view v-if="triaging" class="mask mask-center" :style="{ backgroundColor: colors.mask }" @click="triaging = false">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }" @click.stop="">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">驳回原因（必填）</text>
        <textarea
          v-model="triageReason"
          class="dialog-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="请填写驳回原因，工单将作废并通知上报人"
          :maxlength="200"
        />
        <view class="dialog-actions">
          <text class="dialog-btn" :style="{ color: colors.textSecondary }" @click="triaging = false">取消</text>
          <text class="dialog-btn" :style="{ color: colors.danger }" @click="onTriageRejectConfirm">确认驳回</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import {
  apiManageOrderDetail,
  apiDispatchOrder,
  apiConfirmManageOrder,
  apiTriageOrder,
  apiOrderUsers,
  WorkOrderDetail,
  StaffUser
} from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { toAbsUrl } from '@/utils/url'

type DetailData = {
  colors: ColorTokens
  orderId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  order: WorkOrderDetail
  assigning: boolean
  staffLoading: boolean
  staff: StaffUser[]
  /** 无维修角色用户时的提示（显示全部启用用户） */
  staffTip: string
  assigneeId: string
  assignRemark: string
  rejecting: boolean
  rejectReason: string
  /** 分诊驳回弹层 */
  triaging: boolean
  triageReason: string
  acting: boolean
}

function emptyOrder(): WorkOrderDetail {
  return {
    id: '',
    order_no: '',
    checkin_id: null,
    title: '',
    community_id: '',
    community_name: '',
    point_id: null,
    point_name: '',
    description: '',
    photos: [],
    items: [],
    source: '',
    category: '',
    reporter_id: '',
    reporter_name: '',
    assignee_id: null,
    assignee_name: '',
    dispatcher_name: '',
    priority: '',
    status: '',
    triage_by_name: '',
    triage_at: '',
    triage_note: '',
    dispatch_at: '',
    accept_at: '',
    finish_photos: [],
    finish_note: '',
    finish_at: '',
    confirm_by_name: '',
    confirm_at: '',
    confirm_note: '',
    reject_reason: '',
    sla_deadline: '',
    sla_overdue: false,
    created_at: '',
    logs: []
  }
}

/** 工单状态文案（P2 六态，对齐后端 model.Order* 枚举） */
function statusTextOf(s: string): string {
  if (s == 'reported') return '待分诊'
  if (s == 'pending_dispatch') return '待派单'
  if (s == 'processing') return '处理中'
  if (s == 'pending_confirm') return '待验收'
  if (s == 'closed') return '已闭环'
  if (s == 'closed_invalid') return '已作废'
  return s
}

function statusColorOf(s: string): string {
  if (s == 'reported') return Colors.info
  if (s == 'pending_dispatch') return Colors.warning
  if (s == 'processing') return Colors.primary
  if (s == 'pending_confirm') return Colors.warning
  if (s == 'closed') return Colors.success
  if (s == 'closed_invalid') return Colors.danger
  return Colors.info
}

function priorityTextOf(p: string): string {
  if (p == 'urgent') return '紧急'
  if (p == 'high') return '高'
  if (p == 'low') return '低'
  return '普通'
}

/** 照片展示：优先水印图，其次原图，统一转绝对地址（与工单详情 photoUrl 同规则） */
function photoUrl(p: { url: string; watermarked_url: string }): string {
  return toAbsUrl(p.watermarked_url != '' ? p.watermarked_url : p.url)
}

export default {
  data(): DetailData {
    return {
      colors: Colors,
      orderId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      order: emptyOrder(),
      assigning: false,
      staffLoading: false,
      staff: [] as StaffUser[],
      staffTip: '',
      assigneeId: '',
      assignRemark: '',
      rejecting: false,
      rejectReason: '',
      triaging: false,
      triageReason: '',
      acting: false
    }
  },
  computed: {
    statusText(): string {
      return statusTextOf(this.order.status)
    },
    statusColor(): string {
      return statusColorOf(this.order.status)
    },
    priorityText(): string {
      return priorityTextOf(this.order.priority)
    },
    /** 分诊入口：待分诊 + workorder:triage 权限（通过 → 待派单；驳回 → 已作废） */
    canTriage(): boolean {
      return this.order.status == 'reported' && useAuthStore().hasPerm('workorder:triage')
    },
    /** 派单入口：待派单 + workorder:dispatch 权限（处理人须在本项目接单名单内，后端兜底校验） */
    canDispatch(): boolean {
      return this.order.status == 'pending_dispatch' && useAuthStore().hasPerm('workorder:dispatch')
    },
    /** 验收入口：待验收 + workorder:confirm 权限 */
    canConfirmAct(): boolean {
      return this.order.status == 'pending_confirm' && useAuthStore().hasPerm('workorder:confirm')
    },
    /** 展示驳回/退回原因：作废（分诊驳回）或处理中（验收退回返工） */
    showRejectReason(): boolean {
      if (this.order.reject_reason == '') return false
      return this.order.status == 'closed_invalid' || this.order.status == 'processing'
    },
    reportPhotoUrls(): string[] {
      return this.order.photos.map(photoUrl)
    },
    fixPhotoUrls(): string[] {
      return this.order.finish_photos.map(photoUrl)
    }
  },
  onLoad(options: any) {
    this.orderId = options && options.id ? String(options.id) : ''
    this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    load() {
      if (this.orderId == '') {
        this.loading = false
        this.errorMsg = '缺少工单参数'
        return
      }
      this.loading = !this.loaded
      apiManageOrderDetail(this.orderId)
        .then((res) => {
          this.order = res
          this.loading = false
          this.loaded = true
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    absUrls(urls: string[]): string[] {
      return (urls ?? []).map((u) => toAbsUrl(u))
    },
    preview(urls: string[], idx: number) {
      uni.previewImage({ urls: urls, current: urls[idx] })
    },
    /** 流转动作文案（对齐后端 model.Action* 枚举） */
    logActionText(a: string): string {
      if (a == 'create') return '上报工单'
      if (a == 'triage_pass') return '分诊通过'
      if (a == 'triage_reject') return '分诊驳回'
      if (a == 'dispatch') return '派单'
      if (a == 'grab') return '抢单'
      if (a == 'finish') return '完工提交'
      if (a == 'confirm_pass') return '验收通过'
      if (a == 'confirm_reject') return '验收退回'
      return a
    },
    /** 分诊通过（二次确认）→ 待派单 */
    onTriagePass() {
      if (this.acting) return
      uni.showModal({
        title: '分诊通过',
        content: '确认该问题属实有效？通过后工单进入待派单。',
        confirmText: '通过',
        success: (res) => {
          if (!res.confirm) return
          this.acting = true
          apiTriageOrder(this.orderId, 'pass', '')
            .then(() => {
              uni.showToast({ title: '分诊通过，请派单', icon: 'none' })
              this.load()
            })
            .catch((e: Error) => {
              uni.showToast({ title: e.message, icon: 'none' })
            })
            .finally(() => {
              this.acting = false
            })
        }
      })
    },
    onTriageRejectTap() {
      this.triageReason = ''
      this.triaging = true
    },
    /** 分诊驳回（原因必填）→ 已作废 */
    onTriageRejectConfirm() {
      if (this.acting) return
      const reason = this.triageReason.trim()
      if (reason == '') {
        uni.showToast({ title: '请填写驳回原因', icon: 'none' })
        return
      }
      this.acting = true
      apiTriageOrder(this.orderId, 'reject', reason)
        .then(() => {
          uni.showToast({ title: '已驳回，工单作废', icon: 'none' })
          this.triaging = false
          this.load()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
        })
    },
    /** 打开派单弹层并加载候选人：优先维修角色，无则显示全部启用用户并提示 */
    openAssign() {
      this.assigning = true
      this.assigneeId = ''
      this.assignRemark = ''
      this.staffLoading = true
      apiOrderUsers()
        .then((list) => {
          // 注意：/system/users 的 roles 元素无 code 字段（后端 UserService.toItem 只下发 id/name），
          // 维修员按角色名「维修人员」匹配（code=repair 兜底，防后端后续补 code 字段）
          const repairs = list.filter((u) =>
            (u.roles ?? []).some((r: any) => r.code == 'repair' || r.name == '维修人员')
          )
          if (repairs.length > 0) {
            this.staff = repairs
            this.staffTip = ''
          } else {
            this.staff = list
            this.staffTip = '未找到维修角色的用户，已显示全部启用人员'
          }
          this.staffLoading = false
        })
        .catch((e: Error) => {
          this.staffLoading = false
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    onAssignConfirm() {
      if (this.acting) return
      if (this.assigneeId == '') {
        uni.showToast({ title: '请选择处理人', icon: 'none' })
        return
      }
      this.acting = true
      apiDispatchOrder(this.orderId, this.assigneeId, this.assignRemark.trim())
        .then(() => {
          uni.showToast({ title: '已派单', icon: 'none' })
          this.assigning = false
          this.load()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
        })
    },
    onPass() {
      if (this.acting) return
      uni.showModal({
        title: '验收通过',
        content: '确认维修结果验收通过？工单将闭环。',
        confirmText: '通过',
        success: (res) => {
          if (!res.confirm) return
          this.acting = true
          apiConfirmManageOrder(this.orderId, 'pass', '')
            .then(() => {
              uni.showToast({ title: '验收通过，工单已闭环', icon: 'none' })
              this.load()
            })
            .catch((e: Error) => {
              uni.showToast({ title: e.message, icon: 'none' })
            })
            .finally(() => {
              this.acting = false
            })
        }
      })
    },
    onRejectTap() {
      this.rejectReason = ''
      this.rejecting = true
    },
    onRejectConfirm() {
      if (this.acting) return
      const reason = this.rejectReason.trim()
      if (reason == '') {
        uni.showToast({ title: '请填写退回原因', icon: 'none' })
        return
      }
      this.acting = true
      apiConfirmManageOrder(this.orderId, 'reject', reason)
        .then(() => {
          uni.showToast({ title: '已退回处理人', icon: 'none' })
          this.rejecting = false
          this.load()
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.acting = false
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
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 8rpx;
}

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.check-item {
  margin-top: 16rpx;
}

.check-item-name {
  font-size: 28rpx;
  font-weight: 600;
}

.check-item-note {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.log-row {
  margin-top: 16rpx;
}

.log-action {
  font-size: 28rpx;
  font-weight: 600;
}

.log-detail {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.log-time {
  font-size: 22rpx;
  margin-top: 4rpx;
}

/* 底部操作栏 */
.bottom-actions {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  flex-direction: row;
  padding: 24rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
  z-index: 10;
}

.btn-action {
  flex: 1;
  height: 88rpx;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
}

.btn-action:last-child {
  margin-right: 0;
}

.btn-action-outline {
  border-width: 2rpx;
  border-style: solid;
}

.btn-action-text {
  font-size: 30rpx;
  font-weight: 600;
}

.bottom-space {
  height: 160rpx;
}

/* 派单弹层 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  justify-content: flex-end;
  z-index: 99;
}

.mask-center {
  justify-content: center;
  align-items: center;
}

.sheet {
  height: 70%;
}

.sheet-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  padding: 24rpx 32rpx 8rpx;
}

.sheet-title {
  font-size: 34rpx;
  font-weight: 600;
  flex: 1;
}

.sheet-close {
  font-size: 48rpx;
  padding: 0 16rpx;
  line-height: 48rpx;
}

.sheet-tip {
  font-size: 24rpx;
  padding: 0 32rpx 8rpx;
}

.staff-list {
  flex: 1;
  min-height: 0;
  padding: 8rpx 24rpx;
}

.staff-empty {
  align-items: center;
  padding-top: 96rpx;
}

.staff-empty-text {
  font-size: 26rpx;
}

.staff-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-radius: 16rpx;
  border-width: 2rpx;
  border-style: solid;
  padding: 24rpx;
  margin-bottom: 16rpx;
}

.staff-main {
  flex: 1;
}

.staff-name {
  font-size: 30rpx;
  font-weight: 600;
}

.staff-sub {
  font-size: 24rpx;
  margin-top: 4rpx;
}

.staff-check {
  font-size: 36rpx;
  margin-left: 16rpx;
}

.sheet-foot {
  padding: 24rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
}

.remark-input {
  width: 100%;
  height: 80rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  font-size: 28rpx;
  margin-bottom: 24rpx;
}

/* 退回原因对话框 */
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
