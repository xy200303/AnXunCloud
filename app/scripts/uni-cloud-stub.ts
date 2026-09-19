/**
 * @dcloudio/uni-cloud 的空壳替身（vite alias 用）。
 *
 * 项目不使用 uniCloud 云开发；uni-ui 的 uni-data-checkbox/uni-data-select 内部写了
 * `mixins: [uniCloud.mixinDatacom || {}]`，引用全局 uniCloud 会被 uni-app 自动从
 * @dcloudio/uni-cloud 注入整包（dist 多出 uni-cloud.es.*.js ≈106KB）。
 * 我们只用 localdata 本地数据，mixinDatacom 用不到——alias 到本空壳即可剔除整包。
 */
export const uniCloud = {}
export class UniCloudError extends Error {}
export default uniCloud
