<template>
  <!-- App 端 PDF 预览：web-view 内嵌后端 pdf.js 查看器（报告正文在内嵌页内渲染） -->
  <web-view :src="src"></web-view>
</template>

<script lang="ts">
/**
 * 报告预览页（App）。
 * 导航栏「打开/分享」按钮（pages.json titleNView 配置）：
 * 下载 PDF 到本地（ticket 公开地址，无需登录头）→ 系统面板调起其他应用：
 * - 用其他应用打开：WPS/QQ 浏览器等（ACTION_VIEW 选择器）；
 * - 分享发送：微信/QQ/钉钉等（系统分享面板，需在 manifest 勾选 Share 模块）。
 */
export default {
  data() {
    return {
      src: '',
      fileUrl: '',
      localPath: '' // 已下载的本地文件路径（避免重复下载）
    }
  },
  onLoad(options: any) {
    if (options != null && options['src'] != null) {
      this.src = decodeURIComponent(String(options['src']))
    }
    if (options != null && options['file'] != null) {
      this.fileUrl = decodeURIComponent(String(options['file']))
    }
  },
  onNavigationBarButtonTap() {
    this.openWith()
  },
  methods: {
    openWith() {
      if (this.fileUrl == '') {
        uni.showToast({ title: '文件地址缺失，请返回重进', icon: 'none' })
        return
      }
      uni.showActionSheet({
        itemList: ['用其他应用打开（WPS 等）', '分享发送（微信/QQ）'],
        success: (res) => {
          this.ensureFile((path) => {
            if (res.tapIndex == 0) this.openExternal(path)
            if (res.tapIndex == 1) this.shareFile(path)
          })
        },
        fail: (_e) => {}
      })
    },
    /** 下载到本地（已下载过直接复用）；下载的临时文件无 .pdf 扩展名，需先改名为规范文件名，否则系统无法匹配可打开的应用 */
    ensureFile(cb: (path: string) => void) {
      if (this.localPath != '') {
        cb(this.localPath)
        return
      }
      uni.showLoading({ title: '正在获取文件…', mask: true })
      uni.downloadFile({
        url: this.fileUrl,
        success: (res) => {
          if (res.statusCode != 200) {
            uni.hideLoading()
            // ticket 过期会返回 JSON 而非 PDF
            uni.showToast({ title: '文件已过期，请返回重新打开', icon: 'none' })
            return
          }
          const tmp = res.tempFilePath
          // #ifdef APP-PLUS
          const name = '月度巡检报告_' + Date.now() + '.pdf'
          plus.io.resolveLocalFileSystemURL(tmp, (entry: any) => {
            plus.io.requestFileSystem(plus.io.PRIVATE_DOC, (fs: any) => {
              entry.moveTo(fs.root, name, (e2: any) => {
                uni.hideLoading()
                this.localPath = e2.fullPath
                cb(e2.fullPath)
              }, (_e: any) => {
                uni.hideLoading()
                uni.showToast({ title: '文件保存失败', icon: 'none' })
              })
            })
          }, (_e: any) => {
            uni.hideLoading()
            uni.showToast({ title: '文件读取失败', icon: 'none' })
          })
          // #endif
          // #ifndef APP-PLUS
          uni.hideLoading()
          this.localPath = tmp
          cb(tmp)
          // #endif
        },
        fail: (e) => {
          uni.hideLoading()
          uni.showToast({ title: '下载失败：' + (e.errMsg || ''), icon: 'none' })
        }
      })
    },
    /** 系统「打开方式」选择器（WPS/QQ 浏览器等） */
    openExternal(path: string) {
      // #ifdef APP-PLUS
      plus.runtime.openFile(
        path,
        {},
        (e) => {
          uni.showToast({ title: '没有可打开 PDF 的应用：' + (e.message || ''), icon: 'none' })
        }
      )
      // #endif
    },
    /** 系统分享面板（微信/QQ/钉钉等，type=file 分享文件本体） */
    shareFile(path: string) {
      // #ifdef APP-PLUS
      uni.shareWithSystem({
        type: 'file',
        filePath: path,
        summary: '月度巡检报告',
        fail: (e) => {
          const msg = (e && e.errMsg) ? e.errMsg : ''
          if (msg.indexOf('cancel') < 0) {
            uni.showToast({ title: '分享失败：' + msg, icon: 'none' })
          }
        }
      })
      // #endif
    }
  }
}
</script>
