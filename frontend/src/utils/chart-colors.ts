// 图表用色：设计令牌的 JS 镜像（ECharts option 无法读 SCSS 变量，集中在此维护）
// 与 src/styles/variables.scss 保持一致
// 独立成文件：不依赖 echarts 本体，避免 dashboard 首屏仅取色就把 echarts chunk 拉进静态依赖
export const CHART_COLORS = {
  primary: '#2B5AED',
  success: '#2BA471',
  warning: '#ED7B2F',
  danger: '#D54941',
  info: '#909399',
  grid: '#EBEEF5',
  textSecondary: '#86909C',
  textRegular: '#4E5969'
}
