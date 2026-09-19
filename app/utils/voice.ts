/**
 * 极简巡检结果语音播报（连续打卡页用）。
 *
 * 播放 /static/audio/{type}.m4a（AAC，InnerAudioContext，四端一致）；
 * 失败静默跳过（音频只是辅助提醒，不阻断打卡主链路）。
 *
 * 音频文件约定：
 * - normal.m4a   正常（绿）
 * - abnormal.m4a 发现异常（红）
 * - blurry.m4a   照片不合格，请重拍（红）
 * - review.m4a   已拍照，等管理员确认（黄）
 */

export type VoiceType = 'normal' | 'abnormal' | 'blurry' | 'review'

let ctx: UniApp.InnerAudioContext | null = null

/** 播放结果语音；失败静默跳过 */
export function playVoice(type: VoiceType): void {
  if (ctx != null) {
    ctx.destroy()
    ctx = null
  }
  const c = uni.createInnerAudioContext()
  ctx = c
  c.src = '/static/audio/' + type + '.m4a'
  c.onError(() => {
    // 缺失/解码失败静默跳过
  })
  c.onEnded(() => {
    if (ctx == c) {
      c.destroy()
      ctx = null
    }
  })
  try {
    c.play()
  } catch (e) {
    // 播放抛错（如端不支持）静默跳过
  }
}
