<template>
  <view class="page bg-page" >
    <!-- 状态筛选 chips（客户端过滤：一次拉最近 100 条） -->
    <view class="filter-bar bg-card border-default" >
      <uni-segmented-control
        :values="statusChips.map((c) => c.label)"
        :current="statusTabIdx"
        style-type="button"
        active-color="#2B5AED"
        @clickItem="onStatusTab"
      />
    </view>

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="filtered.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="还没有提交记录"
      empty-sub="在维保待办里选设备拍新标签即可提交"
     
      @retry="load"
    >
      <template #default>
      <view class="content">
        <view v-for="m in filtered" :key="m.id" class="card bg-card" >
          <view class="card-head">
            <text class="card-title text-main" >{{ m.equipment_name }}</text>
            <text class="card-status" :style="{ color: statusColor(m.confirm_status) }">{{ statusText(m.confirm_status) }}</text>
          </view>
          <text class="card-sub text-secondary" >编号：{{ m.equipment_code }} · 维保日期 {{ m.maintenance_date }}</text>
          <text v-if="m.point_name" class="card-sub text-secondary" >点位：{{ m.point_name }}</text>

          <!-- 标签照片（点击放大） -->
          <view v-if="m.photos.length > 0" class="thumbs">
            <image
              v-for="(p, pi) in m.photos"
              :key="p.file_id"
              :src="toAbsUrl(p.url)"
              class="thumb"
              lazy-load
              mode="aspectFill"
              @click="preview(m, pi)"
            />
          </view>

          <!-- AI 核对结论（经理确认前的参考意见） -->
          <text v-if="m.ai_reason" class="ai-line text-secondary" >系统核对：{{ m.ai_reason }}</text>

          <!-- 驳回原因（红底提示） -->
          <view v-if="m.confirm_status == 'rejected' && m.reject_reason" class="reject-box" :style="{ backgroundColor: '#FDECEC' }">
            <text class="reject-text text-danger" >经理驳回：{{ m.reject_reason }}</text>
          </view>

          <!-- 操作：待确认可修改照片；已驳回可重新拍照登记 -->
          <view
            v-if="m.confirm_status == 'pending'"
            hover-class="hover-dim"
            class="btn-outline border-brand"
            
            @click="goEdit(m)"
          >
            <text class="btn-outline-text text-brand" >修改照片</text>
          </view>
          <view
            v-else-if="m.confirm_status == 'rejected'"
            hover-class="hover-dim"
            class="btn-outline border-brand"
            
            @click="goReshoot(m)"
          >
            <text class="btn-outline-text text-brand" >重新拍照登记</text>
          </view>
          <text v-else class="done-line text-secondary" >
            {{ m.confirmed_by_name ? '经理 ' + m.confirmed_by_name + ' 已确认，台账已更新' : '系统核对通过，台账已更新' }}
          </text>
        </view>
      </view>
      </template>
    </AppListShell>
  </view>
</template>

<script lang="ts">

import { apiMaintenanceMine, MaintenanceItem } from '@/services/api'
import { toAbsUrl } from '@/utils/url'
import { MAINTAIN_EDIT_KEY } from '@/utils/storage'
import { toastErr } from '@/utils/ui'
import AppListShell from '@/components/AppListShell.vue'

type PageData = {
  statusFilter: string
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: MaintenanceItem[]
}

export default {
  components: { AppListShell },
  data(): PageData {
    return {
      statusFilter: '',
      loading: true,
      loaded: false,
      errorMsg: '',
      list: [] as MaintenanceItem[]
    }
  },
  computed: {
    statusChips(): Array<{ value: string; label: string }> {
      return [
        { value: '', label: '全部' },
        { value: 'pending', label: '待确认' },
        { value: 'confirmed', label: '已生效' },
        { value: 'rejected', label: '已驳回' }
      ]
    },
    /** 分段器当前下标（statusChips 下标 ↔ statusFilter 值换算） */
    statusTabIdx(): number {
      const i = this.statusChips.findIndex((c) => c.value == this.statusFilter)
      return i >= 0 ? i : 0
    },
    filtered(): MaintenanceItem[] {
      if (this.statusFilter == '') return this.list
      return this.list.filter((m) => m.confirm_status == this.statusFilter)
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 修改照片/重新登记返回后刷新状态
    if (this.loaded) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    onStatusTab(e: { currentIndex: number }) {
      const c = this.statusChips[e.currentIndex]
      if (c != null) this.statusFilter = c.value
    },
    toAbsUrl,
    statusText(s: string): string {
      if (s == 'confirmed') return '已生效'
      if (s == 'rejected') return '已驳回'
      return '待确认'
    },
    statusColor(s: string): string {
      if (s == 'confirmed') return '#2BA471'
      if (s == 'rejected') return '#D54941'
      return '#2B5AED'
    },
    load() {
      this.loading = !this.loaded
      apiMaintenanceMine(1, 100)
        .then((p) => {
          this.list = p.list
          this.loading = false
          this.loaded = true
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          else toastErr(e)
          uni.stopPullDownRefresh()
        })
    },
    preview(m: MaintenanceItem, idx: number) {
      uni.previewImage({ urls: m.photos.map((p) => toAbsUrl(p.url)), current: idx })
    },
    /** 修改照片：带草稿进维保拍照页编辑模式（record 仍 pending 才可改，后端兜底校验） */
    goEdit(m: MaintenanceItem) {
      uni.setStorageSync(MAINTAIN_EDIT_KEY, {
        id: m.id,
        equipment_id: m.equipment_id,
        name: m.equipment_name,
        code: m.equipment_code,
        photos: m.photos
      })
      uni.navigateTo({
        url:
          '/pages/equipment/maintain?maintenance_id=' + encodeURIComponent(m.id) +
          '&equipment_id=' + encodeURIComponent(m.equipment_id) +
          '&name=' + encodeURIComponent(m.equipment_name) +
          '&code=' + encodeURIComponent(m.equipment_code)
      })
    },
    /** 已驳回：同设备重新拍照登记（新流水，不带旧照片） */
    goReshoot(m: MaintenanceItem) {
      uni.navigateTo({
        url:
          '/pages/equipment/maintain?equipment_id=' + encodeURIComponent(m.equipment_id) +
          '&name=' + encodeURIComponent(m.equipment_name) +
          '&code=' + encodeURIComponent(m.equipment_code)
      })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
}

.filter-bar {
  padding: 16rpx 24rpx 24rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.content {
  padding: 24rpx;
}

.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 34rpx;
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  font-weight: 600;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.thumbs {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
}

.thumb {
  width: 144rpx;
  height: 144rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
  margin-bottom: 8rpx;
}

.ai-line {
  font-size: 24rpx;
  margin-top: 12rpx;
  line-height: 36rpx;
}

.reject-box {
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-top: 16rpx;
}

.reject-text {
  font-size: 26rpx;
  line-height: 40rpx;
}

.btn-outline {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-top: 24rpx;
}

.btn-outline-text {
  font-size: 32rpx;
  font-weight: 600;
}

.done-line {
  font-size: 24rpx;
  margin-top: 16rpx;
}
</style>
