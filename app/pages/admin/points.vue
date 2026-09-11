<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 筛选栏：超级管理员支持企业/小区选择 + 名称搜索 + 新增 -->
    <view class="filter-bar" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <view class="filter-row">
        <AppFilterField v-if="canTenantFilter" :text="tenantName" :selected="tenantId != ''" :colors="colors" @click="openTenantSheet" />
        <AppFilterField :text="communityName" :selected="communityId != ''" :colors="colors" @click="openCommunitySheet" />
        <AppFilterField v-if="communityId != ''" :text="buildingName" :selected="buildingId != ''" :colors="colors" @click="openBuildingSheet" />
        <view v-if="canCreate" class="btn-add" :style="{ backgroundColor: colors.primary }" @click="goCreate">
          <text class="btn-add-text" :style="{ color: colors.white }">+ 新增</text>
        </view>
      </view>
      <view class="search-row">
        <input
          v-model="keyword"
          class="search-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="搜索点位名称"
          confirm-type="search"
          @confirm="reload"
        />
        <text class="search-btn" :style="{ color: colors.primary }" @click="reload">搜索</text>
      </view>
      <AppChipScroller :items="typeChips" :value="typeFilter" :colors="colors" @change="pickType" />
      <AppChipScroller :items="credChips" :value="credFilter" :colors="colors" @change="pickCred" />
    </view>

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无点位"
      :empty-sub="canCreate ? '点右上角「新增」现场建点' : '切换小区或关键词试试'"
      :colors="colors"
      @retry="reload"
    >

    <!-- 点位列表 -->
    <template #default>
    <view class="content">
      <view
        v-for="p in list"
        :key="p.id"
        class="card"
        :style="{ backgroundColor: colors.bgCard }"
        @click="goEdit(p.id)"
      >
        <view class="card-head">
          <text class="card-title" :style="{ color: colors.textPrimary }">{{ p.name }}</text>
          <text class="card-status" :style="{ color: p.state_color }">{{ p.state_text }}</text>
        </view>
        <text class="card-sub" :style="{ color: colors.textSecondary }">
          编号：{{ p.qrcode_no }} · {{ p.community_name }}<text v-if="p.building_name != ''"> · {{ p.building_name }}</text>
        </text>
        <view class="card-foot">
          <view class="foot-tags">
            <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ p.type_label != '' ? p.type_label : p.type }}</text>
            <text class="tag" :style="{ color: colors.primary, borderColor: colors.primary }">{{ p.credential_text }}</text>
          </view>
        </view>
      </view>

    </view>
    </template>
    <template #footer>
      <AppListFooter :loading-more="loadingMore" :no-more="noMore" :visible="list.length > 0" :colors="colors" />
    </template>
    </AppListShell>

    <!-- 企业/小区/楼栋筛选面板（自绘，替代原生 showActionSheet；sheetFor 区分三组选项） -->
    <AppActionSheet
      :visible="sheetShow"
      :items="sheetItems"
      @update:visible="sheetShow = $event"
      @select="onSheetSelect"
    />
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiPointList, apiCommunityTree, apiDictOptions, PointItem, CommunityTreeNode, DictOption } from '@/services/api'
import { useAuthStore } from '@/stores/auth'
import AppListShell from '@/components/AppListShell.vue'
import AppListFooter from '@/components/AppListFooter.vue'
import AppChipScroller from '@/components/AppChipScroller.vue'
import AppFilterField from '@/components/AppFilterField.vue'
import AppActionSheet from '@/components/AppActionSheet.vue'

const PAGE_SIZE = 20

/** 列表项视图模型：文案/颜色在数据层预计算 */
type PointView = PointItem & {
  credential_text: string
  state_text: string
  state_color: string
}

type ListData = {
  colors: ColorTokens
  communities: CommunityTreeNode[]
  tenantId: string
  communityId: string
  buildingId: string
  keyword: string
  typeFilter: string
  credFilter: string
  typeOptions: DictOption[]
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: PointView[]
  lastLoadedAt: number
  /** 筛选面板：sheetFor 区分企业/小区/楼栋三组选项 */
  sheetShow: boolean
  sheetFor: string
}

function credentialTextOf(c: string): string {
  if (c == 'qrcode') return '二维码'
  if (c == 'nfc') return 'NFC'
  if (c == 'any') return '任一'
  return '免凭证'
}

/**
 * 就绪状态（管理端建档视角）：
 * 停用 > 未录坐标（经纬度为空或 0）> 未绑 NFC（凭证含 nfc 但 nfc_id 空）> 已就绪
 */
function stateOf(p: PointItem): { text: string; color: string } {
  if (p.status != 1) return { text: '已停用', color: Colors.info }
  if (p.longitude == 0 || p.latitude == 0) return { text: '未录坐标', color: Colors.danger }
  if ((p.credential == 'nfc' || p.credential == 'any') && p.nfc_id == '') {
    return { text: '未绑NFC', color: Colors.warning }
  }
  return { text: '已就绪', color: Colors.success }
}

function toPointView(p: PointItem): PointView {
  const st = stateOf(p)
  return Object.assign({}, p, {
    credential_text: credentialTextOf(p.credential),
    state_text: st.text,
    state_color: st.color
  })
}

export default {
  components: { AppListShell, AppListFooter, AppChipScroller, AppFilterField, AppActionSheet },
  data(): ListData {
    return {
      colors: Colors,
      communities: [] as CommunityTreeNode[],
      tenantId: '',
      communityId: '',
      buildingId: '',
      keyword: '',
      typeFilter: '',
      credFilter: '',
      typeOptions: [] as DictOption[],
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as PointView[],
      lastLoadedAt: 0,
      sheetShow: false,
      sheetFor: ''
    }
  },
  computed: {
    canCreate(): boolean {
      return useAuthStore().hasPerm('inspection:point:create')
    },
    canTenantFilter(): boolean {
      const roles = useAuthStore().userInfo?.roles ?? []
      return roles.indexOf('super_admin') >= 0
    },
    tenantOptions(): Array<{ id: string; name: string }> {
      const seen: Record<string, boolean> = {}
      const options: Array<{ id: string; name: string }> = []
      this.communities.forEach((c) => {
        const id = c.tenant_id ?? ''
        if (id == '' || seen[id]) return
        seen[id] = true
        options.push({ id: id, name: c.tenant_name ?? id })
      })
      options.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
      return options
    },
    visibleCommunities(): CommunityTreeNode[] {
      if (this.tenantId == '') return this.communities
      return this.communities.filter((c) => (c.tenant_id ?? '') == this.tenantId)
    },
    tenantName(): string {
      if (this.tenantId == '') return '全部企业'
      const t = this.tenantOptions.find((x) => x.id == this.tenantId)
      return t != null ? t.name : '全部企业'
    },
    communityName(): string {
      if (this.communityId == '') return '全部小区'
      const c = this.visibleCommunities.find((x) => x.id == this.communityId)
      return c != null ? c.name : '全部小区'
    },
    buildings(): Array<{ id: string; name: string; type: string }> {
      const c = this.visibleCommunities.find((x) => x.id == this.communityId)
      return c != null ? c.buildings : []
    },
    buildingName(): string {
      if (this.buildingId == '') return '楼栋/区域'
      const b = this.buildings.find((x) => x.id == this.buildingId)
      return b != null ? b.name : '楼栋/区域'
    },
    typeChips(): Array<{ value: string; label: string }> {
      return [{ value: '', label: '全部类型' }].concat(this.typeOptions.map((o) => ({ value: o.value, label: o.label })))
    },
    credChips(): Array<{ value: string; label: string }> {
      return [
        { value: '', label: '全部凭证' },
        { value: 'qrcode', label: '二维码' },
        { value: 'nfc', label: 'NFC' },
        { value: 'any', label: '任一' },
        { value: 'none', label: '免凭证' }
      ]
    },
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    },
    /** 筛选面板选项（按 sheetFor 取对应维度，首项均为全部） */
    sheetItems(): string[] {
      if (this.sheetFor == 'tenant') return ['全部企业'].concat(this.tenantOptions.map((t) => t.name))
      if (this.sheetFor == 'building') return ['全部楼栋/区域'].concat(this.buildings.map((b) => b.name))
      return ['全部小区'].concat(this.visibleCommunities.map((c) => c.name))
    }
  },
  onLoad() {
    this.fetchCommunities()
    apiDictOptions('point_type').then((opts) => {
      this.typeOptions = opts
    }).catch(() => {})
    this.reload()
  },
  onShow() {
    // 新增/编辑返回后刷新
    if (this.loaded && Date.now() - this.lastLoadedAt > 20000) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    fetchCommunities() {
      apiCommunityTree()
        .then((list) => {
          this.communities = list
        })
        .catch((_e: any) => {})
    },
    /** 小区筛选：底部面板选择（首项为全部） */
    openCommunitySheet() {
      this.sheetFor = 'community'
      this.sheetShow = true
    },
    /** 企业筛选：仅超级管理员可见，首项为全部 */
    openTenantSheet() {
      this.sheetFor = 'tenant'
      this.sheetShow = true
    },
    /** 楼栋/区域筛选：当前小区的楼栋与区域行（首项为全部） */
    openBuildingSheet() {
      this.sheetFor = 'building'
      this.sheetShow = true
    },
    onSheetSelect(idx: number) {
      if (this.sheetFor == 'tenant') {
        const id = idx == 0 ? '' : this.tenantOptions[idx - 1].id
        if (id == this.tenantId) return
        this.tenantId = id
        this.communityId = ''
        this.reload()
        return
      }
      if (this.sheetFor == 'building') {
        const id = idx == 0 ? '' : this.buildings[idx - 1].id
        if (id == this.buildingId) return
        this.buildingId = id
        this.reload()
        return
      }
      const id = idx == 0 ? '' : this.visibleCommunities[idx - 1].id
      if (id == this.communityId) return
      this.communityId = id
      this.buildingId = ''
      this.reload()
    },
    pickType(v: string) {
      this.typeFilter = v
      this.reload()
    },
    pickCred(v: string) {
      this.credFilter = v
      this.reload()
    },
    reload() {
      this.page = 1
      this.fetchPage(false)
    },
    loadMore() {
      if (this.loading || this.loadingMore || this.noMore || !this.loaded) return
      this.page += 1
      this.fetchPage(true)
    },
    fetchPage(append: boolean) {
      if (append) {
        this.loadingMore = true
      } else {
        this.loading = true
      }
      apiPointList(this.page, PAGE_SIZE, this.communityId, this.keyword.trim(), this.tenantId, {
        type: this.typeFilter,
        credential: this.credFilter,
        buildingId: this.buildingId
      })
        .then((res) => {
          const views: PointView[] = []
          res.list.forEach((p: PointItem) => {
            views.push(toPointView(p))
          })
          this.total = res.total
          this.list = append ? this.list.concat(views) : views
          this.loading = false
          this.loadingMore = false
          this.loaded = true
          this.lastLoadedAt = Date.now()
          uni.stopPullDownRefresh()
        })
        .catch((e: Error) => {
          this.loading = false
          this.loadingMore = false
          if (append) this.page -= 1
          if (!this.loaded) this.errorMsg = e.message
          uni.stopPullDownRefresh()
          if (this.loaded || append) uni.showToast({ title: e.message, icon: 'none' })
        })
    },
    goCreate() {
      this.lastLoadedAt = 0
      uni.navigateTo({ url: '/pages/admin/point-form' })
    },
    goEdit(id: string) {
      this.lastLoadedAt = 0
      uni.navigateTo({ url: '/pages/admin/point-form?id=' + encodeURIComponent(id) })
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

.filter-row {
  flex-direction: row;
  align-items: center;
}

.btn-add {
  height: 72rpx;
  border-radius: 12rpx;
  align-items: center;
  justify-content: center;
  padding: 0 32rpx;
  margin-left: 16rpx;
}

.btn-add-text {
  font-size: 28rpx;
  font-weight: 600;
}

.search-row {
  flex-direction: row;
  align-items: center;
  margin-top: 16rpx;
}

.search-input {
  flex: 1;
  height: 72rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx;
  padding: 0 24rpx;
  font-size: 28rpx;
}

.search-btn {
  font-size: 28rpx;
  padding: 0 24rpx;
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

.sk-short {
  height: 96rpx;
}

.empty {
  align-items: center;
  padding-top: 192rpx;
}

.empty-title {
  font-size: 34rpx;
  margin-bottom: 16rpx;
}

.empty-sub {
  font-size: 26rpx;
}

.empty-retry {
  font-size: 30rpx;
  padding: 16rpx 32rpx;
}

.content {
  padding: 24rpx;
}

.card {
  border-radius: 24rpx; /* Radius.card */
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.card-head {
  flex-direction: row;
  justify-content: space-between;
  align-items: center;
}

.card-title {
  font-size: 34rpx; /* FontSize.bodyL */
  font-weight: 600;
  flex: 1;
}

.card-status {
  font-size: 26rpx;
  margin-left: 16rpx;
}

.card-sub {
  font-size: 26rpx;
  margin-top: 8rpx;
}

.card-foot {
  flex-direction: row;
  margin-top: 24rpx;
}

.foot-tags {
  flex-direction: row;
}

.tag {
  font-size: 22rpx;
  border-width: 2rpx;
  border-style: solid;
  border-radius: 12rpx; /* Radius.tag */
  padding: 4rpx 16rpx;
  margin-right: 16rpx;
}

</style>
