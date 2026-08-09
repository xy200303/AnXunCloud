// 菜单图标注册表：按需引入，替代 main.ts 的全量注册（293 个组件）
// 新增菜单需要图标时在此补充，菜单管理表单的图标下拉即读取本表
import type { Component } from 'vue'
import {
  Odometer, Location, MapLocation, Calendar, Monitor, Document, Tools, List,
  DataAnalysis, TrendCharts, Medal, OfficeBuilding, Setting, User, Avatar, Menu,
  Collection, Operation, Bell, Tickets, UserFilled,
  House, Folder, Files, Grid, Histogram, PieChart, Clock, Flag, Position, Compass,
  Warning, Key, Lock, Message, Camera, Picture, Notebook, Finished, Aim, Guide,
  Place, Platform, Stamp, Link, Star, PriceTag, Suitcase
} from '@element-plus/icons-vue'

export const menuIconMap: Record<string, Component> = {
  Odometer, Location, MapLocation, Calendar, Monitor, Document, Tools, List,
  DataAnalysis, TrendCharts, Medal, OfficeBuilding, Setting, User, Avatar, Menu,
  Collection, Operation, Bell, Tickets, UserFilled,
  House, Folder, Files, Grid, Histogram, PieChart, Clock, Flag, Position, Compass,
  Warning, Key, Lock, Message, Camera, Picture, Notebook, Finished, Aim, Guide,
  Place, Platform, Stamp, Link, Star, PriceTag, Suitcase
}

export const menuIconOptions = Object.keys(menuIconMap)
