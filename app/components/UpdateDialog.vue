<template>
  <!-- 版本更新弹窗（自定义 UI；强制更新无关闭入口，弱更新可「以后再说」） -->
  <view v-if="visible" class="upd-mask">
    <view class="upd-dialog" :style="{ backgroundColor: colors.bgCard }">
      <!-- 顶部横幅 -->
      <view class="upd-banner" :style="{ backgroundColor: colors.primary }">
        <text class="upd-banner-icon">⤴</text>
        <text class="upd-banner-title" :style="{ color: colors.white }">发现新版本</text>
        <text class="upd-banner-ver" :style="{ color: colors.white }">v{{ info != null ? info.version : '' }}</text>
      </view>

      <view class="upd-body">
        <view v-if="info != null && info.force_update" class="upd-force">
          <text class="upd-force-text" :style="{ color: colors.danger }">本次为强制更新，更新后才能继续使用</text>
        </view>
        <text v-if="noteText != ''" class="upd-note" :style="{ color: colors.textRegular }">{{ noteText }}</text>
        <text v-else class="upd-note" :style="{ color: colors.textSecondary }">版本更新，体验更流畅</text>
        <text v-if="sizeText != ''" class="upd-size" :style="{ color: colors.textSecondary }">安装包大小 {{ sizeText }}</text>

        <!-- 下载进度 -->
        <view v-if="downloading" class="upd-progress" :style="{ backgroundColor: colors.border }">
          <view class="upd-progress-inner" :style="{ width: progress + '%', backgroundColor: colors.primary }"></view>
        </view>
      </view>

      <!-- 按钮区 -->
      <view class="upd-actions">
        <view
          v-if="info != null && !info.force_update && !downloading"
           hover-class="hover-dim" class="upd-btn-later"
          :style="{ borderColor: colors.border }"
          @click="later"
        >
          <text  hover-class="hover-dim" class="upd-btn-later-text" :style="{ color: colors.textSecondary }">以后再说</text>
        </view>
        <view
           hover-class="hover-dim" class="upd-btn-go"
          :style="{ backgroundColor: downloading ? colors.info : colors.primary }"
          @click="startUpdate"
        >
          <text  hover-class="hover-dim" class="upd-btn-go-text" :style="{ color: colors.white }">{{ goText }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { getPublicOrigin } from '@/services/request'
import { LatestRelease } from '@/utils/update'
import { platformOf } from '@/utils/nfc'

type UpdData = {
  colors: ColorTokens
  visible: boolean
  info: LatestRelease | null
  downloading: boolean
  progress: number
}

export default {
  data(): UpdData {
    return {
      colors: Colors,
      visible: false,
      info: null,
      downloading: false,
      progress: 0
    }
  },
  computed: {
    noteText(): string {
      return this.info != null ? this.info.note : ''
    },
    sizeText(): string {
      if (this.info == null || this.info.size <= 0) return ''
      return (this.info.size / 1024 / 1024).toFixed(1) + ' MB'
    },
    goText(): string {
      if (this.downloading) return '下载中 ' + this.progress + '%'
      // iOS 无法应用内安装，走浏览器下载页
      return platformOf() == 'ios' ? '去下载' : '立即更新'
    }
  },
  methods: {
    open(info: LatestRelease) {
      this.info = info
      this.downloading = false
      this.progress = 0
      this.visible = true
    },
    later() {
      this.visible = false
    },
    /** 立即更新：Android/鸿蒙 包内下载→安装（带进度）；iOS 打开下载页 */
    startUpdate() {
      if (this.info == null || this.downloading) return
      const url = getPublicOrigin() + this.info.download_url
      // #ifdef APP-PLUS
      if (platformOf() == 'ios') {
        plus.runtime.openURL(url)
        return
      }
      this.downloading = true
      this.progress = 0
      const task = uni.downloadFile({
        url: url,
        success: (r) => {
          this.downloading = false
          if (r.statusCode != 200) {
            uni.showToast({ title: '下载失败，请稍后重试', icon: 'none' })
            return
          }
          plus.runtime.install(
            r.tempFilePath,
            {},
            () => {},
            () => {
              uni.showModal({
                title: '安装失败',
                content: '请前往官网下载页手动安装：' + getPublicOrigin() + '/download',
                showCancel: false,
                confirmText: '知道了'
              })
            }
          )
        },
        fail: () => {
          this.downloading = false
          uni.showToast({ title: '下载失败，请检查网络', icon: 'none' })
        }
      })
      task.onProgressUpdate((p) => {
        this.progress = p.progress
      })
      // #endif
    }
  }
}
</script>

<style scoped>
.upd-mask {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.55);
  z-index: 999;
  align-items: center;
  justify-content: center;
  padding: 64rpx;
}

.upd-dialog {
  width: 100%;
  border-radius: 28rpx;
  overflow: hidden;
}

.upd-banner {
  align-items: center;
  padding: 48rpx 0 40rpx;
}

.upd-banner-icon {
  font-size: 64rpx;
  color: #ffffff;
}

.upd-banner-title {
  font-size: 38rpx;
  font-weight: 600;
  margin-top: 16rpx;
}

.upd-banner-ver {
  font-size: 26rpx;
  margin-top: 8rpx;
  opacity: 0.85;
}

.upd-body {
  padding: 32rpx 36rpx 8rpx;
}

.upd-force {
  border-radius: 12rpx;
  padding: 16rpx 20rpx;
  margin-bottom: 20rpx;
  background-color: #fef0f0;
}

.upd-force-text {
  font-size: 24rpx;
}

.upd-note {
  font-size: 28rpx;
  line-height: 44rpx;
}

.upd-size {
  font-size: 24rpx;
  margin-top: 12rpx;
}

.upd-progress {
  height: 12rpx;
  border-radius: 6rpx;
  margin-top: 24rpx;
  overflow: hidden;
}

.upd-progress-inner {
  height: 12rpx;
  border-radius: 6rpx;
}

.upd-actions {
  flex-direction: row;
  padding: 28rpx 36rpx 36rpx;
}

.upd-btn-later {
  flex: 1;
  height: 88rpx;
  border-radius: 44rpx;
  border-width: 1rpx;
  border-style: solid;
  align-items: center;
  justify-content: center;
  margin-right: 24rpx;
}

.upd-btn-later-text {
  font-size: 30rpx;
}

.upd-btn-go {
  flex: 1;
  height: 88rpx;
  border-radius: 44rpx;
  align-items: center;
  justify-content: center;
}

.upd-btn-go-text {
  font-size: 30rpx;
  font-weight: 600;
}
</style>
