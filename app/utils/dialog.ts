import { useDialog } from '@/uni_modules/wot-ui/composables'
import type { Dialog, DialogOptions } from '@/uni_modules/wot-ui/components/wd-dialog/types'

const APP_DIALOG_Z_INDEX = 1100

function withAppLayer(options: DialogOptions | string): DialogOptions | string {
  if (typeof options === 'string') return { title: options, zIndex: APP_DIALOG_Z_INDEX }
  return { ...options, zIndex: options.zIndex ?? APP_DIALOG_Z_INDEX }
}

export function useAppDialog(selector = ''): Dialog {
  const dialog = useDialog(selector)
  return {
    show: (options) => dialog.show(withAppLayer(options)),
    alert: (options) => dialog.alert(withAppLayer(options)),
    confirm: (options) => dialog.confirm(withAppLayer(options)),
    prompt: (options) => dialog.prompt(withAppLayer(options)),
    close: () => dialog.close()
  }
}
