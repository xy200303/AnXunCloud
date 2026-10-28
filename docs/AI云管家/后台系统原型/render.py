import importlib.util
import json
import re
import struct
import sys
from pathlib import Path

BASE = Path(__file__).resolve().parent
ROOT = BASE.parents[2]
FRAGMENTS = BASE / 'fragments'
SRC = BASE / 'src'
OUT = BASE / '预览' / 'html'
RENDERER = ROOT / 'backend' / '.tmp' / 'render_protos.py'
CHROME = r'C:\Program Files\Google\Chrome\Application\chrome.exe'

FLOW_LINKS = {
    '74-小区列表': {'编制': '77-小区设置-人员与职责.html', '继续向导': '59-管理员初始化导入.html'},
    '77-小区设置-人员与职责': {'流程与签字': '83-小区设置-流程与签字.html'},
    '94-计划任务': {'编辑': '#plan-editor'},
    '91-工单池': {'详情': '92-工单详情与处理.html'},
    '72-承接批次管理': {'详情': '84-承接查验-批次详情.html'},
    '84-承接查验-批次详情': {'报告与签署': '84b-承接查验-报告与签署.html'},
    '40-模板中心-模板列表': {'编辑': '41-模板编辑器.html', '新建检查项模板': '53-新建检查项模板.html', '批量录入': '43-批量录入.html'},
    '53-新建检查项模板': {'保存并录入条目': '41-模板编辑器.html'},
    '80c-报告中心-归档台账': {'查看': '80-报告中心.html'},
}


def load_legacy_renderer():
    spec = importlib.util.spec_from_file_location('legacy_proto_renderer', RENDERER)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def page_key(name):
    return name.split('-', 1)[0]


def refine_body(name, body):
    body = re.sub(r'<div[^>]*>\s*<b>现有实现.*?</div>', '', body, flags=re.S)
    body = re.sub(r'<div[^>]*>\s*<b[^>]*>AI 存疑 / 识别失败.*?</div>', '', body, flags=re.S)
    cleaned = []
    for line in body.splitlines():
        if '<div' in line and any(token in line for token in ('现有实现', '本期必改', '期1 必须', '期2 新增', 'flow_service.go')):
            line = line.replace('<div ', '<div class="prototype-note" ', 1)
        cleaned.append(line)
    body = '\n'.join(cleaned)
    for text, href in FLOW_LINKS.get(name, {}).items():
        body = re.sub(r'<span class="elink"([^>]*)>' + re.escape(text) + r'</span>',
                      f'<a class="elink" href="{href}"\\1>{text}</a>', body)
    if name == '80-报告中心':
        body = body.replace('<span class="ebtn">＋ 生成报告</span>', '<a class="ebtn" href="80b-报告中心-生成报告.html">＋ 生成报告</a>')
    if name == '74-小区列表':
        body = body.replace('<span class="ebtn">＋ 新增小区</span>', '<a class="ebtn" href="59-管理员初始化导入.html">＋ 新增小区</a>')
    if name == '94-计划任务':
        body = body.replace('<span class="ebtn">＋ 新增计划</span>', '<a class="ebtn" href="#plan-editor">＋ 新增计划</a>')
    if name == '78-运营看板':
        body = body.replace('>进行中任务</span>', '><a href="95-任务监控.html">进行中任务</a></span>')
        body = body.replace('>逾期任务</span>', '><a href="95-任务监控.html">逾期任务</a></span>')
        body = body.replace('>去配置</span>', '><a href="83-小区设置-流程与签字.html">去配置</a></span>')
        body = body.replace('>去签字</span>', '><a href="80-报告中心.html">去签字</a></span>')
    if name == '79-AI配置-模型供应商':
        body = body.replace('<span class="ebtn">＋ 添加供应商</span>', '<a class="ebtn" href="79b-AI配置-添加供应商.html">＋ 添加供应商</a>')
        body = body.replace('<span style="padding:12px 0">能力绑定</span>', '<a href="79c-AI配置-能力绑定.html" style="padding:12px 0">能力绑定</a>')
    if name == '79b-AI配置-添加供应商':
        body = body.replace('<span class="ebtn-w">取消</span>', '<a class="ebtn-w" href="79-AI配置-模型供应商.html">取消</a>')
        body = body.replace('<span class="ebtn">保存</span>', '<a class="ebtn" href="79-AI配置-模型供应商.html">保存供应商</a>')
    if name == '84-承接查验-批次详情':
        body = body.replace('<span style="padding:12px 0">报告与签署</span>', '<a href="84b-承接查验-报告与签署.html" style="padding:12px 0">报告与签署</a>')
    return body


def build_html(renderer, name, meta):
    body = refine_body(name, (FRAGMENTS / f'{name}.html').read_text(encoding='utf-8'))
    html = renderer.render_pc_page(name, meta, body)
    for label in ('计划任务', '巡检规则', '复查', '归档台账', '社区运营', '设备接入', '能力中心'):
        html = re.sub(r'<div class="si(?: on)?">' + re.escape(label) + r'</div>', '', html)
    if name == '94-计划任务':
        plan_nav = f'<div class="mi2 open">{renderer.pc_ico("clip")}计划管理<span class="arr">▾</span></div><div class="sub"><div class="si on">计划任务</div></div>'
        html = re.sub(r'<div class="mi2(?: open)?">(?:(?!</div>).)*巡检管理<span class="arr">', plan_nav + r'\g<0>', html, count=1)
    html = html.replace('>Skills 管理</div>', '>助手设置</div>')
    heading = f'''<div class="page-head"><div><h1>{meta['title']}</h1></div></div>'''
    if name == '64-PC助手':
        html = html.replace('<div class="content2">', '<div class="content2 assistant-content">', 1)
    else:
        html = html.replace('<div class="content2">', f'<div class="content2">{heading}', 1)
    html = html.replace('</head>', '<link rel="stylesheet" href="../assets/refine.css"></head>', 1)
    html = html.replace('</body>', '<script src="../assets/prototype.js"></script></body>', 1)
    return html


def write_gallery(meta):
    cards = []
    for name in sorted(meta):
        title = meta[name]['title']
        cards.append(f'<section><h2>{name} · {title}</h2><a href="预览/html/{name}.png" target="_blank"><img src="预览/html/{name}.png" alt="{title}" loading="lazy"></a><p><a href="src/{name}.html">打开静态原型</a></p></section>')
    html = '''<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>AI云管家 · PC 后台原型</title><style>body{margin:0;background:#F5F6F8;color:#1F2329;font:14px -apple-system,"Microsoft YaHei",sans-serif}header{padding:24px 32px;background:#fff;border-bottom:1px solid #E5E6EB}h1{margin:0 0 8px;font-size:24px}header p{margin:0;color:#6B7385}.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(640px,1fr));gap:24px;padding:24px}section{background:#fff;border:1px solid #E5E6EB;border-radius:6px;overflow:hidden}h2{font-size:16px;margin:0;padding:16px;border-bottom:1px solid #E5E6EB}img{display:block;width:100%;height:auto}a{color:#2B5AED;text-decoration:none}section p{padding:0 16px 16px;margin:0}</style></head><body><header><h1>AI云管家 · PC 后台原型</h1><p>''' + str(len(meta)) + ''' 页 · 1440×900 · Element Plus 风格 · 静态 UI 评审</p></header><main class="grid">''' + ''.join(cards) + '</main></body></html>'
    (BASE / 'index.html').write_text(html, encoding='utf-8')


def main():
    from playwright.sync_api import sync_playwright
    meta = json.loads((FRAGMENTS / 'meta.json').read_text(encoding='utf-8'))
    fragments = {p.stem for p in FRAGMENTS.glob('*.html')}
    if set(meta) != fragments:
        raise ValueError(f'片段与元数据不一致: {set(meta) ^ fragments}')
    requested = set(sys.argv[1:])
    names = [name for name in sorted(meta) if not requested or name in requested or page_key(name) in requested]
    if requested and not names:
        raise ValueError('没有匹配页面')
    unknown = requested - set(names) - {page_key(n) for n in names}
    if unknown:
        raise ValueError(f'未知页面: {sorted(unknown)}')
    renderer = load_legacy_renderer()
    SRC.mkdir(exist_ok=True)
    OUT.mkdir(parents=True, exist_ok=True)
    failures = []
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=CHROME, headless=True)
        for name in names:
            html = build_html(renderer, name, meta[name])
            src = SRC / f'{name}.html'
            src.write_text(html, encoding='utf-8')
            page = browser.new_page(viewport={'width': 1440, 'height': 900}, device_scale_factor=2)
            errors = []
            page.on('pageerror', lambda error: errors.append(str(error)))
            page.goto(src.as_uri(), wait_until='load')
            page.evaluate('document.fonts.ready')
            for href in page.locator('a[href]').evaluate_all('(nodes) => nodes.map(n => n.getAttribute("href"))'):
                if href.endswith('.html') and Path(href).stem not in meta:
                    failures.append(f'{name}: 无效链接 {href}')
            overflow = page.evaluate('document.documentElement.scrollWidth > 1440 || document.body.scrollWidth > 1440')
            if overflow:
                failures.append(f'{name}: 页面横向溢出')
            png = OUT / f'{name}.png'
            page.screenshot(path=str(png), full_page=False, animations='disabled')
            data = png.read_bytes()
            width, height = struct.unpack('>II', data[16:24])
            if len(data) < 10000 or (width, height) != (2880, 1800):
                failures.append(f'{name}: 无效截图 {width}x{height}')
            if errors:
                failures.append(f'{name}: JS {errors}')
            print(f'{name} OK {width}x{height} {len(data)} bytes', flush=True)
            page.close()
        browser.close()
    write_gallery(meta)
    print(json.dumps({'expected': len(meta), 'rendered': len(names), 'failures': failures}, ensure_ascii=False, indent=2))
    if failures:
        raise SystemExit(1)


if __name__ == '__main__':
    main()
