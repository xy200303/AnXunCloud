import { createSSRApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import UniBadge from './uni_modules/uni-badge/components/uni-badge/uni-badge.vue'
import UniCard from './uni_modules/uni-card/components/uni-card/uni-card.vue'
import UniDataCheckbox from './uni_modules/uni-data-checkbox/components/uni-data-checkbox/uni-data-checkbox.vue'
import UniDateformat from './uni_modules/uni-dateformat/components/uni-dateformat/uni-dateformat.vue'
import UniEasyinput from './uni_modules/uni-easyinput/components/uni-easyinput/uni-easyinput.vue'
import UniIcons from './uni_modules/uni-icons/components/uni-icons/uni-icons.vue'
import UniList from './uni_modules/uni-list/components/uni-list/uni-list.vue'
import UniListItem from './uni_modules/uni-list/components/uni-list-item/uni-list-item.vue'
import UniLoadMore from './uni_modules/uni-load-more/components/uni-load-more/uni-load-more.vue'
import UniNoticeBar from './uni_modules/uni-notice-bar/components/uni-notice-bar/uni-notice-bar.vue'
import UniPopup from './uni_modules/uni-popup/components/uni-popup/uni-popup.vue'
import UniSegmentedControl from './uni_modules/uni-segmented-control/components/uni-segmented-control/uni-segmented-control.vue'
import UniTag from './uni_modules/uni-tag/components/uni-tag/uni-tag.vue'
import UniTransition from './uni_modules/uni-transition/components/uni-transition/uni-transition.vue'

const uniUiComponents = {
  'uni-badge': UniBadge,
  'uni-card': UniCard,
  'uni-data-checkbox': UniDataCheckbox,
  'uni-dateformat': UniDateformat,
  'uni-easyinput': UniEasyinput,
  'uni-icons': UniIcons,
  'uni-list': UniList,
  'uni-list-item': UniListItem,
  'uni-load-more': UniLoadMore,
  'uni-notice-bar': UniNoticeBar,
  'uni-popup': UniPopup,
  'uni-segmented-control': UniSegmentedControl,
  'uni-tag': UniTag,
  'uni-transition': UniTransition
}

export function createApp() {
  const app = createSSRApp(App)
  app.use(createPinia())
  Object.entries(uniUiComponents).forEach(([name, component]) => app.component(name, component))
  return {
    app
  }
}
