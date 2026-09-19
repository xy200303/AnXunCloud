/**
 * 设计令牌（布局类）——对应《移动端四端UI-UX设计方案》§二，750 设计稿基准。
 *
 * 颜色令牌已废除：颜色唯一来源为 app/uni.scss 的 $uni-* 变量（uni-ui 官方换色机制）；
 * 模板用 App.vue 全局工具类（text-/bg-/border- 前缀），组件 scss 内直接用 $uni-* 变量。
 * 布局尺寸值（字号/圆角/尺寸/阴影）保留在本文件。
 */

/** 字号（750 基准，单位 rpx） */
export type FontSizeTokens = {
  display: string
  title: string
  bodyL: string
  body: string
  caption: string
  number: string
}

export const FontSize: FontSizeTokens = {
  display: '56rpx',
  title: '40rpx',
  bodyL: '34rpx',
  body: '30rpx',
  caption: '26rpx',
  number: '48rpx'
}

/** 圆角（750 基准，单位 rpx） */
export type RadiusTokens = {
  card: string
  button: string
  tag: string
  sheet: string
}

export const Radius: RadiusTokens = {
  card: '24rpx',
  button: '20rpx',
  tag: '12rpx',
  sheet: '32rpx'
}

/** 关键尺寸（750 基准，单位 rpx） */
export type SizeTokens = {
  /** 最小触控目标 */
  touch: string
  /** 主按钮/输入框高度 */
  btnHeight: string
  /** 列表行最小高度 */
  listRow: string
  /** 底部导航中央扫码按钮直径 */
  scanBtn: string
  /** tabBar 内容区高度（不含 safe-area） */
  tabBar: string
}

export const Size: SizeTokens = {
  /** 最小触控目标 */
  touch: '88rpx',
  /** 主按钮/输入框高度 */
  btnHeight: '104rpx',
  /** 列表行最小高度 */
  listRow: '128rpx',
  /** 底部导航中央扫码按钮直径 */
  scanBtn: '112rpx',
  /** tabBar 内容区高度（不含 safe-area） */
  tabBar: '112rpx'
}

/** 卡片唯一一级阴影 */
export const ShadowCard = '0 4rpx 16rpx rgba(0, 0, 0, 0.06)'
