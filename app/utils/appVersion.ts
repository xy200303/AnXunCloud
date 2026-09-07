import appPackage from '../package.json'

/** 应用版本唯一来源：app/package.json。原生安装包版本由构建前脚本同步到 manifest.json。 */
export const APP_VERSION = String(appPackage.version || '0.0.0')
