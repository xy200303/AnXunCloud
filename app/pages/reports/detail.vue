<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <template v-else-if="loaded && d != null">
      <!-- 头部：标题 + 状态 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <view class="head-row">
          <text class="title" :style="{ color: colors.textPrimary }">{{ d.title }}</text>
          <text class="status-tag" :style="{ color: statusColor, backgroundColor: colors.primaryLight }">{{ statusText }}</text>
        </view>
        <text class="sub" :style="{ color: colors.textSecondary }">{{ d.community_name }} · {{ d.period }} · 生成于 {{ d.created_at }}</text>
        <!-- 最近一次驳回原因 -->
        <view v-if="d.reject_reason != ''" class="reject-bar" :style="{ backgroundColor: '#FEF0EF' }">
          <text class="reject-text" :style="{ color: colors.danger }">最近驳回原因：{{ d.reject_reason }}</text>
        </view>
      </view>

      <!-- 汇总统计 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="section-title" :style="{ color: colors.textPrimary }">汇总统计</text>
        <view class="stats-grid">
          <view v-for="s in statsItems" :key="s.label" class="stats-cell">
            <text class="stats-value" :style="{ color: colors.textPrimary }">{{ s.value }}</text>
            <text class="stats-label" :style="{ color: colors.textSecondary }">{{ s.label }}</text>
          </view>
        </view>
      </view>

      <!-- 三级签字进度 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="section-title" :style="{ color: colors.textPrimary }">签字进度</text>

        <!-- 第一级：巡检员电子确认 -->
        <view class="sign-block">
          <text class="sign-block-title" :style="{ color: colors.textRegular }">
            巡检员电子确认{{ d.inspector_ids.length > 0 ? '（' + signedInspectorCount + '/' + d.inspector_ids.length + '）' : '' }}
          </text>
          <text v-if="d.inspector_ids.length == 0" class="sign-none" :style="{ color: colors.textSecondary }">无需签字（该级已跳过）</text>
          <view v-for="p in d.inspectors" :key="p.user_id" class="signer-row">
            <view class="signer-main">
              <text class="signer-name" :style="{ color: colors.textPrimary }">{{ p.name }}</text>
              <text v-if="p.proxy_name" class="proxy-tag" :style="{ color: colors.warning }">{{ p.proxy_name }} 代签：{{ p.proxy_reason }}</text>
              <text v-if="p.signed" class="signer-time" :style="{ color: colors.textSecondary }">{{ p.signed_at }}</text>
            </view>
            <image
              v-if="p.signed && p.signature_url"
              class="sign-img"
              :src="withFileToken(p.signature_url)"
              mode="aspectFit"
              @click="previewImage(p.signature_url)"
            />
            <text v-else class="signer-state" :style="{ color: p.signed ? colors.success : colors.warning }">
              {{ p.signed ? '已确认' : '待确认' }}
            </text>
          </view>
        </view>

        <!-- 第二级：安全主管审批 -->
        <view class="sign-block">
          <text class="sign-block-title" :style="{ color: colors.textRegular }">安全主管审批</text>
          <text v-if="d.supervisor_ids.length == 0" class="sign-none" :style="{ color: colors.textSecondary }">无需签字（该级已跳过）</text>
          <template v-else>
            <view v-if="d.supervisor_name != null" class="signer-row">
              <view class="signer-main">
                <text class="signer-name" :style="{ color: colors.textPrimary }">{{ d.supervisor_name }}</text>
                <text v-if="d.supervisor_remark != ''" class="signer-time" :style="{ color: colors.textSecondary }">审批意见：{{ d.supervisor_remark }}</text>
                <text class="signer-time" :style="{ color: colors.textSecondary }">{{ d.supervisor_at }}</text>
              </view>
              <image
                v-if="d.supervisor_signature_url"
                class="sign-img"
                :src="withFileToken(d.supervisor_signature_url)"
                mode="aspectFit"
                @click="previewImage(d.supervisor_signature_url)"
              />
              <text v-else class="signer-state" :style="{ color: colors.success }">已审批</text>
            </view>
            <view v-else>
              <view v-for="s in d.supervisors" :key="s.user_id" class="signer-row">
                <text class="signer-name" :style="{ color: colors.textPrimary }">{{ s.name }}</text>
                <text class="signer-state" :style="{ color: colors.warning }">待签</text>
              </view>
            </view>
          </template>
        </view>

        <!-- 第三级：物业经理终审 -->
        <view class="sign-block">
          <text class="sign-block-title" :style="{ color: colors.textRegular }">物业经理终审</text>
          <text v-if="d.manager_ids.length == 0" class="sign-none" :style="{ color: colors.textSecondary }">无需签字（该级已跳过）</text>
          <template v-else>
            <view v-if="d.manager_name != null" class="signer-row">
              <view class="signer-main">
                <text class="signer-name" :style="{ color: colors.textPrimary }">{{ d.manager_name }}</text>
                <text v-if="d.manager_remark != ''" class="signer-time" :style="{ color: colors.textSecondary }">终审意见：{{ d.manager_remark }}</text>
                <text class="signer-time" :style="{ color: colors.textSecondary }">{{ d.manager_at }}</text>
              </view>
              <image
                v-if="d.manager_signature_url"
                class="sign-img"
                :src="withFileToken(d.manager_signature_url)"
                mode="aspectFit"
                @click="previewImage(d.manager_signature_url)"
              />
              <text v-else class="signer-state" :style="{ color: colors.success }">已终审</text>
            </view>
            <view v-else>
              <view v-for="m in d.managers" :key="m.user_id" class="signer-row">
                <text class="signer-name" :style="{ color: colors.textPrimary }">{{ m.name }}</text>
                <text class="signer-state" :style="{ color: colors.warning }">待签</text>
              </view>
            </view>
          </template>
        </view>
      </view>

      <!-- 完整报告 PDF：报告相关人（含巡检员）均可查看，签字完成后仍可回看 -->
      <view class="btn" :style="{ backgroundColor: colors.primaryLight }" @click="openPdf">
        <text class="btn-text" :style="{ color: colors.primary }">查看完整报告 PDF</text>
      </view>

      <!-- 操作区：按当前环节 + 指定签字人名单显隐（与 PC 同口径） -->
      <view v-if="showInspectorSign || showProxySign || showSupervisorSign || showManagerSign || showRejectBtn || separationHint != ''" class="card" :style="{ backgroundColor: colors.bgCard }">
        <view
          v-if="showInspectorSign"
          class="btn"
          :style="{ backgroundColor: colors.primary, opacity: signing ? 0.6 : 1 }"
          @click="onInspectorSign"
        >
          <text class="btn-text" :style="{ color: colors.white }">签字通过</text>
        </view>
        <view
          v-if="showProxySign"
          class="btn"
          :style="{ backgroundColor: colors.warning, opacity: signing ? 0.6 : 1 }"
          @click="openProxy"
        >
          <text class="btn-text" :style="{ color: colors.white }">代签</text>
        </view>
        <view
          v-if="showSupervisorSign || showManagerSign"
          class="btn"
          :style="{ backgroundColor: colors.success, opacity: signing ? 0.6 : 1 }"
          @click="openApprove"
        >
          <text class="btn-text" :style="{ color: colors.white }">签字通过</text>
        </view>
        <view
          v-if="showRejectBtn"
          class="btn"
          :style="{ backgroundColor: colors.danger, opacity: signing ? 0.6 : 1 }"
          @click="openReject"
        >
          <text class="btn-text" :style="{ color: colors.white }">驳回</text>
        </view>
        <text v-if="separationHint != ''" class="hint" :style="{ color: colors.textSecondary }">{{ separationHint }}</text>
      </view>

      <view class="bottom-space"></view>
    </template>

    <!-- 加载失败 -->
    <view v-else class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <!-- 代签弹层：选被代签人 + 代签原因（必填） -->
    <view v-if="proxyShow" class="mask" :style="{ backgroundColor: colors.mask }">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">代签确认</text>
        <text class="dialog-tip" :style="{ color: colors.warning }">代签将记录你的身份与原因，报告签字栏会标注「由你代签」，请谨慎操作</text>
        <text class="dialog-label" :style="{ color: colors.textRegular }">被代签人</text>
        <view
          v-for="p in unsignedInspectors"
          :key="p.user_id"
          class="proxy-item"
          :style="{ borderColor: proxyUserId == p.user_id ? colors.primary : colors.border }"
          @click="proxyUserId = p.user_id"
        >
          <text :style="{ color: proxyUserId == p.user_id ? colors.primary : colors.textRegular }">{{ p.name }}</text>
        </view>
        <text class="dialog-label" :style="{ color: colors.textRegular }">代签原因（必填）</text>
        <textarea v-model="proxyReason" class="dialog-textarea" :style="{ borderColor: colors.border }" placeholder="如：巡检员休假/离职，主管代为确认" maxlength="100" />
        <view class="dialog-actions">
          <view class="dialog-btn" :style="{ borderColor: colors.border }" @click="proxyShow = false">
            <text :style="{ color: colors.textRegular }">取消</text>
          </view>
          <view class="dialog-btn dialog-btn-solid" :style="{ backgroundColor: colors.warning, opacity: signing ? 0.6 : 1 }" @click="confirmProxy">
            <text :style="{ color: colors.white }">确认代签</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 审批通过弹层：审批意见（选填） -->
    <view v-if="approveShow" class="mask" :style="{ backgroundColor: colors.mask }">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">{{ d != null && d.status == 'pending_manager' ? '终审通过' : '审批通过' }}</text>
        <text class="dialog-label" :style="{ color: colors.textRegular }">审批意见（选填）</text>
        <textarea v-model="approveRemark" class="dialog-textarea" :style="{ borderColor: colors.border }" placeholder="如：情况属实，同意" maxlength="100" />
        <view class="dialog-actions">
          <view class="dialog-btn" :style="{ borderColor: colors.border }" @click="approveShow = false">
            <text :style="{ color: colors.textRegular }">取消</text>
          </view>
          <view class="dialog-btn dialog-btn-solid" :style="{ backgroundColor: colors.success, opacity: signing ? 0.6 : 1 }" @click="confirmApprove">
            <text :style="{ color: colors.white }">通过</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 驳回弹层：驳回原因（必填），退回巡检员确认环节 -->
    <view v-if="rejectShow" class="mask" :style="{ backgroundColor: colors.mask }">
      <view class="dialog" :style="{ backgroundColor: colors.bgCard }">
        <text class="dialog-title" :style="{ color: colors.textPrimary }">驳回报告</text>
        <text class="dialog-label" :style="{ color: colors.textRegular }">驳回原因（必填，驳回后退回巡检员确认环节）</text>
        <textarea v-model="rejectReason" class="dialog-textarea" :style="{ borderColor: colors.border }" placeholder="如：覆盖率不达标，请核实后重新确认" maxlength="100" />
        <view class="dialog-actions">
          <view class="dialog-btn" :style="{ borderColor: colors.border }" @click="rejectShow = false">
            <text :style="{ color: colors.textRegular }">取消</text>
          </view>
          <view class="dialog-btn dialog-btn-solid" :style="{ backgroundColor: colors.danger, opacity: signing ? 0.6 : 1 }" @click="confirmReject">
            <text :style="{ color: colors.white }">驳回</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 手写签名板（未配置签名时签字前弹出） -->
    <SignaturePad ref="pad" :show-save-option="true" @save="onPadSave" @cancel="onPadCancel" />
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import {
  apiReportDetail,
  apiSignInspector,
  apiSignSupervisor,
  apiSignManager,
  apiUploadLocal,
  apiUpdateProfile,
  openReportPdf,
  hasPerm,
  ReportDetail,
  ReportInspector
} from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { withFileToken } from '@/utils/fileurl'
import SignaturePad from '@/components/SignaturePad.vue'

type StatItem = { label: string; value: string }

type DetailData = {
  colors: ColorTokens
  reportId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  d: ReportDetail | null
  signing: boolean
  proxyShow: boolean
  proxyUserId: string
  proxyReason: string
  approveShow: boolean
  approveRemark: string
  rejectShow: boolean
  rejectReason: string
}

function statusTextOf(s: string): string {
  if (s == 'pending_inspector') return '待巡检员确认'
  if (s == 'pending_supervisor') return '待主管审批'
  if (s == 'pending_manager') return '待经理终审'
  if (s == 'approved') return '已归档'
  return s
}

function pctText(v: any): string {
  const n = Number(v)
  return isNaN(n) ? '0%' : n + '%'
}

function numText(v: any): string {
  const n = Number(v)
  return isNaN(n) ? '0' : String(n)
}

export default {
  components: { SignaturePad },
  data(): DetailData {
    return {
      colors: Colors,
      reportId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      d: null,
      signing: false,
      proxyShow: false,
      proxyUserId: '',
      proxyReason: '',
      approveShow: false,
      approveRemark: '',
      rejectShow: false,
      rejectReason: ''
    }
  },
  computed: {
    uid(): string {
      const u = useAuthStore().userInfo
      return u != null ? u.id : ''
    },
    hasSignature(): boolean {
      const u = useAuthStore().userInfo
      return u != null && (u.signature_url ?? '') != ''
    },
    statusText(): string {
      return this.d != null ? statusTextOf(this.d.status) : ''
    },
    statusColor(): string {
      if (this.d == null) return Colors.primary
      if (this.d.status == 'approved') return Colors.success
      return Colors.primary
    },
    statsItems(): StatItem[] {
      if (this.d == null) return []
      const s = this.d.stats
      return [
        { label: '任务总数', value: numText(s.task_total) },
        { label: '已完成任务', value: numText(s.task_done) },
        { label: '逾期任务', value: numText(s.task_overdue) },
        { label: '点位覆盖率', value: pctText(s.coverage_rate) },
        { label: '异常打卡', value: numText(s.abnormal_count) },
        { label: '疑似作弊', value: numText(s.suspect_count) },
        { label: '新增工单', value: numText(s.wo_created) },
        { label: '工单闭环率', value: pctText(s.wo_close_rate) }
      ]
    },
    signedInspectorCount(): number {
      if (this.d == null) return 0
      return this.d.inspectors.filter((p) => p.signed).length
    },
    unsignedInspectors(): ReportInspector[] {
      if (this.d == null) return []
      return this.d.inspectors.filter((p) => !p.signed)
    },
    inSupervisorList(): boolean {
      return this.d != null && this.uid != '' && this.d.supervisor_ids.indexOf(this.uid) >= 0
    },
    inManagerList(): boolean {
      return this.d != null && this.uid != '' && this.d.manager_ids.indexOf(this.uid) >= 0
    },
    // 职责分离：已完成上一级签字（含代签）的用户不再显示本级通过按钮，改提示
    signedLevel1(): boolean {
      if (this.d == null || this.uid == '') return false
      return this.d.inspector_signed.some((e) => e.user_id == this.uid || e.proxy_by == this.uid)
    },
    isLevel2Signer(): boolean {
      return this.d != null && this.d.supervisor_by != null && this.d.supervisor_by == this.uid
    },
    showInspectorSign(): boolean {
      const d = this.d
      const u = useAuthStore().userInfo
      if (d == null || d.status != 'pending_inspector') return false
      if (!hasPerm(u, 'report:sign:inspector')) return false
      if (d.inspector_signed.some((e) => e.user_id == this.uid)) return false
      return this.uid != '' && d.inspector_ids.indexOf(this.uid) >= 0
    },
    showProxySign(): boolean {
      const d = this.d
      if (d == null || d.status != 'pending_inspector') return false
      if (!hasPerm(useAuthStore().userInfo, 'report:sign:proxy')) return false
      return d.inspectors.some((p) => !p.signed)
    },
    // 主管/经理签字不挂权限点：显隐以报告名单成员身份为准（与后端 service 校验同口径）
    showSupervisorSign(): boolean {
      return (
        this.d != null &&
        this.d.status == 'pending_supervisor' &&
        !this.signedLevel1 &&
        this.inSupervisorList
      )
    },
    showManagerSign(): boolean {
      return (
        this.d != null &&
        this.d.status == 'pending_manager' &&
        !this.isLevel2Signer &&
        this.inManagerList
      )
    },
    // 驳回不受职责分离限制（负向动作），但须在本级指定签字人名单内（与后端同口径）
    showRejectBtn(): boolean {
      const d = this.d
      if (d == null) return false
      if (d.status == 'pending_supervisor') return this.inSupervisorList
      if (d.status == 'pending_manager') return this.inManagerList
      return false
    },
    separationHint(): string {
      const d = this.d
      if (d == null) return ''
      if (d.status == 'pending_supervisor') {
        if (this.signedLevel1) return '你已完成巡检员确认，主管审批须由其他人员操作'
        if (!this.inSupervisorList) return '你不在本报告主管签字人名单内'
      }
      if (d.status == 'pending_manager') {
        if (this.isLevel2Signer) return '你已完成主管审批，终审须由其他人员操作'
        if (!this.inManagerList) return '你不在本报告经理签字人名单内'
      }
      return ''
    }
  },
  onLoad(options: any) {
    if (options != null && options['id'] != null) {
      this.reportId = String(options['id'])
    }
    // 先刷新个人信息（签名配置/权限点可能变化），再拉报告详情
    useAuthStore()
      .fetchProfile()
      .catch((_e: any) => {})
      .then(() => {
        this.load()
      })
  },
  methods: {
    load() {
      if (this.reportId == '') {
        this.loading = false
        this.errorMsg = '缺少报告 ID'
        return
      }
      this.loading = !this.loaded
      apiReportDetail(this.reportId)
        .then((res) => {
          this.d = res
          this.loading = false
          this.loaded = true
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          else uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    previewImage(url: string | null) {
      if (url == null || url == '') return
      uni.previewImage({ urls: [withFileToken(url)] })
    },
    withFileToken,
    openPdf() {
      if (this.reportId == '') return
      openReportPdf(this.reportId)
    },

    // ===== 签字时的手写签名补齐：已配置签名直接执行（sigKey 空串，后端取资产快照）；否则先弹签名板 =====
    withSignature(action: (sigKey: string) => void) {
      if (this.hasSignature) {
        action('')
        return
      }
      this.pendingAction = action
      const pad: any = this.$refs.pad
      pad.open()
    },
    /** 签名板保存：上传 PNG（scene=signature）；勾选保存则写入签章资产，否则仅本次签字使用 */
    onPadSave(filePath: string, saveForLater: boolean) {
      const pad: any = this.$refs.pad
      apiUploadLocal(filePath, 'signature')
        .then((up) => {
          if (saveForLater) {
            const u = useAuthStore().userInfo
            return apiUpdateProfile(u != null ? u.name : '', u != null ? u.phone : '', up.file_key)
              .then(() => useAuthStore().fetchProfile())
              .then(() => {
                uni.showToast({ title: '签名已保存，下次签字将直接使用', icon: 'none' })
                return ''
              })
          }
          return Promise.resolve(up.file_key)
        })
        .then((sigKey) => {
          pad.finish(true)
          const action = this.pendingAction
          this.pendingAction = null
          if (action != null) action(sigKey)
        })
        .catch((e: Error) => {
          pad.finish(false)
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    onPadCancel() {
      this.pendingAction = null
    },

    // ===== 巡检员电子确认 =====
    onInspectorSign() {
      if (this.signing) return
      uni.showModal({
        title: '电子确认',
        content: '确认已完成本期全部巡检工作，进行电子确认？',
        confirmText: '确认',
        success: (res) => {
          if (!res.confirm) return
          this.withSignature((sigKey) => {
            this.signing = true
            const body = sigKey != '' ? { signature_file_key: sigKey } : null
            apiSignInspector(this.reportId, body)
              .then((r) => {
                this.afterSign(r.status == 'pending_supervisor' ? '全员已确认，已流转主管审批' : '已确认签字')
              })
              .catch((e: Error) => {
                uni.showToast({ title: e.message, icon: 'none' })
              })
              .finally(() => {
                this.signing = false
              })
          })
        }
      })
    },

    // ===== 代签（须 report:sign:proxy；先选人被原因，再进签名板，用代签人本人签名） =====
    openProxy() {
      if (this.signing) return
      this.proxyUserId = ''
      this.proxyReason = ''
      this.proxyShow = true
    },
    confirmProxy() {
      if (this.signing) return
      if (this.proxyUserId == '') {
        uni.showToast({ title: '请选择被代签人', icon: 'none' })
        return
      }
      if (this.proxyReason.trim() == '') {
        uni.showToast({ title: '请填写代签原因', icon: 'none' })
        return
      }
      const proxyFor = this.proxyUserId
      const reason = this.proxyReason.trim()
      this.withSignature((sigKey) => {
        this.signing = true
        const body: any = { proxy_for: proxyFor, reason: reason }
        if (sigKey != '') body.signature_file_key = sigKey
        apiSignInspector(this.reportId, body)
          .then((r) => {
            this.proxyShow = false
            this.afterSign(r.status == 'pending_supervisor' ? '全员已确认，已流转主管审批' : '代签已记录')
          })
          .catch((e: Error) => {
            uni.showToast({ title: e.message, icon: 'none' })
          })
          .finally(() => {
            this.signing = false
          })
      })
    },

    // ===== 主管/经理审批通过 =====
    openApprove() {
      if (this.signing) return
      this.approveRemark = ''
      this.approveShow = true
    },
    confirmApprove() {
      if (this.signing) return
      const isManager = this.d != null && this.d.status == 'pending_manager'
      const remark = this.approveRemark.trim()
      this.withSignature((sigKey) => {
        this.signing = true
        const body: any = { action: 'approve', remark: remark }
        if (sigKey != '') body.signature_file_key = sigKey
        const req = isManager ? apiSignManager(this.reportId, body) : apiSignSupervisor(this.reportId, body)
        req
          .then(() => {
            this.approveShow = false
            this.afterSign(isManager ? '终审通过，报告已归档' : '审批通过，已流转下一环节')
          })
          .catch((e: Error) => {
            uni.showToast({ title: e.message, icon: 'none' })
          })
          .finally(() => {
            this.signing = false
          })
      })
    },

    // ===== 驳回（驳回原因必填，无需签名） =====
    openReject() {
      if (this.signing) return
      this.rejectReason = ''
      this.rejectShow = true
    },
    confirmReject() {
      if (this.signing) return
      if (this.rejectReason.trim() == '') {
        uni.showToast({ title: '驳回原因不能为空', icon: 'none' })
        return
      }
      const isManager = this.d != null && this.d.status == 'pending_manager'
      const body = { action: 'reject' as const, reason: this.rejectReason.trim() }
      this.signing = true
      const req = isManager ? apiSignManager(this.reportId, body) : apiSignSupervisor(this.reportId, body)
      req
        .then(() => {
          this.rejectShow = false
          this.afterSign('已驳回，退回巡检员确认环节')
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.signing = false
        })
    },

    afterSign(message: string) {
      uni.showToast({ title: message, icon: 'none' })
      this.load()
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

.head-row {
  flex-direction: row;
  align-items: flex-start;
}

.title {
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
  line-height: 46rpx;
}

.status-tag {
  font-size: 24rpx;
  padding: 6rpx 16rpx;
  border-radius: 12rpx;
  margin-left: 16rpx;
}

.sub {
  font-size: 24rpx;
  margin-top: 12rpx;
}

.reject-bar {
  border-radius: 12rpx;
  padding: 16rpx 20rpx;
  margin-top: 20rpx;
}

.reject-text {
  font-size: 24rpx;
  line-height: 34rpx;
}

.section-title {
  font-size: 32rpx;
  font-weight: 600;
  margin-bottom: 24rpx;
}

.stats-grid {
  flex-direction: row;
  flex-wrap: wrap;
}

.stats-cell {
  width: 25%;
  align-items: center;
  padding-top: 12rpx;
  padding-bottom: 12rpx;
}

.stats-value {
  font-size: 36rpx;
  font-weight: 600;
}

.stats-label {
  font-size: 22rpx;
  margin-top: 8rpx;
}

.sign-block {
  margin-bottom: 24rpx;
}

.sign-block-title {
  font-size: 28rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.sign-none {
  font-size: 26rpx;
}

.signer-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  padding-top: 12rpx;
  padding-bottom: 12rpx;
}

.signer-main {
  flex: 1;
}

.signer-name {
  font-size: 28rpx;
}

.proxy-tag {
  font-size: 22rpx;
  margin-top: 6rpx;
}

.signer-time {
  font-size: 22rpx;
  margin-top: 6rpx;
}

.sign-img {
  width: 160rpx;
  height: 72rpx;
  background-color: #ffffff;
  border-radius: 8rpx;
}

.signer-state {
  font-size: 24rpx;
}

.btn {
  height: 96rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 20rpx;
}

.btn-text {
  font-size: 32rpx;
  font-weight: 600;
}

.hint {
  font-size: 24rpx;
  line-height: 34rpx;
}

.bottom-space {
  height: 48rpx;
}

/* 弹层 */
.mask {
  position: fixed;
  left: 0;
  top: 0;
  right: 0;
  bottom: 0;
  z-index: 998;
  align-items: center;
  justify-content: center;
  padding: 48rpx;
}

.dialog {
  width: 100%;
  border-radius: 24rpx;
  padding: 32rpx;
}

.dialog-title {
  font-size: 34rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.dialog-tip {
  font-size: 24rpx;
  line-height: 34rpx;
  margin-bottom: 16rpx;
}

.dialog-label {
  font-size: 26rpx;
  margin-top: 16rpx;
  margin-bottom: 12rpx;
}

.proxy-item {
  border-width: 1rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 12rpx;
}

.dialog-textarea {
  width: 100%;
  height: 160rpx;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 20rpx;
  font-size: 28rpx;
  box-sizing: border-box;
}

.dialog-actions {
  flex-direction: row;
  justify-content: space-between;
  margin-top: 24rpx;
}

.dialog-btn {
  width: 46%;
  height: 88rpx;
  border-radius: 16rpx;
  border-width: 1rpx;
  border-style: solid;
  border-color: transparent;
  align-items: center;
  justify-content: center;
}

.dialog-btn-solid {
  border-width: 0;
}
</style>
