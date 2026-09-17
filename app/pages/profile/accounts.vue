<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 说明 -->
    <view class="tip-card" :style="{ backgroundColor: colors.primaryLight }">
      <text class="tip-text" :style="{ color: colors.primary }">测试工具：账号凭据仅保存在本机，点选即可免密切换。请勿在正式设备上保存真实账号。</text>
    </view>

    <!-- 已保存账号列表 -->
    <view class="card list-card" :style="{ backgroundColor: colors.bgCard }">
      <text class="group-title" :style="{ color: colors.textSecondary }">已保存账号（{{ accounts.length }}）</text>
      <view v-if="accounts.length == 0" class="empty">
        <text class="empty-text" :style="{ color: colors.textSecondary }">暂无保存的账号，先在下方添加</text>
      </view>
      <view v-for="(acc, idx) in accounts" :key="acc.username + ':' + acc.tenant_code" class="acc-row" hover-class="hover-dim" @click="onSwitchTap(idx)">
        <view class="acc-meta">
          <view class="acc-name-line">
            <text class="acc-name" :style="{ color: colors.textPrimary }">{{ acc.remark != '' ? acc.remark : acc.username }}</text>
            <text v-if="acc.username == currentUsername" class="acc-current" :style="{ color: colors.primary, borderColor: colors.primary }">当前</text>
          </view>
          <text class="acc-sub" :style="{ color: colors.textSecondary }">{{ acc.username }}{{ acc.tenant_code != '' ? ' · ' + acc.tenant_code : '' }}</text>
        </view>
        <text class="acc-del" :style="{ color: colors.danger }" @click.stop="onDeleteTap(idx)">删除</text>
      </view>
    </view>

    <!-- 添加账号 -->
    <view class="card form-card" :style="{ backgroundColor: colors.bgCard }">
      <text class="group-title" :style="{ color: colors.textSecondary }">添加账号</text>
      <view class="input-wrap" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
        <input v-model="form.username" class="input" :style="{ color: colors.textPrimary }" placeholder="账号（必填）" placeholder-class="input-ph" />
      </view>
      <view class="input-wrap" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
        <input v-model="form.password" class="input" :style="{ color: colors.textPrimary }" placeholder="密码（必填）" placeholder-class="input-ph" :password="true" />
      </view>
      <view class="input-wrap" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
        <input v-model="form.tenant_code" class="input" :style="{ color: colors.textPrimary }" placeholder="公司编码（跨公司重名时必填，可留空）" placeholder-class="input-ph" />
      </view>
      <view class="input-wrap" :style="{ backgroundColor: colors.bgPage, borderColor: colors.border }">
        <input v-model="form.remark" class="input" :style="{ color: colors.textPrimary }" placeholder="备注名（如：巡检员小张，可留空）" placeholder-class="input-ph" />
      </view>
      <text v-if="formError != ''" class="error" :style="{ color: colors.danger }">{{ formError }}</text>
      <view class="btn-add" :style="{ backgroundColor: colors.primary }" hover-class="hover-dim" @click="onAdd">
        <text class="btn-add-text" :style="{ color: colors.white }">保存到列表</text>
      </view>
    </view>

    <!-- 切换/删除确认 -->
    <AppDialog
      :visible="switchDlgShow"
      kind="primary"
      title="切换账号"
      :content="switchDlgContent"
      confirm-text="切换"
      cancel-text="取消"
      @update:visible="switchDlgShow = $event"
      @confirm="onSwitchConfirm"
    />
    <AppDialog
      :visible="deleteDlgShow"
      kind="danger"
      title="删除账号"
      :content="deleteDlgContent"
      confirm-text="删除"
      cancel-text="取消"
      @update:visible="deleteDlgShow = $event"
      @confirm="onDeleteConfirm"
    />
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { KEY_SWITCH_ACCOUNTS } from '@/utils/storage'
import { useAuthStore } from '@/stores/auth'
import AppDialog from '@/components/AppDialog.vue'

/** 保存的测试账号凭据（本机 storage，明文仅限开发测试用途） */
type SwitchAccount = {
  username: string
  password: string
  /** 公司编码：用户名跨租户重名时消歧，空 = 不传 */
  tenant_code: string
  /** 列表展示备注名，空 = 显示用户名 */
  remark: string
}

type AccountsData = {
  colors: ColorTokens
  accounts: SwitchAccount[]
  form: SwitchAccount
  formError: string
  switching: boolean
  switchDlgShow: boolean
  switchIndex: number
  deleteDlgShow: boolean
  deleteIndex: number
}

function loadAccounts(): SwitchAccount[] {
  const raw = uni.getStorageSync(KEY_SWITCH_ACCOUNTS) as string
  if (raw == '') return []
  try {
    const list = JSON.parse(raw) as SwitchAccount[]
    return Array.isArray(list) ? list : []
  } catch (e) {
    return []
  }
}

function saveAccounts(list: SwitchAccount[]): void {
  uni.setStorageSync(KEY_SWITCH_ACCOUNTS, JSON.stringify(list))
}

export default {
  components: { AppDialog },
  data(): AccountsData {
    return {
      colors: Colors,
      accounts: [],
      form: { username: '', password: '', tenant_code: '', remark: '' },
      formError: '',
      switching: false,
      switchDlgShow: false,
      switchIndex: -1,
      deleteDlgShow: false,
      deleteIndex: -1
    }
  },
  onShow() {
    this.accounts = loadAccounts()
  },
  computed: {
    currentUsername(): string {
      const u = useAuthStore().userInfo
      return u != null ? u.username : ''
    },
    switchDlgContent(): string {
      const acc = this.accounts[this.switchIndex]
      if (acc == null) return ''
      return '确定退出当前账号，切换到「' + (acc.remark != '' ? acc.remark : acc.username) + '」吗？'
    },
    deleteDlgContent(): string {
      const acc = this.accounts[this.deleteIndex]
      if (acc == null) return ''
      return '确定从列表删除「' + (acc.remark != '' ? acc.remark : acc.username) + '」吗？仅删除本机保存的凭据。'
    }
  },
  methods: {
    onAdd() {
      const f = this.form
      f.username = f.username.trim()
      f.tenant_code = f.tenant_code.trim()
      f.remark = f.remark.trim()
      if (f.username == '') {
        this.formError = '请输入账号'
        return
      }
      if (f.password == '') {
        this.formError = '请输入密码'
        return
      }
      // 同账号同公司视为同一条，覆盖更新（方便改密码后重新保存）
      const idx = this.accounts.findIndex((a) => a.username == f.username && a.tenant_code == f.tenant_code)
      const entry: SwitchAccount = { username: f.username, password: f.password, tenant_code: f.tenant_code, remark: f.remark }
      if (idx >= 0) {
        this.accounts.splice(idx, 1, entry)
      } else {
        this.accounts.push(entry)
      }
      saveAccounts(this.accounts)
      this.form = { username: '', password: '', tenant_code: '', remark: '' }
      this.formError = ''
      uni.showToast({ title: '已保存', icon: 'success' })
    },
    onSwitchTap(idx: number) {
      if (this.switching) return
      this.switchIndex = idx
      this.switchDlgShow = true
    },
    onSwitchConfirm() {
      const acc = this.accounts[this.switchIndex]
      if (acc == null) return
      this.switching = true
      uni.showLoading({ title: '切换中…', mask: true })
      useAuthStore().switchAccount(acc.username, acc.password, acc.tenant_code != '' ? acc.tenant_code : undefined)
        .then(() => {
          uni.hideLoading()
        })
        .catch((e: Error) => {
          uni.hideLoading()
          this.switching = false
          uni.showToast({ title: '切换失败：' + e.message, icon: 'none' })
        })
    },
    onDeleteTap(idx: number) {
      this.deleteIndex = idx
      this.deleteDlgShow = true
    },
    onDeleteConfirm() {
      if (this.deleteIndex < 0) return
      this.accounts.splice(this.deleteIndex, 1)
      saveAccounts(this.accounts)
      this.deleteIndex = -1
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.tip-card {
  border-radius: 20rpx;
  padding: 24rpx 28rpx;
  margin-bottom: 24rpx;
}

.tip-text {
  font-size: 26rpx;
  line-height: 38rpx;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.group-title {
  font-size: 24rpx;
  margin-bottom: 16rpx;
}

.empty {
  align-items: center;
  padding-top: 32rpx;
  padding-bottom: 32rpx;
}

.empty-text {
  font-size: 28rpx;
}

.acc-row {
  min-height: 112rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-bottom-width: 1rpx;
  border-bottom-color: #f2f3f5;
}

.acc-meta {
  flex: 1;
}

.acc-name-line {
  flex-direction: row;
  align-items: center;
}

.acc-name {
  font-size: 32rpx;
  font-weight: 600;
}

.acc-current {
  font-size: 22rpx;
  border-width: 1rpx;
  border-radius: 8rpx;
  padding: 0 10rpx;
  margin-left: 16rpx;
}

.acc-sub {
  font-size: 26rpx;
  margin-top: 6rpx;
}

.acc-del {
  font-size: 28rpx;
  padding: 16rpx 0 16rpx 24rpx;
}

.input-wrap {
  height: 96rpx;
  border-radius: 16rpx;
  border-width: 1rpx;
  flex-direction: row;
  align-items: center;
  padding-left: 28rpx;
  padding-right: 28rpx;
  margin-bottom: 16rpx;
}

.input {
  flex: 1;
  height: 96rpx;
  font-size: 30rpx;
}

.input-ph {
  color: #86909c;
}

.error {
  font-size: 26rpx;
  line-height: 36rpx;
  margin-bottom: 16rpx;
}

.btn-add {
  height: 96rpx;
  border-radius: 16rpx;
  align-items: center;
  justify-content: center;
}

.btn-add-text {
  font-size: 32rpx;
  font-weight: 600;
}
</style>
