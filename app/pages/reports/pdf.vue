<template>
  <!-- App 端 PDF 预览：web-view 内嵌后端 pdf.js 查看器（报告正文在内嵌页内渲染） -->
  <web-view :src="src"></web-view>
</template>

<script lang="ts">
/**
 * 报告预览页（App）。
 * 导航栏「打开」按钮（pages.json titleNView 配置）：
 * 下载 PDF 到本地（ticket 公开地址，无需登录头）→ 直接调起系统分享面板（ACTION_SEND）：
 * 不限文件类型，凡能接收文件的应用都会出现——微信好友/QQ/钉钉/邮箱/WPS/保存到文件等，
 * 选 WPS 即打开查看，选微信即发送好友，一个面板全覆盖（需在 manifest 勾选 Share 模块）。
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
      this.ensureFile((path) => this.shareFile(path))
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
    /** 系统分享面板（ACTION_SEND：微信/QQ/WPS/邮箱/保存到文件等所有能接收文件的应用） */
    shareFile(path: string) {
      // #ifdef APP-PLUS
      uni.shareWithSystem({
        type: 'file',
        filePath: path,
        summary: '月度巡检报告',
        fail: (e) => {
          const msg = (e && e.errMsg) ? e.errMsg : ''
          if (msg.indexOf('cancel') >= 0) return // 用户取消，不提示
          uni.showToast({ title: '调起失败，请重新打包基座后重试', icon: 'none' })
        }
      })
      // #endif
    }
  }
}
</script>
