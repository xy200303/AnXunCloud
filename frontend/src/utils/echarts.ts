// ECharts 按需引入（对应 UI 文档 §五 图表规范；本阶段工作台为占位，后续统计页使用）
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import {
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
  LineChart,
  BarChart,
  PieChart,
  GridComponent,
  TooltipComponent,
  LegendComponent,
  TitleComponent,
  CanvasRenderer
])

// 图表用色：设计令牌的 JS 镜像（ECharts option 无法读 SCSS 变量，集中在此维护）
// 与 src/styles/variables.scss 保持一致
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

export default echarts
