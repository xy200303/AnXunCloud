/**
 * 全局导航防抖：跳登录页统一入口。
 * 冷启动时入口页 onLoad 请求的 401 踢出（request.forceLogoutToLogin）与 App.vue
 * 登录守卫、推送路由等可能并发触发 reLaunch('/pages/login/index')，
 * uni-app 会警告 "do not operate continuously"。所有跳登录页的调用走这里，
 * pending 期间重复调用直接丢弃。
 */
let loginNavPending = false

export function reLaunchToLogin(url: string = '/pages/login/index'): void {
  if (loginNavPending) return
  loginNavPending = true
  uni.reLaunch({
    url,
    complete: () => {
      // reLaunch 完成后留短暂窗口，覆盖跳转进行中到达的重复调用
      setTimeout(() => {
        loginNavPending = false
      }, 1500)
    }
  })
}
