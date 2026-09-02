/**
 * 消息角标 store（M4 消息中心：unread_count → tabBar 消息 tab 角标）。
 * tabBar 消息 tab 展示 unread 角标（>99 显示 99+），为 0 时移除角标。
 * 刷新点：消息页 onShow/操作后、今日任务页 onShow（轻量请求 /messages?page=1&page_size=1 只取 unread_count）。
 */

import { defineStore } from 'pinia'
import { apiMessages } from '@/services/api'

/** tabBar 中「消息」tab 的索引（pages.json tabBar list 顺序：任务0/报告1/消息2/我的3） */
const MESSAGE_TAB_INDEX = 2

function applyBadge(n: number) {
  if (n > 0) {
    uni.setTabBarBadge({ index: MESSAGE_TAB_INDEX, text: n > 99 ? '99+' : String(n) })
  } else {
    uni.removeTabBarBadge({ index: MESSAGE_TAB_INDEX })
  }
}

export const useMessageStore = defineStore('message', {
  state() {
    return {
      unread: 0
    }
  },
  actions: {
    setUnread(n: number) {
      this.unread = n
      applyBadge(n)
    },
    clear() {
      this.setUnread(0)
    },
    /** 拉取未读数并刷新 tabBar 角标（失败静默，不打断页面） */
    refresh(): Promise<void> {
      return new Promise<void>((resolve) => {
        apiMessages(1, 1)
          .then((res) => {
            this.setUnread(res.unread_count)
            resolve()
          })
          .catch(() => {
            resolve()
          })
      })
    }
  }
})
