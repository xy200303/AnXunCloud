// 分页列表通用逻辑：query(page/page_size + 筛选) + loading/list/total + 查询/重置/分页回调
// fetcher 内自行做空值收敛（空串转 undefined）；initQuery 每次返回全新初始条件供重置
import { reactive, ref, type Ref } from 'vue'
import type { PageResult } from '@/utils/request'

export interface PagedListQuery {
  page: number
  page_size: number
  [key: string]: any
}

export function usePagedList<T, Q extends PagedListQuery>(
  fetcher: (query: Q) => Promise<PageResult<T>>,
  initQuery: () => Q
) {
  const loading = ref(false)
  const list = ref<T[]>([]) as Ref<T[]>
  const total = ref(0)
  const query = reactive(initQuery()) as Q

  async function fetchList() {
    loading.value = true
    try {
      const data = await fetcher(query)
      list.value = data.list
      total.value = data.total
    } finally {
      loading.value = false
    }
  }

  function handleSearch() {
    query.page = 1
    fetchList()
  }

  function handleReset() {
    Object.assign(query, initQuery())
    fetchList()
  }

  return { loading, list, total, query, fetchList, handleSearch, handleReset }
}
