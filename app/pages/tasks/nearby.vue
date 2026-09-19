<template>
  <view class="page bg-page" >
    <!-- 顶部说明：GPS 只能定到楼栋级，楼内仍需扫码/NFC 确认 -->
    <view class="tip bg-brand-light" >
      <text class="tip-text text-brand" >按距离推荐今日任务点位，点击直达打卡；楼内密集点位请以扫码/NFC 为准</text>
    </view>

    <!-- 定位中（§18.6 统一 uni-load-more 加载态） -->
    <view v-if="loading" class="center-box">
      <uni-load-more status="loading" :content-text="{ contentrefresh: '正在获取位置…' }" :color="'#86909C'" />
    </view>

    <!-- 定位/加载失败 -->
    <view v-else-if="errorMsg != ''" class="center-box">
      <text class="center-text text-regular" >{{ errorMsg }}</text>
      <text class="center-retry text-brand"  @click="load">重新定位</text>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && list.length == 0" class="center-box">
      <text class="center-text text-regular" >今天没有待巡的任务点位</text>
      <text class="center-sub text-secondary" >或点位还没有坐标（可在电脑端导入时留空，现场到点位编辑「获取当前位置」补齐）</text>
    </view>

    <!-- 点位列表（§18.2 uni-list 标准行：左距离圆徽标 + 标题/分区·计划 + 右状态徽标） -->
    <view v-else-if="loaded" class="content">
      <uni-list class="point-list bg-card" >
        <uni-list-item
          v-for="(p, i) in list"
          :key="p.task_id + '-' + p.point_id"
          :title="p.point_name"
          :note="(p.building_name || '未分区') + ' · ' + p.plan_name"
          clickable
          @click="onTap(p)"
        >
          <template #header>
            <view class="dist-badge" :style="{ backgroundColor: p.checked ? '#E5E6EB' : (p.distance <= 50 ? '#2BA471' : '#2B5AED') }">
              <text class="dist-text text-white" >{{ p.distance }}m</text>
            </view>
          </template>
          <template #footer>
            <view>
              <uni-tag v-if="p.checked" text="已打卡" type="success" :inverted="true" size="small" />
              <uni-tag v-else :text="credentialTextOf(p.credential, p.require_fence)" type="warning" :inverted="true" size="small" />
            </view>
          </template>
        </uni-list-item>
      </uni-list>
      <text class="foot-note text-secondary" >仅显示最近 {{ list.length }} 个点位 · 下拉可刷新距离</text>
    </view>
  </view>
</template>

<script lang="ts">

import { getLocationGcj02 } from '@/utils/geo'
import { apiNearbyPoints, NearbyPoint } from '@/services/api'

type PageData = {
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: NearbyPoint[]
  aiEnabled: boolean
}

function credentialTextOf(credential: string, requireFence: boolean): string {
  let base = '不需要'
  if (credential == 'qrcode') base = '扫码'
  else if (credential == 'nfc') base = 'NFC'
  else if (credential == 'any') base = '扫码/NFC'
  return requireFence ? base + '+围栏' : base
}

export default {
  data(): PageData {
    return {
      loading: true,
      loaded: false,
      errorMsg: '',
      list: [],
      aiEnabled: false
    }
  },
  onLoad() {
    this.load()
  },
  onShow() {
    // 打卡/补拍返回后刷新进度与点位状态（避免「需要手动刷新才显示」）
    if (this.loaded) this.load()
  },
  onPullDownRefresh() {
    this.load()
  },
  methods: {
    load() {
      this.loading = !this.loaded
      this.errorMsg = ''
      getLocationGcj02(
        (loc) => {
          apiNearbyPoints(loc.longitude, loc.latitude)
            .then((res) => {
              this.loading = false
              this.loaded = true
              this.list = res.list
              this.aiEnabled = res.ai_enabled
              uni.stopPullDownRefresh()
            })
            .catch((e: Error) => {
              this.loading = false
              this.errorMsg = e.message
              uni.stopPullDownRefresh()
            })
        },
        () => {
          this.loading = false
          this.errorMsg = '定位失败，请检查定位权限后重试'
          uni.stopPullDownRefresh()
        }
      )
    },
    onTap(p: NearbyPoint) {
      if (p.checked) {
        // 已打卡：进记录卡（先看后改）
        uni.navigateTo({
          url: '/pages/checkin/record?task_id=' + encodeURIComponent(p.task_id) +
            '&point_id=' + encodeURIComponent(p.point_id)
        })
        return
      }
      // 未打卡：AI 启用进连续巡检向导（从该点位开始），否则进手动档向导
      if (this.aiEnabled) {
        uni.navigateTo({
          url: '/pages/checkin/quick?task_id=' + encodeURIComponent(p.task_id) +
            '&point_id=' + encodeURIComponent(p.point_id)
        })
      } else {
        uni.navigateTo({
          url: '/pages/checkin/quick?task_id=' + encodeURIComponent(p.task_id) +
            '&point_id=' + encodeURIComponent(p.point_id) + '&mode=manual'
        })
      }
    },
    credentialTextOf(credential: string, requireFence: boolean): string {
      return credentialTextOf(credential, requireFence)
    }
  }
}
</script>

<style lang="scss" scoped>
.page {
  min-height: 100vh;
  flex-direction: column;
}

.tip {
  margin: 24rpx;
  padding: 16rpx 24rpx;
  border-radius: 12rpx;
}

.tip-text {
  font-size: 24rpx;
  line-height: 1.5;
}

.center-box {
  padding: 120rpx 48rpx;
  align-items: center;
}

.center-text {
  font-size: 28rpx;
  text-align: center;
}

.center-sub {
  font-size: 24rpx;
  text-align: center;
  margin-top: 16rpx;
  line-height: 1.5;
}

.center-retry {
  font-size: 28rpx;
  margin-top: 24rpx;
  padding: 8rpx 32rpx;
}

.content {
  padding: 0 24rpx 32rpx;
  flex-direction: column;
}



.dist-badge {
  width: 88rpx;
  height: 88rpx;
  border-radius: 44rpx;
  align-items: center;
  justify-content: center;
  margin-right: 20rpx;
}

.dist-text {
  font-size: 26rpx;
  font-weight: 600;
}






.point-list {
  border-radius: 24rpx;
  overflow: hidden;
}

.foot-note {
  font-size: 22rpx;
  text-align: center;
  padding: 16rpx 0 32rpx;
}
</style>
