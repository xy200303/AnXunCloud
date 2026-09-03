<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 品牌区 -->
    <view class="brand">
      <image class="brand-icon" src="/static/brand/app-icon-1024.png" mode="aspectFit" />
      <text class="brand-name" :style="{ color: colors.textPrimary }">安巡云</text>
      <text class="brand-version" :style="{ color: colors.textSecondary }">v{{ version }}</text>
    </view>

    <!-- 简介 -->
    <view class="card" :style="{ backgroundColor: colors.bgCard }">
      <text class="intro" :style="{ color: colors.textRegular }">安巡云是面向物业巡检场景的数字化管理平台：扫码 / NFC / GPS 围栏三重到点校验，拍照留证、AI 审核、月度报告电子签，让每一次巡检都有据可查。</text>
    </view>

    <!-- 链接区 -->
    <view class="card menu-card" :style="{ backgroundColor: colors.bgCard }">
      <view  hover-class="hover-dim" class="row" @click="copySite">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">官网 / 下载页</text>
        <text  hover-class="hover-dim" class="row-value" :style="{ color: colors.textSecondary }">{{ siteUrl }}</text>
      </view>
      <view  hover-class="hover-dim" class="row" @click="checkUpdate">
        <text  hover-class="hover-dim" class="row-text" :style="{ color: colors.textRegular }">检查更新</text>
        <text  hover-class="hover-dim" class="row-value" :style="{ color: colors.textSecondary }">当前 v{{ version }}</text>
      </view>
    </view>

    <!-- 版本更新弹窗（手动检查） -->
    <UpdateDialog ref="updDialog" />

    <text class="copyright" :style="{ color: colors.textSecondary }">安巡云 AnXunCloud · 物业巡检数字化</text>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { getPublicOrigin } from '@/services/request'
import { fetchLatestRelease, currentVersion } from '@/utils/update'
import UpdateDialog from '@/components/UpdateDialog.vue'

type AboutData = {
  colors: ColorTokens
  version: string
  siteUrl: string
  checking: boolean
}

export default {
  components: { UpdateDialog },
  data(): AboutData {
    return {
      colors: Colors,
      version: '1.0.0',
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

.row {
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
}

.row-text {
  font-size: 30rpx;
}

.row-value {
  font-size: 26rpx;
  flex: 1;
  text-align: right;
  margin-left: 24rpx;
}

.copyright {
  font-size: 24rpx;
  text-align: center;
  margin-top: 48rpx;
}
</style>
