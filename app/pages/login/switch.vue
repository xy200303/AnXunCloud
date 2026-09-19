<template>
  <view class="page bg-page" :style="{ paddingTop: statusPadTop }">
    <!-- 自定义导航栏（左侧返回登录页，右侧管理/完成切换删除模式） -->
    <view class="nav">
      <view class="nav-back" @click="goLogin"><uni-icons type="back" size="24" :color="'#1F2329'" /></view>
      <text class="nav-manage text-main"  @click="toggleManage">{{ managing ? '完成' : '管理' }}</text>
    </view>

    <view class="header">
      <text class="title text-main" >轻触头像以切换账号</text>
    </view>

    <!-- 账号列表（§18.4 uni-list 标准行：头像 + 名称/用户名 + 右侧状态；删除模式右侧出「删除」） -->
    <uni-list class="list bg-card" :style="{ borderRadius: '24rpx' }">
      <uni-list-item
        v-for="(acc, idx) in accounts"
        :key="acc.username + ':' + acc.tenant_code"
        :title="acc.remark != '' ? acc.remark : acc.username"
        :note="acc.username"
        clickable
        @click="onAccountTap(idx)"
      >
        <template #header>
          <view class="acc-avatar bg-brand-light" >
            <text class="acc-avatar-text text-brand" >{{ avatarText(acc) }}</text>
            <view v-if="loginKey == acc.username + ':' + acc.tenant_code" class="acc-loading bg-mask" >
              <text class="acc-loading-text text-white" >…</text>
            </view>
          </view>
        </template>
        <template #footer>
          <view>
            <uni-tag v-if="idx == 0 && !managing" text="最近使用" type="success" :inverted="true" size="small" />
            <text v-if="managing" class="acc-del text-danger"  @click.stop="onDeleteTap(idx)">删除</text>
          </view>
        </template>
      </uni-list-item>

      <!-- 添加账号：去登录页手动登录 -->
      <uni-list-item title="添加账号" clickable show-arrow @click="goLogin">
        <template #header>
          <view class="acc-avatar acc-add border-default" >
            <uni-icons type="plusempty" size="22" :color="'#86909C'" />
          </view>
        </template>
      </uni-list-item>
    </uni-list>

    <text v-if="errorMsg != ''" class="error text-danger" >{{ errorMsg }}</text>

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

import { useAuthStore } from '@/stores/auth'
import {
  SwitchAccount,
  loadSwitchAccounts,
  saveSwitchAccounts,
  upsertSwitchAccount
} from '@/utils/storage'
import AppDialog from '@/components/AppDialog.vue'

type SwitchData = {
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
  width: 88rpx;
  height: 88rpx;
  align-items: center;
  justify-content: center;
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
  border-radius: 24rpx;
  overflow: hidden;
  flex-direction: column;
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





.acc-del {
  font-size: 28rpx;
  padding: 16rpx 0 16rpx 24rpx;
}

/* 添加账号：虚线框占位头像 */
.acc-add {
  border-width: 2rpx;
  border-style: dashed;
}



.error {
  font-size: 26rpx;
  text-align: center;
  margin-top: 24rpx;
}
</style>
