// ECharts 按需引入（对应 UI 文档 §五 图表规范；本阶段工作台为占位，后续统计页使用）
// 用色常量见 @/utils/chart-colors（不引入 echarts 本体，可进首屏静态依赖）
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

export default echarts
