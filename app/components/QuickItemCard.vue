<template>
  <!-- 逐项卡片（方案 §三统一布局）：引导语大字号 + 项名小字灰 + PhotoSlot + ResultBar + 观察点行 + 类型差异区。
       自身无推进/作答按钮——主操作全部在底部操作栏（WizardBottomBar）。 -->
  <view class="item-card" :style="{ backgroundColor: colors.bgCard, boxShadow: shadow }">
    <!-- 合成项（台账有效期/标签抽查）保持原标题+要求文案；拍照/感官项以引导语为主标题 -->
    <template v-if="equipJudge != null || isSpot">
      <text class="item-name" :style="{ color: colors.textPrimary }">{{ item.name }}</text>
      <text class="item-hint" :style="{ color: colors.textSecondary }">{{ item.requirement != '' ? item.requirement : (isPhoto || manualMode ? '拍一张该项的照片' : '这项正常吗？') }}</text>
    </template>
    <template v-else>
      <text class="item-name" :style="{ color: colors.textPrimary }">{{ cardTitle }}</text>
      <text class="item-sub" :style="{ color: colors.textSecondary }">{{ item.name }}</text>
      <text v-if="item.requirement != ''" class="item-req" :style="{ color: colors.textSecondary }">{{ item.requirement }}</text>
    </template>

    <!-- 台账有效期（equipment_validity）：一行小字状态 + （逾期/缺数据时）拍新标签多图槽（至多 3 张） -->
    <template v-if="equipJudge != null">
      <text class="equip-status" :style="{ color: equipColor }">{{ equipStatusText }}</text>
      <block v-if="canLabelPhoto">
        <WizardPhotoSlot
          variant="multi"
          :max="3"
          empty-add-text="拍新标签（至多 3 张）"
          add-text="拍新标签"
          :photos="item.photos"
          :img-error="item.img_error == true"
          status="done"
          :required="false"
          :colors="colors"
          @take-photo="$emit('equip-label-photo')"
          @preview="$emit('preview-photo')"
          @image-error="$emit('image-error')"
        />
        <text v-if="item.file_ids.length > 0" class="equip-photo-hint" :style="{ color: colors.textSecondary }">
          已拍 {{ item.file_ids.length }} 张新标签，提交时系统自动核对
        </text>
      </block>
    </template>

    <template v-else>
      <!-- 照片槽（六态） -->
      <WizardPhotoSlot
        v-if="showSlot"
        :variant="slotVariant"
        :max="3"
        :photos="item.photos"
        :img-error="item.img_error == true"
        :status="item.status"
        :pending-local="item.pending_local ?? ''"
        :escaped="escaped"
        :escape-text="exceptionLabel"
        :required="slotRequired"
        :colors="colors"
        @take-photo="$emit('take-photo')"
        @preview="$emit('preview-photo')"
        @retry-upload="$emit('retry-upload')"
        @image-error="$emit('image-error')"
        @escape-undo="$emit('escape-undo')"
      />

      <!-- 状态条（AI 档拍照项回退查看时的只读展示；无状态整行不占位） -->
      <WizardResultBar
        v-if="showResult"
        :status="item.status"
        :verdict="item.verdict"
        :reason="item.reason"
        :quality-pass="item.quality_pass"
        :quality-issue="item.quality_issue"
        :has-job="item.job_id != ''"
        :colors="colors"
      />

      <!-- 观察点下拉多选入口行（无 tag 不渲染） -->
      <view v-if="item.tags.length > 0" hover-class="hover-dim" class="tag-entry" :style="{ borderColor: colors.border }" @click="$emit('open-tags')">
        <text class="tag-entry-name" :style="{ color: colors.textPrimary }">观察点（{{ item.tags.length }}）</text>
        <text class="tag-entry-val" :style="{ color: item.abnormal_tags.length == 0 ? colors.success : colors.danger }">
          {{ item.abnormal_tags.length == 0 ? '全部正常 ✓' : item.abnormal_tags.length + ' 项异常' }} ›
        </text>
      </view>
      <!-- 已勾选异常观察点红芯片回显（点入口行可改） -->
      <view v-if="item.abnormal_tags.length > 0" class="tag-row">
        <text
          v-for="(t, ti) in item.abnormal_tags"
          :key="ti"
          class="tag-chip"
          :style="{ color: colors.white, backgroundColor: colors.danger, borderColor: colors.danger }"
        >✕ {{ t }}</text>
      </view>

      <!-- 差异区：抽查合成项 AI 读数一行小字 -->
      <text v-if="isSpot && spotReadingText != ''" class="reading-line" :style="{ color: colors.textSecondary }">{{ spotReadingText }}</text>
    </template>
  </view>
</template>

<script lang="ts">
import type { ColorTokens } from '@/utils/theme'
import type { WizardItemSnap } from '@/utils/checkinWizard'
import type { EquipmentAutoJudge } from '@/services/api'
import WizardPhotoSlot from '@/components/WizardPhotoSlot.vue'
import WizardResultBar from '@/components/WizardResultBar.vue'

export default {
  components: { WizardPhotoSlot, WizardResultBar },
  props: {
    item: { type: Object as () => WizardItemSnap, required: true },
    isPhoto: { type: Boolean, default: true },
    /** 台账有效期项的自动判定（judge_type=equipment_validity 时由父组件传入；null=非该类型） */
    equipJudge: { type: Object as () => EquipmentAutoJudge | null, default: null },
    /** 是否标签抽查合成项（equipment_date_spot；外观同普通拍照项，仅多读数一行小字） */
    isSpot: { type: Boolean, default: false },
    /** 手动档（mode=manual）：不建 AI job，拍照后停留作答「✓ 正常 / ⚠ 有异常」 */
    manualMode: { type: Boolean, default: false },
    exceptionLabel: { type: String, default: '设备不存在/无法检测，提交异常' },
    colors: { type: Object as () => ColorTokens, required: true },
    shadow: { type: String, default: '' }
  },
  emits: [
    'take-photo',
    'preview-photo',
    'image-error',
    'retry-upload',
    'open-tags',
    'equip-label-photo',
    'escape-undo'
  ],
  computed: {
    /** 卡片主标题：guide 非空用引导语；空兜底「拍「项名」照片」（无照片要求的项兜底「这项正常吗？」） */
    cardTitle(): string {
      if ((this.item.guide || '') != '') return this.item.guide
      const photoItem = this.isPhoto || (this.manualMode && (this.item.photo_required ?? '') != 'none')
      return photoItem ? '拍「' + this.item.name + '」照片' : '这项正常吗？'
    },
    /** 抽查合成项读数行：AI 读标签结果 M{生产年月}|W{维修年月} 解析为一行小字；读不出转人工核对 */
    spotReadingText(): string {
      const it = this.item
      if (it.status == 'recognizing') return 'AI 读标签中…'
      if (it.status != 'done' || it.exception_type != '') return ''
      const m = (it.reading || '').match(/M(\d{4}-\d{2}|无)\|W(\d{4}-\d{2}|无)/)
      if (m == null) return '未读出，将转人工核对'
      const mfg = m[1] == '无' ? '未读出' : m[1]
      const maint = m[2] == '无' ? '未读出' : m[2]
      return '识别到：生产 ' + mfg + ' / 维修 ' + maint
    },
    /** 台账有效期项「拍新标签」入口：逾期/缺数据（或后端仍下发展示登记入口）时才出现 */
    canLabelPhoto(): boolean {
      const aj = this.equipJudge
      if (aj == null) return false
      return aj.show_register || aj.status == 'overdue' || aj.status == 'no_data'
    },
    /** 台账有效期一行小字状态（逐台：equipJudge 即该设备自己的判定） */
    equipStatusText(): string {
      const aj = this.equipJudge
      if (aj == null) return ''
      let t = '台账正常'
      if (aj.status == 'overdue') t = '台账已逾期，拍照登记维保'
      else if (aj.status == 'warning') t = '台账即将到期'
      else if (aj.status == 'no_data') t = '台账数据缺失待补录'
      else if (aj.status == 'label_missing') t = '标签缺失，拍照登记维保'
      if (aj.has_pending_register) t += '（已登记维保待确认）'
      return t
    },
    equipColor(): string {
      const aj = this.equipJudge
      if (aj == null) return this.colors.success
      if (aj.status == 'overdue') return this.colors.danger
      if (aj.status == 'warning') return this.colors.warning
      if (aj.status == 'no_data' || aj.status == 'label_missing') return this.colors.info
      return this.colors.success
    },
    escaped(): boolean {
      return (this.item.exception_type ?? '') != ''
    },
    /** 照片槽是否渲染：手动档 photo_required=none 的项无照片要求，直接作答 */
    showSlot(): boolean {
      if (this.manualMode && (this.item.photo_required ?? '') == 'none') return false
      return true
    },
    /** 必拍=灰占位不可点（拍照只走底栏大按钮）；选拍=列表行 */
    slotRequired(): boolean {
      if (this.item.judge_type == 'manual') return false // 感官项恒选拍
      if (this.manualMode) return (this.item.photo_required ?? '') == 'required'
      return true
    },
    /** 多图变体：手动档拍照项与感官项（至多 3 张）；AI 档拍照/抽查项一图一位 */
    slotVariant(): 'single' | 'multi' {
      if (!this.manualMode && this.isPhoto) return 'single'
      return 'multi'
    },
    /** ResultBar：AI 档拍照项已拍/处理中且非逃生/待补传时展示 */
    showResult(): boolean {
      if (this.manualMode || !this.isPhoto || this.escaped) return false
      if ((this.item.pending_local ?? '') != '') return false
      return this.item.photos.length > 0 || this.item.status == 'recognizing'
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

/* 项名降级为次要信息（与后台术语对齐用） */
.item-sub {
  font-size: 26rpx;
  margin-top: 12rpx;
  text-align: center;
}

/* 标准要求折叠为小字灰字单行省略 */
.item-req {
  width: 100%;
  font-size: 24rpx;
  margin-top: 8rpx;
  text-align: center;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

/* 台账有效期项：一行小字状态 */
.equip-status {
  font-size: 30rpx;
  font-weight: 600;
  margin-top: 24rpx;
}

.equip-photo-hint {
  font-size: 24rpx;
  margin-top: 12rpx;
}

/* 观察点入口行（微信列表行：左文右 ›） */
.tag-entry {
  width: 100%;
  min-height: 104rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-width: 1rpx;
  border-style: solid;
  border-radius: 20rpx;
  padding: 0 28rpx;
  margin-top: 24rpx;
}

.tag-entry-name {
  font-size: 34rpx;
  font-weight: 600;
}

.tag-entry-val {
  font-size: 30rpx;
  font-weight: 600;
}

/* 异常观察点红芯片回显 */
.tag-row {
  width: 100%;
  flex-direction: row;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 16rpx;
}

.tag-chip {
  font-size: 30rpx;
  font-weight: 600;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 999rpx;
  padding: 12rpx 28rpx;
  margin: 8rpx;
}

/* 抽查合成项 AI 读数行：小字灰字，不阻塞 */
.reading-line {
  font-size: 26rpx;
  margin-top: 16rpx;
}
</style>
