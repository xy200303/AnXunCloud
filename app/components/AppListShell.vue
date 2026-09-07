<template>
  <AppLoadState
    :loading="loading"
    :loaded="loaded"
    :empty="empty"
    :error="error"
    :empty-title="emptyTitle"
    :empty-sub="emptySub"
    :retry-text="retryText"
    :show-skeleton="showSkeleton"
    :colors="colors"
    @retry="$emit('retry')"
  />
  <slot v-if="loaded && !empty" />
  <slot name="footer" />
</template>

<script lang="ts">
import { Colors, ColorTokens } from '@/utils/theme'
import AppLoadState from '@/components/AppLoadState.vue'

export default {
  components: { AppLoadState },
  props: {
    loading: { type: Boolean, default: false },
    loaded: { type: Boolean, default: false },
    empty: { type: Boolean, default: false },
    error: { type: String, default: '' },
    emptyTitle: { type: String, default: '暂无数据' },
    emptySub: { type: String, default: '' },
    retryText: { type: String, default: '重试' },
    showSkeleton: { type: Boolean, default: true },
    colors: { type: Object, default: () => Colors as ColorTokens }
  },
  emits: ['retry']
}
</script>
