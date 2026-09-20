<template>
  <view class="page bg-page" >
    <!-- 用户卡片（点头像更换） -->
    <view class="card bg-card" >
      <view class="avatar bg-brand-light"  @click="changeAvatar">
        <image v-if="avatarUrl != ''" class="avatar-img" :src="avatarUrl" mode="aspectFill" />
        <text v-else class="avatar-text text-brand" >{{ avatarText }}</text>
      </view>
      <view class="user-meta">
        <text class="user-name text-main" >{{ name }}</text>
        <text class="user-role text-secondary" >{{ postText }}</text>
        <text v-if="orgText != ''" class="user-role text-secondary" >{{ orgText }}</text>
      </view>
    </view>

    <!-- 管理功能（按权限点显隐；全部无权限时整个区块不显示）：wot wd-cell 列表行 + wd-badge 角标 -->
    <view v-if="showAdmin" class="card menu-card bg-card" >
      <text class="menu-group-title text-secondary" >管理功能</text>
      <wd-cell-group border>
        <wd-cell v-if="isSuperAdmin" title="当前公司" :value="currentTenantName" is-link clickable @click="switchTenant" />
        <wd-cell v-if="canDashboard" title="今日看板" is-link clickable @click="goAdmin('/pages/admin/dashboard')" />
        <wd-cell v-if="canReview" title="打卡审核" is-link clickable @click="goAdmin('/pages/admin/review')" />
        <wd-cell v-if="canPointManage" title="点位管理" is-link clickable @click="goAdmin('/pages/admin/points')" />
        <wd-cell v-if="canEquipment" is-link clickable @click="goAdmin('/pages/equipment/index')">
          <template #title>
            <wd-badge :value="equipmentDueCount" type="danger" :hidden="equipmentDueCount <= 0">
              <text class="cell-title">设备台账</text>
            </wd-badge>
          </template>
        </wd-cell>
        <wd-cell v-if="canEquipmentConfirm" is-link clickable @click="goAdmin('/pages/equipment/confirm')">
          <template #title>
            <wd-badge :value="equipmentPendingCount" type="danger" :hidden="equipmentPendingCount <= 0">
              <text class="cell-title">维保确认</text>
            </wd-badge>
          </template>
        </wd-cell>
      </wd-cell-group>
    </view>

    <!-- 功能入口 -->
    <view class="card menu-card bg-card" >
      <wd-cell-group border>
        <wd-cell title="手写签名" :value="signatureText" is-link clickable @click="openSignaturePad" />
        <wd-cell title="修改密码" is-link clickable @click="goPassword" />
        <wd-cell title="关于安巡云" :value="'v' + appVersion" is-link clickable @click="goAbout" />
      </wd-cell-group>
    </view>

    <!-- 切换账号：退出当前账号回登录页，已保存的账号可一键登录（微信风格，与退出登录上下排列） -->
    <wd-button block plain size="large" custom-class="btn-block-wd" @click="onSwitchAccount">切换账号</wd-button>

    <!-- 退出登录（danger 独立区块，二次确认） -->
    <wd-button block plain type="error" size="large" custom-class="btn-block-wd" @click="onLogout">退出登录</wd-button>

    <!-- 手写签名板（个人中心配置入口：保存即写入签章资产，下次签字直接用） -->
    <SignaturePad ref="pad" :show-save-option="false" @save="onPadSave" />

    <!-- 切换公司面板 / 头像来源面板（wd-action-sheet，选中即关） / 确认对话框（useDialog 函数式） -->
    <wd-action-sheet
      v-model="avatarSheetShow"
      title="更换头像"
      :actions="[{ name: '拍摄' }, { name: '从相册选择' }]"
      cancel-text="取消"
      @select="onAvatarSourceSelect"
    />
    <wd-action-sheet
      v-model="tenantSheetShow"
      title="切换公司"
      :actions="tenantActions"
      cancel-text="取消"
      @select="onTenantSelect"
    />
    <wd-dialog />

    <view class="tabbar-space"></view>
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { APP_VERSION } from '@/utils/appVersion'
import { apiUploadLocal, apiUpdateProfile, apiTenants, apiEquipmentDue, apiMaintenancePending } from '@/services/api'
import { withFileToken } from '@/utils/fileurl'
import { useAuthStore } from '@/stores/auth'
import { useTenantStore } from '@/stores/tenant'
import { toAbsUrl } from '@/utils/url'
import SignaturePad from '@/components/SignaturePad.vue'
import { useDialog } from '@/uni_modules/wot-ui/composables'

type ProfileData = {
  /** 安装包版本号（App 端取 plus.runtime，其他端回落默认值） */
  appVersion: string
  /** 设备台账角标：待维保台数（临期+逾期） */
  equipmentDueCount: number
  /** 维保确认角标：待确认登记数 */
  equipmentPendingCount: number
  /** 切换公司面板：待选租户列表（面板选择时按下标回取） */
  tenantSheetShow: boolean
  /** 头像来源选择面板 */
  avatarSheetShow: boolean
  tenantList: Array<{ id: string; name: string }>
}

export default {
  components: { SignaturePad },
  setup() {
    // wot 确认对话框（函数式调用，Promise 化；页面须挂一个 <wd-dialog /> 宿主）
    const dlg = useDialog()
    return { dlg }
  },
  data(): ProfileData {
    return {
      appVersion: APP_VERSION,
      equipmentDueCount: 0,
      equipmentPendingCount: 0,
      tenantSheetShow: false,
      avatarSheetShow: false,
      tenantList: []
    }
  },
  onLoad() {
    // #ifdef APP-PLUS
    const rt: any = plus.runtime
    if (rt != null && rt.version != null && rt.version != '') this.appVersion = rt.version
    // #endif
  },
  computed: {
    name(): string {
      const u = useAuthStore().userInfo
      return u != null && u.name != '' ? u.name : '未登录'
    },
    avatarText(): string {
      const u = useAuthStore().userInfo
      return u != null && u.name != '' ? u.name.substring(0, 1) : '?'
    },
    /** 头像 URL：avatar 存 file_id 对应的访问地址；空 = 显示姓氏占位 */
    avatarUrl(): string {
      const u = useAuthStore().userInfo
      if (u == null || u.avatar == null || u.avatar == '') return ''
      return withFileToken(toAbsUrl('/api/files/' + u.avatar))
    },
    /** 岗位：在职编制岗位名去重拼接；无编制回落角色名，皆无显示未分配 */
    postText(): string {
      const u = useAuthStore().userInfo
      if (u == null) return ''
      const names: string[] = []
      ;(u.staffs ?? []).forEach((s) => {
        ;(s.post_names ?? []).forEach((n) => {
          if (n != '' && names.indexOf(n) < 0) names.push(n)
        })
      })
      if (names.length > 0) return names.join(' / ')
      if (u.roles.length == 0) return '未分配岗位'
      const roleNames: Record<string, string> = {
        super_admin: '超级管理员',
        manager: '物业主管',
        inspector: '巡检员',
        repair: '维修人员'
      }
      return u.roles.map((r) => (roleNames[r] != null ? roleNames[r] : r)).join(' / ')
    },
    /** 所属组织：公司 · 小区（多小区顿号拼接） */
    orgText(): string {
      const u = useAuthStore().userInfo
      if (u == null) return ''
      const comms: string[] = []
      ;(u.staffs ?? []).forEach((s) => {
        if (s.community_name != '' && comms.indexOf(s.community_name) < 0) comms.push(s.community_name)
      })
      const tenant = u.tenant_name ?? ''
      if (tenant == '' && comms.length == 0) return ''
      if (comms.length == 0) return tenant
      if (tenant == '') return comms.join('、')
      return `${tenant} · ${comms.join('、')}`
    },
    signatureText(): string {
      const u = useAuthStore().userInfo
      return u != null && (u.signature_url ?? '') != '' ? '已配置' : '未设置'
    },
    /** 今日看板入口：inspection:task:list 或 inspection:task:monitor 任一 */
    canDashboard(): boolean {
      return useAuthStore().hasPerm(['inspection:task:list', 'inspection:task:monitor'])
    },
    /** 打卡审核入口 */
    canReview(): boolean {
      return useAuthStore().hasPerm('inspection:checkin:review')
    },
    /** 点位管理入口 */
    canPointManage(): boolean {
      return useAuthStore().hasPerm('inspection:point:list')
    },
    /** 设备台账入口（逾期数角标） */
    canEquipment(): boolean {
      return useAuthStore().hasPerm('equipment:list')
    },
    /** 维保确认入口（pending 数角标；默认仅经理/管理员角色） */
    canEquipmentConfirm(): boolean {
      return useAuthStore().hasPerm('equipment:confirm')
    },
    /** 管理区块整体显隐：任一入口可见即显示 */
    showAdmin(): boolean {
      return this.canDashboard || this.canReview || this.canPointManage || this.canEquipment || this.canEquipmentConfirm
    },
    /** 是否超级管理员（「当前公司」切换入口仅超管可见） */
    isSuperAdmin(): boolean {
      const u = useAuthStore().userInfo
      return u != null && u.roles.indexOf('super_admin') >= 0
    },
    /** 当前公司名（未选择 = 默认租户） */
    currentTenantName(): string {
      const t = useTenantStore().tenantName
      return t != '' ? t : '默认租户'
    },
    /** 切换公司面板的选项 */
    tenantActions(): Array<{ name: string }> {
      return this.tenantList.map((t) => ({ name: t.name }))
    }
  },
  onShow() {
    // 兜底拉一次个人信息（登录回包无 user 或角色/签名变更时刷新）
    const store = useAuthStore()
    if (store.isLoggedIn) {
      store.fetchProfile().catch((_e: any) => {})
    }
    // 设备台账角标（失败静默：无权限/未上线不打扰）
    if (this.canEquipment) {
      apiEquipmentDue()
        .then((list) => {
          this.equipmentDueCount = list.length
        })
        .catch((_e: any) => {})
    }
    if (this.canEquipmentConfirm) {
      apiMaintenancePending(1, 1)
        .then((res) => {
          this.equipmentPendingCount = res.total
        })
        .catch((_e: any) => {})
    }
  },
  methods: {
    /** 修改密码页 */
    goPassword() {
      uni.navigateTo({ url: '/pages/profile/password' })
    },
    /** 切换账号：确认后退出当前账号，回落到账号选择页（已保存账号一键登录） */
    onSwitchAccount() {
      this.dlg.confirm({
        title: '切换账号',
        msg: '将退出当前账号并进入账号选择页，已保存的账号可一键登录。',
        confirmButtonText: '切换'
      }).then(() => {
        useAuthStore().logout('/pages/login/switch')
      }).catch(() => {})
    },
    /** 超管切换「当前公司」：拉租户列表 → 底部面板选择 → 写租户上下文（后续请求按所选租户隔离） */
    switchTenant() {
      uni.showLoading({ title: '加载中…', mask: true })
      apiTenants()
        .then((list) => {
          uni.hideLoading()
          if (list.length == 0) {
            uni.showToast({ title: '暂无公司', icon: 'none' })
            return
          }
          this.tenantList = list
          this.tenantSheetShow = true
        })
        .catch((e: Error) => {
          uni.hideLoading()
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    onTenantSelect(e: { item: { name: string }; index: number }) {
      const t = this.tenantList[e.index]
      if (t == null) return
      useTenantStore().set(t.id, t.name)
      uni.showToast({ title: '已切换到「' + t.name + '」', icon: 'none' })
    },
    /** 关于页 */
    goAbout() {
      uni.navigateTo({ url: '/pages/profile/about' })
    },
    /** 更换头像：底部面板选来源（拍摄/相册，替代系统来源框，观感与 uni-popup 统一）→ 单来源选图 → 上传（scene=avatar）→ PUT /profile 写 avatar → 刷新资料 */
    changeAvatar() {
      this.avatarSheetShow = true
    },
    onAvatarSourceSelect(e: { index: number }) {
      this.chooseAvatar(e.index == 0 ? ['camera'] : ['album'])
    },
    chooseAvatar(sourceType: string[]) {
      uni.chooseImage({
        count: 1,
        sizeType: ['compressed'],
        sourceType: sourceType,
        success: (res) => {
          const path = res.tempFilePaths[0]
          uni.showLoading({ title: '上传中…', mask: true })
          apiUploadLocal(path, 'avatar')
            .then((up) => {
              const u = useAuthStore().userInfo
              return apiUpdateProfile(u != null ? u.name : '', u != null ? u.phone : '', undefined, up.file_id)
            })
            .then(() => useAuthStore().fetchProfile())
            .then(() => {
              uni.hideLoading()
              uni.showToast({ title: '头像已更新', icon: 'success' })
            })
            .catch((e: Error) => {
              uni.hideLoading()
              uni.showToast({ title: e.message, icon: 'none' })
            })
        }
      })
    },
    /** 管理入口跳转（入口显隐已按权限控制） */
    goAdmin(url: string) {
      uni.navigateTo({ url: url })
    },
    openSignaturePad() {
      const pad: any = this.$refs.pad
      pad.open()
    },
    /** 保存签名：上传 PNG（scene=signature）→ PUT /profile 写入签章资产 */
    onPadSave(filePath: string, _saveForLater: boolean) {
      const pad: any = this.$refs.pad
      apiUploadLocal(filePath, 'signature')
        .then((up) => {
          const u = useAuthStore().userInfo
          return apiUpdateProfile(u != null ? u.name : '', u != null ? u.phone : '', up.file_id)
        })
        .then(() => useAuthStore().fetchProfile())
        .then(() => {
          pad.finish(true)
          uni.showToast({ title: '签名已保存，签字时将直接使用', icon: 'none' })
        })
        .catch((e: Error) => {
          pad.finish(false)
          uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    onLogout() {
      this.dlg.confirm({
        title: '退出登录',
        msg: '确定要退出当前账号吗？',
        confirmButtonText: '退出',
        confirmButtonProps: { type: 'danger' }
      }).then(() => {
        useAuthStore().logout()
      }).catch(() => {})
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
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
  flex-direction: row;
  align-items: center;
}

/* 菜单卡：覆盖 .card 的横向布局，功能行纵向堆叠 */
.menu-card {
  flex-direction: column;
  align-items: stretch;
  padding-top: 8rpx;
  padding-bottom: 8rpx;
}

.avatar {
  width: 112rpx;
  height: 112rpx;
  border-radius: 56rpx;
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
  overflow: hidden;
}

.avatar-img {
  width: 112rpx;
  height: 112rpx;
  border-radius: 56rpx;
}

.avatar-text {
  font-size: 48rpx;
  font-weight: 600;
}

.user-meta {
  flex: 1;
}

.user-name {
  font-size: 40rpx; /* FontSize.title */
  font-weight: 600;
}

.user-role {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.menu-group-title {
  font-size: 24rpx;
  padding-top: 16rpx;
}

/* 菜单列表行字号放大（年龄偏大用户可读性优先；wd-cell 默认 14px 偏小） */
.menu-card :deep(.wd-cell__title),
.menu-card .cell-title {
  font-size: 34rpx;
}

.menu-card :deep(.wd-cell__value) {
  font-size: 28rpx;
}

/* wd-button 大号块状按钮（custom-class 落在组件根节点） */
.btn-block-wd {
  margin-bottom: 24rpx;
}

.tabbar-space {
  height: 160rpx;
}
</style>
