/**
 * 设计令牌（唯一来源）——对应《移动端四端UI-UX设计方案》§二，750 设计稿基准。
 *
 * 使用方式：
 * - 颜色一律在模板里通过 :style 绑定引用本文件常量，页面 style 块内不写死色值；
 *   布局尺寸值同理尽量引用本文件。
 * - App.vue 全局样式与 pages.json 中的色值与本文件保持一致（修改时同步）。
 */
export type ColorTokens = {
  primary: string
  primaryLight: string
  success: string
  warning: string
  danger: string
  info: string
  bgPage: string
  bgCard: string
  textPrimary: string
  textRegular: string
  textSecondary: string
  border: string
  white: string
  /** 弹层遮罩 45% 黑 */
  mask: string
}

export const Colors: ColorTokens = {
  primary: '#2B5AED',
  primaryLight: '#EAEFFF',
  success: '#2BA471',
  warning: '#ED7B2F',
  danger: '#D54941',
  info: '#909399',
  bgPage: '#F5F6F8',
  bgCard: '#FFFFFF',
  textPrimary: '#1F2329',
  textRegular: '#4E5969',
  textSecondary: '#86909C',
  border: '#E5E6EB',
  white: '#FFFFFF',
  /** 弹层遮罩 45% 黑 */
  mask: 'rgba(0, 0, 0, 0.45)'
}

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

/** 间距（8 的倍数，750 基准，单位 rpx） */
export type SpacingTokens = {
  s8: string
  s16: string
  s24: string
  s32: string
  s48: string
  s64: string
}

export const Spacing: SpacingTokens = {
  s8: '8rpx',
  s16: '16rpx',
  s24: '24rpx',
  s32: '32rpx',
  s48: '48rpx',
  s64: '64rpx'
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
