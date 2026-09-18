<template>
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

    <!-- 台账有效期（equipment_validity）合成项：外观与普通项一致——一行小字状态 + （逾期/缺数据时）拍新标签登记维保 -->
    <template v-if="equipJudge != null">
      <text class="equip-status" :style="{ color: equipColor }">{{ equipStatusText }}</text>
      <!-- 逾期/缺数据时：已维保的拍新维修标签（随打卡上送，服务端 AI 核对，可信自动回写台账，存疑转经理确认） -->
      <block v-if="canLabelPhoto">
        <view class="label-photo" :style="{ borderColor: item.file_ids.length > 0 ? colors.success : colors.primary }" @click="$emit('equip-label-photo')">
          <image
            v-if="item.photos.length > 0 && !item.img_error"
            :src="item.photos[0]"
            class="label-photo-img"
            mode="aspectFill"
            @click.stop="$emit('preview-photo')"
            @error="$emit('image-error')"
          />
          <text v-else class="label-photo-text" :style="{ color: colors.primary }">已维保？点这里拍新标签</text>
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

    <!-- 手动档（mode=manual）：拍照项拍完照（photo_required=none 可直接作答）→「这项正常吗？」+ 观察点 tag，与感官项同一套交互 -->
    <template v-else-if="manualMode">
      <!-- 待补传态：上传失败压缩图保留在项上，点橙色按钮重试补传；本地图失效可重拍 -->
      <block v-if="item.pending_local">
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
            <text class="shot-img-fallback-text">照片已保留，待补传</text>
          </view>
        </view>
        <view hover-class="hover-dim" class="btn-big shot-next" :style="{ backgroundColor: colors.warning }" @click="$emit('retry-upload')">
          <text class="btn-big-text" :style="{ color: colors.white }">照片待补传，点击重试</text>
        </view>
        <view hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
          <text class="btn-outline-text" :style="{ color: colors.primary }">重新拍</text>
        </view>
      </block>
      <block v-else>
        <!-- 照片区（photo_required=none 的项无照片要求，直接作答） -->
        <block v-if="item.photo_required != 'none'">
          <view
            v-if="item.file_ids.length == 0"
            hover-class="hover-dim"
            class="shot-empty"
            :style="{ borderColor: colors.primary }"
            @click="$emit('take-photo')"
          >
            <view class="cam-icon" :style="{ borderColor: colors.primary }">
              <view class="cam-lens" :style="{ borderColor: colors.primary }"></view>
            </view>
            <text class="shot-empty-text" :style="{ color: colors.primary }">点这里拍照</text>
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
            <view v-if="item.file_ids.length < 3" hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
              <text class="btn-outline-text" :style="{ color: colors.primary }">补拍（已拍 {{ item.file_ids.length }} 张）</text>
            </view>
          </block>
        </block>
        <!-- 观察点 tag：默认全部正常（绿描边），点选标记异常（红实心），再点恢复 -->
        <view v-if="item.tags.length > 0" class="tag-row">
          <text
            v-for="(t, ti) in item.tags"
            :key="ti"
            class="tag-chip"
            :style="isAbnTag(t) ? { color: colors.white, backgroundColor: colors.danger, borderColor: colors.danger } : { color: colors.success, borderColor: colors.success }"
            @click="$emit('toggle-tag', t)"
          >{{ isAbnTag(t) ? '✕ ' + t : t }}</text>
        </view>
        <text v-if="item.tags.length > 0" class="tag-hint" :style="{ color: colors.textSecondary }">观察点默认正常，异常的点一下标红</text>
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
        <text
          v-if="item.photo_required != 'none'"
          hover-class="hover-dim"
          class="escape-link"
          :style="{ color: item.exception_type != '' ? colors.danger : colors.warning }"
          @click="$emit('report-missing')"
        >{{ exceptionLabel }}</text>
      </block>
    </template>

    <template v-else-if="isPhoto">
      <!-- 待补传态：上传失败压缩图保留在项上，点橙色按钮重试补传（成功继续原 AI 链路）；本地图失效可重拍 -->
      <block v-if="item.pending_local">
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
            <text class="shot-img-fallback-text">照片已保留，待补传</text>
          </view>
        </view>
        <view hover-class="hover-dim" class="btn-big shot-next" :style="{ backgroundColor: colors.warning }" @click="$emit('retry-upload')">
          <text class="btn-big-text" :style="{ color: colors.white }">照片待补传，点击重试</text>
        </view>
        <view hover-class="hover-dim" class="btn-outline reshot" :style="{ borderColor: colors.primary }" @click="$emit('take-photo')">
          <text class="btn-outline-text" :style="{ color: colors.primary }">重新拍</text>
        </view>
      </block>
      <view
        v-else-if="item.status == 'todo' || item.status == 'failed'"
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
        <!-- 抽查合成项：AI 读标签读数一行小字展示（不阻塞下一项；未读出转人工核对） -->
        <text v-if="isSpot && spotReadingText != ''" class="reading-line" :style="{ color: colors.textSecondary }">{{ spotReadingText }}</text>
        <!-- 观察点 tag：默认全部正常（绿描边），点选标记异常（红实心），再点恢复；AI 判出的异常 tag 已预标记 -->
        <view v-if="item.tags.length > 0" class="tag-row">
          <text
            v-for="(t, ti) in item.tags"
            :key="ti"
            class="tag-chip"
            :style="isAbnTag(t) ? { color: colors.white, backgroundColor: colors.danger, borderColor: colors.danger } : { color: colors.success, borderColor: colors.success }"
            @click="$emit('toggle-tag', t)"
          >{{ isAbnTag(t) ? '✕ ' + t : t }}</text>
        </view>
        <text v-if="item.tags.length > 0" class="tag-hint" :style="{ color: colors.textSecondary }">观察点默认正常，异常的点一下标红</text>
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

    <template v-else>
      <!-- 观察点 tag（感官项）：默认全部正常，点选异常标红；有异常 tag 时该项按异常计 -->
      <view v-if="item.tags.length > 0" class="tag-row">
        <text
          v-for="(t, ti) in item.tags"
          :key="ti"
          class="tag-chip"
          :style="isAbnTag(t) ? { color: colors.white, backgroundColor: colors.danger, borderColor: colors.danger } : { color: colors.success, borderColor: colors.success }"
          @click="$emit('toggle-tag', t)"
        >{{ isAbnTag(t) ? '✕ ' + t : t }}</text>
      </view>
      <text v-if="item.tags.length > 0" class="tag-hint" :style="{ color: colors.textSecondary }">观察点默认正常，异常的点一下标红</text>
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

export default {
  props: {
    item: { type: Object as () => WizardItemSnap, required: true },
    isPhoto: { type: Boolean, default: true },
    /** 台账有效期项的自动判定（judge_type=equipment_validity 时由父组件传入；null=非该类型） */
    equipJudge: { type: Object as () => EquipmentAutoJudge | null, default: null },
    /** 是否标签抽查合成项（equipment_date_spot；外观同普通拍照项，仅多读数一行小字与「标签磨损」逃生项） */
    isSpot: { type: Boolean, default: false },
    /** 手动档（mode=manual）：不建 AI job，拍照后直接「这项正常吗？」 */
    manualMode: { type: Boolean, default: false },
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
    'retry-upload',
    'toggle-tag'
  ],
  methods: {
    isAbnTag(t: string): boolean {
      return this.item.abnormal_tags != null && this.item.abnormal_tags.indexOf(t) >= 0
    }
  },
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

/* 抽查合成项 AI 读数行：小字灰字，不阻塞 */
.reading-line {
  font-size: 26rpx;
  margin-top: 16rpx;
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

/* 台账有效期项「拍新标签」照片框（手动档/有效期项共用） */
.label-photo {
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

.label-photo-img {
  width: 100%;
  height: 360rpx;
}

.label-photo-text {
  font-size: 36rpx;
  font-weight: 600;
}

/* 台账有效期项：一行小字状态 */
.equip-status {
  font-size: 30rpx;
  font-weight: 600;
  margin-top: 24rpx;
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

/* 观察点 tag chips（拍照项/感官项/手动档共用）：默认正常绿描边，点选异常红实心 */
.tag-row {
  width: 100%;
  flex-direction: row;
  flex-wrap: wrap;
  justify-content: center;
  margin-top: 24rpx;
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

.tag-hint {
  font-size: 24rpx;
  margin-top: 8rpx;
}
</style>
