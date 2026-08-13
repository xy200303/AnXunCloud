<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 骨架屏（编辑模式加载详情） -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
      <view class="sk-block" :style="{ backgroundColor: colors.border }"></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="loadError != ''" class="empty">
      <text class="empty-title" :style="{ color: colors.textRegular }">{{ loadError }}</text>
      <text class="empty-retry" :style="{ color: colors.primary }" @click="loadDetail">重试</text>
    </view>

    <view v-else class="content">
      <!-- 只读提示 -->
      <view v-if="readonly" class="readonly-tip" :style="{ backgroundColor: colors.primaryLight }">
        <text class="readonly-tip-text" :style="{ color: colors.primary }">无点位编辑权限，仅可查看</text>
      </view>

      <!-- 基础信息 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">基础信息</text>

        <text class="label" :style="{ color: colors.textRegular }">所属小区 *</text>
        <picker :range="communityNames" :value="communityIndex" :disabled="readonly" @change="onCommunityChange">
          <view class="field" :style="{ borderColor: colors.border }">
            <text class="field-text" :style="{ color: communityId == '' ? colors.textSecondary : colors.textPrimary }">
              {{ communityId == '' ? '请选择小区' : communityName }}
            </text>
            <text class="field-arrow" :style="{ color: colors.textSecondary }">▾</text>
          </view>
        </picker>

        <text class="label" :style="{ color: colors.textRegular }">楼栋（选填）</text>
        <picker :range="buildingNames" :value="buildingIndex" :disabled="readonly || communityId == ''" @change="onBuildingChange">
          <view class="field" :style="{ borderColor: colors.border }">
            <text class="field-text" :style="{ color: buildingId == '' ? colors.textSecondary : colors.textPrimary }">
              {{ communityId == '' ? '请先选择小区' : (buildingId == '' ? '不分区/整小区' : buildingName) }}
            </text>
            <text class="field-arrow" :style="{ color: colors.textSecondary }">▾</text>
          </view>
        </picker>

        <text class="label" :style="{ color: colors.textRegular }">点位名称 *</text>
        <input
          v-model="name"
          class="field-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="如：1号楼配电房"
          :maxlength="50"
          :disabled="readonly"
        />

        <text class="label" :style="{ color: colors.textRegular }">点位类型 *</text>
        <picker :range="typeNames" :value="typeIndex" :disabled="readonly" @change="onTypeChange">
          <view class="field" :style="{ borderColor: colors.border }">
            <text class="field-text" :style="{ color: type == '' ? colors.textSecondary : colors.textPrimary }">
              {{ type == '' ? '请选择类型' : typeName }}
            </text>
            <text class="field-arrow" :style="{ color: colors.textSecondary }">▾</text>
          </view>
        </picker>

        <text class="label" :style="{ color: colors.textRegular }">检查项模板（选填）</text>
        <picker :range="templateNames" :value="templateIndex" :disabled="readonly" @change="onTemplateChange">
          <view class="field" :style="{ borderColor: colors.border }">
            <text class="field-text" :style="{ color: templateId == '' ? colors.textSecondary : colors.textPrimary }">
              {{ templateId == '' ? '不关联模板' : templateName }}
            </text>
            <text class="field-arrow" :style="{ color: colors.textSecondary }">▾</text>
          </view>
        </picker>

        <text class="label" :style="{ color: colors.textRegular }">备注（选填）</text>
        <textarea
          v-model="remark"
          class="field-textarea"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="补充说明"
          :maxlength="200"
          :disabled="readonly"
        />
      </view>

      <!-- 坐标与围栏 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">坐标与围栏</text>

        <view v-if="!readonly" class="btn-outline" :style="{ borderColor: colors.primary }" @click="locate">
          <text class="btn-outline-text" :style="{ color: colors.primary }">{{ locating ? '定位中…' : '获取当前位置' }}</text>
        </view>

        <view v-if="hasLocation" class="loc-info">
          <text class="loc-text" :style="{ color: colors.textRegular }">经度 {{ lngText }}，纬度 {{ latText }}</text>
          <text v-if="accuracy > 0" class="loc-acc" :style="{ color: accuracy > 50 ? colors.danger : colors.textSecondary }">
            定位精度约 {{ accuracy }} m{{ accuracy > 50 ? '，定位精度较差，请靠近点位重试' : '' }}
          </text>
        </view>
        <text v-else class="loc-empty" :style="{ color: colors.textSecondary }">尚未录入坐标（必填），可定位或手动输入</text>

        <view v-if="!readonly" class="loc-manual">
          <input
            v-model="lngText"
            class="field-input loc-input"
            :style="{ borderColor: colors.border, color: colors.textPrimary }"
            placeholder="经度"
            type="digit"
          />
          <input
            v-model="latText"
            class="field-input loc-input loc-input-r"
            :style="{ borderColor: colors.border, color: colors.textPrimary }"
            placeholder="纬度"
            type="digit"
          />
        </view>

        <view class="fence-row">
          <text class="label fence-label" :style="{ color: colors.textRegular }">围栏半径：{{ fenceRadius }} m</text>
          <slider
            :value="fenceRadius"
            :min="50"
            :max="500"
            :step="10"
            :disabled="readonly"
            class="fence-slider"
            :activeColor="colors.primary"
            @change="onFenceChange"
          />
        </view>
      </view>

      <!-- 凭证方式 -->
      <view class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="sec-title" :style="{ color: colors.textPrimary }">打卡凭证</text>
        <view class="cred-row">
          <view
            v-for="c in credentialOptions"
            :key="c.value"
            class="cred-item"
            :style="credential == c.value
              ? { backgroundColor: colors.primaryLight, borderColor: colors.primary }
              : { backgroundColor: colors.bgCard, borderColor: colors.border }"
            @click="onCredentialChange(c.value)"
          >
            <text class="cred-text" :style="{ color: credential == c.value ? colors.primary : colors.textRegular }">{{ c.label }}</text>
          </view>
        </view>
        <text v-if="credential == 'none'" class="cred-tip" :style="{ color: colors.textSecondary }">免凭证时将启用电子围栏校验</text>

        <!-- NFC 区（凭证含 NFC 时显示） -->
        <template v-if="credential == 'nfc' || credential == 'any'">
          <view class="nfc-box" :style="{ borderTopColor: colors.border }">
            <text class="label" :style="{ color: colors.textRegular }">NFC 卡号{{ credential == 'nfc' ? ' *' : '（选填）' }}</text>
            <view class="nfc-row">
              <input
                v-model="nfcId"
                class="field-input nfc-input"
                :style="{ borderColor: colors.border, color: colors.textPrimary }"
                placeholder="读取或手动输入卡号"
                :disabled="readonly"
              />
              <view v-if="!readonly && nfcSupported" class="btn-mini" :style="{ borderColor: colors.primary }" @click="readCard">
                <text class="btn-mini-text" :style="{ color: colors.primary }">读卡号</text>
              </view>
            </view>
            <!-- 卡内已写入的点位编号（读卡后显示；与本点位编号不一致时警示） -->
            <text
              v-if="cardCodeInfo != ''"
              class="cred-tip"
              :style="{ color: cardCodeWarn ? colors.danger : colors.textSecondary }"
            >{{ cardCodeInfo }}</text>

            <!-- 写卡：编辑模式直接可写；新增模式提交后方可写 -->
            <template v-if="!readonly">
              <view v-if="isEdit && nfcSupported" class="btn-outline nfc-write" :style="{ borderColor: colors.primary }" @click="writeCard">
                <text class="btn-outline-text" :style="{ color: colors.primary }">写入点位编号到卡（{{ qrcodeNo }}）</text>
              </view>
              <text v-else-if="!isEdit" class="cred-tip" :style="{ color: colors.textSecondary }">提交创建生成编号后，可写入编号到 NFC 卡</text>
              <text v-if="!nfcSupported" class="cred-tip" :style="{ color: colors.textSecondary }">当前端不支持 NFC，可手动输入卡号</text>
            </template>
          </view>
        </template>
      </view>

      <!-- 编号展示（编辑模式） -->
      <view v-if="isEdit" class="card" :style="{ backgroundColor: colors.bgCard }">
        <text class="info-line" :style="{ color: colors.textRegular }">点位编号：{{ qrcodeNo }}</text>
        <text class="info-line" :style="{ color: colors.textSecondary }">编号由系统生成，不可修改</text>
      </view>

      <!-- 提交 -->
      <view
        v-if="!readonly"
        class="btn-primary"
        :style="{ backgroundColor: submitting ? colors.info : colors.primary }"
        @click="submit"
      >
        <text class="btn-primary-text" :style="{ color: colors.white }">{{ submitting ? '提交中…' : (isEdit ? '保存' : '创建点位') }}</text>
      </view>
      <view class="bottom-space"></view>
    </view>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import {
  apiCommunityTree,
  apiTemplateList,
  apiPointDetail,
  apiPointCreate,
  apiPointUpdate,
  CommunityTreeNode,
  TemplateListItem,
  PointSavePayload
} from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { isNfcSupported, readCardInfoOnce, writePointCode, toastNfcUnavailable } from '@/utils/nfc'
import { getLocationGcj02 } from '@/utils/geo'

/** 点位类型字典（对齐后端 seed：sys_dict point_type） */
const POINT_TYPES = [
  { value: 'common', label: '普通点位' },
  { value: 'power_room', label: '配电房' },
  { value: 'fire_control', label: '消防控制室' },
  { value: 'pump_room', label: '水泵房' },
  { value: 'elevator', label: '电梯机房' },
  { value: 'garage', label: '地下车库' }
]

type FormData = {
  colors: ColorTokens
  isEdit: boolean
  pointId: string
  qrcodeNo: string
  loading: boolean
  loadError: string
  communities: CommunityTreeNode[]
  templates: TemplateListItem[]
  communityId: string
  buildingId: string
  name: string
  type: string
  templateId: string
  remark: string
  lngText: string
  latText: string
  accuracy: number
  locating: boolean
  fenceRadius: number
  credential: string
  credentialOptions: Array<{ label: string; value: string }>
  nfcId: string
  nfcSupported: boolean
  /** 读卡后展示卡内已写入的点位编号（'' 不显示） */
  cardCodeInfo: string
  /** 卡内编号与本点位编号不一致时警示（红色） */
  cardCodeWarn: boolean
  submitting: boolean
  /** 新增提交成功后转编辑态（立即写卡流程），本次会话不按 update 权限切只读 */
  justCreated: boolean
}

export default {
  data(): FormData {
    return {
      colors: Colors,
      isEdit: false,
      pointId: '',
      qrcodeNo: '',
      loading: false,
      loadError: '',
      communities: [] as CommunityTreeNode[],
      templates: [] as TemplateListItem[],
      communityId: '',
      buildingId: '',
      name: '',
      type: '',
      templateId: '',
      remark: '',
      lngText: '',
      latText: '',
      accuracy: 0,
      locating: false,
      fenceRadius: 200,
      credential: 'qrcode',
      credentialOptions: [
        { label: '二维码', value: 'qrcode' },
        { label: 'NFC', value: 'nfc' },
        { label: '任一', value: 'any' },
        { label: '免凭证', value: 'none' }
      ],
      nfcId: '',
      nfcSupported: false,
      cardCodeInfo: '',
      cardCodeWarn: false,
      submitting: false,
      justCreated: false
    }
  },
  computed: {
    /** 编辑且无 point:update 权限 → 只读查看（新增后「立即写卡」转编辑态的本次会话除外） */
    readonly(): boolean {
      if (this.justCreated) return false
      return this.isEdit && !useAuthStore().hasPerm('inspection:point:update')
    },
    communityNames(): string[] {
      return this.communities.map((c) => c.name)
    },
    communityIndex(): number {
      const i = this.communities.findIndex((c) => c.id == this.communityId)
      return i < 0 ? 0 : i
    },
    communityName(): string {
      const c = this.communities.find((x) => x.id == this.communityId)
      return c != null ? c.name : ''
    },
    buildings(): Array<{ id: string; name: string; type: string }> {
      const c = this.communities.find((x) => x.id == this.communityId)
      return c != null ? c.buildings : []
    },
    buildingNames(): string[] {
      return ['不分区/整小区'].concat(this.buildings.map((b) => b.name))
    },
    buildingIndex(): number {
      if (this.buildingId == '') return 0
      const i = this.buildings.findIndex((b) => b.id == this.buildingId)
      return i < 0 ? 0 : i + 1
    },
    buildingName(): string {
      const b = this.buildings.find((x) => x.id == this.buildingId)
      return b != null ? b.name : ''
    },
    typeNames(): string[] {
      return POINT_TYPES.map((t) => t.label)
    },
    typeIndex(): number {
      const i = POINT_TYPES.findIndex((t) => t.value == this.type)
      return i < 0 ? 0 : i
    },
    typeName(): string {
      const t = POINT_TYPES.find((x) => x.value == this.type)
      return t != null ? t.label : ''
    },
    templateNames(): string[] {
      return ['不关联模板'].concat(this.templates.map((t) => t.name))
    },
    templateIndex(): number {
      if (this.templateId == '') return 0
      const i = this.templates.findIndex((t) => t.id == this.templateId)
      return i < 0 ? 0 : i + 1
    },
    templateName(): string {
      const t = this.templates.find((x) => x.id == this.templateId)
      return t != null ? t.name : ''
    },
    hasLocation(): boolean {
      return this.lngText != '' && this.latText != '' && parseFloat(this.lngText) != 0 && parseFloat(this.latText) != 0
    }
  },
  onLoad(options: any) {
    this.nfcSupported = isNfcSupported()
    if (options && options.id) {
      this.isEdit = true
      this.pointId = String(options.id)
      this.loadDetail()
    } else {
      uni.setNavigationBarTitle({ title: '新增点位' })
    }
    this.fetchOptions()
  },
  methods: {
    fetchOptions() {
      apiCommunityTree()
        .then((list) => {
          this.communities = list
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
      apiTemplateList()
        .then((list) => {
          // 只保留启用模板
          this.templates = list.filter((t) => t.status == 1)
        })
        .catch((_e: any) => {})
    },
    loadDetail() {
      if (this.pointId == '') return
      this.loading = true
      this.loadError = ''
      apiPointDetail(this.pointId)
        .then((p) => {
          this.loading = false
          this.qrcodeNo = p.qrcode_no
          this.communityId = p.community_id
          this.buildingId = p.building_id != null ? p.building_id : ''
          this.name = p.name
          this.type = p.type
          this.templateId = p.template_id != null ? p.template_id : ''
          this.remark = p.remark != null ? p.remark : ''
          this.lngText = p.longitude != 0 ? p.longitude.toFixed(6) : ''
          this.latText = p.latitude != 0 ? p.latitude.toFixed(6) : ''
          this.fenceRadius = p.fence_radius > 0 ? p.fence_radius : 200
          this.credential = p.credential != '' ? p.credential : 'qrcode'
          this.nfcId = p.nfc_id
        })
        .catch((e: Error) => {
          this.loading = false
          this.loadError = e.message
        })
    },
    onCommunityChange(e: any) {
      const c = this.communities[Number(e.detail.value)]
      if (c == null) return
      if (c.id != this.communityId) {
        this.communityId = c.id
        this.buildingId = '' // 换小区后楼栋重选
      }
    },
    onBuildingChange(e: any) {
      const i = Number(e.detail.value)
      this.buildingId = i == 0 ? '' : this.buildings[i - 1].id
    },
    onTypeChange(e: any) {
      this.type = POINT_TYPES[Number(e.detail.value)].value
    },
    onTemplateChange(e: any) {
      const i = Number(e.detail.value)
      this.templateId = i == 0 ? '' : this.templates[i - 1].id
    },
    onFenceChange(e: any) {
      this.fenceRadius = Number(e.detail.value)
    },
    onCredentialChange(v: string) {
      if (this.readonly) return
      this.credential = v
    },
    /** GPS 录入：统一 GCJ-02（系统定位不支持时自动 wgs84 本地纠偏，见 utils/geo.ts）；精度 >50m 提示 */
    locate() {
      if (this.locating) return
      this.locating = true
      getLocationGcj02(
        (loc) => {
          this.lngText = loc.longitude.toFixed(6)
          this.latText = loc.latitude.toFixed(6)
          this.accuracy = Math.round(loc.accuracy)
          this.locating = false
          if (this.accuracy > 50) {
            uni.showToast({ title: '定位精度较差，请靠近点位重试', icon: 'none' })
          }
        },
        (errMsg) => {
          this.locating = false
          const reason = errMsg.indexOf('fail') >= 0 ? errMsg.substring(errMsg.indexOf('fail') + 4).trim() : errMsg
          uni.showToast({ title: reason != '' ? '定位失败：' + reason : '定位失败，请检查定位权限后重试', icon: 'none' })
        }
      )
    },
    /** 读取 NFC 卡号 + 卡内编号 → 填入 nfc_id 并展示卡内编号对应关系（空白卡也可读 UID） */
    readCard() {
      if (!this.nfcSupported) {
        toastNfcUnavailable()
        return
      }
      uni.showLoading({ title: '请贴近 NFC 标签', mask: true })
      readCardInfoOnce((res, errMsg) => {
        uni.hideLoading()
        if (res == null || res.cardId == null) {
          uni.showToast({ title: errMsg || 'NFC 读取失败', icon: 'none' })
          return
        }
        this.nfcId = res.cardId
        // 卡内 NDEF 文本（点位编号）：展示与本点位编号的对应关系，便于识别空白卡/错卡
        if (res.code == null) {
          this.cardCodeInfo = '空白卡（未写入点位编号）'
          this.cardCodeWarn = false
        } else if (this.qrcodeNo != '' && res.code != this.qrcodeNo) {
          this.cardCodeInfo = '卡内编号：' + res.code + '（与本点位 ' + this.qrcodeNo + ' 不一致）'
          this.cardCodeWarn = true
        } else {
          this.cardCodeInfo = '卡内编号：' + res.code + '（与本点位一致）'
          this.cardCodeWarn = false
        }
        uni.showToast({ title: '已读取卡号', icon: 'success' })
      })
    },
    /** 写入点位编号到 NFC 卡（NDEF 文本，与打卡读取约定一致） */
    writeCard() {
      if (!this.nfcSupported) {
        toastNfcUnavailable()
        return
      }
      if (this.qrcodeNo == '') return
      uni.showLoading({ title: '请贴近要写入的 NFC 标签', mask: true })
      writePointCode(this.qrcodeNo, (ok, errMsg) => {
        uni.hideLoading()
        if (ok) {
          this.cardCodeInfo = '卡内编号：' + this.qrcodeNo + '（与本点位一致）'
          this.cardCodeWarn = false
          uni.showToast({ title: '写入成功，已校验', icon: 'none' })
        } else {
          uni.showToast({ title: errMsg || '写入失败', icon: 'none' })
        }
      })
    },
    /** 校验并组装提交体；不合法时 toast 并返回 null */
    buildPayload(): PointSavePayload | null {
      if (this.communityId == '') {
        uni.showToast({ title: '请选择小区', icon: 'none' })
        return null
      }
      if (this.name.trim() == '') {
        uni.showToast({ title: '请填写点位名称', icon: 'none' })
        return null
      }
      if (this.type == '') {
        uni.showToast({ title: '请选择点位类型', icon: 'none' })
        return null
      }
      const lng = parseFloat(this.lngText)
      const lat = parseFloat(this.latText)
      if (isNaN(lng) || isNaN(lat) || lng == 0 || lat == 0) {
        uni.showToast({ title: '请录入点位坐标', icon: 'none' })
        return null
      }
      if (lng < -180 || lng > 180 || lat < -90 || lat > 90) {
        uni.showToast({ title: '经纬度取值非法', icon: 'none' })
        return null
      }
      if (this.credential == 'nfc' && this.nfcId.trim() == '') {
        uni.showToast({ title: '凭证方式为 NFC 时须填写 NFC 卡号', icon: 'none' })
        return null
      }
      return {
        community_id: this.communityId,
        building_id: this.buildingId == '' ? null : this.buildingId,
        name: this.name.trim(),
        type: this.type,
        longitude: lng,
        latitude: lat,
        fence_radius: this.fenceRadius,
        credential: this.credential,
        // 免凭证时后端要求至少启用围栏校验（credential=none && !require_fence 会被拒）
        require_fence: this.credential == 'none',
        template_id: this.templateId == '' ? null : this.templateId,
        nfc_id: this.nfcId.trim(),
        remark: this.remark.trim()
      }
    },
    submit() {
      if (this.submitting) return
      const payload = this.buildPayload()
      if (payload == null) return
      this.submitting = true
      if (this.isEdit) {
        apiPointUpdate(this.pointId, payload)
          .then(() => {
            uni.showToast({ title: '已保存', icon: 'success' })
            setTimeout(() => {
              uni.navigateBack()
            }, 600)
          })
          .catch((e: Error) => {
            uni.showToast({ title: e.message, icon: 'none' })
          })
          .finally(() => {
            this.submitting = false
          })
        return
      }
      apiPointCreate(payload)
        .then((res) => {
          uni.showToast({ title: '点位已创建', icon: 'success' })
          const needNfc = payload.credential == 'nfc' || payload.credential == 'any'
          if (needNfc && this.nfcSupported) {
            // 编号创建后才生成：弹窗提示并可直接进入写卡流程（本页转编辑模式）
            uni.showModal({
              title: '创建成功',
              content: '点位已创建，编号 ' + res.qrcode_no,
              confirmText: '立即写卡',
              cancelText: '完成返回',
              success: (r) => {
                if (r.confirm) {
                  this.isEdit = true
                  this.justCreated = true
                  this.pointId = res.id
                  this.qrcodeNo = res.qrcode_no
                  uni.setNavigationBarTitle({ title: '点位编辑' })
                } else {
                  uni.navigateBack()
                }
              }
            })
          } else {
            setTimeout(() => {
              uni.navigateBack()
            }, 600)
          }
        })
        .catch((e: Error) => {
          uni.showToast({ title: e.message, icon: 'none' })
        })
        .finally(() => {
          this.submitting = false
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

.skeleton {
  padding: 24rpx;
}

.sk-block {
  height: 192rpx;
  border-radius: 24rpx;
  margin-bottom: 24rpx;
  opacity: 0.4;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.readonly-tip {
  border-radius: 12rpx;
  padding: 16rpx 24rpx;
  margin-bottom: 24rpx;
}

.readonly-tip-text {
  font-size: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.sec-title {
  font-size: 30rpx;
  font-weight: 600;
  margin-bottom: 16rpx;
}

.label {
  font-size: 26rpx;
  margin-top: 24rpx;
  margin-bottom: 12rpx;
}

.field {
  height: 88rpx;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
}

.field-text {
  font-size: 28rpx;
  flex: 1;
}

.field-arrow {
  font-size: 24rpx;
  margin-left: 16rpx;
}

.field-input {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  font-size: 28rpx;
  width: 100%;
}

.field-textarea {
  width: 100%;
  height: 160rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 16rpx 24rpx;
  font-size: 28rpx;
}

.btn-outline {
  height: 88rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  margin-top: 8rpx;
}

.btn-outline-text {
  font-size: 30rpx;
  font-weight: 600;
}

.loc-info {
  margin-top: 24rpx;
}

.loc-text {
  font-size: 28rpx;
}

.loc-acc {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.loc-empty {
  font-size: 26rpx;
  margin-top: 24rpx;
}

.loc-manual {
  flex-direction: row;
  margin-top: 24rpx;
}

.loc-input {
  flex: 1;
}

.loc-input-r {
  margin-left: 16rpx;
}

.fence-row {
  margin-top: 8rpx;
}

.fence-label {
  margin-top: 16rpx;
}

.fence-slider {
  width: 100%;
}

.cred-row {
  flex-direction: row;
  flex-wrap: wrap;
}

.cred-item {
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  padding: 12rpx 32rpx;
  margin-right: 16rpx;
  margin-bottom: 16rpx;
}

.cred-text {
  font-size: 26rpx;
}

.cred-tip {
  font-size: 24rpx;
  margin-top: 8rpx;
}

.nfc-box {
  border-top-width: 1rpx;
  border-top-style: solid;
  margin-top: 24rpx;
  padding-top: 8rpx;
}

.nfc-row {
  flex-direction: row;
  align-items: center;
}

.nfc-input {
  flex: 1;
}

.btn-mini {
  height: 64rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 32rpx;
  align-items: center;
  justify-content: center;
  padding: 0 24rpx;
  margin-left: 16rpx;
}

.btn-mini-text {
  font-size: 26rpx;
}

.nfc-write {
  margin-top: 24rpx;
}

.info-line {
  font-size: 28rpx;
  margin-top: 8rpx;
}

.btn-primary {
  height: 104rpx; /* Size.btnHeight */
  border-radius: 20rpx; /* Radius.button */
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
