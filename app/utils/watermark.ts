/**
 * 拍照水印烧录（canvas API，App 端可用）。
 *
 * 用法：页面模板放一个屏外隐藏 canvas（canvas-id 与尺寸由数据驱动），
 * 拍完照后调 burnWatermark 得到烧录后的临时文件路径再展示/上传。
 *
 * 降级策略：任何一步失败（取图信息/绘制/导出）都 console.warn 并 resolve 原图路径，
 * 不阻塞打卡主链路（服务端 local 模式仍会兜底打水印，见 checkin_service.applyWatermarks）。
 */

/** 最长边压缩上限（px） */
const MAX_SIDE = 1280

/**
 * 烧录水印。
 * @param filePath  拍照得到的本地临时路径
 * @param lines     水印行（底部半透明黑条 + 白字，多行自下而上排版）
 * @param canvasId  页面中隐藏 canvas 的 canvas-id
 * @param component 页面组件实例（this；用于 canvas 上下文归属与 $nextTick）
 * @returns 烧录后的临时文件路径；失败时 resolve 原 filePath
 */
export function burnWatermark(
  filePath: string,
  lines: string[],
  canvasId: string,
  component: any
): Promise<string> {
  return new Promise<string>((resolve) => {
    const fallback = (why: string) => {
      console.warn('[watermark] 烧录失败，降级使用原图：' + why)
      resolve(filePath)
    }
    uni.getImageInfo({
      src: filePath,
      success: (info) => {
        // 等比缩放：最长边压到 MAX_SIDE
        const scale = Math.min(1, MAX_SIDE / Math.max(info.width, info.height))
        const cw = Math.max(1, Math.round(info.width * scale))
        const ch = Math.max(1, Math.round(info.height * scale))
        // canvas 尺寸由数据驱动：先写入组件数据，nextTick 后再画
        component.canvasW = cw
        component.canvasH = ch
        const draw = () => {
          try {
            const ctx = uni.createCanvasContext(canvasId, component)
            ctx.drawImage(filePath, 0, 0, cw, ch)
            const fs = Math.max(12, Math.round(cw / 28)) // 字号按宽度比例
            const lh = Math.round(fs * 1.4)
            const pad = Math.round(fs * 0.6)
            const barH = lines.length * lh + pad * 2
            // 底部半透明黑条
            ctx.setFillStyle('rgba(0, 0, 0, 0.5)')
            ctx.fillRect(0, ch - barH, cw, barH)
            // 白色多行文字
            ctx.setFillStyle('#FFFFFF')
            ctx.setFontSize(fs)
            lines.forEach((t, i) => {
              ctx.fillText(t, pad, ch - barH + pad + lh * i + fs)
            })
            ctx.draw(false, () => {
              uni.canvasToTempFilePath(
                {
                  canvasId: canvasId,
                  fileType: 'jpg',
                  quality: 0.8,
                  success: (r) => resolve(r.tempFilePath),
                  fail: () => fallback('canvasToTempFilePath 失败')
                },
                component
              )
            })
          } catch (e) {
            fallback('绘制异常')
          }
        }
        if (component != null && typeof component.$nextTick == 'function') {
          component.$nextTick(draw)
        } else {
          setTimeout(draw, 50)
        }
      },
      fail: () => fallback('getImageInfo 失败')
    })
  })
}
