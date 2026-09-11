<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 用户卡片（点头像更换） -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <view class="avatar" :style="{ backgroundColor: colors.primaryLight }" @click="changeAvatar">
        <image v-if="avatarUrl != ''" class="avatar-img" :src="avatarUrl" mode="aspectFill" />
        <text v-else class="avatar-text" :style="{ color: colors.primary }">{{ avatarText }}</text>
      </view>
      <view class="user-meta">
        <text class="user-name" :style="{ color: colors.textPrimary }">{{ name }}</text>
        <text class="user-role" :style="{ color: colors.textSecondary }">{{ postText }}</text>
        <text v-if="orgText != ''" class="user-role" :style="{ color: colors.textSecondary }">{{ orgText }}</text>
      </view>
    </view>

    <!-- 管理功能（按权限点显隐；全部无权限时整个区块不显示） -->
    <view v-if="showAdmin" class="card menu-card" :style="{ backgroundColor: colors.bgCard }">
      <text class="menu-group-title" :style="{ color: colors.textSecondary }">管理功能</text>
      <view v-if="isSuperAdmin"  hover-class="hover-dim" class="row" @click="switchTenant">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">当前公司</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.primary }">{{ currentTenantName }} ></text>
      </view>
      <view v-if="canDashboard"  hover-class="hover-dim" class="row" @click="goAdmin('/pages/admin/dashboard')">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">今日看板</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
      </view>
      <view v-if="canReview"  hover-class="hover-dim" class="row" @click="goAdmin('/pages/admin/review')">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">打卡审核</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
      </view>
      <view v-if="canPointManage"  hover-class="hover-dim" class="row" @click="goAdmin('/pages/admin/points')">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">点位管理</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
      </view>
      <view v-if="canEquipment"  hover-class="hover-dim" class="row" @click="goAdmin('/pages/equipment/index')">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">设备台账</text>
        <view class="row-right">
          <text v-if="equipmentDueCount > 0" class="row-badge" :style="{ backgroundColor: colors.danger, color: colors.white }">{{ equipmentDueCount > 99 ? '99+' : equipmentDueCount }}</text>
          <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
        </view>
      </view>
      <view v-if="canEquipmentConfirm"  hover-class="hover-dim" class="row" @click="goAdmin('/pages/equipment/confirm')">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">维保确认</text>
        <view class="row-right">
          <text v-if="equipmentPendingCount > 0" class="row-badge" :style="{ backgroundColor: colors.danger, color: colors.white }">{{ equipmentPendingCount > 99 ? '99+' : equipmentPendingCount }}</text>
          <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
        </view>
      </view>
    </view>

    <!-- 功能入口 -->
    <view class="card menu-card" :style="{ backgroundColor: colors.bgCard }">
      <view  hover-class="hover-dim" class="row" @click="openSignaturePad">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">手写签名</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">{{ signatureText }} ></text>
      </view>
      <view  hover-class="hover-dim" class="row" @click="goPassword">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">修改密码</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">></text>
      </view>
      <view  hover-class="hover-dim" class="row" @click="goAbout">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">关于安巡云</text>
        <text  hover-class="hover-dim" class="row-arrow" :style="{ color: colors.textSecondary }">v{{ appVersion }} ></text>
      </view>
    </view>

    <!-- 退出登录（danger 独立区块，二次确认） -->
    <view  hover-class="hover-dim" class="btn-logout" :style="{ backgroundColor: colors.bgCard }" @click="onLogout">
      <text  hover-class="hover-dim" class="btn-logout-text" :style="{ color: colors.danger }">退出登录</text>
    </view>

    <!-- 手写签名板（个人中心配置入口：保存即写入签章资产，下次签字直接用） -->
    <SignaturePad ref="pad" :show-save-option="false" @save="onPadSave" />

    <!-- 切换公司面板 / 退出登录确认（自绘，替代原生 showActionSheet/showModal） -->
    <AppActionSheet
      :visible="tenantSheetShow"
      title="切换公司"
      :items="tenantNames"
      @update:visible="tenantSheetShow = $event"
      @select="onTenantSelect"
    />
    <AppDialog
      :visible="logoutDlgShow"
      kind="danger"
      title="退出登录"
      content="确定要退出当前账号吗？"
      confirm-text="退出"
      cancel-text="取消"
      @update:visible="logoutDlgShow = $event"
      @confirm="onLogoutConfirm"
    />

    <view class="tabbar-space"></view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { APP_VERSION } from '@/utils/appVersion'
import { apiUploadLocal, apiUpdateProfile, apiTenants, apiEquipmentDue, apiMaintenancePending } from '@/services/api'
import { withFileToken } from '@/utils/fileurl'
import { useAuthStore } from '@/stores/auth'
import { useTenantStore } from '@/stores/tenant'
import { toAbsUrl } from '@/utils/url'
import SignaturePad from '@/components/SignaturePad.vue'
import AppDialog from '@/components/AppDialog.vue'
import AppActionSheet from '@/components/AppActionSheet.vue'

type ProfileData = {
  colors: ColorTokens
  /** 安装包版本号（App 端取 plus.runtime，其他端回落默认值） */
  appVersion: string
  /** 设备台账角标：待维保台数（临期+逾期） */
  equipmentDueCount: number
  /** 维保确认角标：待确认登记数 */
  equipmentPendingCount: number
  /** 切换公司面板：待选租户列表（面板选择时按下标回取） */
  tenantSheetShow: boolean
  tenantList: Array<{ id: string; name: string }>
  /** 退出登录确认弹窗 */
  logoutDlgShow: boolean
}

export default {
  components: { SignaturePad, AppDialog, AppActionSheet },
  data(): ProfileData {
    return {
      colors: Colors,
      appVersion: APP_VERSION,
      equipmentDueCount: 0,
      equipmentPendingCount: 0,
      tenantSheetShow: false,
      tenantList: [],
      logoutDlgShow: false
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
    /** 切换公司面板的选项文案 */
    tenantNames(): string[] {
      return this.tenantList.map((t) => t.name)
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
    onTenantSelect(idx: number) {
      const t = this.tenantList[idx]
      if (t == null) return
      useTenantStore().set(t.id, t.name)
      uni.showToast({ title: '已切换到「' + t.name + '」', icon: 'none' })
    },
    /** 关于页 */
    goAbout() {
      uni.navigateTo({ url: '/pages/profile/about' })
    },
    /** 更换头像：选图 → 上传（scene=avatar）→ PUT /profile 写 avatar → 刷新资料 */
    changeAvatar() {
      uni.chooseImage({
        count: 1,
        sizeType: ['compressed'],
        sourceType: ['album', 'camera'],
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
      this.logoutDlgShow = true
    },
    onLogoutConfirm() {
      useAuthStore().logout()
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

.row {
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.menu-group-title {
  font-size: 24rpx;
  padding-top: 16rpx;
}

.row-text {
  font-size: 30rpx;
}

.row-arrow {
  font-size: 26rpx;
}

.row-right {
  flex-direction: row;
  align-items: center;
}

/* 入口角标（待维保/待确认数） */
.row-badge {
  font-size: 22rpx;
  border-radius: 20rpx;
  padding: 2rpx 14rpx;
  margin-right: 12rpx;
  overflow: hidden;
}

.btn-logout {
  height: 104rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-logout-text {
  font-size: 34rpx;
  font-weight: 600;
}

.tabbar-space {
  height: 160rpx;
}
</style>
