# 后台系统原型预览

## HTML 静态原型（当前交付）

- 页面清单：`07-页面清单.md`，共 **43 页**，包含 `91d 工单设置` 与 `79g 配额`。
- 可编辑正文：`../fragments/*.html`；菜单与面包屑：`../fragments/meta.json`。
- 完整静态 HTML：`../src/*.html`；逐屏 PNG：`html/*.png`。
- 画廊入口：`../index.html`。
- 视口：**1440×900**；PNG 为 2 倍图（2880×1800）。
- 视觉：沿用旧版 `docs/品管家/后台原型/` 的深色侧栏、白色顶栏与 Element Plus 表格语言，已清理设计批注、低对比度小字和旧业务口径。
- 本交付是**静态 UI 原型**，不证明接口、权限或交互功能已经实现。

## 渲染与校验

在项目根目录执行：

```bash
# 全量渲染 43 页
python "docs/AI云管家/后台系统原型/render.py"

# 单页渲染（编号或完整图名均可）
python "docs/AI云管家/后台系统原型/render.py" 78
python "docs/AI云管家/后台系统原型/render.py" 91d

# 数量、尺寸、链接、横向溢出与可见旧文案校验
python "docs/AI云管家/后台系统原型/verify.py"
```

渲染器复用旧原型的菜单外壳，共享精修样式位于 `assets/refine.css`。旧目录与 App 原型不被修改。

## 旧版 Penpot 预览

`v2/` 是此前 Penpot 试验链路留下的少量图片，不是当前交付，也不参与 43 页计数。当前交付以 `html/` 与 `../src/` 为准。
