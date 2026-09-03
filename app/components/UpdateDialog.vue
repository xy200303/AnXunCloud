<template>
  <!-- 版本更新弹窗（自定义 UI；强制更新无关闭入口，弱更新可「以后再说」）
       安装包按版本缓存：同版本再次弹窗直接「立即安装」，不重复下载 -->
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
        <text v-if="phase == 'ready'" class="upd-cached" :style="{ color: colors.success }">✓ 安装包已下载，无需重复下载</text>

        <!-- 下载进度 -->
        <view v-if="phase == 'downloading'" class="upd-progress" :style="{ backgroundColor: colors.border }">
          <view class="upd-progress-inner" :style="{ width: progress + '%', backgroundColor: colors.primary }"></view>
        </view>
      </view>

      <!-- 按钮区 -->
      <view class="upd-actions">
        <view
          v-if="info != null && !info.force_update && phase != 'downloading'"
           hover-class="hover-dim" class="upd-btn-later"
          :style="{ borderColor: colors.border }"
          @click="later"
        >
          <text  hover-class="hover-dim" class="upd-btn-later-text" :style="{ color: colors.textSecondary }">以后再说</text>
        </view>
        <view
           hover-class="hover-dim" class="upd-btn-go"
          :style="{ backgroundColor: phase == 'downloading' ? colors.info : colors.primary }"
          @click="onGoTap"
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

/** 安装包本地缓存键：{version, path}——按版本匹配，同版本直接安装不重下 */
const CACHE_KEY = 'update_pkg_cache'

type PkgCache = { version: string; path: string }

type UpdData = {
  colors: ColorTokens
  visible: boolean
  info: LatestRelease | null
  /** idle=待下载 / downloading=下载中 / ready=已缓存可直接安装 */
  phase: 'idle' | 'downloading' | 'ready'
  progress: number
  /** 可直接安装的本地包路径（phase=ready 时有效） */
  cachedPath: string
}

export default {
  data(): UpdData {
    return {
      colors: Colors,
      visible: false,
      info: null,
      phase: 'idle',
      progress: 0,
      cachedPath: ''
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
      if (this.phase == 'downloading') return '下载中 ' + this.progress + '%'
      if (this.phase == 'ready') return '立即安装'
      // iOS/鸿蒙无法应用内安装，走浏览器下载页（下载后点击打开安装）
      const p = platformOf()
      if (p == 'ios' || p == 'harmony') return '去下载'
      return '立即更新'
    }
  },
  methods: {
    open(info: LatestRelease) {
      this.info = info
      this.phase = 'idle'
      this.progress = 0
      this.cachedPath = ''
      this.visible = true
      this.restoreCache()
    },
    later() {
      this.visible = false
    },
    /** 安装包缓存命中校验：版本一致且文件真实存在 → ready（temp 目录可能被系统清理，必须验存在性） */
    restoreCache() {
      // #ifdef APP-PLUS
      if (this.info == null) return
      const c = uni.getStorageSync(CACHE_KEY) as PkgCache | ''
      if (c == '' || c.version != this.info.version || c.path == '') return
      plus.io.resolveLocalFileSystemURL(
        c.path,
        () => {
          this.cachedPath = c.path
          this.phase = 'ready'
        },
        () => {
          uni.removeStorageSync(CACHE_KEY)
        }
      )
      // #endif
    },
    onGoTap() {
      if (this.info == null || this.phase == 'downloading') return
      const url = getPublicOrigin() + this.info.download_url
      // #ifdef APP-PLUS
      const plat = platformOf()
      // iOS/鸿蒙：应用内无法程序化安装，打开浏览器下载页（与浏览器下载后点打开一致）
      if (plat == 'ios' || plat == 'harmony') {
        plus.runtime.openURL(url)
        return
      }
      // Android：缓存命中直接唤起系统安装器
      if (this.phase == 'ready' && this.cachedPath != '') {
        this.install(this.cachedPath)
        return
      }
      // Android：下载 → 缓存 → 唤起系统安装器
      this.phase = 'downloading'
      this.progress = 0
      const task = uni.downloadFile({
        url: url,
        success: (r) => {
          if (r.statusCode != 200) {
            this.phase = 'idle'
            uni.showToast({ title: '下载失败，请稍后重试', icon: 'none' })
            return
          }
          if (this.info != null) {
            const c: PkgCache = { version: this.info.version, path: r.tempFilePath }
            uni.setStorageSync(CACHE_KEY, c)
            this.cachedPath = r.tempFilePath
          }
          this.phase = 'ready'
          this.install(r.tempFilePath)
        },
        fail: () => {
          this.phase = 'idle'
          uni.showToast({ title: '下载失败，请检查网络', icon: 'none' })
        }
      })
      task.onProgressUpdate((p) => {
        this.progress = p.progress
      })
      // #endif
    },
    /** 唤起系统安装器（等同浏览器下载完成后点击打开）；失败引导去官网下载页手动安装 */
    install(path: string) {
      // #ifdef APP-PLUS
      plus.runtime.install(
        path,
        {},
        () => {},
        () => {
          uni.showModal({
            title: '无法自动安装',
            content: '请前往官网下载页手动安装：' + getPublicOrigin() + '/download',
            showCancel: false,
            confirmText: '知道了'
          })
        }
      )
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

.upd-cached {
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
