<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage, paddingTop: statusPadTop }">
    <!-- 品牌区 -->
    <view class="brand">
      <image class="logo" src="/static/brand/anxuncloud-lockup.png" mode="aspectFit" />
    </view>

    <!-- 已保存账号：点选一键登录（凭据存本机）；仅当有保存记录时显示 -->
    <view v-if="savedAccounts.length > 0" class="saved">
      <text class="saved-title" :style="{ color: colors.textSecondary }">选择账号登录</text>
      <scroll-view class="saved-scroll" scroll-x enhanced :show-scrollbar="false">
        <view class="saved-row">
          <view v-for="(acc, idx) in savedAccounts" :key="acc.username + ':' + acc.tenant_code" class="saved-item" hover-class="hover-dim" @click="quickLogin(idx)">
            <view class="saved-avatar" :style="{ backgroundColor: colors.primaryLight }">
              <text class="saved-avatar-text" :style="{ color: colors.primary }">{{ accountAvatarText(acc) }}</text>
              <view v-if="quickLoginUser == acc.username + ':' + acc.tenant_code" class="saved-loading" :style="{ backgroundColor: colors.mask }">
                <text class="saved-loading-text" :style="{ color: colors.white }">…</text>
              </view>
              <text class="saved-del" :style="{ backgroundColor: colors.danger, color: colors.white }" @click.stop="onAccountDeleteTap(idx)">×</text>
            </view>
            <text class="saved-name" :style="{ color: colors.textRegular }">{{ acc.remark != '' ? acc.remark : acc.username }}</text>
          </view>
        </view>
      </scroll-view>
    </view>

    <!-- 登录表单 -->
    <view class="form">
      <view class="input-wrap" :style="{ backgroundColor: colors.bgCard, borderColor: colors.border }">
        <input
          v-model="username"
          class="input"
          :style="{ color: colors.textPrimary }"
          placeholder="账号"
          placeholder-class="input-ph"
        />
      </view>
      <view class="input-wrap" :style="{ backgroundColor: colors.bgCard, borderColor: colors.border }">
        <input
          v-model="password"
          class="input input-pwd"
          :style="{ color: colors.textPrimary }"
          placeholder="密码"
          placeholder-class="input-ph"
          :password="!showPwd"
        />
        <image
          class="pwd-toggle"
          :src="showPwd ? '/static/icons/eye-off.png' : '/static/icons/eye.png'"
          mode="aspectFit"
          @click="togglePwd"
        />
      </view>
      <!-- 公司选择：仅当用户名存在于多家公司（后端 40109）时显示，选项来自 40109 响应的 tenants -->
      <view v-if="needTenantCode" class="input-wrap" :style="{ backgroundColor: colors.bgCard, borderColor: colors.border }">
        <picker class="picker" mode="selector" :range="tenantNames" :value="tenantIndex < 0 ? 0 : tenantIndex" @change="onTenantPick">
          <view class="picker-text" :style="{ color: tenantIndex < 0 ? colors.textSecondary : colors.textPrimary }">
            {{ tenantIndex < 0 ? '请选择所属公司' : tenantOptions[tenantIndex].name }}
          </view>
        </picker>
      </view>

      <!-- 错误内联提示（按钮上方，不用 Toast） -->
      <text v-if="errorMsg != ''" class="error" :style="{ color: colors.danger }">{{ errorMsg }}</text>

      <view
        class="btn-primary"
        :style="{ backgroundColor: loginBtnColor }"
        @click="doLogin"
      >
        <text class="btn-primary-text" :style="{ color: colors.white }">{{ loginBtnText }}</text>
      </view>
    </view>

    <!-- 其他方式 -->
    <view class="divider">
      <view class="divider-line" :style="{ backgroundColor: colors.border }"></view>
      <text class="divider-text" :style="{ color: colors.textSecondary }">其他方式</text>
      <view class="divider-line" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!--
      微信登录占位：
      - APP 端（Android/iOS/鸿蒙）走微信开放平台「移动应用」AppID 授权（code 换 unionid），AppID 待申请；
      - 微信小程序端改为一键登录主按钮（wx.login code 换会话），未绑定走绑定页。
      TODO(M5 前): 申请微信开放平台移动应用 AppID / 小程序 AppID 后接入。
    -->
    <view class="btn-wechat" :style="{ borderColor: colors.border }" @click="onWechatLogin">
      <image class="btn-wechat-icon" src="/static/icons/wechat.png" mode="aspectFit" />
      <text class="btn-wechat-text" :style="{ color: colors.textRegular }">微信登录</text>
    </view>

    <!-- 注册入口：仅当后端开关 auth.register_enabled 开启时显示 -->
    <text
      v-if="registerEnabled"
      class="link"
      :style="{ color: colors.primary }"
      @click="openRegister"
    >注册账号</text>
    <text class="helper" :style="{ color: colors.textSecondary }">忘记密码请联系管理员重置</text>

    <!-- 删除已保存账号确认 -->
    <AppDialog
      :visible="delDlgShow"
      kind="danger"
      title="删除账号"
      :content="delDlgContent"
      confirm-text="删除"
      cancel-text="取消"
      @update:visible="delDlgShow = $event"
      @confirm="onAccountDeleteConfirm"
    />

    <!-- 注册弹层（底部弹层，遮罩 45% 黑） -->
    <view v-if="showRegister" class="mask" :style="{ backgroundColor: colors.mask }" @click="closeRegister">
      <view class="sheet" :style="{ backgroundColor: colors.bgCard }" @click.stop="noop">
        <text class="sheet-title" :style="{ color: colors.textPrimary }">注册账号</text>

        <view class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <input v-model="regForm.username" class="input" :style="{ color: colors.textPrimary }" placeholder="用户名（4-20 位字母数字下划线）" placeholder-class="input-ph" />
        </view>
        <view class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <input v-model="regForm.password" class="input" :style="{ color: colors.textPrimary }" placeholder="密码（8-32 位，含字母与数字）" placeholder-class="input-ph" :password="true" />
        </view>
        <view class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <input v-model="regForm.confirm" class="input" :style="{ color: colors.textPrimary }" placeholder="确认密码" placeholder-class="input-ph" :password="true" />
        </view>
        <view class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <input v-model="regForm.name" class="input" :style="{ color: colors.textPrimary }" placeholder="姓名" placeholder-class="input-ph" />
        </view>
        <view class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <input v-model="regForm.phone" class="input" :style="{ color: colors.textPrimary }" placeholder="手机号（必填）" placeholder-class="input-ph" type="number" :maxlength="11" />
        </view>
        <!-- 所属公司：多租户时必选一个；仅 1 个租户时隐藏（默认租户自动归属） -->
        <view v-if="regTenants.length > 1" class="input-wrap sheet-input" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
          <picker class="picker" mode="selector" :range="regTenantNames" :value="regTenantIndex < 0 ? 0 : regTenantIndex" @change="onRegTenantPick">
            <view class="picker-text" :style="{ color: regTenantIndex < 0 ? colors.textSecondary : colors.textPrimary }">
              {{ regTenantIndex < 0 ? '请选择所属公司' : regTenants[regTenantIndex].name }}
            </view>
          </picker>
        </view>

        <text v-if="regError != ''" class="error" :style="{ color: colors.danger }">{{ regError }}</text>

        <view
          class="btn-primary"
          :style="{ backgroundColor: regBtnColor }"
          @click="doRegister"
        >
          <text class="btn-primary-text" :style="{ color: colors.white }">{{ regBtnText }}</text>
        </view>
        <text class="helper sheet-helper" :style="{ color: colors.textSecondary }">注册成功后请联系管理员分配角色与所辖小区</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiRegisterConfig, apiRegister, apiRegisterTenants } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import {
  SwitchAccount,
  loadSwitchAccounts,
  saveSwitchAccounts,
  upsertSwitchAccount
} from '@/utils/storage'
import AppDialog from '@/components/AppDialog.vue'

type RegisterForm = {
  username: string
  password: string
  confirm: string
  name: string
  phone: string
}

type TenantOption = {
  code: string
  name: string
}

type LoginData = {
  colors: ColorTokens
  statusBarH: number
  username: string
  password: string
  showPwd: boolean
  loading: boolean
  errorMsg: string
  /** 用户名跨租户重名（40109）时显示公司选择 */
  needTenantCode: boolean
  /** 40109 响应 data.tenants 下发的候选公司列表 */
  tenantOptions: TenantOption[]
  /** 登录公司下拉选中下标（-1 = 未选） */
  tenantIndex: number
  registerEnabled: boolean
  showRegister: boolean
  regLoading: boolean
  regError: string
  regForm: RegisterForm
  /** 注册可选公司列表（>1 时注册弹层显示"所属公司"选择） */
  regTenants: TenantOption[]
  /** 注册公司下拉选中下标（-1 = 未选） */
  regTenantIndex: number
  /** 本机已保存账号（一键登录列表） */
  savedAccounts: SwitchAccount[]
  /** 正在一键登录的账号 key（username:tenant_code），空 = 无 */
  quickLoginUser: string
  /** 删除账号确认弹窗 */
  delDlgShow: boolean
  delIndex: number
}

export default {
  data(): LoginData {
    return {
      colors: Colors,
      statusBarH: 20,
      username: '',
      password: '',
      showPwd: false,
      loading: false,
      errorMsg: '',
      needTenantCode: false,
      tenantOptions: [],
      tenantIndex: -1,
      registerEnabled: false,
      showRegister: false,
      regLoading: false,
      regError: '',
      regForm: {
        username: '',
        password: '',
        confirm: '',
        name: '',
        phone: ''
      } as RegisterForm,
      regTenants: [],
      regTenantIndex: -1,
      savedAccounts: [],
      quickLoginUser: '',
      delDlgShow: false,
      delIndex: -1
    }
  },
  components: { AppDialog },
  onShow() {
    // 每次进入登录页刷新已保存账号列表（切换账号退出回来 / 冷启动）
    this.savedAccounts = loadSwitchAccounts()
    this.quickLoginUser = ''
  },
  onLoad() {
    const win = uni.getWindowInfo()
    if (win.statusBarHeight != null && win.statusBarHeight > 0) {
      this.statusBarH = win.statusBarHeight
    }
    // 注册入口开关（匿名可读；失败时静默隐藏入口）
    apiRegisterConfig()
      .then((enabled) => { this.registerEnabled = enabled })
      .catch((_e: any) => {})
  },
  computed: {
    /** 状态栏占位高度（模板不做字符串拼接） */
    statusPadTop(): string {
      return `${this.statusBarH}px`
    },
    loginBtnColor(): string {
      return this.loading ? Colors.info : Colors.primary
    },
    loginBtnText(): string {
      return this.loading ? '登录中…' : '登 录'
    },
    regBtnColor(): string {
      return this.regLoading ? Colors.info : Colors.primary
    },
    regBtnText(): string {
      return this.regLoading ? '提交中…' : '注 册'
    },
    /** 登录公司下拉选项（picker range） */
    tenantNames(): string[] {
      return this.tenantOptions.map((t: TenantOption) => t.name)
    },
    /** 注册公司下拉选项（picker range） */
    regTenantNames(): string[] {
      return this.regTenants.map((t: TenantOption) => t.name)
    },
    /** 删除账号确认文案 */
    delDlgContent(): string {
      const acc = this.savedAccounts[this.delIndex]
      if (acc == null) return ''
      return '确定删除「' + (acc.remark != '' ? acc.remark : acc.username) + '」吗？仅删除本机保存的登录凭据。'
    }
  },
  methods: {
    /** 弹层内容区点击阻断冒泡（防止误关弹层） */
    noop() {},
    togglePwd() {
      this.showPwd = !this.showPwd
    },
    /** 登录公司下拉选择（picker change） */
    onTenantPick(e: any) {
      this.tenantIndex = Number(e.detail.value)
    },
    /** 注册公司下拉选择（picker change） */
    onRegTenantPick(e: any) {
      this.regTenantIndex = Number(e.detail.value)
    },
    doLogin() {
      if (this.loading) return
      const username = this.username.trim()
      const password = this.password
      if (username == '') {
        this.errorMsg = '请输入账号'
        return
      }
      if (password == '') {
        this.errorMsg = '请输入密码'
        return
      }
      if (this.needTenantCode && this.tenantIndex < 0) {
        this.errorMsg = '请选择所属公司'
        return
      }
      this.errorMsg = ''
      this.loading = true
      const tenantCode = this.needTenantCode && this.tenantIndex >= 0 ? this.tenantOptions[this.tenantIndex].code : undefined
      useAuthStore().login(username, password, tenantCode)
        .then(() => {
          this.loading = false
          // 登录成功保存账号到本机一键登录列表（同账号同公司覆盖更新并置顶）
          const u = useAuthStore().userInfo
          upsertSwitchAccount({
            username: username,
            password: password,
            tenant_code: tenantCode ?? '',
            remark: u != null ? u.name : ''
          })
          uni.reLaunch({ url: '/pages/tasks/today' })
        })
        .catch((e: Error & { code?: number; data?: { tenants?: TenantOption[] } }) => {
          this.loading = false
          // 用户名存在于多家公司（密码已验证）：展开公司下拉，选项来自响应 data.tenants
          if (e.code === 40109) {
            this.tenantOptions = e.data?.tenants ?? []
            this.needTenantCode = true
          }
          this.errorMsg = e.message
        })
    },
    onWechatLogin() {
      // 占位：微信开放平台 AppID / 小程序一键登录均未接入
      uni.showToast({ title: '微信登录待接入（AppID 申请中）', icon: 'none' })
    },
    /** 已保存账号头像占位字（备注名/用户名首字） */
    accountAvatarText(acc: SwitchAccount): string {
      const label = acc.remark != '' ? acc.remark : acc.username
      return label != '' ? label.substring(0, 1) : '?'
    },
    /** 点选已保存账号一键登录：直接用本机凭据调登录，成功直达首页 */
    quickLogin(idx: number) {
      if (this.loading || this.quickLoginUser != '') return
      const acc = this.savedAccounts[idx]
      if (acc == null) return
      this.errorMsg = ''
      this.quickLoginUser = acc.username + ':' + acc.tenant_code
      useAuthStore().login(acc.username, acc.password, acc.tenant_code != '' ? acc.tenant_code : undefined)
        .then(() => {
          // 置顶为最近使用（姓名可能已变更，顺带刷新）
          const u = useAuthStore().userInfo
          upsertSwitchAccount({
            username: acc.username,
            password: acc.password,
            tenant_code: acc.tenant_code,
            remark: u != null && u.name != '' ? u.name : acc.remark
          })
          uni.reLaunch({ url: '/pages/tasks/today' })
        })
        .catch((e: Error) => {
          this.quickLoginUser = ''
          // 凭据失效（如密码已改）：提示并回填账号密码，便于手动重登
          this.username = acc.username
          this.password = acc.password
          this.errorMsg = e.message
        })
    },
    onAccountDeleteTap(idx: number) {
      this.delIndex = idx
      this.delDlgShow = true
    },
    onAccountDeleteConfirm() {
      if (this.delIndex < 0) return
      this.savedAccounts.splice(this.delIndex, 1)
      saveSwitchAccounts(this.savedAccounts)
      this.delIndex = -1
    },
    openRegister() {
      this.regError = ''
      this.regTenantIndex = -1
      this.showRegister = true
      // 拉取可选公司列表（开关关闭或接口异常时静默按空列表处理，不显示下拉）
      apiRegisterTenants()
        .then((list) => { this.regTenants = list })
        .catch((_e: any) => { this.regTenants = [] })
    },
    closeRegister() {
      this.showRegister = false
    },
    doRegister() {
      if (this.regLoading) return
      const f = this.regForm
      f.username = f.username.trim()
      f.name = f.name.trim()
      f.phone = f.phone.trim()
      // 前端校验与后端 AuthService.Register 同口径
      if (!new RegExp('^[A-Za-z0-9_]{4,20}$').test(f.username)) {
        this.regError = '用户名需为 4-20 位字母、数字或下划线'
        return
      }
      if (!new RegExp('^(?=.*[A-Za-z])(?=.*\\d).{8,32}$').test(f.password)) {
        this.regError = '密码需 8-32 位且包含字母与数字'
        return
      }
      if (f.password != f.confirm) {
        this.regError = '两次输入的密码不一致'
        return
      }
      if (f.name == '') {
        this.regError = '请输入姓名'
        return
      }
      if (!new RegExp('^1\\d{10}$').test(f.phone)) {
        this.regError = '请输入正确的 11 位手机号'
        return
      }
      if (this.regTenants.length > 1 && this.regTenantIndex < 0) {
        this.regError = '请选择所属公司'
        return
      }
      this.regError = ''
      this.regLoading = true
      const tenantCode = this.regTenants.length > 1 && this.regTenantIndex >= 0 ? this.regTenants[this.regTenantIndex].code : undefined
      apiRegister(f.username, f.password, f.name, f.phone, tenantCode)
        .then(() => {
          this.regLoading = false
          this.showRegister = false
          uni.showToast({ title: '注册成功，请联系管理员分配角色与所辖小区后使用', icon: 'none' })
        })
        .catch((e: Error) => {
          this.regLoading = false
          this.regError = e.message
        })
    }
  }
}
</script>

<style scoped>
/* 布局尺寸见 utils/theme.ts（750 基准 rpx）；色值全部经 :style 绑定 */
.page {
  flex: 1;
  padding-left: 48rpx;
  padding-right: 48rpx;
}

.brand {
  align-items: center;
  margin-top: 96rpx;
  margin-bottom: 64rpx;
}

.logo {
  width: 480rpx;
  height: 203rpx; /* lockup 比例 1514:640 ≈ 2.37:1 */
}

.form {
  margin-bottom: 48rpx;
}

/* 已保存账号（一键登录）：横向滑动头像列表 */
.saved {
  margin-bottom: 40rpx;
}

.saved-title {
  font-size: 26rpx;
  margin-bottom: 20rpx;
}

.saved-scroll {
  width: 100%;
  white-space: nowrap;
}

.saved-row {
  flex-direction: row;
  align-items: flex-start;
}

.saved-item {
  width: 128rpx;
  align-items: center;
  margin-right: 24rpx;
}

.saved-avatar {
  width: 104rpx;
  height: 104rpx;
  border-radius: 52rpx;
  align-items: center;
  justify-content: center;
  position: relative;
}

.saved-avatar-text {
  font-size: 40rpx;
  font-weight: 600;
}

/* 一键登录中的遮罩（圆形覆盖头像） */
.saved-loading {
  position: absolute;
  left: 0;
  top: 0;
  width: 104rpx;
  height: 104rpx;
  border-radius: 52rpx;
  align-items: center;
  justify-content: center;
}

.saved-loading-text {
  font-size: 32rpx;
}

/* 删除角标（头像右上角 ×） */
.saved-del {
  position: absolute;
  right: -8rpx;
  top: -8rpx;
  width: 36rpx;
  height: 36rpx;
  border-radius: 18rpx;
  font-size: 24rpx;
  text-align: center;
  line-height: 36rpx;
}

.saved-name {
  font-size: 24rpx;
  margin-top: 12rpx;
  max-width: 128rpx;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.input-wrap {
  height: 104rpx; /* Size.btnHeight */
  border-radius: 20rpx; /* Radius.button */
  border-width: 1rpx;
  flex-direction: row;
  align-items: center;
  padding-left: 32rpx;
  padding-right: 32rpx;
  margin-bottom: 24rpx;
}

.sheet-input {
  margin-bottom: 16rpx;
}

.input {
  flex: 1;
  height: 104rpx;
  font-size: 34rpx; /* FontSize.bodyL */
}

/* 公司下拉（picker 占满 input-wrap，与输入框同高同字号） */
.picker {
  flex: 1;
  height: 104rpx;
}

.picker-text {
  height: 104rpx;
  line-height: 104rpx;
  font-size: 34rpx; /* FontSize.bodyL */
}

.input-ph {
  color: #86909c; /* Colors.textSecondary */
}

.input-pwd {
  flex: 1;
}

.pwd-toggle {
  width: 36rpx;
  height: 36rpx;
  padding: 16rpx 8rpx;
}

.error {
  font-size: 26rpx;
  line-height: 36rpx;
  margin-bottom: 16rpx;
}

.btn-primary {
  height: 104rpx; /* Size.btnHeight 主按钮全宽 */
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

.divider {
  flex-direction: row;
  align-items: center;
  margin-top: 16rpx;
  margin-bottom: 32rpx;
}

.divider-line {
  flex: 1;
  height: 1rpx;
}

.divider-text {
  font-size: 26rpx;
  margin-left: 24rpx;
  margin-right: 24rpx;
}

.btn-wechat {
  height: 96rpx;
  border-radius: 20rpx;
  border-width: 1rpx;
  flex-direction: row;
  align-items: center;
  justify-content: center;
}

.btn-wechat-icon {
  width: 40rpx;
  height: 40rpx;
  margin-right: 12rpx;
}

.btn-wechat-text {
  font-size: 30rpx; /* FontSize.body */
}

.link {
  font-size: 30rpx;
  text-align: center;
  margin-top: 48rpx;
  padding: 16rpx;
}

.helper {
  font-size: 26rpx;
  text-align: center;
  margin-top: 24rpx;
}

.sheet-helper {
  margin-bottom: 16rpx;
}

.mask {
  position: fixed;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  justify-content: flex-end;
}

.sheet {
  border-top-left-radius: 32rpx; /* Radius.sheet */
  border-top-right-radius: 32rpx;
  padding: 48rpx 32rpx;
  padding-bottom: env(safe-area-inset-bottom);
}

.sheet-title {
  font-size: 40rpx;
  font-weight: 600;
  text-align: center;
  margin-bottom: 32rpx;
}
</style>
