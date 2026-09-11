<template>
  <view class="item-card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
    <text class="item-name" :style="{ color: colors.textPrimary }">{{ item.name }}</text>
    <text class="item-hint" :style="{ color: colors.textSecondary }">{{ item.requirement != '' ? item.requirement : (isPhoto ? '拍一张该项的照片' : '这项正常吗？') }}</text>

    <template v-if="isPhoto">
      <view
        v-if="item.status == 'todo' || item.status == 'failed'"
        hover-class="hover-dim"
        class="shot-empty"
        :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }"
        @click="$emit('take-photo')"
      >
        <view class="cam-icon" :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }">
          <view class="cam-lens" :style="{ borderColor: item.status == 'failed' ? colors.danger : colors.primary }"></view>
        </view>
        <text class="shot-empty-text" :style="{ color: item.status == 'failed' ? colors.danger : colors.primary }">
          {{ item.status == 'failed' ? '不合格，点这里重拍' : '点这里拍照' }}
        </text>
      </view>
      <block v-else>
        <view class="shot-preview" :style="{ backgroundColor: colors.bgPage }" @click="$emit('preview-photo')">
          <image
            v-if="item.photos.length > 0 && !item.img_error"
            :src="item.photos[0]"
            class="shot-img"
            mode="aspectFill"
            lazy-load
            @error="$emit('image-error')"
          />
          <view v-else class="shot-img shot-img-fallback">
            <text class="shot-img-fallback-text">照片加载失败，可重新拍</text>
          </view>
        </view>
        <view hover-class="hover-dim" class="btn-big shot-next" :style="{ backgroundColor: colors.success }" @click="$emit('next')">
          <text class="btn-big-text" :style="{ color: colors.white }">下一项</text>
        </view>
        <view v-if="item.status != 'recognizing'" hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
          <text class="btn-outline-text" :style="{ color: colors.primary }">重新拍</text>
        </view>
        <text v-else class="recognizing-hint" :style="{ color: colors.textSecondary }">AI 检查中，完成后可重新拍</text>
      </block>
      <text
        v-if="item.status != 'recognizing'"
        hover-class="hover-dim"
        class="escape-link"
        :style="{ color: item.exception_type != '' ? colors.danger : colors.warning }"
        @click="$emit('report-missing')"
      >{{ exceptionLabel }}</text>
    </template>

    <!-- 标签抽查（equipment_date_spot）合成项：必拍 1 张 + 生产日期/维修日期（服务端四规则比对） -->
    <template v-else-if="isSpot">
      <view class="spot-photo" :style="{ borderColor: item.file_ids.length > 0 ? colors.success : colors.primary }" @click="$emit('spot-photo')">
        <image
          v-if="item.photos.length > 0 && !item.img_error"
          :src="item.photos[0]"
          class="spot-img"
          mode="aspectFill"
          @error="$emit('image-error')"
        />
        <text v-else class="spot-photo-text" :style="{ color: colors.primary }">点这里拍标签/设备照片</text>
      </view>
      <view class="spot-row">
        <text class="spot-label" :style="{ color: colors.textRegular }">生产日期</text>
        <picker mode="date" :value="item.spot_mfg" :disabled="item.spot_label_missing" @change="$emit('spot-field', { field: 'spot_mfg', value: $event.detail.value })">
          <view class="spot-picker" :style="{ borderColor: colors.border, color: (item.spot_mfg || '') != '' ? colors.textPrimary : colors.textSecondary }">
            {{ (item.spot_mfg || '') != '' ? item.spot_mfg : '选择日期' }}
          </view>
        </picker>
      </view>
      <view class="spot-row">
        <text class="spot-label" :style="{ color: colors.textRegular }">维修日期</text>
        <picker v-if="!item.spot_no_sticker" mode="date" :value="item.spot_maint" :disabled="item.spot_label_missing" @change="$emit('spot-field', { field: 'spot_maint', value: $event.detail.value })">
          <view class="spot-picker" :style="{ borderColor: colors.border, color: (item.spot_maint || '') != '' ? colors.textPrimary : colors.textSecondary }">
            {{ (item.spot_maint || '') != '' ? item.spot_maint : '选择日期' }}
          </view>
        </picker>
        <text
          class="spot-check"
          :style="{ color: item.spot_no_sticker ? colors.primary : colors.textSecondary }"
          @click="$emit('spot-field', { field: 'spot_no_sticker', value: !item.spot_no_sticker })"
        >{{ item.spot_no_sticker ? '✓ 无贴纸' : '无贴纸' }}</text>
      </view>
      <view class="spot-row">
        <text
          class="spot-check"
          :style="{ color: item.spot_label_missing ? colors.danger : colors.textSecondary }"
          @click="$emit('spot-field', { field: 'spot_label_missing', value: !item.spot_label_missing })"
        >{{ item.spot_label_missing ? '✓ 标签缺失/无法辨认' : '标签缺失/无法辨认' }}</text>
        <text
          v-if="item.file_ids.length > 0 && !item.spot_label_missing"
          class="spot-ai"
          :style="{ color: colors.primary }"
          @click="$emit('spot-ai')"
        >{{ item.spot_ai_loading ? 'AI 识别中…' : 'AI 读标签' }}</text>
      </view>
      <text class="spot-hint" :style="{ color: colors.textSecondary }">抽查只核对不改台账；比对不符将转经理审核</text>
      <view hover-class="hover-dim" class="btn-big spot-done" :style="{ backgroundColor: colors.primary }" @click="$emit('spot-confirm')">
        <text class="btn-big-text" :style="{ color: colors.white }">完成，下一项</text>
      </view>
    </template>

    <!-- 台账有效期（equipment_validity）合成项：服务端按该设备台账自动判定，只读展示，不可人工改判 -->
    <template v-else-if="equipJudge != null">
      <view class="equip-banner" :style="{ backgroundColor: colors.bgPage }">
        <text class="equip-state" :style="{ color: equipColor }">{{ equipText }}</text>
        <text class="equip-sub" :style="{ color: colors.textSecondary }">
          {{ item.name }}
        </text>
        <text v-if="equipJudge.has_pending_register" class="equip-pending" :style="{ color: colors.primary }">已登记维保，待经理确认</text>
      </view>
      <!-- 逾期/缺数据时：已维保的拍新维修标签（随打卡上送，服务端 AI 核对，可信自动回写台账，存疑转经理确认） -->
      <block v-if="canLabelPhoto">
        <view class="spot-photo" :style="{ borderColor: item.file_ids.length > 0 ? colors.success : colors.primary }" @click="$emit('equip-label-photo')">
          <image
            v-if="item.photos.length > 0 && !item.img_error"
            :src="item.photos[0]"
            class="spot-img"
            mode="aspectFill"
            @click.stop="$emit('preview-photo')"
            @error="$emit('image-error')"
          />
          <text v-else class="spot-photo-text" :style="{ color: colors.primary }">已维保？点这里拍新标签</text>
        </view>
        <view v-if="item.photos.length > 1" class="equip-thumbs">
          <image
            v-for="(p, pi) in item.photos"
            :key="pi"
            :src="p"
            class="equip-thumb"
            mode="aspectFill"
            @click="$emit('preview-photo')"
            @error="$emit('image-error')"
          />
        </view>
        <text v-if="item.file_ids.length > 0" class="equip-photo-hint" :style="{ color: colors.textSecondary }">
          已拍 {{ item.file_ids.length }} 张新标签{{ item.file_ids.length < 3 ? '（点上方可补拍，至多 3 张）' : '' }}，提交时系统自动核对
        </text>
      </block>
      <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.primary }" @click="$emit('next')">
        <text class="btn-big-text" :style="{ color: colors.white }">下一项</text>
      </view>
    </template>

    <template v-else>
      <view hover-class="hover-dim" class="btn-big btn-normal" :style="{ backgroundColor: colors.success }" @click="$emit('manual-ok')">
        <text class="btn-big-text" :style="{ color: colors.white }">✓ 正常</text>
      </view>
      <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.danger }" @click="$emit('manual-abnormal')">
        <text class="btn-big-text" :style="{ color: colors.white }">⚠ 有异常</text>
      </view>
      <block v-if="manualAbnormalOpen">
        <textarea
          class="manual-note"
          :value="manualNote"
          :style="{ borderColor: colors.danger, color: colors.textPrimary, backgroundColor: colors.bgPage }"
          placeholder="说说哪里不对劲（可不填）"
          :maxlength="200"
          @input="$emit('update:manual-note', $event.detail.value)"
        />
        <view hover-class="hover-dim" class="btn-big" :style="{ backgroundColor: colors.danger }" @click="$emit('confirm-manual-abnormal')">
          <text class="btn-big-text" :style="{ color: colors.white }">确认异常，下一项</text>
        </view>
      </block>
    </template>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import type { WizardItemSnap } from '@/utils/checkinWizard'
import type { EquipmentAutoJudge } from '@/services/api'

/** 今日 0 点（本地时区），到期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

export default {
  props: {
    item: { type: Object as () => WizardItemSnap, required: true },
    isPhoto: { type: Boolean, default: true },
    /** 台账有效期项的自动判定（judge_type=equipment_validity 时由父组件传入；null=非该类型） */
    equipJudge: { type: Object as () => EquipmentAutoJudge | null, default: null },
    /** 是否标签抽查合成项（equipment_date_spot） */
    isSpot: { type: Boolean, default: false },
    manualAbnormalOpen: { type: Boolean, default: false },
    manualNote: { type: String, default: '' },
    exceptionLabel: { type: String, default: '设备不存在/无法检测，提交异常' },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: [
    'take-photo',
    'preview-photo',
    'image-error',
    'next',
    'report-missing',
    'manual-ok',
    'manual-abnormal',
    'update:manual-note',
    'confirm-manual-abnormal',
    'equip-label-photo',
    'spot-photo',
    'spot-ai',
    'spot-field',
    'spot-confirm'
  ],
  computed: {
    /** 台账有效期项「拍新标签」入口：逾期/缺数据（或后端仍下发展示登记入口）时才出现 */
    canLabelPhoto(): boolean {
      const aj = this.equipJudge
      if (aj == null) return false
      return aj.show_register || aj.status == 'overdue' || aj.status == 'no_data'
    },
    /** 台账有效期状态文案（逐台：equipJudge 即该设备自己的判定） */
    equipText(): string {
      const aj = this.equipJudge
      if (aj == null) return ''
      if (aj.status == 'no_data') return '台账数据缺失，请补录'
      const due = aj.next_due_date
      if (due == '') return ''
      if (aj.status == 'overdue') return '已逾期 ' + aj.overdue_days + ' 天（到期日 ' + due + '）'
      if (aj.status == 'warning') {
        const days = Math.round((new Date(due.replace(/-/g, '/')).getTime() - todayZero()) / 86400000)
        return '将于 ' + days + ' 天内到期（' + due + '）'
      }
      return '台账有效（至 ' + due + '）'
    },
    equipColor(): string {
      const aj = this.equipJudge
      if (aj == null) return this.colors.success
      if (aj.status == 'overdue') return this.colors.danger
      if (aj.status == 'warning') return this.colors.warning
      if (aj.status == 'no_data') return this.colors.info
      return this.colors.success
    }
  }
}
</script>

<style scoped>
.item-card {
  border-radius: 24rpx;
  padding: 48rpx 32rpx;
  align-items: center;
  margin-bottom: 24rpx;
}

.item-name {
  font-size: 56rpx;
  font-weight: 700;
}

.item-hint {
  font-size: 34rpx;
  text-align: center;
  margin-top: 16rpx;
  line-height: 48rpx;
}

.shot-empty {
  width: 100%;
  height: 360rpx;
  border-width: 3rpx;
  border-style: dashed;
  border-radius: 24rpx;
  margin-top: 40rpx;
  align-items: center;
  justify-content: center;
}

.cam-icon {
  width: 120rpx;
  height: 96rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.cam-lens {
  width: 40rpx;
  height: 40rpx;
  border-width: 6rpx;
  border-style: solid;
  border-radius: 20rpx;
}

.shot-empty-text {
  font-size: 40rpx;
  font-weight: 700;
  margin-top: 24rpx;
}

.shot-preview {
  width: 100%;
  height: 480rpx;
  border-radius: 20rpx;
  margin-top: 40rpx;
  overflow: hidden;
  align-items: center;
  justify-content: center;
}

.shot-img {
  width: 100%;
  height: 480rpx;
}

.shot-img-fallback {
  display: flex;
  align-items: center;
  justify-content: center;
  background-color: #f2f3f5;
}

.shot-img-fallback-text {
  font-size: 26rpx;
  color: #9ca3af;
}

.btn-big {
  width: 100%;
  height: 140rpx;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
  margin-bottom: 24rpx;
}

.btn-big-text {
  font-size: 44rpx;
  font-weight: 700;
}

.btn-outline {
  height: 112rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx;
  align-items: center;
  justify-content: center;
}

.btn-outline-text {
  font-size: 40rpx;
  font-weight: 600;
}

.btn-normal {
  margin-top: 32rpx;
}

.shot-next {
  margin-top: 32rpx;
}

.reshot {
  width: 100%;
  margin-top: 8rpx;
}

.recognizing-hint {
  font-size: 28rpx;
  margin-top: 8rpx;
  margin-bottom: 8rpx;
}

.manual-note {
  width: 100%;
  height: 192rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 16rpx;
  padding: 24rpx;
  font-size: 34rpx;
  margin-top: 8rpx;
  margin-bottom: 24rpx;
}

.escape-link {
  display: block;
  font-size: 30rpx;
  font-weight: 700;
  margin: 30rpx 12rpx 8rpx;
  padding: 22rpx 18rpx;
  border: 2rpx solid currentColor;
  border-radius: 14rpx;
  text-align: center;
  background-color: rgba(255, 150, 0, 0.1);
}

/* 标签抽查项 */
.spot-photo {
  width: 100%;
  height: 360rpx;
  border-width: 3rpx;
  border-style: dashed;
  border-radius: 24rpx;
  margin-top: 40rpx;
  align-items: center;
  justify-content: center;
  overflow: hidden;
}

.spot-img {
  width: 100%;
  height: 360rpx;
}

.spot-photo-text {
  font-size: 36rpx;
  font-weight: 600;
}

.spot-row {
  flex-direction: row;
  align-items: center;
  margin-top: 24rpx;
  width: 100%;
}

.spot-label {
  font-size: 30rpx;
  width: 160rpx;
}

.spot-picker {
  height: 88rpx;
  min-width: 280rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  justify-content: center;
  font-size: 30rpx;
}

.spot-check {
  font-size: 28rpx;
  margin-left: 24rpx;
  padding: 12rpx 20rpx;
}

.spot-ai {
  font-size: 28rpx;
  padding: 12rpx 20rpx;
}

.spot-hint {
  font-size: 24rpx;
  margin-top: 16rpx;
}

.spot-done {
  margin-top: 24rpx;
}

/* 台账有效期项：自动判定结果横幅 + 登记入口 */
.equip-banner {
  width: 100%;
  border-radius: 20rpx;
  margin-top: 40rpx;
  padding: 32rpx;
  align-items: center;
}

.equip-state {
  font-size: 40rpx;
  font-weight: 700;
}

.equip-sub {
  font-size: 28rpx;
  margin-top: 12rpx;
}

.equip-pending {
  font-size: 26rpx;
  margin-top: 8rpx;
}

/* 台账有效期项「拍新标签」缩略图行与提示 */
.equip-thumbs {
  flex-direction: row;
  flex-wrap: wrap;
  margin-top: 16rpx;
  width: 100%;
}

.equip-thumb {
  width: 120rpx;
  height: 120rpx;
  border-radius: 12rpx;
  margin-right: 16rpx;
}

.equip-photo-hint {
  font-size: 24rpx;
  margin-top: 12rpx;
}
</style>
