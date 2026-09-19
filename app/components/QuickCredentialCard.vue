<template>
  <!-- 凭证步（方案 §十三 无感化）：到场即自动核验——定位自动获取一次；Android NFC 常驻监听贴卡即亮；
       扫码走点击拉起全屏 uni.scanCode（官方 <camera> 不支持 App，内嵌扫码窗已废弃）；any 同屏两入口任一完成即锁死另一入口；
       核验齐全 → 「✓ 核验通过」绿色过渡 → 自动进入第一项，无「开始检查」按钮。 -->
  <view class="card bg-card" :style="{ boxShadow: shadow }">
    <text class="cred-title text-main" >到场打卡</text>

    <!-- 核验通过绿色过渡（0.6s 后自动进入第一项） -->
    <view v-if="credFlash" class="cred-flash bg-success" >
      <text class="cred-flash-text text-white" >✓ 核验通过</text>
    </view>

    <template v-else>
      <text v-if="!needsCred" class="cred-none text-success" >✓ 本点位到场直接检查</text>

      <!-- 二维码：点行拉起全屏扫码（uni.scanCode，App/小程序通吃；官方 <camera> 组件不支持 App，内嵌扫码窗方案已废弃）；
           any 已被 NFC 先行核验时锁死不渲染 -->
      <template v-if="point != null && (point.credential == 'qrcode' || point.credential == 'any')">
        <view v-if="wizPoint != null && wizPoint.scannedNo != ''" class="cred-done-row">
          <text class="cred-done-text text-success" >{{ credType == 'qrcode' && verifiedHm != '' ? '✓ 已于 ' + verifiedHm + ' 核验' : '✓ 已通过二维码核验' }}</text>
        </view>
        <view v-else-if="!anyLockedByNfc" hover-class="hover-dim" class="cred-row" @click="$emit('scan-fallback')">
          <text class="cred-row-name text-main" >扫点位二维码</text>
          <text class="cred-status text-brand" >点我扫码 ›</text>
        </view>
      </template>

      <!-- any 同屏：两种方式，任选其一 -->
      <text v-if="point != null && point.credential == 'any' && wizPoint != null && wizPoint.scannedNo == '' && wizPoint.nfcCardId == ''" class="cred-any-hint text-secondary" >
        两种方式，任选其一
      </text>

      <!-- NFC：贴卡自动识别（Android 常驻监听）；any 已被扫码先行核验时锁死不渲染 -->
      <template v-if="point != null && (point.credential == 'nfc' || point.credential == 'any')">
        <view v-if="wizPoint != null && wizPoint.nfcCardId != ''" class="cred-done-row">
          <text class="cred-done-text text-success" >{{ credType == 'nfc' && verifiedHm != '' ? '✓ 已于 ' + verifiedHm + ' 核验' : '✓ 已通过 NFC 核验' }}</text>
        </view>
        <view v-else-if="!anyLockedByScan" hover-class="hover-dim" class="cred-row" @click="$emit('nfc-tap')">
          <text class="cred-row-name text-main" >NFC</text>
          <text class="cred-status text-secondary" >贴卡自动识别…</text>
        </view>
      </template>

      <!-- 围栏：进凭证步自动获取一次，不实时刷新；失败自动重试一次，仍失败点行重试 -->
      <view v-if="point != null && point.require_fence" hover-class="hover-dim" class="cred-row" @click="$emit('retry-location')">
        <text class="cred-row-name text-main" >📍 位置</text>
        <text v-if="locating" class="cred-status text-secondary" >定位中…</text>
        <text v-else-if="locFailed" class="cred-status text-danger" >定位失败，点我重试</text>
        <text v-else-if="credType == 'fence' && verifiedHm != '' && distance >= 0 && distance <= point.fence_radius" class="cred-status text-success" >✓ 已于 {{ verifiedHm }} 核验（距点位 {{ distance }} 米）</text>
        <text v-else-if="distance >= 0 && distance <= point.fence_radius" class="cred-status text-success" >距点位 {{ distance }} 米（围栏内 ✓）</text>
        <text v-else-if="distance >= 0" class="cred-status text-danger" >超出围栏（当前 {{ distance }} 米）</text>
        <text v-else class="cred-status text-secondary" >自动获取中…</text>
      </view>

      <!-- 核验未齐：缺哪项哪行红字说明，停在凭证步；全部核验后自动开始 -->
      <text v-if="needsCred && !(credOk && fenceOk)" class="cred-hint text-secondary" >核验齐全后自动开始检查</text>
      <!-- 全部已核验（如回退查看）：保留一个继续入口 -->
      <text v-if="needsCred && credOk && fenceOk" hover-class="hover-dim" class="cred-go text-success"  @click="$emit('start')">✓ 核验齐全，点我继续检查 ›</text>
    </template>
  </view>
</template>

<script lang="ts">
import type { WizardPointSnap } from '@/utils/checkinWizard'
import type { TaskPoint } from '@/services/api'

export default {
  props: {
    point: { type: Object as () => TaskPoint | null, default: null },
    wizPoint: { type: Object as () => WizardPointSnap | null, default: null },
    needsCred: { type: Boolean, default: false },
    credOk: { type: Boolean, default: false },
    fenceOk: { type: Boolean, default: false },
    locating: { type: Boolean, default: false },
    locFailed: { type: Boolean, default: false },
    distance: { type: Number, default: -1 },
    /** 「✓ 核验通过」绿色过渡态（0.6s，随后自动进入第一项） */
    credFlash: { type: Boolean, default: false },
    shadow: { type: String, default: '' }
  },
  emits: ['scan-fallback', 'nfc-tap', 'retry-location', 'start'],
  computed: {
    /** 云端凭证草稿恢复的核验方式（空 = 无草稿或会话内新核验） */
    credType(): string {
      return this.wizPoint != null && this.wizPoint.cred_type != null ? this.wizPoint.cred_type : ''
    },
    /** 云端凭证草稿恢复的核验时间 HH:mm（verified_at "YYYY-MM-DD HH:mm:ss" 截取；空 = 不显示） */
    verifiedHm(): string {
      const wp = this.wizPoint
      const at = wp != null && wp.cred_verified_at != null ? wp.cred_verified_at : ''
      return at.length >= 16 ? at.substring(11, 16) : ''
    },
    /** any 同屏锁死：扫码已完成 → NFC 入口隐藏 */
    anyLockedByScan(): boolean {
      return this.point != null && this.point.credential == 'any' && this.wizPoint != null && this.wizPoint.scannedNo != ''
    },
    /** any 同屏锁死：NFC 已完成 → 扫码窗收起 */
    anyLockedByNfc(): boolean {
      return this.point != null && this.point.credential == 'any' && this.wizPoint != null && this.wizPoint.nfcCardId != ''
    }
  }
}
</script>

<style scoped>
.card {
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.cred-title {
  font-size: 36rpx;
  font-weight: 700;
  margin-bottom: 8rpx;
}

.cred-none {
  font-size: 40rpx;
  font-weight: 600;
  text-align: center;
  padding: 24rpx 0;
}

/* 核验通过绿色过渡 */
.cred-flash {
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  height: 200rpx;
  margin-top: 16rpx;
}

.cred-flash-text {
  font-size: 48rpx;
  font-weight: 700;
}

.cred-any-hint {
  font-size: 26rpx;
  text-align: center;
  padding: 16rpx 0 4rpx;
}

.cred-done-row {
  min-height: 96rpx;
  flex-direction: row;
  align-items: center;
}

.cred-done-text {
  font-size: 36rpx;
  font-weight: 700;
}

.cred-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  min-height: 104rpx;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
  border-bottom-color: rgba(0, 0, 0, 0.05);
}

.cred-row-name {
  font-size: 40rpx;
  font-weight: 600;
}

.cred-status {
  font-size: 32rpx;
  font-weight: 600;
}

.cred-hint {
  font-size: 26rpx;
  text-align: center;
  margin-top: 24rpx;
}

.cred-go {
  font-size: 34rpx;
  font-weight: 700;
  text-align: center;
  padding: 32rpx 0 16rpx;
}
</style>
