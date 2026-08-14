// 安巡云官网：拉取后台「品牌官网」配置并应用（主题色/标语/联系方式/页脚文案）。
// 页面中带 data-site="slogan|contact|footer-note" 的元素会被填充；接口失败时保持 HTML 默认值。
(function () {
  function darken(hex, f) {
    var n = parseInt(hex.slice(1), 16);
    var r = Math.round(((n >> 16) & 255) * f), g = Math.round(((n >> 8) & 255) * f), b = Math.round((n & 255) * f);
    return '#' + ((1 << 24) + (r << 16) + (g << 8) + b).toString(16).slice(1);
  }
  function fill(name, text) {
    document.querySelectorAll('[data-site="' + name + '"]').forEach(function (el) {
      el.textContent = text;
    });
  }
  fetch('/api/public/site-config', { headers: { Accept: 'application/json' } })
    .then(function (r) { return r.json(); })
    .then(function (res) {
      var cfg = (res && res.data) || {};
      if (cfg.theme_color && /^#[0-9a-fA-F]{6}$/.test(cfg.theme_color)) {
        var root = document.documentElement.style;
        root.setProperty('--blue', cfg.theme_color);
        root.setProperty('--blue-dark', darken(cfg.theme_color, 0.8));
      }
      if (cfg.slogan) fill('slogan', cfg.slogan);
      if (cfg.footer_note) fill('footer-note', cfg.footer_note);
      if (cfg.show_admin_entry === 'true') {
        document.querySelectorAll('[data-admin-entry]').forEach(function (el) {
          el.removeAttribute('hidden');
        });
      }
      var contact = [];
      if (cfg.contact_phone) contact.push('电话 ' + cfg.contact_phone);
      if (cfg.contact_email) contact.push('邮箱 ' + cfg.contact_email);
      if (contact.length) fill('contact', contact.join(' · '));
    })
    .catch(function () { /* 保持默认 */ });
})();
