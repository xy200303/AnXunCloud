<template>
  <view class="page bg-page" >
    <!-- 骨架屏（编辑模式加载详情） -->
    <view v-if="loading" class="skeleton">
      <view class="sk-block bg-border" ></view>
      <view class="sk-block bg-border" ></view>
    </view>

    <!-- 加载失败 -->
    <view v-else-if="loadError != ''" class="empty">
      <text class="empty-title text-regular" >{{ loadError }}</text>
      <text class="empty-retry text-brand"  @click="loadDetail">重试</text>
    </view>

    <view v-else class="content">
      <!-- 只读提示 -->
      <view v-if="readonly" class="readonly-tip bg-brand-light" >
        <text class="readonly-tip-text text-brand" >无点位编辑权限，仅可查看</text>
      </view>

      <!-- 基础信息 -->
      <view class="card bg-card" >
        <text class="sec-title text-main" >基础信息</text>

        <text class="label text-regular" >所属小区 *</text>
        <picker :range="communityNames" :value="communityIndex" :disabled="readonly" @change="onCommunityChange">
          <view class="field border-default" >
            <text class="field-text"  :class="(communityId == '' ? 'text-secondary' : 'text-main')">
              {{ communityId == '' ? '请选择小区' : communityName }}
            </text>
            <text class="field-arrow text-secondary" >▾</text>
          </view>
        </picker>

        <text class="label text-regular" >楼栋（选填）</text>
        <picker :range="buildingNames" :value="buildingIndex" :disabled="readonly || communityId == ''" @change="onBuildingChange">
          <view class="field border-default" >
            <text class="field-text"  :class="(buildingId == '' ? 'text-secondary' : 'text-main')">
              {{ communityId == '' ? '请先选择小区' : (buildingId == '' ? '不分区/整小区' : buildingName) }}
            </text>
            <text class="field-arrow text-secondary" >▾</text>
          </view>
        </picker>

        <text class="label text-regular" >点位名称 *</text>
        <input
          v-model="name"
          class="field-input border-default text-main"
          
          placeholder="如：1号楼配电房"
          :maxlength="50"
          :disabled="readonly"
        />

        <text class="label text-regular" >点位类型 *</text>
        <picker :range="typeNames" :value="typeIndex" :disabled="readonly" @change="onTypeChange">
          <view class="field border-default" >
            <text class="field-text"  :class="(type == '' ? 'text-secondary' : 'text-main')">
              {{ type == '' ? '请选择类型' : typeName }}
            </text>
            <text class="field-arrow text-secondary" >▾</text>
          </view>
        </picker>

        <text class="label text-regular" >检查项模板（必选，可多选）</text>
        <view class="field border-default"  @click="openTemplateSheet">
          <text class="field-text"  :class="(templateIds.length == 0 ? 'text-secondary' : 'text-main')">
            {{ templateIds.length == 0 ? '请选择模板' : templateNamesText }}
          </text>
          <text class="field-arrow text-secondary" >▾</text>
        </view>
        <text class="cred-tip text-secondary" >检查项 = 所选模板的并集，可多选组合</text>

        <text class="label text-regular" >备注（选填）</text>
        <textarea
          v-model="remark"
          class="field-textarea border-default text-main"
          
          placeholder="补充说明"
          :maxlength="200"
          :disabled="readonly"
        />
      </view>

      <!-- 坐标与围栏 -->
      <view class="card bg-card" >
        <text class="sec-title text-main" >坐标与围栏</text>

        <button v-if="!readonly" plain="true" class="btn-outline" hover-class="hover-dim" @click="locate">
          <text class="btn-outline-text">{{ locating ? '定位中…' : '获取当前位置' }}</text>
        </button>

        <view v-if="hasLocation" class="loc-info">
          <text class="loc-text text-regular" >经度 {{ lngText }}，纬度 {{ latText }}</text>
          <text v-if="accuracy > 0" class="loc-acc"  :class="(accuracy > 50 ? 'text-danger' : 'text-secondary')">
            定位精度约 {{ accuracy }} m{{ accuracy > 50 ? '，定位精度较差，请靠近点位重试' : '' }}
          </text>
        </view>
        <text v-else class="loc-empty text-secondary" >尚未录入坐标（选填），可定位或手动输入；开启围栏时补录坐标后方生效</text>

        <view v-if="!readonly" class="loc-manual">
          <input
            v-model="lngText"
            class="field-input loc-input border-default text-main"
            
            placeholder="经度"
            type="digit"
          />
          <input
            v-model="latText"
            class="field-input loc-input loc-input-r border-default text-main"
            
            placeholder="纬度"
            type="digit"
          />
        </view>

        <view class="fence-row">
          <text class="label fence-label text-regular" >电子围栏校验</text>
          <switch :checked="fenceOn" :disabled="readonly" :color="'#2B5AED'" @change="fenceOn = $event.detail.value" />
        </view>
        <text v-if="!fenceOn" class="cred-tip text-secondary" >关闭后不校验到场位置（扫码/NFC 仍可凭证打卡）</text>
        <view v-if="fenceOn" class="fence-row">
          <text class="label fence-label text-regular" >围栏半径：{{ fenceRadius }} m</text>
          <view class="fence-slider-wrap">
            <slider
              :value="fenceRadius"
              :min="50"
              :max="1000"
              :step="10"
              :disabled="readonly"
              class="fence-slider"
              :activeColor="'#2B5AED'"
              @changing="onFenceChanging"
              @change="onFenceChange"
            />
          </view>
        </view>
      </view>

      <!-- 凭证方式 -->
      <view class="card bg-card" >
        <text class="sec-title text-main" >打卡凭证</text>
        <view class="cred-row">
          <view
            v-for="c in credentialOptions"
            :key="c.value"
            class="cred-item"
            :style="credential == c.value
              ? { backgroundColor: '#EAEFFF', borderColor: '#2B5AED' }
              : { backgroundColor: '#FFFFFF', borderColor: '#E5E6EB' }"
            @click="onCredentialChange(c.value)"
          >
            <text class="cred-text"  :class="(credential == c.value ? 'text-brand' : 'text-regular')">{{ c.label }}</text>
          </view>
        </view>
        <text v-if="credential == 'none'" class="cred-tip text-secondary" >不需要凭证的点位建议开启电子围栏校验，否则不做任何到场核验</text>

        <!-- NFC 区（凭证含 NFC 时显示） -->
        <template v-if="credential == 'nfc' || credential == 'any'">
          <view class="nfc-box border-default" >
            <text class="label text-regular" >NFC 卡号{{ credential == 'nfc' ? ' *' : '（选填）' }}</text>
            <view class="nfc-row">
              <input
                v-model="nfcId"
                class="field-input nfc-input border-default text-main"
                
                placeholder="读取或手动输入卡号"
                :disabled="readonly"
              />
              <button v-if="!readonly && nfcSupported" plain="true" class="btn-mini btn-outline" hover-class="hover-dim" @click="readCard">
                <text class="btn-mini-text">读卡号</text>
              </button>
            </view>
            <!-- 卡内已写入的点位编号（读卡后显示；与本点位编号不一致时警示） -->
            <text
              v-if="cardCodeInfo != ''"
              class="cred-tip"
               :class="(cardCodeWarn ? 'text-danger' : 'text-secondary')"
            >{{ cardCodeInfo }}</text>

            <!-- 写卡：编辑模式直接可写；新增模式提交后方可写 -->
            <template v-if="!readonly">
              <button v-if="isEdit && nfcSupported" plain="true" class="btn-outline nfc-write" hover-class="hover-dim" @click="writeCard">
                <text class="btn-outline-text">写入点位编号到卡（{{ qrcodeNo }}）</text>
              </button>
              <text v-else-if="!isEdit" class="cred-tip text-secondary" >提交创建生成编号后，可写入编号到 NFC 卡</text>
              <text v-if="!nfcSupported" class="cred-tip text-secondary" >当前端不支持 NFC，可手动输入卡号</text>
            </template>
          </view>
        </template>
      </view>

      <!-- 编号展示 + 启用状态（编辑模式） -->
      <view v-if="isEdit" class="card bg-card" >
        <text class="info-line text-regular" >点位编号：{{ qrcodeNo }}</text>
        <text class="info-line text-secondary" >编号由系统生成，不可修改</text>
        <view class="fence-row status-row">
          <text class="label fence-label text-regular" >启用状态</text>
          <switch :checked="status == 1" :disabled="readonly" :color="'#2B5AED'" @change="onStatusChange" />
        </view>
        <text class="cred-tip text-secondary" >停用后巡检员不可打卡该点位，已关联任务也不再下发</text>
      </view>

      <!-- 提交 -->
      <button
        v-if="!readonly"
        plain="true"
        class="btn-primary"
        :class="(submitting ? 'btn-disabled' : 'btn-primary')"
        hover-class="hover-dim"
        @click="submit"
      >
        <text class="btn-primary-text">{{ submitting ? '提交中…' : (isEdit ? '保存' : '创建点位') }}</text>
      </button>
      <view class="bottom-space"></view>
    </view>

    <!-- 模板多选弹层（AppBottomSheet→uni-popup 底部面板 + 复选列表；勾选即生效，「完成」关闭） -->
    <AppBottomSheet :visible="tplSheetShow" @close="closeTemplateSheet">
      <view class="tpl-panel bg-card" >
        <view class="tpl-head border-default" >
          <text class="tpl-head-text text-secondary" >选择检查项模板（可多选）</text>
        </view>
        <scroll-view scroll-y class="tpl-list">
          <uni-data-checkbox
            v-if="templates.length > 0"
            multiple
            mode="list"
            :localdata="templateItems"
            :modelValue="templateIds"
            :disabled="readonly"
            @change="onTemplatesChange"
          />
          <view v-else class="tpl-empty">
            <text class="tpl-empty-text text-secondary" >暂无启用的模板</text>
          </view>
        </scroll-view>
        <view class="tpl-actions border-default" >
          <view class="tpl-clear border-default"  hover-class="hover-dim" @click="clearTemplates">
            <text class="tpl-clear-text text-regular" >清空</text>
          </view>
          <view class="tpl-done bg-brand"  hover-class="hover-dim" @click="closeTemplateSheet">
            <text class="tpl-done-text text-white" >完成（已选 {{ templateIds.length }} 项）</text>
          </view>
        </view>
      </view>
    </AppBottomSheet>

    <!-- 创建成功弹窗（自绘，替代原生 showModal）：可直接进入写卡流程（本页转编辑模式） -->
    <AppDialog
      :visible="createdDlg.show"
      kind="success"
      title="创建成功"
      :content="'点位已创建，编号 ' + createdDlg.qrcodeNo"
      confirm-text="立即写卡"
      cancel-text="完成返回"
      @update:visible="createdDlg.show = $event"
      @confirm="onCreatedConfirm"
      @cancel="onCreatedCancel"
    />
  </view>
</template>

<script lang="ts">
import { toastErr } from '@/utils/ui'

import {
  apiCommunityTree,
  apiTemplateList,
  apiPointDetail,
  apiPointCreate,
  apiPointUpdate,
  apiDictOptions,
  CommunityTreeNode,
  TemplateListItem,
  DictOption,
  PointSavePayload
} from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import { isNfcSupported, readCardInfoOnce, writePointCode, toastNfcUnavailable } from '@/utils/nfc'
import { getLocationGcj02 } from '@/utils/geo'
import AppDialog from '@/components/AppDialog.vue'
import AppBottomSheet from '@/components/AppBottomSheet.vue'

type FormData = {
  isEdit: boolean
  pointId: string
  qrcodeNo: string
  loading: boolean
  loadError: string
  communities: CommunityTreeNode[]
  templates: TemplateListItem[]
  typeOptions: DictOption[]
  communityId: string
  buildingId: string
  name: string
  type: string
  templateIds: string[]
  tplSheetShow: boolean
  status: number
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
  /** 创建成功弹窗：暂存新点位 id/编号供「立即写卡」转编辑态 */
  createdDlg: { show: boolean; id: string; qrcodeNo: string }
}

export default {
  components: { AppDialog, AppBottomSheet },
  data(): FormData {
    return {
      isEdit: false,
      pointId: '',
      qrcodeNo: '',
      loading: false,
      loadError: '',
      communities: [] as CommunityTreeNode[],
      templates: [] as TemplateListItem[],
      typeOptions: [] as DictOption[],
      communityId: '',
      buildingId: '',
      name: '',
      type: '',
      templateIds: [] as string[],
      tplSheetShow: false,
      status: 1,
      remark: '',
      lngText: '',
      latText: '',
      accuracy: 0,
      locating: false,
      fenceRadius: 100,
      fenceOn: true,
      credential: 'qrcode',
      credentialOptions: [
        { label: '二维码', value: 'qrcode' },
        { label: 'NFC', value: 'nfc' },
        { label: '任一', value: 'any' },
        { label: '不需要', value: 'none' }
      ],
      nfcId: '',
      nfcSupported: false,
      cardCodeInfo: '',
      cardCodeWarn: false,
      submitting: false,
      justCreated: false,
      createdDlg: { show: false, id: '', qrcodeNo: '' }
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
    /** 类型选项（字典驱动）：详情回填的类型不在字典内时追加原值选项，避免 picker 静默改写 */
    typeOptionsView(): DictOption[] {
      if (this.type != '' && this.typeOptions.findIndex((o) => o.value == this.type) < 0) {
        return this.typeOptions.concat([{ label: this.type, value: this.type, sort: 999 }])
      }
      return this.typeOptions
    },
    typeNames(): string[] {
      return this.typeOptionsView.map((t) => t.label)
    },
    typeIndex(): number {
      const i = this.typeOptionsView.findIndex((t) => t.value == this.type)
      return i < 0 ? 0 : i
    },
    typeName(): string {
      const t = this.typeOptionsView.find((x) => x.value == this.type)
      return t != null ? t.label : ''
    },
    /** 模板下拉项（uni-data-checkbox localdata 形态；value=模板 id） */
    templateItems(): Array<{ text: string; value: string }> {
      return this.templates.map((t) => ({ text: t.name, value: t.id }))
    },
    /** 已选模板名拼接展示（未知 id 兜底显示原值） */
    templateNamesText(): string {
      const names: string[] = []
      this.templateIds.forEach((id) => {
        const t = this.templates.find((x) => x.id == id)
        names.push(t != null ? t.name : id)
      })
      return names.join('、')
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
        .catch((e: any) => {
          uni.showToast({ title: e?.message || '模板列表加载失败', icon: 'none' })
        })
      apiDictOptions('point_type')
        .then((opts) => {
          this.typeOptions = opts
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
          this.templateIds = p.template_ids != null ? p.template_ids : []
          this.status = p.status
          this.remark = p.remark != null ? p.remark : ''
          this.lngText = p.longitude != 0 ? p.longitude.toFixed(6) : ''
          this.latText = p.latitude != 0 ? p.latitude.toFixed(6) : ''
          this.fenceRadius = p.fence_radius > 0 ? p.fence_radius : 100
          this.fenceOn = !!p.require_fence
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
      const opt = this.typeOptionsView[Number(e.detail.value)]
      if (opt != null) this.type = opt.value
    },
    openTemplateSheet() {
      if (this.readonly) return
      this.tplSheetShow = true
    },
    closeTemplateSheet() {
      this.tplSheetShow = false
    },
    /** 模板勾选（uni-data-checkbox change，整体值直写；勾选即生效） */
    onTemplatesChange(e: { detail: { value: string[] } }) {
      if (this.readonly) return
      const v = e != null && e.detail != null && Array.isArray(e.detail.value) ? e.detail.value : []
      this.templateIds = v
    },
    clearTemplates() {
      if (this.readonly) return
      this.templateIds = []
    },
    onStatusChange(e: any) {
      this.status = e.detail.value ? 1 : 0
    },
    setFenceRadius(value: any) {
      const n = Number(value)
      if (Number.isNaN(n)) return
      this.fenceRadius = Math.min(1000, Math.max(50, Math.round(n / 10) * 10))
    },
    onFenceChanging(e: any) {
      this.setFenceRadius(e.detail.value)
    },
    onFenceChange(e: any) {
      this.setFenceRadius(e.detail.value)
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
      // 坐标选填（0,0=未录，后续可补录）：须同时填写或同时留空
      const lngEmpty = this.lngText.trim() == ''
      const latEmpty = this.latText.trim() == ''
      if (lngEmpty != latEmpty) {
        uni.showToast({ title: '经纬度须同时填写或同时留空', icon: 'none' })
        return null
      }
      let lng = 0
      let lat = 0
      if (!lngEmpty) {
        lng = parseFloat(this.lngText)
        lat = parseFloat(this.latText)
        if (isNaN(lng) || isNaN(lat) || lng < -180 || lng > 180 || lat < -90 || lat > 90) {
          uni.showToast({ title: '经纬度格式或取值非法', icon: 'none' })
          return null
        }
      }
      if (this.credential == 'nfc' && this.nfcId.trim() == '') {
        uni.showToast({ title: '凭证方式为 NFC 时须填写 NFC 卡号', icon: 'none' })
        return null
      }
      if (this.templateIds.length == 0) {
        uni.showToast({ title: '请选择检查项模板', icon: 'none' })
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
        require_fence: this.fenceOn,
        template_ids: this.templateIds,
        status: this.status,
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
            this.createdDlg = { show: true, id: res.id, qrcodeNo: res.qrcode_no }
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
    },
    /** 创建成功弹窗「立即写卡」：本页转编辑模式（写卡需要 pointId/qrcodeNo） */
    onCreatedConfirm() {
      this.isEdit = true
      this.justCreated = true
      this.pointId = this.createdDlg.id
      this.qrcodeNo = this.createdDlg.qrcodeNo
      uni.setNavigationBarTitle({ title: '点位编辑' })
    },
    onCreatedCancel() {
      uni.navigateBack()
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

.fence-slider-wrap {
  width: 100%;
  box-sizing: border-box;
  padding: 0 32rpx;
  margin-top: 8rpx;
}

.fence-slider {
  width: 100%;
  margin: 0;
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

.status-row {
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  margin-top: 24rpx;
}


.tpl-panel {
  width: 100%;
  flex-shrink: 0;
  flex-direction: column;
  border-radius: 24rpx 24rpx 0 0;
  overflow: hidden;
}

.tpl-head {
  height: 96rpx;
  align-items: center;
  justify-content: center;
  border-bottom-width: 1rpx;
  border-bottom-style: solid;
}

.tpl-head-text {
  font-size: 26rpx;
}

.tpl-list {
  max-height: 720rpx;
}





.tpl-empty {
  height: 160rpx;
  align-items: center;
  justify-content: center;
}

.tpl-empty-text {
  font-size: 26rpx;
}

.tpl-actions {
  flex-direction: row;
  align-items: center;
  padding: 24rpx 32rpx;
  padding-bottom: calc(24rpx + env(safe-area-inset-bottom));
  border-top-width: 1rpx;
  border-top-style: solid;
}

.tpl-clear {
  height: 80rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
  padding: 0 48rpx;
  margin-right: 24rpx;
}

.tpl-clear-text {
  font-size: 28rpx;
}

.tpl-done {
  height: 80rpx;
  flex: 1;
  border-radius: 20rpx; /* Radius.button */
  align-items: center;
  justify-content: center;
}

.tpl-done-text {
  font-size: 28rpx;
  font-weight: 600;
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
