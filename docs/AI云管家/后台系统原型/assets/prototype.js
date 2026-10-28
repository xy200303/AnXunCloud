const annotationPattern = /现有实现|现有能力保留|本期新增|本期扩展|本期必改|本链改造要点|为什么不换成|新增筛选：|整页为左树右表|平台预置链在开通|六个类别对应|13 §|14 §|03 §|08 §|flow_service\.go|「环节分工」Tab 已取消|供应商与模型在此注册一次/;
const redundantTexts = new Set([
  '只管「怎么查、合格标准是啥」；点「检查项」进入条目列表',
  '交给甲方的册子怎么排版',
  '这里只建基本信息；版式块、表头分组、照片墙在创建后进入 layout 编辑器配置。无 LibreOffice 环境只交 docx，界面明示 PDF 服务未启用。',
  '承接查验「查什么」：按册/专项维护检查项与清单行；新建批次时按此带出',
  '室外场地 · 8 个专项（与母版一致），每专项配记录表 + 照片表',
  '清单行带稳定 code，改名不换 code，动态增行由系统分配；矩阵检查项复用 purpose=verify 的检查标准。问题只进报告，不在这里配置责任方与整改期限。',
  '选择行业预置包，一键导入标准点位类型、检查项模板、台账字段、审批流程和输出模板；管理员只改差异。',
  '只列与预置包不同的项：点位类型别名、设备编码规则、审核人、报告样式。冲突必须预览确认，不静默覆盖。',
  '目标：一个新项目从开通到试点可跑，按天计，不按周计。',
  '项目经理只为自己管辖小区建批次；租户管理员可看本租户全部小区并建批次。',
  '创建后自动带出对应类别结构（机构更迭六类别 / 分户六类别）；新建物业承接查验母版待甲方提供后开放。创建编辑类操作一律弹窗，不跳页。',
  '「编辑」配基本信息与岗位；「小区配置」侧滑配任职小区。未配任职小区 = 无数据范围，登录后看不到任何小区。',
  '岗位在「编辑」里改；本抽屉只配他在哪些小区生效',
  '岗位决定能干什么',
  '任职小区决定在哪些小区生效',
  '欢迎使用 AI云管家物业巡检管理平台 · 管辖 3 个小区',
  '点击下钻任务监控',
  '>0 时卡片标红并可下钻',
  '选中预设自动回填 API 协议与 Base URL，并带出该服务商的常用模型；仍可逐项手改。',
  '保存后只显掩码、不回显明文；明文走 env/KMS 引用（09 §4.2）',
  'MCP 是把本系统的能力暴露给外部 AI 客户端，模型在对方那边，本系统只管 token 范围与审计（12 §3）。',
  '工具注册表在代码里、不落库',
  '默认值须按现有 8 worker / 180s 基线实测后决定',
  '全局资源与厂商配额上限在平台层设',
  '超出即排队，不丢任务',
  '独立部署时一般留空（资源是客户自己的机器）；共享部署按合同填。',
  'token 以该用户身份调用，权限判定走与接口相同的链路',
  '签发时固化为该用户当时范围的子集',
  '勾选写工具时，外部调用仍只返回“待确认”句柄',
  '读写分档；全局配额与内部 AI 共用公平性约束',
  '仅在签发成功后显示一次',
  '未配手写签名时弹签名板现场手写，可保存下次使用',
  '归档件不可改，纠错走作废 + 重出新报告',
  '缺口一眼可见，不用逐月翻',
  '内置岗位全平台共享，仅超管可维护；复制为租户自有后可改。',
  '范围（看哪些小区）由人员任职决定，不在本页配——原「数据范围：全部数据/所在项目/仅本人」三档的范围语义已移除。',
  '父子联动勾选，按钮级为叶子节点',
  '保存后，被影响的 3 名用户将在下次刷新或重新登录时生效；收窄权限属高危变更，记入审计',
  '送审前可改；送审后改配置不影响本批次，须重开批次。',
  '两项全关会让巡检员无路可走——至少保留一项，保存时校验。',
  '可配，但第三项不可开',
  '租户默认与工单类型级覆盖',
  '保存后仅影响之后提交的工单，在途工单保持原设置。',
  '按风险排序：安全相关 → 逾期 → AI 异常 → 人工改判 → 无法检查',
  '到期状态六态：正常 / 临期 / 已逾期 / 无到期日 / 应报废 / 标签缺失',
  '全局仅保留一条生效公章，上传后原公章自动转「已替换」',
  '保存后该租户的品牌展示立即生效',
  '拖动左侧手柄调整 App 端板块排序；停用板块后已在架内容对业主不可见，不删除。',
  '本卡为摘要，处理在「内容审核队列」Tab',
  '设备类型 + 指标 + 阈值 + 动作',
  'AI 分组整组移出 → 「AI 配置」菜单',
  '每个平台只生效最新上传的一条：官网下载页展示它，App 启动检查更新也对齐它（强制更新不可跳过）。',
  '弱更新：App 弹窗可「以后再说」；强制更新：弹窗不可跳过，必须更新后才能使用',
  '「+」上传附件；可多行输入，Ctrl+Enter 发送；写操作确认后执行'
]);
const simplifiedTexts = new Map([
  ['批次 2026-03 · 负责人 李敏 · 建批次时已快照当前内容/签署/审核链配置', '批次 2026-03 · 负责人 李敏'],
  ['全景照片（水印缩略图，点击查看原图）', '全景照片'],
  ['严勇 打包 3 项事项建单并派给 老周', '系统 审核通过并自动建单']
]);
document.querySelectorAll('.content2 *').forEach((element) => {
  if (element.children.length) return;
  const text = element.textContent.trim();
  if (redundantTexts.has(text)) element.remove();
  else if (simplifiedTexts.has(text)) element.textContent = simplifiedTexts.get(text);
});

document.querySelectorAll('.content2 div').forEach((element) => {
  const text = element.textContent.trim();
  const isAssistant = location.pathname.includes('64-PC助手');
  const isLongAnnotation = text.length > 100 && !element.closest('table, .ncard') && !isAssistant;
  if (!element.querySelector('div,table,input,textarea,select') && (annotationPattern.test(text) || isLongAnnotation)) element.remove();
});
document.querySelectorAll('.content2 span').forEach((element) => {
  const text = element.textContent.trim();
  if (!element.querySelector('span') && text.length > 140 && !element.closest('table, .ncard') && !location.pathname.includes('64-PC助手')) element.remove();
});
document.querySelectorAll('.content2 .etag, .content2 .res').forEach((element) => {
  if (annotationPattern.test(element.textContent) && (element.classList.contains('etag') || element.querySelector('.res-h'))) element.remove();
});
document.querySelectorAll('.content2 span').forEach((element) => {
  if (/^(本期新增|期2 新增)$/.test(element.textContent.trim())) element.remove();
});
document.querySelectorAll('.content2 span[style*="display:inline-block"]').forEach((element) => {
  const style = element.getAttribute('style') || '';
  if (/width:(38|72|88|112)px/.test(style) && /height:(38|72|88|112)px/.test(style)) element.classList.add('prototype-photo');
});

const planEditor = document.getElementById('plan-editor');
const providerModels = document.getElementById('provider-models');
const syncPlanEditor = () => {
  if (planEditor) planEditor.style.display = location.hash === '#plan-editor' ? 'block' : 'none';
};
const syncProviderModels = () => {
  if (providerModels) providerModels.style.display = location.hash === '#provider-models' ? 'block' : 'none';
};
const syncOverlays = () => {
  syncPlanEditor();
  syncProviderModels();
};
syncOverlays();
window.addEventListener('hashchange', syncOverlays);

document.addEventListener('click', (event) => {
  const close = event.target.closest('a[href="#"]');
  if (close && (planEditor || providerModels)) {
    event.preventDefault();
    history.replaceState(null, '', location.pathname);
    syncOverlays();
    return;
  }
  const target = event.target.closest('[data-to], .si, .mi2, .tg');
  if (!target || target.classList.contains('x')) return;
  if (target.dataset.to) return;
  const text = target.textContent.replace(/[×▾▸]/g, '').trim();
  const routes = {
    '运营看板': '78-运营看板.html', '小区列表': '74-小区列表.html', '小区点位': '75-小区点位.html',
    '计划管理': '94-计划任务.html', '计划任务': '94-计划任务.html', '任务监控': '95-任务监控.html', '巡检记录': '96-巡检记录.html',
    '设备台账': '97-设备台账.html', '工单池': '91-工单池.html', '批次管理': '72-承接批次管理.html',
    '检查项模板': '40-模板中心-模板列表.html', '输出模板': '42-输出模板.html', '承接查验内容模板': '70-承接查验内容配置.html',
    '报告生成': '80-报告中心.html', '对话': '64-PC助手.html', '助手设置': '79f-AI配置-MCP接入.html',
    '用户管理': '76-用户管理-小区配置.html', '岗位': '81-岗位角色定义.html', '审批流程': '82-审批流程配置.html',
    'AI 配置': '79-AI配置-模型供应商.html', '签章管理': '98-签章管理.html', '日志管理': '99-日志管理.html',
    '通知公告': '100-通知公告.html', '企业品牌': '101-企业品牌.html', '租户管理': '104-租户管理.html',
    '菜单管理': '105-菜单管理.html', '字典': '106-字典.html', '系统配置': '107-系统配置.html', '发布管理': '108-发布管理.html',
    '配额': '79g-AI配置-配额.html', '用量': '79e-AI配置-配额与用量.html'
  };
  if (routes[text]) window.location.href = routes[text];
});
