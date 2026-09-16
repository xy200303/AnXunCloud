// 巡检员（启用用户）列表共享加载：模块级缓存 + inflight 去重（模式同 usePatrolTypes）
// 各列表页的巡检员下拉口径一致（status=1 前 100 条），整个会话加载一次即可
import { ref } from 'vue'
import { listUsers } from '@/api/user'
import type { UserItem } from '@/api/types'

const inspectors = ref<UserItem[]>([])
let loadingPromise: Promise<void> | null = null

function ensureLoaded(force = false) {
  if (!force && inspectors.value.length) return
  if (loadingPromise) return
  loadingPromise = listUsers({ page: 1, page_size: 100, status: 1 })
    .then((d) => {
      inspectors.value = d.list
    })
    .catch(() => {})
    .finally(() => {
      loadingPromise = null
    })
}

export function useInspectors() {
  ensureLoaded()

  return { inspectors, refresh: () => ensureLoaded(true) }
}
