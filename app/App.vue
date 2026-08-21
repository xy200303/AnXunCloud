<script lang="ts">
import { getAccessToken } from '@/utils/storage'
import { useAuthStore } from '@/stores/auth'
import { launchCheckinScan, resolvePointCode } from '@/utils/scan'
import { startGlobalListener, platformOf } from '@/utils/nfc'
import { syncOfflineCheckins } from '@/utils/offline'
import { initPushClickListener, bindPushDevice, syncBadge } from '@/utils/push'

export default {
  onLaunch: function () {
    // 恢复缓存的用户信息（token 由 request 层直接从 storage 读）
    useAuthStore().restore()
    // 原生 tabBar 中央扫码按钮（midButton）点击 → 直接拉起系统扫码，无中间页
    uni.onTabBarMidButtonTap(() => {
      if (getAccessToken() == '') return
      launchCheckinScan()
    })
    // 登录守卫：无 token 直接进登录页（登录页 navigationStyle: custom）
    if (getAccessToken() == '') {
      uni.reLaunch({ url: '/pages/login/index' })
    }
    // 网络恢复监听：断网重连后自动补传离线打卡（技术方案 §5.7）
    uni.onNetworkStatusChange((res) => {
      if (!res.isConnected || getAccessToken() == '') return
      syncOfflineCheckins().then((r) => {
        if (r.done > 0) {
          uni.showToast({ title: '已补传 ' + r.done + ' 条离线打卡', icon: 'none' })
        }
      })
    })
    // #ifdef APP-PLUS
    // uniPush 2.0：注册通知点击监听（全局一次，按 payload {type,biz_id} 路由，详见 utils/push.ts）；
    // 已登录（token 在）时补一次 CID 绑定（cid 可能变化；未登录时 bindPushDevice 内部跳过，登录成功后由 auth.login 触发）
    initPushClickListener()
    bindPushDevice()
    // 图标角标同步（未登录时内部跳过；未读数口径与消息中心一致）
    syncBadge()
    // NFC 全局前台识别（技术方案 §5.4）：App 打开任何页面贴标签自动定位任务。
    // 内部按运行时平台分流：Android 注册即听；鸿蒙自动启动无弹窗会话；iOS 忽略（走按钮触发）。
    // 注意：经典 uni-app 不支持 APP-HARMONY 条件编译，这里必须写 APP-PLUS。
    if (getAccessToken() != '' && platformOf() != 'ios') {
      startGlobalListener((cardId) => {
        // 已在打卡表单页时不打断当前填写，仅提示（与技术方案的「顶部提示条」简化差异）
        const pages = getCurrentPages()
        const cur = pages.length > 0 ? (pages[pages.length - 1] as any).route : ''
        if (cur == 'pages/checkin/form') {
          uni.showToast({ title: '已识别 NFC 标签，请用表单内「NFC 校验」完成凭证', icon: 'none' })
          return
        }
        // 按卡片 UID 定位点位（后端 by-code 按 nfc_id 匹配；未备案的卡轻提示「未找到相关点位信息」）
        resolvePointCode(cardId)
      })
    }
    // #endif
  },
  onShow: function () {
    // 回到前台自动补传离线打卡（无网/有任务在执行时 sync 内部自判，单飞防并发）
    if (getAccessToken() == '') return
    syncOfflineCheckins().then((r) => {
      if (r.done > 0) {
        uni.showToast({ title: '已补传 ' + r.done + ' 条离线打卡', icon: 'none' })
      }
    })
  },
  onHide: function () {}
}
</script>

<style>
/* 全局样式：色值与 utils/theme.ts 一一对应，修改时请同步 */
page {
  background-color: #F5F6F8; /* Colors.bgPage */
  color: #1F2329;            /* Colors.textPrimary */
}

/*
 * 布局默认值对齐 uvue 模型：页面代码按 flex 布局编写（uvue 中 view 默认 flex 纵向），
 * 经典版 view 渲染为 block，这里全局补齐，避免每个容器重复声明 display:flex。
 * H5 端标签被编译为 uni-* 自定义元素，且 uni-h5 组件样式（uni-view{display:block} 等）
 * 按组件懒加载注入、顺序在全局样式之后，故用 uni-app 前缀提高优先级（与 uvue.css 同写法）。
 */
view,
scroll-view,
swiper,
swiper-item,
uni-app uni-view,
uni-app uni-scroll-view,
uni-app uni-swiper,
uni-app uni-swiper-item {
  display: flex;
  flex-direction: column;
  box-sizing: border-box;
}

/* 设计令牌 CSS 变量（页面内颜色仍以 theme.ts 绑定为准，二者保持一致） */
:root {
  --primary: #2B5AED;
  --primary-light: #EAEFFF;
  --success: #2BA471;
  --warning: #ED7B2F;
  --danger: #D54941;
  --info: #909399;
  --bg-page: #F5F6F8;
  --bg-card: #FFFFFF;
  --text-primary: #1F2329;
  --text-regular: #4E5969;
  --text-secondary: #86909C;
  --border: #E5E6EB;
}
</style>
