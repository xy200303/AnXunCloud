<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 加载中 / 失败 -->
    <view v-if="loading" class="hint">
      <text class="hint-text" :style="{ color: colors.textSecondary }">加载中…</text>
    </view>
    <view v-else-if="!loaded" class="hint">
      <text class="hint-text" :style="{ color: colors.textRegular }">{{ errorMsg }}</text>
      <text class="hint-retry" :style="{ color: colors.primary }" @click="load">重试</text>
    </view>

    <block v-else>
      <!-- 头部：点位 + 结果 + 元信息 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <view class="head-row">
          <text class="point-name" :style="{ color: colors.textPrimary }">{{ pointName }}</text>
          <text
            class="result-badge"
            :style="{
              color: result == 'abnormal' ? colors.danger : colors.success,
              backgroundColor: result == 'abnormal' ? '#FDECEC' : '#E7F6EF'
            }"
          >{{ result == 'abnormal' ? '⚠ 有异常' : '✓ 正常' }}</text>
        </view>
        <text v-if="buildingName != ''" class="head-sub" :style="{ color: colors.textSecondary }">{{ buildingName }}</text>
        <view class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">打卡时间</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ checkinTime }}</text>
        </view>
        <view class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">打卡方式</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ checkinTypeText }}</text>
        </view>
        <view v-if="distance != null" class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">距点位</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ distance }} 米</text>
        </view>
        <view v-if="altitude != null || accuracy != null" class="meta-row">
          <text class="meta-label" :style="{ color: colors.textSecondary }">定位信息</text>
          <text class="meta-value" :style="{ color: colors.textRegular }">{{ locInfoText }}</text>
        </view>
      </view>

      <!-- 逐项明细 -->
      <view v-for="(it, i) in items" :key="i" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <view class="item-head">
          <text class="item-name" :style="{ color: colors.textPrimary }">{{ it.name }}</text>
          <text class="item-result" :style="{ color: it.pass ? colors.success : colors.danger }">{{ it.pass ? '✓ 正常' : '⚠ 异常' }}</text>
        </view>
        <image
          v-if="it.photo_urls != null && it.photo_urls.length > 0"
          :src="it.photo_urls[0]"
          class="item-photo"
          mode="aspectFill"
          @click="preview(it)"
        />
        <text v-if="it.ai_reason != null && it.ai_reason != ''" class="item-ai" :style="{ color: colors.textSecondary }">{{ it.ai_reason }}</text>
        <text v-if="it.note != null && it.note != ''" class="item-note" :style="{ color: colors.textRegular }">备注：{{ it.note }}</text>
      </view>
      <view v-if="items.length == 0" class="card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
        <text class="hint-text" :style="{ color: colors.textSecondary }">这次是纯打卡，没有检查项</text>
      </view>

      <!-- 修改入口（提交即封存：可改时绿色大按钮，不可改时置灰写明原因） -->
      <view class="bottom">
        <view v-if="canModify" class="btn-big" hover-class="hover-dim" :style="{ backgroundColor: colors.primary }" @click="goModify">
          <text class="btn-big-text" :style="{ color: colors.white }">修改本次打卡</text>
        </view>
        <view v-else class="btn-big" :style="{ backgroundColor: colors.border }">
          <text class="btn-big-text" :style="{ color: colors.textSecondary }">{{ lockReason }}</text>
        </view>
        <text v-if="canModify" class="bottom-tip" :style="{ color: colors.textSecondary }">修改需要重新拍照，提交后覆盖原记录</text>
      </view>
    </block>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens, ShadowCard } from '@/utils/theme'
import { apiTaskDetail, apiCheckinItems, CheckinItemAI } from '@/services/api'

/** 打卡方式展示映射（checkin_type：qrcode/fence/nfc/offline） */
function checkinTypeTextOf(t: string): string {
  if (t == 'qrcode') return '扫二维码'
  if (t == 'nfc') return '刷 NFC 卡'
  if (t == 'fence') return '到场确认'
  if (t == 'offline') return '离线补传'
  return t
}

export default {
  data() {
    return {
      colors: Colors,
      shadow: ShadowCard,
      taskId: '',
      pointId: '',
      loading: true,
      loaded: false,
      errorMsg: '加载失败',
      pointName: '',
      buildingName: '',
      result: '',
      checkinTime: '',
      checkinType: '',
      distance: null as number | null,
      altitude: null as number | null,
      accuracy: null as number | null,
      locked: false,
      taskStatus: '',
      aiEnabled: false,
      items: [] as CheckinItemAI[]
    }
  },
  computed: {
    checkinTypeText(): string {
      return checkinTypeTextOf(this.checkinType)
    },
    /** 定位辅助信息文案：海拔 xx 米 · 精度 xx 米（仅有值的部分） */
    locInfoText(): string {
      const parts: string[] = []
      if (this.altitude != null) parts.push('海拔 ' + Math.round(this.altitude) + ' 米')
      if (this.accuracy != null) parts.push('精度 ±' + Math.round(this.accuracy) + ' 米')
      return parts.join(' · ')
    },
    /** 可修改：AI 启用（修改走向导重拍重识别）+ 未归档锁定 + 任务未巡完（收工即定稿） */
    canModify(): boolean {
      return this.aiEnabled && !this.locked && this.taskStatus != 'done'
    },
    lockReason(): string {
      if (this.locked) return '月度报告已归档封存，不可修改'
      if (this.taskStatus == 'done') return '任务已巡完，不能改了'
      if (!this.aiEnabled) return 'AI 未启用，暂不支持修改'
      return '当前不可修改'
    }
  },
  onLoad(options: any) {
    this.taskId = options && options.task_id ? String(options.task_id) : ''
    this.pointId = options && options.point_id ? String(options.point_id) : ''
    this.load()
  },
  onShow() {
    // 修改返回后刷新（覆盖修改产生新记录）
    if (this.loaded) this.load()
  },
  methods: {
    load() {
      if (this.taskId == '' || this.pointId == '') {
        this.loading = false
        this.errorMsg = '缺少参数'
        return
      }
      this.loading = !this.loaded
      apiTaskDetail(this.taskId)
        .then((res) => {
          const pt = res.points.find((p) => p.point_id == this.pointId)
          if (pt == null || pt.my_checkin == null) {
            this.loading = false
            this.errorMsg = '没有找到打卡记录'
            return
          }
          this.pointName = pt.point_name
          this.buildingName = pt.building_name
          this.checkinTime = pt.my_checkin.checkin_time
          this.checkinType = pt.my_checkin.checkin_type
          this.distance = pt.my_checkin.distance_to_point
          this.altitude = pt.my_checkin.altitude ?? null
          this.accuracy = pt.my_checkin.accuracy ?? null
          this.result = pt.my_checkin.result
          this.locked = pt.my_checkin.locked
          this.taskStatus = res.status
          this.aiEnabled = res.ai_enabled ?? false
          return apiCheckinItems(pt.my_checkin.id).then((items) => {
            this.items = items
            this.loading = false
            this.loaded = true
          })
        })
        .catch((e: Error) => {
          this.loading = false
          if (!this.loaded) this.errorMsg = e.message
          else uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    preview(it: CheckinItemAI) {
      if (it.photo_urls == null || it.photo_urls.length == 0) return
      uni.previewImage({ urls: it.photo_urls })
    },
    /** 修改 = 重走该点位向导（逐项重拍 + AI 重识别，提交覆盖原记录并留痕） */
    goModify() {
      if (!this.canModify) return
      uni.navigateTo({
        url:
          '/pages/checkin/quick?task_id=' + encodeURIComponent(this.taskId) +
          '&point_id=' + encodeURIComponent(this.pointId) + '&mode=modify'
      })
    }
  }
}
</script>

<style scoped>
.page {
  flex: 1;
  padding: 24rpx;
}

.hint {
  align-items: center;
  padding-top: 192rpx;
}

.hint-text {
  font-size: 30rpx;
}

.hint-retry {
  margin-top: 24rpx;
  font-size: 30rpx;
}

.card {
  border-radius: 24rpx;
  padding: 28rpx;
  margin-bottom: 24rpx;
}

.head-row {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.point-name {
  font-size: 36rpx;
  font-weight: 600;
  flex: 1;
}

.result-badge {
  font-size: 26rpx;
  font-weight: 600;
  padding: 8rpx 20rpx;
  border-radius: 999rpx;
}

.head-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.meta-row {
  flex-direction: row;
  justify-content: space-between;
  margin-top: 20rpx;
}

.meta-label {
  font-size: 28rpx;
}

.meta-value {
  font-size: 28rpx;
}

.item-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.item-name {
  font-size: 32rpx;
  font-weight: 600;
  flex: 1;
}

.item-result {
  font-size: 28rpx;
  font-weight: 600;
}

.item-photo {
  width: 100%;
  height: 380rpx;
  border-radius: 16rpx;
  margin-top: 20rpx;
}

.item-ai {
  font-size: 26rpx;
  margin-top: 16rpx;
  line-height: 1.6;
}

.item-note {
  font-size: 26rpx;
  margin-top: 12rpx;
}

.bottom {
  margin-top: 16rpx;
  padding-bottom: 48rpx;
}

.btn-big {
  height: 104rpx;
  border-radius: 52rpx;
  align-items: center;
  justify-content: center;
}

.btn-big-text {
  font-size: 34rpx;
  font-weight: 600;
}

.bottom-tip {
  font-size: 24rpx;
  text-align: center;
  margin-top: 20rpx;
}
</style>
