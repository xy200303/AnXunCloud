<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏 -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block sk-short" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 错误态 -->
    <view v-else-if="errorMsg" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <!-- 正文 -->
    <view v-else-if="loaded" class="content">
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="title" :style="{ color: colors.textPrimary }">{{ detail.title }}</text>
        <text class="meta" :style="{ color: colors.textSecondary }">{{ metaText }}</text>
        <!-- 纯文本正文：http(s) 链接切段渲染为可点链接 -->
        <view class="body">
          <text
            v-for="(seg, i) in contentSegs"
            :key="i"
            class="body-text"
            :class="{ 'body-link': seg.isLink }"
            :style="{ color: seg.isLink ? colors.primary : colors.textRegular }"
            @click="seg.isLink ? openLink(seg.text) : noop()"
          >{{ seg.text }}</text>
        </view>
      </view>

      <!-- 附件区 -->
      <view v-if="detail.attachments.length > 0" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="attach-title" :style="{ color: colors.textPrimary }">附件（{{ detail.attachments.length }}）</text>
        <!-- 图片附件：缩略图，点击预览 -->
        <view v-if="imageAttachments.length > 0" class="thumb-grid">
          <image
            v-for="(img, i) in imageAttachments"
            :key="i"
            class="thumb"
            :src="img.absUrl"
            mode="aspectFill"
            lazy-load
            @click="previewImage(i)"
          />
        </view>
        <!-- 文件附件：类型徽章 + 文件名，点击复制链接 -->
        <view
          v-for="(f, i) in fileAttachments"
          :key="i"
          class="file-item"
          :style="{ borderColor: colors.border }"
          @click="copyFileLink(f)"
        >
          <view class="file-badge" :style="{ backgroundColor: colors.primaryLight }">
            <text class="file-badge-text" :style="{ color: colors.primary }">{{ f.extText }}</text>
          </view>
          <text class="file-name" :style="{ color: colors.textRegular }">{{ f.name }}</text>
        </view>
      </view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { toAbsUrl } from '@/utils/url'
import { apiAnnouncementDetail, AnnouncementDetail, NoticeAttachment } from '@/services/api'

type ContentSeg = {
  text: string
  isLink: boolean
}

type ImageAttachment = {
  name: string
  absUrl: string
}

type FileAttachment = {
  name: string
  absUrl: string
  extText: string
}

type NoticeDetailData = {
  colors: ColorTokens
  noticeId: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  detail: AnnouncementDetail
}

const IMAGE_EXTS = ['jpg', 'jpeg', 'png', 'gif', 'webp']

/** URL 识别：排除常见中英文尾随标点，避免把句号/括号吞进链接 */
const URL_RE = /https?:\/\/[^\s，。；、！？（）【】“”"'<>]+/g

/** 把纯文本按 URL 切成片段（纯前端拼装，content 不落地 HTML） */
function splitContent(content: string): ContentSeg[] {
  const segs: ContentSeg[] = []
  let last = 0
  let m: RegExpExecArray | null
  URL_RE.lastIndex = 0
  while ((m = URL_RE.exec(content)) != null) {
    if (m.index > last) segs.push({ text: content.slice(last, m.index), isLink: false })
    segs.push({ text: m[0], isLink: true })
    last = m.index + m[0].length
  }
  if (last < content.length) segs.push({ text: content.slice(last), isLink: false })
  return segs
}

function extOf(name: string): string {
  const m = /\.([a-zA-Z0-9]+)$/.exec(name || '')
  return m != null ? m[1].toLowerCase() : ''
}

export default {
  data(): NoticeDetailData {
    return {
      colors: Colors,
      noticeId: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      detail: {
        id: '', title: '', content: '', status: 0,
        attachments: [] as NoticeAttachment[],
        publish_at: '', created_by: '', created_at: ''
      }
    }
  },
  computed: {
    metaText(): string {
      const parts: string[] = []
      if (this.detail.publish_at) parts.push(this.detail.publish_at)
      if (this.detail.created_by) parts.push(this.detail.created_by)
      return parts.join(' · ')
    },
    contentSegs(): ContentSeg[] {
      return splitContent(this.detail.content || '')
    },
    imageAttachments(): ImageAttachment[] {
      return this.detail.attachments
        .filter((a) => IMAGE_EXTS.indexOf(extOf(a.name) || extOf(a.url)) >= 0)
        .map((a) => ({ name: a.name, absUrl: toAbsUrl(a.url) }))
    },
    fileAttachments(): FileAttachment[] {
      return this.detail.attachments
        .filter((a) => IMAGE_EXTS.indexOf(extOf(a.name) || extOf(a.url)) < 0)
        .map((a) => {
          const ext = extOf(a.name) || extOf(a.url)
          return { name: a.name, absUrl: toAbsUrl(a.url), extText: ext ? ext.toUpperCase() : '文件' }
        })
    }
  },
  onLoad(options: Record<string, any>) {
    this.noticeId = options && options.id ? String(options.id) : ''
    this.load()
  },
  methods: {
    noop() {},
    load() {
      if (this.noticeId == '') {
        this.loading = false
        this.errorMsg = '缺少公告参数'
        return
      }
      this.loading = true
      this.errorMsg = ''
      apiAnnouncementDetail(this.noticeId)
        .then((res) => {
          this.detail = res
          this.loading = false
          this.loaded = true
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
        })
    },
    /** 点击正文链接：App 端系统浏览器打开，失败回退复制；小程序端直接复制 */
    openLink(url: string) {
      // #ifdef APP-PLUS
      plus.runtime.openURL(url, () => {
        this.copyText(url, '链接已复制')
      })
      // #endif
      // #ifndef APP-PLUS
      this.copyText(url, '链接已复制')
      // #endif
    },
    /** 文件附件：复制链接到剪贴板（App 内直接下载打开体验不可控，复制链接最稳） */
    copyFileLink(f: FileAttachment) {
      this.copyText(f.absUrl, '请复制链接下载')
    },
    copyText(text: string, toast: string) {
      uni.setClipboardData({
        data: text,
        success: () => {
          uni.showToast({ title: toast, icon: 'none' })
        }
      })
    },
    previewImage(idx: number) {
      const urls = this.imageAttachments.map((a) => a.absUrl)
      uni.previewImage({ urls: urls, current: urls[idx] })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.skeleton {
  padding-top: 8rpx;
}

.sk-block {
  height: 160rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.sk-short {
  height: 96rpx;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 30rpx;
  margin-bottom: 16rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.title {
  font-size: 38rpx;
  font-weight: 600;
  line-height: 52rpx;
}

.meta {
  font-size: 24rpx;
  margin-top: 16rpx;
}

.body {
  margin-top: 28rpx;
  /* view 全局为 flex-column，正文片段需回归行内换行排列 */
  display: block;
}

.body-text {
  font-size: 28rpx;
  line-height: 46rpx;
}

.body-link {
  text-decoration: underline;
}

.attach-title {
  font-size: 30rpx;
  font-weight: 600;
}

.thumb-grid {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 20rpx;
}

.thumb {
  width: 200rpx;
  height: 200rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.file-item {
  flex-direction: row;
  align-items: center;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 20rpx 24rpx;
  margin-top: 16rpx;
}

.file-badge {
  border-radius: 8rpx;
  padding: 6rpx 12rpx;
  margin-right: 20rpx;
}

.file-badge-text {
  font-size: 20rpx;
  font-weight: 600;
}

.file-name {
  font-size: 26rpx;
  flex: 1;
}
</style>
