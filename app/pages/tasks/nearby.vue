<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 顶部说明：GPS 只能定到楼栋级，楼内仍需扫码/NFC 确认 -->
    <view class="tip" :style="{ backgroundColor: colors.primaryLight }">
      <text class="tip-text" :style="{ color: colors.primary }">按距离推荐今日任务点位，点击直达打卡；楼内密集点位请以扫码/NFC 为准</text>
    </view>

    <!-- 定位中 -->
    <view v-if="loading" class="center-box">
      <text class="center-text" :style="{ color: colors.textSecondary }">正在获取位置…</text>
    </view>

    <!-- 定位/加载失败 -->
    <view v-else-if="errorMsg != ''" class="center-box">
      <text class="center-text" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="center-retry" :style="{ color: colors.primary }" @click="load">重新定位</text>
    </view>

    <!-- 空态 -->
    <view v-else-if="loaded && list.length == 0" class="center-box">
      <text class="center-text" :style="{ color: colors.textRegular }">今天没有待巡的任务点位</text>
      <text class="center-sub" :style="{ color: colors.textSecondary }">或点位还没有坐标（可在电脑端导入时留空，现场到点位编辑「获取当前位置」补齐）</text>
    </view>

    <!-- 点位列表 -->
    <view v-else-if="loaded" class="content">
      <view
        v-for="(p, i) in list"
        :key="p.task_id + '-' + p.point_id"
        hover-class="hover-dim"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="onTap(p)"
      >
        <view class="row-main">
          <view class="dist-badge" :style="{ backgroundColor: p.checked ? colors.border : (p.distance <= 50 ? colors.success : colors.primary) }">
            <text class="dist-text" :style="{ color: colors.white }">{{ p.distance }}m</text>
          </view>
          <view class="row-texts">
            <text class="point-name" :style="{ color: colors.textPrimary }">{{ p.point_name }}</text>
            <text class="point-sub" :style="{ color: colors.textSecondary }">{{ (p.building_name || '未分区') + ' · ' + p.plan_name }}</text>
          </view>
        </view>
        <view class="row-side">
          <text v-if="p.checked" class="state-text" :style="{ color: colors.success }">已打卡</text>
          <text v-else class="state-text" :style="{ color: colors.warning }">{{ credentialTextOf(p.credential, p.require_fence) }}</text>
        </view>
      </view>
      <text class="foot-note" :style="{ color: colors.textSecondary }">仅显示最近 {{ list.length }} 个点位 · 下拉可刷新距离</text>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { getLocationGcj02 } from '@/utils/geo'
import { apiNearbyPoints, NearbyPoint } from '@/services/api'

type PageData = {
  colors: ColorTokens
  loading: boolean
  loaded: boolean
  errorMsg: string
  list: NearbyPoint[]
  aiEnabled: boolean
}

function credentialTextOf(credential: string, requireFence: boolean): string {
  let base = '免凭证'
  if (credential == 'qrcode') base = '扫码'
  else if (credential == 'nfc') base = 'NFC'
  else if (credential == 'any') base = '扫码/NFC'
  return requireFence ? base + '+围栏' : base
}

export default {
  data(): PageData {
    return {
      colors: Colors,
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
      // 未打卡：AI 启用进连续巡检向导（从该点位开始），否则进手动表单
      if (this.aiEnabled) {
        uni.navigateTo({
          url: '/pages/checkin/quick?task_id=' + encodeURIComponent(p.task_id) +
            '&point_id=' + encodeURIComponent(p.point_id)
        })
      } else {
        uni.navigateTo({
          url: '/pages/checkin/form?task_id=' + encodeURIComponent(p.task_id) +
            '&point_id=' + encodeURIComponent(p.point_id)
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

.card {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-radius: 16rpx;
  padding: 24rpx;
  margin-bottom: 16rpx;
}

.row-main {
  flex-direction: row;
  align-items: center;
  flex: 1;
  min-width: 0;
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

.row-texts {
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.point-name {
  font-size: 30rpx;
  font-weight: 500;
}

.point-sub {
  font-size: 24rpx;
  margin-top: 6rpx;
}

.row-side {
  margin-left: 16rpx;
}

.state-text {
  font-size: 24rpx;
}

.foot-note {
  font-size: 22rpx;
  text-align: center;
  padding: 16rpx 0 32rpx;
}
</style>
