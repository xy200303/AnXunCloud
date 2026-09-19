<template>
  <view class="page bg-page" >
    <!-- 品牌区 -->
    <view class="brand">
      <image class="brand-icon" src="/static/brand/app-icon-1024.png" mode="aspectFit" />
      <text class="brand-name text-main" >安巡云</text>
      <text class="brand-version text-secondary" >v{{ version }}</text>
    </view>

    <!-- 简介 -->
    <view class="card bg-card" >
      <text class="intro text-regular" >安巡云是面向物业巡检场景的数字化管理平台：扫码 / NFC / GPS 围栏三重到点校验，拍照留证、AI 审核、月度报告电子签，让每一次巡检都有据可查。</text>
    </view>

    <!-- 链接区：官方 uni-list 列表行 -->
    <view class="card menu-card bg-card" >
      <uni-list :border="false">
        <uni-list-item title="官网 / 下载页" :right-text="siteUrl" clickable @click="copySite" />
        <uni-list-item title="检查更新" :right-text="'当前 v' + version" clickable show-arrow @click="checkUpdate" />
      </uni-list>
    </view>

    <!-- 版本更新弹窗（手动检查） -->
    <UpdateDialog ref="updDialog" />

    <text class="copyright text-secondary" >安巡云 AnXunCloud · 物业巡检数字化</text>
  </view>
</template>

<script lang="ts">

import { getPublicOrigin } from '@/services/request'
import { fetchLatestRelease, currentVersion } from '@/utils/update'
import { APP_VERSION } from '@/utils/appVersion'
import UpdateDialog from '@/components/UpdateDialog.vue'

type AboutData = {
  version: string
  siteUrl: string
  checking: boolean
}

export default {
  components: { UpdateDialog },
  data(): AboutData {
    return {
      version: APP_VERSION,
      siteUrl: '',
      checking: false
    }
  },
  onLoad() {
    this.version = currentVersion()
    this.siteUrl = getPublicOrigin() + '/download'
  },
  methods: {
    copySite() {
      uni.setClipboardData({
        data: this.siteUrl,
        success: () => uni.showToast({ title: '链接已复制', icon: 'none' })
      })
    },
    /** 检查更新：有更新弹自定义更新窗；无更新明确提示已是最新 */
    checkUpdate() {
      if (this.checking) return
      this.checking = true
      fetchLatestRelease()
        .then((rel) => {
          this.checking = false
          if (rel == null) {
            uni.showToast({ title: '当前已是最新版本', icon: 'none' })
            return
          }
          const dlg: any = this.$refs.updDialog
          if (dlg != null) dlg.open(rel)
        })
        .catch(() => {
          this.checking = false
          uni.showToast({ title: '检查失败，请稍后重试', icon: 'none' })
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

.brand {
  align-items: center;
  padding: 64rpx 0 48rpx;
}

.brand-icon {
  width: 144rpx;
  height: 144rpx;
  border-radius: 32rpx;
}

.brand-name {
  font-size: 40rpx;
  font-weight: 600;
  margin-top: 24rpx;
}

.brand-version {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.menu-card {
  padding-top: 8rpx;
  padding-bottom: 8rpx;
}

.intro {
  font-size: 28rpx;
  line-height: 44rpx;
}

.copyright {
  font-size: 24rpx;
  text-align: center;
  margin-top: 48rpx;
}
</style>
