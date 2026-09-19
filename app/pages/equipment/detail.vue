<template>
  <view class="page bg-page" >
    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="false"
      :error="errorMsg"
     
      @retry="load"
    >
      <template #default>
      <view class="content" v-if="detail != null">
        <!-- 基础信息 -->
        <view class="card bg-card" >
          <view class="card-head">
            <view class="card-title-row">
              <uni-badge :is-dot="true" :custom-style="{ backgroundColor: dueColorOf(detail.due_state), marginRight: '12rpx' }" />
              <text class="card-title text-main" >{{ detail.name }}</text>
            </view>
            <text class="card-status" :style="{ color: dueColorOf(detail.due_state) }">{{ dueText }}</text>
          </view>
          <text class="info-line text-regular" >编号：{{ detail.code }}</text>
          <text class="info-line text-regular" >类型：{{ detail.type_label != '' ? detail.type_label : detail.type }}</text>
          <text class="info-line text-regular" >位置：{{ locationText }}</text>
          <text class="info-line text-regular" >出厂日期：{{ detail.manufacture_date != '' ? detail.manufacture_date : '未录' }}</text>
          <text class="info-line text-regular" >最近维保：{{ detail.last_maintenance_date != '' ? detail.last_maintenance_date : '无记录' }}</text>
          <text class="info-line"  :class="(detail.due_state == 'overdue' ? 'text-danger' : 'text-regular')">
            下次到期：{{ detail.next_due_date != '' ? detail.next_due_date : '无到期日（无规则或未录日期）' }}
          </text>
          <text v-if="detail.scrap_date != ''" class="info-line text-regular" >报废日期：{{ detail.scrap_date }}</text>
          <text class="info-line text-regular" >状态：{{ detail.status_label }}</text>
          <text v-if="detail.remark != ''" class="info-line text-secondary" >备注：{{ detail.remark }}</text>
        </view>

        <!-- 档案信息（extra 口袋键值展示，空值不显示） -->
        <view v-if="archiveRows.length > 0" class="card bg-card" >
          <text class="sec-title text-main" >档案信息</text>
          <view v-for="r in archiveRows" :key="r.key" class="archive-row">
            <text class="archive-label text-secondary" >{{ r.label }}</text>
            <text class="archive-value text-regular" >{{ r.value }}</text>
          </view>
        </view>

        <!-- 维保历史时间线 -->
        <view class="card bg-card" >
          <text class="sec-title text-main" >维保历史</text>
          <view v-if="history.length == 0 && historyLoaded">
            <text class="history-empty text-secondary" >暂无维保记录</text>
          </view>
          <view v-for="m in history" :key="m.id" class="history-item">
            <view class="history-rail">
              <uni-badge :is-dot="true" :custom-style="{ backgroundColor: confirmColorOf(m.confirm_status), marginRight: '12rpx' }" />
              <view class="history-line-rail bg-border" ></view>
            </view>
            <view class="history-body">
              <view class="history-head">
                <text class="history-date text-main" >{{ m.maintenance_date }}</text>
                <text class="history-type text-brand" >{{ maintTypeText(m.maintenance_type) }}</text>
                <text class="history-status" :style="{ color: confirmColorOf(m.confirm_status) }">{{ confirmTextOf(m.confirm_status) }}</text>
              </view>
              <text class="history-line text-regular" >经办人：{{ m.operator_name }}<text v-if="m.vendor != null && m.vendor != ''">　单位：{{ m.vendor }}</text></text>
              <text v-if="m.ai_verdict == 'review'" class="history-line text-warning" >AI 存疑{{ m.ai_reason != null && m.ai_reason != '' ? '：' + m.ai_reason : '' }}</text>
              <text v-if="m.reject_reason != null && m.reject_reason != ''" class="history-line text-danger" >驳回理由：{{ m.reject_reason }}</text>
              <text v-if="m.note != ''" class="history-line text-secondary" >备注：{{ m.note }}</text>
              <view v-if="m.photos.length > 0" class="photos">
                <image
                  v-for="(p, pi) in m.photos"
                  :key="p.file_id"
                  class="photo"
                  :src="photoUrl(p.url)"
                  mode="aspectFill"
                  lazy-load
                  @click="previewHistory(m, pi)"
                />
              </view>
            </view>
          </view>
          <view v-if="historyMore" class="history-more" @click="loadMoreHistory">
            <text class="history-more-text text-brand" >加载更多</text>
          </view>
        </view>

        <view class="bottom-space"></view>
      </view>
      </template>
    </AppListShell>

    <!-- 底部维保登记按钮（逾期红/临期黄高亮） -->
    <view v-if="detail != null && canRegister" class="footer-bar bg-card border-default" >
      <view
        hover-class="hover-dim"
        class="btn-primary"
        :style="{ backgroundColor: detail.due_state == 'overdue' ? '#D54941' : (detail.due_state == 'warning' ? '#ED7B2F' : '#2B5AED') }"
        @click="goRegister"
      >
        <text class="btn-primary-text text-white" >维保登记</text>
      </view>
    </view>
  </view>
</template>

<script lang="ts">

import { apiEquipmentDetail, apiMaintenanceHistory, apiDictOptions, EquipmentDetail, MaintenanceItem, DictOption } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { toAbsUrl } from '@/utils/url'
import AppListShell from '@/components/AppListShell.vue'

const PAGE_SIZE = 20

type DetailData = {
  id: string
  detail: EquipmentDetail | null
  maintTypeOptions: DictOption[]
  history: MaintenanceItem[]
  historyLoaded: boolean
  historyPage: number
  historyTotal: number
  loading: boolean
  loaded: boolean
  errorMsg: string
}

function dueColorOf(s: string): string {
  if (s == 'normal') return '#2BA471'
  if (s == 'warning') return '#ED7B2F'
  if (s == 'overdue' || s == 'scrap') return '#D54941'
  return '#909399'
}

function confirmColorOf(s: string): string {
  if (s == 'confirmed') return '#2BA471'
  if (s == 'rejected') return '#D54941'
  return '#ED7B2F'
}

function confirmTextOf(s: string): string {
  if (s == 'confirmed') return '已确认'
  if (s == 'rejected') return '已驳回'
  return '待确认'
}

function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

export default {
  components: { AppListShell },
  data(): DetailData {
    return {
      id: '',
      detail: null,
      maintTypeOptions: [] as DictOption[],
      history: [] as MaintenanceItem[],
      historyLoaded: false,
      historyPage: 1,
      historyTotal: 0,
      loading: true,
      loaded: false,
      errorMsg: ''
    }
  },
  computed: {
    dueText(): string {
      const d = this.detail
      if (d == null) return ''
      if (d.due_state == 'label_missing') return '标签缺失，待经理处置'
      if (d.due_state == 'scrap') return '应报废' + (d.scrap_date != '' ? '（' + d.scrap_date + '）' : '')
      if (d.due_state == 'none') return '无到期日'
      if (d.next_due_date == '') return ''
      const due = new Date(d.next_due_date.replace(/-/g, '/')).getTime()
      const days = Math.round((due - todayZero()) / 86400000)
      if (days < 0) return '已逾期 ' + (-days) + ' 天'
      if (days == 0) return '今天到期'
      if (d.due_state == 'warning') return days + ' 天后到期'
      return '正常'
    },
    locationText(): string {
      const d = this.detail
      if (d == null) return ''
      const parts: string[] = []
      if (d.community_name != '') parts.push(d.community_name)
      if (d.building_name != null && d.building_name != '') parts.push(d.building_name)
      if (d.point_name != null && d.point_name != '') parts.push(d.point_name)
      return parts.length > 0 ? parts.join(' · ') : '未绑定点位'
    },
    historyMore(): boolean {
      return this.historyLoaded && this.history.length < this.historyTotal
    },
    /** 档案信息：extra 口袋按键值中文对照展示（空值不显示） */
    archiveRows(): Array<{ key: string; label: string; value: string }> {
      const d = this.detail
      if (d == null || d.extra == null) return []
      const labels: Array<[string, string]> = [
        ['project_name', '项目名称'], ['room', '机房名称'], ['level', '设备等级'], ['dept', '责任部门'],
        ['system', '所属设备系统'], ['brand', '品牌'], ['spec', '规格型号'], ['original_value', '设备原值'],
        ['quantity', '数量'], ['put_into_service', '投运日期'], ['maint_status', '维保状态'], ['run_status', '运行状态'],
        ['origin', '产地'], ['manufacturer_contact', '厂家联系人'], ['installer_contact', '安装单位联系人电话'],
        ['vendor', '维保单位'], ['vendor_contact', '维保单位联系人电话'], ['other_info', '其他信息']
      ]
      const out: Array<{ key: string; label: string; value: string }> = []
      labels.forEach(([key, label]) => {
        const v = (d.extra as Record<string, any>)[key]
        if (v != null && String(v).trim() != '') out.push({ key, label, value: String(v) })
      })
      return out
    },
    /** 维保登记入口：equipment:maintenance 权限；已报废设备不显示（后端同样拒绝，前置避免拍完照才被拦） */
    canRegister(): boolean {
      return useAuthStore().hasPerm('equipment:maintenance') && this.detail?.status != 'scrapped'
    }
  },
  onLoad(options: any) {
    this.id = options && options.id ? String(options.id) : ''
    apiDictOptions('equipment_maint_type').then((opts) => {
      this.maintTypeOptions = opts
    }).catch(() => {})
    this.load()
  },
  onShow() {
    // 登记返回后刷新（确认链生效后台账才会变，但待确认状态可能变化）
    if (this.loaded) this.load()
  },
  methods: {
    dueColorOf,
    confirmColorOf,
    confirmTextOf,
    maintTypeText(t: string): string {
      const o = this.maintTypeOptions.find((x) => x.value == t)
      return o != null ? o.label : t
    },
    load() {
      if (this.id == '') {
        this.loading = false
        this.errorMsg = '缺少设备参数'
        return
      }
      this.loading = !this.loaded
      apiEquipmentDetail(this.id)
        .then((d) => {
          this.detail = d
          this.loading = false
          this.loaded = true
          this.historyPage = 1
          this.fetchHistory(false)
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
        })
    },
    fetchHistory(append: boolean) {
      apiMaintenanceHistory(this.id, this.historyPage, PAGE_SIZE)
        .then((res) => {
          this.historyTotal = res.total
          this.history = append ? this.history.concat(res.list) : res.list
          this.historyLoaded = true
        })
        .catch((e: Error) => {
          this.historyLoaded = true
          uni.showToast({ title: e.message || '维保历史加载失败', icon: 'none' })
        })
    },
    loadMoreHistory() {
      if (!this.historyMore) return
      this.historyPage += 1
      this.fetchHistory(true)
    },
    photoUrl(url: string): string {
      return toAbsUrl(url)
    },
    previewHistory(m: MaintenanceItem, idx: number) {
      uni.previewImage({ urls: m.photos.map((p) => toAbsUrl(p.url)), current: idx })
    },
    goRegister() {
      const d = this.detail
      uni.navigateTo({
        url:
          '/pages/equipment/maintain?equipment_id=' + encodeURIComponent(this.id) +
          (d != null ? '&name=' + encodeURIComponent(d.name) + '&code=' + encodeURIComponent(d.code) : '')
      })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.content {
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16rpx;
}

.card-title-row {
  flex-direction: row;
  align-items: center;
  flex: 1;
}


.card-title {
  font-size: 34rpx;
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.archive-row {
  flex-direction: row;
  margin-top: 8rpx;
}

.archive-label {
  font-size: 26rpx;
  width: 220rpx;
  flex-shrink: 0;
}

.archive-value {
  font-size: 26rpx;
  flex: 1;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.history-empty {
  font-size: 26rpx;
}

/* 维保历史时间线：左侧圆点 + 竖线轨道 */
.history-item {
  flex-direction: row;
  margin-top: 8rpx;
}

.history-rail {
  width: 32rpx;
  align-items: center;
}


.history-line-rail {
  flex: 1;
  width: 2rpx;
  margin-top: 8rpx;
}

.history-body {
  flex: 1;
  padding-left: 16rpx;
  padding-bottom: 24rpx;
}

.history-head {
  flex-direction: row;
  align-items: center;
}

.history-date {
  font-size: 28rpx;
  font-weight: 600;
}

.history-type {
  font-size: 24rpx;
  margin-left: 16rpx;
}

.history-status {
  font-size: 24rpx;
  margin-left: 16rpx;
}

.history-line {
  font-size: 26rpx;
  margin-top: 4rpx;
}

.photos {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 12rpx;
}

.photo {
  width: 160rpx;
  height: 160rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.history-more {
  align-items: center;
  padding: 16rpx;
}

.history-more-text {
  font-size: 28rpx;
}

.footer-bar {
  padding: 16rpx 24rpx 32rpx;
  border-top-width: 1rpx;
  border-top-style: solid;
}

.btn-primary {
  height: 104rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-primary-text {
  font-size: 34rpx;
  font-weight: 600;
}

.bottom-space {
  height: 64rpx;
}
</style>
