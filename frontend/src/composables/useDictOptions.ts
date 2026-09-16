// 业务字典选项共享加载：按 typeCode 分 key 做模块级缓存 + inflight 去重（模式同 usePatrolTypes）
// 字典项极少变动，整个会话加载一次即可；refresh(force) 供需要强刷的场景使用
import { ref, type Ref } from 'vue'
import { listDictOptions, type DictOption } from '@/api/dict'

const cache = new Map<string, Ref<DictOption[]>>()
const inflight = new Map<string, Promise<void>>()

function ensureLoaded(typeCode: string, options: Ref<DictOption[]>, force = false) {
  if (!force && options.value.length) return
  if (inflight.has(typeCode)) return
  const p = listDictOptions(typeCode)
    .then((list) => {
      options.value = list || []
    })
    .catch(() => {})
    .finally(() => {
      inflight.delete(typeCode)
    })
  inflight.set(typeCode, p)
}

export function useDictOptions(typeCode: string) {
  let options = cache.get(typeCode)
  if (!options) {
    options = ref<DictOption[]>([])
    cache.set(typeCode, options)
  }
  ensureLoaded(typeCode, options)

  return { options, refresh: () => ensureLoaded(typeCode, options!, true) }
}
