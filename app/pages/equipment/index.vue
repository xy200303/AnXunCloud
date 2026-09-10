<template>
  <view class="page" :style="{ backgroundColor: colors.bgPage }">
    <!-- 筛选栏：小区 + 关键字 + 类型/到期状态 chips（字典驱动） -->
    <view class="filter-bar" :style="{ backgroundColor: colors.bgCard, borderBottomColor: colors.border }">
      <view class="filter-row">
        <AppFilterField :text="communityName" :selected="communityId != ''" :colors="colors" @click="openCommunitySheet" />
      </view>
      <view class="search-row">
        <input
          v-model="keyword"
          class="search-input"
          :style="{ borderColor: colors.border, color: colors.textPrimary }"
          placeholder="搜索设备编号或名称"
          confirm-type="search"
          @confirm="reload"
        />
        <text class="search-btn" :style="{ color: colors.primary }" @click="reload">搜索</text>
      </view>
      <AppChipScroller :items="typeChips" :value="typeFilter" :colors="colors" @change="pickType" />
      <AppChipScroller :items="dueChips" :value="dueFilter" :colors="colors" @change="pickDue" />
    </view>

    <AppListShell
      :loading="loading"
      :loaded="loaded"
      :empty="list.length == 0"
      :error="errorMsg"
      :show-skeleton="list.length == 0"
      empty-title="暂无设备"
      empty-sub="切换筛选条件试试，或联系管理员导入台账"
      :colors="colors"
      @retry="reload"
    >
      <!-- 设备卡片列表：红黄绿灰状态灯 -->
      <template #default>
      <view class="content">
        <view
          v-for="e in list"
          :key="e.id"
          hover-class="hover-dim"
          class="card"
          :style="{ backgroundColor: colors.bgCard }"
          @click="goDetail(e.id)"
        >
          <view class="card-head">
            <view class="card-title-row">
              <view class="due-dot" :style="{ backgroundColor: dueColorOf(e.due_state) }"></view>
              <text class="card-title" :style="{ color: colors.textPrimary }">{{ e.name }}</text>
            </view>
            <text class="card-status" :style="{ color: dueColorOf(e.due_state) }">{{ dueTextOf(e) }}</text>
          </view>
          <text class="card-sub" :style="{ color: colors.textSecondary }">编号：{{ e.code }}</text>
          <view class="card-foot">
            <view class="foot-tags">
              <text class="tag" :style="{ color: colors.textSecondary, borderColor: colors.border }">{{ e.type_label != '' ? e.type_label : e.type }}</text>
              <text v-if="e.status != 'in_service'" class="tag" :style="{ color: colors.info, borderColor: colors.info }">{{ e.status_label }}</text>
            </view>
            <text class="card-loc" :style="{ color: colors.textSecondary }">{{ locationText(e) }}</text>
          </view>
        </view>
      </view>
      </template>
      <template #footer>
        <AppListFooter :loading-more="loadingMore" :no-more="noMore" :visible="list.length > 0" :colors="colors" />
      </template>
    </AppListShell>
  </view>
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import { apiEquipmentList, apiCommunityTree, apiDictOptions, EquipmentListItem, EquipmentDueState, CommunityTreeNode, DictOption } from '@/services/api'
import AppListShell from '@/components/AppListShell.vue'
import AppListFooter from '@/components/AppListFooter.vue'
import AppChipScroller from '@/components/AppChipScroller.vue'
import AppFilterField from '@/components/AppFilterField.vue'

const PAGE_SIZE = 20

type ListData = {
  colors: ColorTokens
  communities: CommunityTreeNode[]
  communityId: string
  keyword: string
  typeFilter: string
  dueFilter: string
  typeOptions: DictOption[]
  loading: boolean
  loadingMore: boolean
  loaded: boolean
  errorMsg: string
  page: number
  total: number
  list: EquipmentListItem[]
  lastLoadedAt: number
}

/** 状态灯颜色：normal 绿 / warning 黄 / overdue 红 / scrap 红 / none、label_missing 灰 */
function dueColorOf(s: EquipmentDueState): string {
  if (s == 'normal') return Colors.success
  if (s == 'warning') return Colors.warning
  if (s == 'overdue' || s == 'scrap') return Colors.danger
  return Colors.info
}

/** 今日 0 点（本地时区），逾期天数计算用 */
function todayZero(): number {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  return d.getTime()
}

export default {
  components: { AppListShell, AppListFooter, AppChipScroller, AppFilterField },
  data(): ListData {
    return {
      colors: Colors,
      communities: [] as CommunityTreeNode[],
      communityId: '',
      keyword: '',
      typeFilter: '',
      dueFilter: '',
      typeOptions: [] as DictOption[],
      loading: true,
      loadingMore: false,
      loaded: false,
      errorMsg: '',
      page: 1,
      total: 0,
      list: [] as EquipmentListItem[],
      lastLoadedAt: 0
    }
  },
  computed: {
    communityName(): string {
      if (this.communityId == '') return '全部小区'
      const c = this.communities.find((x) => x.id == this.communityId)
      return c != null ? c.name : '全部小区'
    },
    typeChips(): Array<{ value: string; label: string }> {
      return [{ value: '', label: '全部类型' }].concat(this.typeOptions.map((o) => ({ value: o.value, label: o.label })))
    },
    dueChips(): Array<{ value: string; label: string }> {
      return [
        { value: '', label: '全部状态' },
        { value: 'overdue', label: '已逾期' },
        { value: 'warning', label: '临期' },
        { value: 'normal', label: '正常' },
        { value: 'none', label: '无到期日' },
        { value: 'scrap', label: '应报废' },
        { value: 'label_missing', label: '标签缺失' }
      ]
    },
    noMore(): boolean {
      return this.loaded && this.list.length >= this.total
    }
  },
  onLoad(options: any) {
    // 今日任务「维保待办」卡片带筛选跳入（due_state=overdue/warning）
    if (options && options.due_state) this.dueFilter = String(options.due_state)
    apiCommunityTree().then((list) => {
      this.communities = list
    }).catch((_e: any) => {})
    apiDictOptions('equipment_type').then((opts) => {
      this.typeOptions = opts
    }).catch(() => {})
    this.reload()
  },
  onShow() {
    // 登记/详情返回后刷新（台账可能被 confirmed 流水回写）
    if (this.loaded && Date.now() - this.lastLoadedAt > 10000) this.reload()
  },
  onPullDownRefresh() {
    this.reload()
  },
  onReachBottom() {
    this.loadMore()
  },
  methods: {
    dueColorOf,
    /** 到期状态文案：逾期红字带天数，无到期日提示补录 */
    dueTextOf(e: EquipmentListItem): string {
      if (e.due_state == 'label_missing') return '标签缺失'
      if (e.due_state == 'scrap') return '应报废'
      if (e.due_state == 'none') return '无到期日'
      if (e.next_due_date == '') return ''
      const due = new Date(e.next_due_date.replace(/-/g, '/')).getTime()
      const days = Math.round((due - todayZero()) / 86400000)
      if (days < 0) return '已逾期 ' + (-days) + ' 天'
      if (days == 0) return '今天到期'
      if (e.due_state == 'warning') return days + ' 天后到期'
      return e.next_due_date + ' 到期'
    },
    locationText(e: EquipmentListItem): string {
      const parts: string[] = []
      if (e.community_name != '') parts.push(e.community_name)
      if (e.building_name != null && e.building_name != '') parts.push(e.building_name)
      if (e.point_name != null && e.point_name != '') parts.push(e.point_name)
      return parts.length > 0 ? parts.join(' · ') : (e.remark != '' ? e.remark : '未绑定位置')
    },
    openCommunitySheet() {
      const names = ['全部小区'].concat(this.communities.map((c) => c.name))
      uni.showActionSheet({
        itemList: names,
        success: (res) => {
          const id = res.tapIndex == 0 ? '' : this.communities[res.tapIndex - 1].id
          if (id == this.communityId) return
          this.communityId = id
          this.reload()
        }
      })
    },
    pickType(v: string) {
      this.typeFilter = v
      this.reload()
    },
    pickDue(v: string) {
      this.dueFilter = v
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
      apiEquipmentList(this.page, PAGE_SIZE, {
        type: this.typeFilter,
        communityId: this.communityId,
        dueState: this.dueFilter,
        keyword: this.keyword.trim()
      })
        .then((res) => {
          this.total = res.total
          this.list = append ? this.list.concat(res.list) : res.list
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
    goDetail(id: string) {
      this.lastLoadedAt = 0
      uni.navigateTo({ url: '/pages/equipment/detail?id=' + encodeURIComponent(id) })
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

.card-title-row {
  flex-direction: row;
  align-items: center;
  flex: 1;
}

/* 红黄绿灰状态灯 */
.due-dot {
  width: 20rpx;
  height: 20rpx;
  border-radius: 10rpx;
  margin-right: 16rpx;
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
  justify-content: space-between;
  align-items: center;
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

.card-loc {
  font-size: 24rpx;
  flex: 1;
  text-align: right;
}
</style>
