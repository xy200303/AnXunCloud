<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <view class="field" :style="{ borderColor: colors.border }">
        <text class="field-label" :style="{ color: colors.textRegular }">原密码</text>
        <input
          v-model="oldPwd"
          class="field-input"
          :style="{ color: colors.textPrimary }"
          :password="!showOld"
          placeholder="请输入当前密码"
          placeholder-class="ph"
          :maxlength="32"
        />
        <image class="eye" :src="showOld ? eyeOff : eyeOn" @click="showOld = !showOld" />
      </view>
      <view class="field" :style="{ borderColor: colors.border }">
        <text class="field-label" :style="{ color: colors.textRegular }">新密码</text>
        <input
          v-model="newPwd"
          class="field-input"
          :style="{ color: colors.textPrimary }"
          :password="!showNew"
          placeholder="8-32 位，含字母和数字"
          placeholder-class="ph"
          :maxlength="32"
        />
        <image class="eye" :src="showNew ? eyeOff : eyeOn" @click="showNew = !showNew" />
      </view>
      <view class="field field-last">
        <text class="field-label" :style="{ color: colors.textRegular }">确认新密码</text>
        <input
          v-model="confirmPwd"
          class="field-input"
          :style="{ color: colors.textPrimary }"
          :password="!showConfirm"
          placeholder="再输入一次新密码"
          placeholder-class="ph"
          :maxlength="32"
        />
        <image class="eye" :src="showConfirm ? eyeOff : eyeOn" @click="showConfirm = !showConfirm" />
      </view>
    </view>

    <view
       hover-class="hover-dim" class="btn-submit"
      :style="{ backgroundColor: submitting ? colors.border : colors.primary }"
      @click="submit"
    >
      <text  hover-class="hover-dim" class="btn-submit-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : '确认修改' }}</text>
    </view>

    <text class="tip" :style="{ color: colors.textSecondary }">修改成功后下次登录请使用新密码</text>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiChangePassword } from '@/services/api'

type PasswordData = {
  colors: ColorTokens
  oldPwd: string
  newPwd: string
  confirmPwd: string
  showOld: boolean
  showNew: boolean
  showConfirm: boolean
  submitting: boolean
  eyeOn: string
  eyeOff: string
}

export default {
  data(): PasswordData {
    return {
      colors: Colors,
      oldPwd: '',
      newPwd: '',
      confirmPwd: '',
      showOld: false,
      showNew: false,
      showConfirm: false,
      submitting: false,
      eyeOn: '/static/icons/eye.png',
      eyeOff: '/static/icons/eye-off.png'
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
  font-size: 30rpx;
  height: 104rpx;
}

.ph {
  color: #a8abb2;
  font-size: 28rpx;
}

.eye {
  width: 44rpx;
  height: 44rpx;
  padding: 10rpx;
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
