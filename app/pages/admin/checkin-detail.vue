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

    <block v-else-if="d != null">
      <CheckinDetailView :record="d" />
      <view class="bottom-space"></view>
    </block>
  </view>
</template>

<script lang="ts">
import { Colors } from '@/utils/theme'
import { apiAdminCheckinDetail, AdminCheckinDetail } from '@/services/api'
import CheckinDetailView from '@/components/CheckinDetailView.vue'

export default {
  components: { CheckinDetailView },
  data() {
    return {
      colors: Colors,
      checkinId: '',
      loading: true,
      loaded: false,
      errorMsg: '加载失败',
      d: null as AdminCheckinDetail | null
    }
  },
  onLoad(options: any) {
    this.checkinId = options && options.id ? String(options.id) : ''
    this.load()
  },
  methods: {
    load() {
      if (this.checkinId == '') {
        this.loading = false
        this.errorMsg = '缺少参数'
        return
      }
      this.loading = !this.loaded
      apiAdminCheckinDetail(this.checkinId)
        .then((d) => {
          this.d = d
          this.loading = false
          this.loaded = true
        })
        .catch((e: Error) => {
          this.loading = false
          this.errorMsg = e.message
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

.bottom-space {
  height: 48rpx;
}
</style>
