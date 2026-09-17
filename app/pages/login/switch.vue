<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage, paddingTop: statusPadTop }">
    <!-- 自定义导航栏（左侧返回登录页，右侧管理/完成切换删除模式） -->
    <view class="nav">
      <text class="nav-back" :style="{ color: colors.textPrimary }" @click="goLogin">‹</text>
      <text class="nav-manage" :style="{ color: colors.textPrimary }" @click="toggleManage">{{ managing ? '完成' : '管理' }}</text>
    </view>

    <view class="header">
      <text class="title" :style="{ color: colors.textPrimary }">轻触头像以切换账号</text>
    </view>

    <!-- 账号卡片列表 -->
    <view class="list">
      <view
        v-for="(acc, idx) in accounts"
        :key="acc.username + ':' + acc.tenant_code"
        class="acc-card"
        :style="{ backgroundColor: colors.bgCard }"
        hover-class="hover-dim"
        @click="onAccountTap(idx)"
      >
        <view class="acc-avatar" :style="{ backgroundColor: colors.primaryLight }">
          <text class="acc-avatar-text" :style="{ color: colors.primary }">{{ avatarText(acc) }}</text>
          <view v-if="loginKey == acc.username + ':' + acc.tenant_code" class="acc-loading" :style="{ backgroundColor: colors.mask }">
            <text class="acc-loading-text" :style="{ color: colors.white }">…</text>
          </view>
        </view>
        <view class="acc-meta">
          <text class="acc-name" :style="{ color: colors.textPrimary }">{{ acc.remark != '' ? acc.remark : acc.username }}</text>
          <text class="acc-sub" :style="{ color: colors.textSecondary }">{{ acc.username }}</text>
        </view>
        <text v-if="idx == 0 && !managing" class="acc-current" :style="{ color: colors.success }">● 最近使用</text>
        <text v-if="managing" class="acc-del" :style="{ color: colors.danger }" @click.stop="onDeleteTap(idx)">删除</text>
      </view>

      <!-- 添加账号：去登录页手动登录 -->
      <view class="acc-card" :style="{ backgroundColor: colors.bgCard }" hover-class="hover-dim" @click="goLogin">
        <view class="acc-avatar acc-add" :style="{ borderColor: colors.border }">
          <text class="acc-add-icon" :style="{ color: colors.textSecondary }">+</text>
        </view>
        <view class="acc-meta">
          <text class="acc-name acc-add-text" :style="{ color: colors.textSecondary }">添加账号</text>
        </view>
      </view>
    </view>

    <text v-if="errorMsg != ''" class="error" :style="{ color: colors.danger }">{{ errorMsg }}</text>

    <!-- 删除确认 -->
    <AppDialog
      :visible="delDlgShow"
      kind="danger"
      title="删除账号"
      :content="delDlgContent"
      confirm-text="删除"
      cancel-text="取消"
      @update:visible="delDlgShow = $event"
      @confirm="onDeleteConfirm"
    />
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { useAuthStore } from '@/stores/auth'
import {
  SwitchAccount,
  loadSwitchAccounts,
  saveSwitchAccounts,
  upsertSwitchAccount
} from '@/utils/storage'
import AppDialog from '@/components/AppDialog.vue'

type SwitchData = {
  colors: ColorTokens
  statusBarH: number
  accounts: SwitchAccount[]
  /** 删除管理模式（右上角 管理/完成 切换） */
  managing: boolean
  /** 正在一键登录的账号 key（username:tenant_code），空 = 无 */
  loginKey: string
  errorMsg: string
  delDlgShow: boolean
  delIndex: number
}

export default {
  components: { AppDialog },
  data(): SwitchData {
    return {
      colors: Colors,
      statusBarH: 20,
      accounts: [],
      managing: false,
      loginKey: '',
      errorMsg: '',
      delDlgShow: false,
      delIndex: -1
    }
  },
  onLoad() {
    const win = uni.getWindowInfo()
    if (win.statusBarHeight != null && win.statusBarHeight > 0) {
      this.statusBarH = win.statusBarHeight
    }
  },
  onShow() {
    this.accounts = loadSwitchAccounts()
    this.loginKey = ''
  },
  computed: {
    statusPadTop(): string {
      return `${this.statusBarH}px`
    },
    delDlgContent(): string {
      const acc = this.accounts[this.delIndex]
      if (acc == null) return ''
      return '确定删除「' + (acc.remark != '' ? acc.remark : acc.username) + '」吗？仅删除本机保存的登录凭据。'
    }
  },
  methods: {
    avatarText(acc: SwitchAccount): string {
      const label = acc.remark != '' ? acc.remark : acc.username
      return label != '' ? label.substring(0, 1) : '?'
    },
    /** 返回/添加账号：去登录页（本页由 reLaunch 打开，无返回栈） */
    goLogin() {
      uni.reLaunch({ url: '/pages/login/index' })
    },
    toggleManage() {
      this.managing = !this.managing
    },
    /** 点选账号一键登录：直接用本机凭据登录，成功直达首页；失败回填账号去登录页 */
    onAccountTap(idx: number) {
      if (this.managing || this.loginKey != '') return
      const acc = this.accounts[idx]
      if (acc == null) return
      this.errorMsg = ''
      this.loginKey = acc.username + ':' + acc.tenant_code
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
          this.loginKey = ''
          // 凭据失效（如密码已改）：回登录页并回填账号，手动重登后自动更新凭据
          uni.reLaunch({ url: '/pages/login/index?username=' + encodeURIComponent(acc.username) + '&err=' + encodeURIComponent(e.message) })
        })
    },
    onDeleteTap(idx: number) {
      this.delIndex = idx
      this.delDlgShow = true
    },
    onDeleteConfirm() {
      if (this.delIndex < 0) return
      this.accounts.splice(this.delIndex, 1)
      saveSwitchAccounts(this.accounts)
      this.delIndex = -1
      if (this.accounts.length == 0) this.managing = false
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding-left: 32rpx;
  padding-right: 32rpx;
}

.nav {
  height: 88rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.nav-back {
  font-size: 56rpx;
  line-height: 88rpx;
  width: 88rpx;
}

.nav-manage {
  font-size: 30rpx;
  padding: 16rpx 8rpx;
}

.header {
  align-items: center;
  margin-top: 120rpx;
  margin-bottom: 80rpx;
}

.title {
  font-size: 48rpx;
  font-weight: 600;
}

.list {
  flex-direction: column;
}

.acc-card {
  border-radius: 24rpx;
  padding: 28rpx 32rpx;
  margin-bottom: 24rpx;
  flex-direction: row;
  align-items: center;
}

.acc-avatar {
  width: 96rpx;
  height: 96rpx;
  border-radius: 16rpx;
  align-items: center;
  justify-content: center;
  margin-right: 28rpx;
  position: relative;
  overflow: hidden;
}

.acc-avatar-text {
  font-size: 40rpx;
  font-weight: 600;
}

.acc-loading {
  position: absolute;
  left: 0;
  top: 0;
  width: 96rpx;
  height: 96rpx;
  align-items: center;
  justify-content: center;
}

.acc-loading-text {
  font-size: 32rpx;
}

.acc-meta {
  flex: 1;
}

.acc-name {
  font-size: 34rpx;
  font-weight: 600;
}

.acc-sub {
  font-size: 26rpx;
  margin-top: 6rpx;
}

.acc-current {
  font-size: 26rpx;
}

.acc-del {
  font-size: 28rpx;
  padding: 16rpx 0 16rpx 24rpx;
}

/* 添加账号：虚线框占位头像 */
.acc-add {
  border-width: 2rpx;
  border-style: dashed;
}

.acc-add-icon {
  font-size: 48rpx;
}

.acc-add-text {
  font-weight: 400;
}

.error {
  font-size: 26rpx;
  text-align: center;
  margin-top: 24rpx;
}
</style>
