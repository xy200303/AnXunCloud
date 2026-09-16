// 启用小区列表共享加载：模块级缓存 + inflight 去重（模式同 usePatrolTypes）
// 各列表页的小区下拉口径一致（status=1 前 100 条），整个会话加载一次即可
import { ref } from 'vue'
import { listCommunities } from '@/api/community'
import type { CommunityItem } from '@/api/biz-types'

const communities = ref<CommunityItem[]>([])
const loading = ref(false)
let loadingPromise: Promise<void> | null = null

function ensureLoaded(force = false) {
  if (!force && communities.value.length) return
  if (loadingPromise) return
  loading.value = true
  loadingPromise = listCommunities({ page: 1, page_size: 100, status: 1 })
    .then((d) => {
      communities.value = d.list
    })
    .catch(() => {})
    .finally(() => {
      loading.value = false
      loadingPromise = null
    })
}

export function useCommunities() {
  ensureLoaded()

  return { communities, loading, refresh: () => ensureLoaded(true) }
}
