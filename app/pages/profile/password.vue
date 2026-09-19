<template>
  <view class="page bg-page" >
    <view class="card bg-card" >
      <view class="field border-default" >
        <text class="field-label text-regular" >原密码</text>
        <uni-easyinput class="field-input" v-model="oldPwd" type="password" placeholder="请输入当前密码" :maxlength="32" :input-border="false" :clearable="false" :primary-color="'#2B5AED'" />
      </view>
      <view class="field border-default" >
        <text class="field-label text-regular" >新密码</text>
        <uni-easyinput class="field-input" v-model="newPwd" type="password" placeholder="8-32 位，含字母和数字" :maxlength="32" :input-border="false" :clearable="false" :primary-color="'#2B5AED'" />
      </view>
      <view class="field field-last">
        <text class="field-label text-regular" >确认新密码</text>
        <uni-easyinput class="field-input" v-model="confirmPwd" type="password" placeholder="再输入一次新密码" :maxlength="32" :input-border="false" :clearable="false" :primary-color="'#2B5AED'" />
      </view>
    </view>

    <button
      plain="true" hover-class="hover-dim" class="btn-submit"
      :class="(submitting ? 'btn-disabled' : 'btn-primary')"
      @click="submit"
    >
      <text class="btn-submit-text">{{ submitting ? '提交中…' : '确认修改' }}</text>
    </button>

    <text class="tip text-secondary" >修改成功后下次登录请使用新密码</text>
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import { apiChangePassword } from '@/services/api'

type PasswordData = {
  oldPwd: string
  newPwd: string
  confirmPwd: string
  submitting: boolean
}

export default {
  data(): PasswordData {
    return {
      oldPwd: '',
      newPwd: '',
      confirmPwd: '',
      submitting: false
    }
  },
  methods: {
    /** 前端校验与后端规则一致：8–32 位且含字母与数字、新旧不同、两次输入一致 */
    validate(): string {
      if (this.oldPwd == '') return '请输入原密码'
      const p = this.newPwd
      if (p.length < 8 || p.length > 32 || !/[A-Za-z]/.test(p) || !/[0-9]/.test(p)) {
        return '新密码须为 8-32 位且含字母与数字'
      }
      if (p == this.oldPwd) return '新密码不能与原密码相同'
      if (p != this.confirmPwd) return '两次输入的新密码不一致'
      return ''
    },
    submit() {
      if (this.submitting) return
      const msg = this.validate()
      if (msg != '') {
        uni.showToast({ title: msg, icon: 'none' })
        return
      }
      this.submitting = true
      apiChangePassword(this.oldPwd, this.newPwd)
        .then(() => {
          this.submitting = false
          uni.showToast({ title: '密码已修改', icon: 'success' })
          setTimeout(() => uni.navigateBack(), 800)
        })
        .catch((e: Error) => {
          this.submitting = false
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

.card {
  border-radius: 24rpx;
  padding: 8rpx 32rpx;
}

.field {
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.field-last {
  border-bottom-width: 0;
}

.field-label {
  font-size: 30rpx;
  width: 160rpx;
}

.field-input {
  flex: 1;
}

.btn-submit {
  height: 96rpx;
  border-radius: 48rpx;
  align-items: center;
  justify-content: center;
  margin-top: 40rpx;
}

.btn-submit-text {
  font-size: 32rpx;
  font-weight: 600;
}

.tip {
  font-size: 24rpx;
  text-align: center;
  margin-top: 24rpx;
}
</style>
