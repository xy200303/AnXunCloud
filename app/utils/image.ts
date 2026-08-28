/**
 * 打卡照片上传前压缩。
 *
 * 档位依据：视觉大模型编码器上限约 1344×1344（高分也是切块），压到长边 1920px
 * 仍在模型编码分辨率之上，不损失 AI 可用信息；1920px 下压力表/铅封/标签/仪表读数
 * 均可辨（1080px 以下才开始伤读数类项）。JPEG 质量 80 避免细文字块状伪影。
 * 实测读数类项误判上升时把 MAX_EDGE 调到 2560。
 *
 * 代价说明：重编码会剥离 EXIF（拍摄时间偏差判定自动跳过）；防旧照片由
 * 禁相册选图 + 服务端打卡时间 + 服务端水印三重兜底。
 */

/** 压缩长边像素（AI 安全下限 1920；需要更高细节时调 2560） */
export const PHOTO_MAX_EDGE = 1920
/** JPEG 压缩质量（1-100；80 为清晰度/体积平衡点，勿低于 70） */
export const PHOTO_QUALITY = 80

/**
 * 压缩照片用于上传：成功返回压缩后路径，失败/不支持返回原路径（不阻断拍照主链路）。
 * chooseImage 已带 sizeType:['compressed'] 系统级压缩，这里再做一次定标压缩保证体积稳定。
 */
export function compressForUpload(filePath: string): Promise<string> {
  return new Promise((resolve) => {
    // #ifdef H5 || APP-PLUS || MP-WEIXIN
    uni.compressImage({
      src: filePath,
      quality: PHOTO_QUALITY,
      width: PHOTO_MAX_EDGE + 'px',
      success: (res) => {
        resolve(res.tempFilePath != '' ? res.tempFilePath : filePath)
      },
      fail: () => resolve(filePath)
    })
    // #endif
    // #ifndef H5 || APP-PLUS || MP-WEIXIN
    resolve(filePath)
    // #endif
  })
}
