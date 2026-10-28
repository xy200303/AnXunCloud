import json
import re
import struct
from pathlib import Path

BASE = Path(__file__).resolve().parent
FRAGMENTS = BASE / 'fragments'
SRC = BASE / 'src'
OUT = BASE / '预览' / 'html'
CHROME = r'C:\Program Files\Google\Chrome\Application\chrome.exe'
FORBIDDEN = re.compile(r'本期新增|本期扩展|本期必改|现有实现|现有能力保留|待打包|用途（kind）|第 [1-6] 册|flow_service\.go|duty_binding|kind=report|自动跳过|粘贴|复查')


def main():
    from playwright.sync_api import sync_playwright
    meta = json.loads((FRAGMENTS / 'meta.json').read_text(encoding='utf-8'))
    names = sorted(meta)
    errors = []
    warnings = []
    for name in names:
        if not (FRAGMENTS / f'{name}.html').exists():
            errors.append(f'{name}: 缺少片段')
        if not (SRC / f'{name}.html').exists():
            errors.append(f'{name}: 缺少 HTML')
        png = OUT / f'{name}.png'
        if not png.exists():
            errors.append(f'{name}: 缺少 PNG')
            continue
        data = png.read_bytes()
        if len(data) < 10000:
            errors.append(f'{name}: PNG 异常小图')
        if struct.unpack('>II', data[16:24]) != (2880, 1800):
            errors.append(f'{name}: PNG 尺寸错误')
    gallery = BASE / 'index.html'
    if not gallery.exists():
        errors.append('缺少预览画廊')
    else:
        for ref in re.findall(r'(?:src|href)="([^"]+)"', gallery.read_text(encoding='utf-8')):
            if not ref.startswith(('http', '#')) and not (BASE / ref).exists():
                errors.append(f'画廊无效引用 {ref}')
    with sync_playwright() as p:
        browser = p.chromium.launch(executable_path=CHROME, headless=True)
        for name in names:
            src = SRC / f'{name}.html'
            if not src.exists():
                continue
            page = browser.new_page(viewport={'width': 1440, 'height': 900})
            page.goto(src.as_uri(), wait_until='load')
            page.evaluate('document.fonts.ready')
            if page.locator('.page-head p').count():
                errors.append(f'{name}: 存在冗余页头副标题')
            if name == '70-承接查验内容配置' and page.locator('table tr').count() < 8:
                errors.append(f'{name}: 专项表格内容缺失')
            visible = page.locator('body').inner_text()
            for line in visible.splitlines():
                if FORBIDDEN.search(line):
                    errors.append(f'{name}: 可见旧文案 {line.strip()[:120]}')
            for href in page.locator('a[href]').evaluate_all('(nodes) => nodes.map(n => n.getAttribute("href"))'):
                if href.endswith('.html') and not (SRC / href).exists():
                    errors.append(f'{name}: 无效链接 {href}')
            overflow = page.evaluate('''() => {
              const content = document.querySelector('.content2');
              return document.documentElement.scrollWidth > 1440 || (content && content.scrollWidth > content.clientWidth + 1);
            }''')
            if overflow:
                warnings.append(f'{name}: 内容区存在横向滚动')
            page.close()
        browser.close()
    result = {'expected': len(meta), 'fragments': len(list(FRAGMENTS.glob('*.html'))), 'html': len(list(SRC.glob('*.html'))), 'png': len(list(OUT.glob('*.png'))), 'errors': errors, 'warnings': warnings}
    print(json.dumps(result, ensure_ascii=False, indent=2))
    if errors or result['fragments'] != len(meta) or result['html'] != len(meta) or result['png'] != len(meta):
        raise SystemExit(1)


if __name__ == '__main__':
    main()
