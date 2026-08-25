/**
 * 极简巡检结果语音播报（连续打卡页用）。
 *
 * 播放 /static/audio/{type}.mp3（InnerAudioContext，四端一致）；
 * mp3 缺失或解码失败时自动降级 .wav 再试一次；
 * 仍失败则静默跳过（音频只是辅助提醒，不阻断打卡主链路）。
 *
 * 音频文件约定：
 * - normal.mp3   正常（绿）
 * - abnormal.mp3 发现异常（红）
 * - blurry.mp3   照片不合格，请重拍（红）
 * - review.mp3   已拍照，等管理员确认（黄）
 */

export type VoiceType = 'normal' | 'abnormal' | 'blurry' | 'review'

let ctx: UniApp.InnerAudioContext | null = null

function play(path: string, fallback: string | null) {
  if (ctx != null) {
    ctx.destroy()
    ctx = null
  }
  const c = uni.createInnerAudioContext()
  ctx = c
  c.src = path
  c.onError(() => {
    if (fallback != null) {
      play(fallback, null)
    }
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

/** 播放结果语音；失败静默跳过 */
export function playVoice(type: VoiceType): void {
  play('/static/audio/' + type + '.mp3', '/static/audio/' + type + '.wav')
}
