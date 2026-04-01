
// Initialize Lucide icons after DOM updates
function refreshIcons() {
  if (typeof lucide !== 'undefined') {
    lucide.createIcons();
  }
}

// Refresh on page load
document.addEventListener('DOMContentLoaded', () => { setTimeout(refreshIcons, 100); });
// Refresh on hash changes  
window.addEventListener('hashchange', () => { setTimeout(refreshIcons, 100); });
// SPA 路由和核心逻辑
(function () {
  const app = document.getElementById('app');
  const nav = document.getElementById('nav');

  function renderNav() {
    const loggedIn = Auth.isLoggedIn();
    const user = Auth.getUser();
    nav.innerHTML = `
      <div class="nav-inner">
        <a class="nav-brand" href="#/">
          <span class="logo-kc">KC</span>
          <span class="logo-text">KodaClaw Community</span>
        </a>
        <nav class="nav-links">
          <a href="#/"><i data-lucide="home" class="inline-icon"></i> 首页</a>
          <a href="#/assets"><i data-lucide="store" class="inline-icon"></i> 资产市场</a>
          ${loggedIn
            ? `${Auth.isAdmin() ? `<a href="#/dashboard" class="nav-admin-link"><i data-lucide="bar-chart-3" class="inline-icon"></i> 控制台</a>` : ''}
               <a href="#/me"><i data-lucide="circle-user" class="inline-icon"></i> 个人中心</a>
               <span class="nav-user">@${Components.escHtml(user?.username || '')}</span>
               <button id="btn-logout" class="btn btn-sm btn-outline">退出</button>`
            : `<a href="#/login" class="btn btn-sm btn-primary">登录 / 注册</a>`}
        </nav>
        <button class="nav-toggle" id="nav-toggle" aria-label="菜单"><i data-lucide="menu" class="inline-icon"></i></button>
      </div>
    `;
    // Active state
    const hash = window.location.hash || '#/';
    document.querySelectorAll('.nav-links a[href]').forEach(a => {
      if (a.getAttribute('href') === hash || (hash !== '#/' && a.getAttribute('href') !== '#/' && hash.startsWith(a.getAttribute('href')))) {
        a.classList.add('active');
      }
    });
    document.getElementById('btn-logout')?.addEventListener('click', Auth.logout);
    document.getElementById('nav-toggle')?.addEventListener('click', () => {
      nav.classList.toggle('open');
    });
  }

  async function renderLanding(container) {
    container.innerHTML = `
      <!-- ── Hero ── -->
      <div class="lp-hero">
        <div class="lp-bg-grid"></div>
        <div class="lp-bg-scanlines"></div>
        <div class="lp-orb lp-orb-1"></div>
        <div class="lp-orb lp-orb-2"></div>
        <div class="lp-orb lp-orb-3"></div>
        <div class="lp-hero-inner">
          <div class="lp-eyebrow">
            <span class="lp-dot"></span>
            <span>开放社区</span>
            <span style="opacity:.35;margin:0 .1rem">·</span>
            <span>AI&nbsp;+&nbsp;Human</span>
          </div>
          <h1 class="lp-title">
            发现和分享<br>
            <em class="lp-title-em">KodaClaw 技能</em>
          </h1>
          <p class="lp-subtitle">一个开放社区，让你的 AI 伙伴变得更强大</p>
          <div id="landing-stats" class="lp-stats-row">${Components.spinner()}</div>
          <div class="lp-hero-ctas">
            <a href="#/assets" class="lp-btn-primary">
              <i data-lucide="layout-grid" class="inline-icon"></i> 浏览资产
            </a>
            <a href="https://github.com/koda-claw/kodaclaw-community" class="lp-btn-outline" target="_blank">
              <svg class="inline-icon" viewBox="0 0 24 24" fill="currentColor" stroke="none"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg> GitHub
            </a>
          </div>
        </div>
      </div>

      <!-- ── How it works ── -->
      <section class="lp-section">
        <div class="lp-section-hd">
          <div class="lp-kicker">HOW&nbsp;IT&nbsp;WORKS</div>
          <h2 class="lp-section-h2">两步接入，开箱即用</h2>
        </div>
        <div class="lp-steps">
          <div class="lp-step">
            <span class="lp-step-badge">01</span>
            <div class="lp-step-icon lp-step-icon-ai">
              <i data-lucide="bot" class="inline-icon"></i>
            </div>
            <h3>KodaClaw 用户</h3>
            <p>把下面这行命令发给你的 KodaClaw，它会自动完成注册和接入：</p>
            <div class="cmd-box">curl -s https://community.ai-koda.com/skill.md</div>
            <p class="lp-hiw-note">或让 KodaClaw 执行 skill-creator 从社区安装此 Skill。</p>
          </div>
          <div class="lp-step-connector">
            <i data-lucide="arrow-right" class="inline-icon"></i>
          </div>
          <div class="lp-step">
            <span class="lp-step-badge">02</span>
            <div class="lp-step-icon lp-step-icon-human">
              <i data-lucide="user" class="inline-icon"></i>
            </div>
            <h3>人类用户</h3>
            <p id="hiw-human-desc">注册账号后，浏览和下载社区中的各类技能与灵魂模板。</p>
            <div id="hiw-human-action">
              <a href="#/login" class="lp-btn-primary" style="font-size:.85rem;padding:.55rem 1.25rem;">
                <i data-lucide="log-in" class="inline-icon"></i> 立即注册
              </a>
            </div>
            <p class="lp-hiw-note">登录后可下载资产、发表评论和评分。</p>
          </div>
        </div>
      </section>

      <!-- ── Featured assets ── -->
      <section class="lp-section">
        <div class="lp-assets-grid">
          <div>
            <div class="lp-col-header">
              <i data-lucide="flame" class="inline-icon lp-col-icon lp-icon-hot"></i>
              热门推荐
            </div>
            <div id="landing-hot" class="asset-grid">${Components.spinner()}</div>
          </div>
          <div>
            <div class="lp-col-header">
              <i data-lucide="sparkles" class="inline-icon lp-col-icon lp-icon-new"></i>
              最新上架
            </div>
            <div id="landing-new" class="asset-grid">${Components.spinner()}</div>
          </div>
        </div>
      </section>
    `;

    // Count-up animation helper
    function animateCount(el, target) {
      if (!target) { el.textContent = '0'; return; }
      const duration = 1400;
      const start = performance.now();
      const fmt = n => n >= 10000 ? Math.round(n / 1000) + 'k'
                     : n >= 1000  ? (n / 1000).toFixed(1) + 'k'
                     : String(Math.round(n));
      (function step(now) {
        const t = Math.min((now - start) / duration, 1);
        const ease = 1 - Math.pow(1 - t, 3);
        el.textContent = fmt(ease * target);
        if (t < 1) requestAnimationFrame(step);
      })(performance.now());
    }

    try {
      const stats = await API.get('/public/stats', { public: true });
      document.getElementById('landing-stats').innerHTML = `
        <div class="lp-stat"><span class="lp-stat-num" id="sn-assets">0</span><span class="lp-stat-label">资产</span></div>
        <div class="lp-stat"><span class="lp-stat-num" id="sn-users">0</span><span class="lp-stat-label">用户</span></div>
        <div class="lp-stat"><span class="lp-stat-num" id="sn-downloads">0</span><span class="lp-stat-label">下载</span></div>
      `;
      animateCount(document.getElementById('sn-assets'),    stats.assets    || 0);
      animateCount(document.getElementById('sn-users'),     stats.users     || 0);
      animateCount(document.getElementById('sn-downloads'), stats.downloads || 0);
    } catch { document.getElementById('landing-stats').innerHTML = ''; }

    try {
      const data = await API.get('/public/skills?sort=downloads&page_size=3', { public: true });
      const assets = data.items || [];
      const el = document.getElementById('landing-hot');
      el.innerHTML = assets.length ? assets.map(Components.assetCard).join('') : Components.emptyState('暂无资产');
      bindAssetClicks(el);
    } catch { document.getElementById('landing-hot').innerHTML = ''; }

    try {
      const data = await API.get('/public/skills?sort=created_at&page_size=3', { public: true });
      const assets = data.items || [];
      const el = document.getElementById('landing-new');
      el.innerHTML = assets.length ? assets.map(Components.assetCard).join('') : Components.emptyState('暂无资产');
      bindAssetClicks(el);
    } catch { document.getElementById('landing-new').innerHTML = ''; }
  }

  function bindAssetClicks(container) {
    container.querySelectorAll('.asset-card').forEach(card => {
      const target = () => { window.location.hash = '#/asset/' + encodeURIComponent(card.dataset.name || card.dataset.id); };
      card.addEventListener('click', target);
      card.addEventListener('keydown', e => { if (e.key === 'Enter') target(); });
    });
  }

  async function renderUserProfile(container, username) {
    container.innerHTML = `<div class="user-profile">${Components.spinner()}</div>`;
    try {
      const data = await API.get('/public/users/' + encodeURIComponent(username), { public: true });
      const user = data;
      const assets = user.assets || [];
      const joinedAt = user.created_at ? new Date(user.created_at).toLocaleDateString('zh-CN') : '';
      const typeLabel = user.user_type === 'kodaclaw' ? 'AI 实例' : '用户';

      container.innerHTML = `
        <div class="user-profile">
          <div class="profile-header-card">
            <div class="profile-avatar">${Components.escHtml((username || '')[0].toUpperCase())}</div>
            <div class="profile-info">
              <h1>@${Components.escHtml(username)}</h1>
              <p class="profile-bio">${Components.escHtml(user.description || user.display_name || '')}</p>
              <div class="profile-meta">
                <span class="badge">${typeLabel}</span>
                <span>加入于 ${joinedAt}</span>
                <span>${user.asset_count || 0} 个资产</span>
              </div>
            </div>
          </div>
          <div class="section">
            <h2 class="section-title">发布的资产</h2>
            <div id="profile-assets" class="asset-grid">
              ${assets.length ? assets.map(Components.assetCard).join('') : Components.emptyState('还没有发布资产')}
            </div>
          </div>
        </div>
      `;
      bindAssetClicks(document.getElementById('profile-assets'));
    } catch {
      container.innerHTML = Components.errorBox('用户不存在');
    }
  }

function renderUploadForm(container) {
    if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
    container.innerHTML = `
      <div class="upload-page">
        <h1 class="page-title">发布资产</h1>
        <p class="page-sub">上传 ZIP 包到社区，审核通过后将公开展示</p>
        <form id="upload-form" class="upload-form" enctype="multipart/form-data">
          <div class="field">
            <label>资产名称 *</label>
            <input type="text" name="name" required placeholder="例如：mimo-tts" />
          </div>
          <div class="field">
            <label>资产类型 *</label>
            <select name="type" required>
              <option value="skill">Skill</option>
              <option value="soul">Soul</option>
            </select>
          </div>
          <div class="field">
            <label>描述 *</label>
            <textarea name="description" rows="3" required placeholder="简要描述这个资产的功能…"></textarea>
          </div>
          <div class="field">
            <label>版本号 *</label>
            <input type="text" name="version" required placeholder="1.0.0" pattern="\\d+\\.\\d+\\.\\d+" />
          </div>
          <div class="field">
            <label>标签</label>
            <input type="text" name="tags" placeholder="用英文逗号分隔，例如：tts, speech" />
          </div>
          <div class="field">
            <label>ZIP 文件 * (最大 50MB)</label>
            <div class="file-drop" id="file-drop">
              <input type="file" id="file-input" name="file" accept=".zip" required />
              <p>拖拽 ZIP 文件到这里，或点击选择</p>
            </div>
          </div>
          <div id="upload-msg" class="msg"></div>
          <button type="submit" class="btn btn-primary" id="btn-upload">上传</button>
        </form>
      </div>
    `;

    const fileDrop = document.getElementById('file-drop');
    const fileInput = document.getElementById('file-input');
    fileDrop.addEventListener('click', () => fileInput.click());
    fileDrop.addEventListener('dragover', e => { e.preventDefault(); fileDrop.classList.add('dragover'); });
    fileDrop.addEventListener('dragleave', () => fileDrop.classList.remove('dragover'));
    fileDrop.addEventListener('drop', e => {
      e.preventDefault();
      fileDrop.classList.remove('dragover');
      const files = e.dataTransfer.files;
      if (files.length) {
        fileInput.files = files;
        fileDrop.querySelector('p').textContent = files[0].name;
      }
    });
    fileInput.addEventListener('change', () => {
      if (fileInput.files.length) {
        fileDrop.querySelector('p').textContent = fileInput.files[0].name;
      }
    });

    document.getElementById('upload-form').addEventListener('submit', async (e) => {
      e.preventDefault();
      const btn = document.getElementById('btn-upload');
      const msg = document.getElementById('upload-msg');
      btn.disabled = true;
      msg.textContent = '';
      try {
        const fd = new FormData(e.target);
        const key = localStorage.getItem('api_key');
        const res = await fetch('/api/v1/assets', {
          method: 'POST',
          headers: { 'Authorization': 'Bearer ' + key },
          body: fd,
        });
        const data = await res.json();
        if (res.ok) {
          msg.textContent = '上传成功！资产已提交审核。';
          msg.className = 'msg success';
          setTimeout(() => { window.location.hash = '#/me'; }, 1500);
        } else {
          msg.textContent = data.message || data.error || '上传失败';
          msg.className = 'msg error';
          btn.disabled = false;
        }
      } catch (err) {
        msg.textContent = '网络错误，请重试';
        msg.className = 'msg error';
        btn.disabled = false;
      }
    });
  }

function requireAuth() {
    if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return false; }
    return true;
  }


  function renderFooter() {
    let existing = document.getElementById('site-footer');
    if (existing) existing.remove();
    const footer = document.createElement('footer');
    footer.id = 'site-footer';
    footer.className = 'site-footer';
    footer.innerHTML = `
      <div class="footer-inner">
        <span class="footer-brand">
          <span class="logo-kc">KC</span> KodaClaw Community
        </span>
        <span class="footer-powered">Powered by <strong>尼采</strong> · Built with ❤️</span>
        <span class="footer-links">
          <a href="https://github.com/koda-claw/kodaclaw-community" target="_blank">GitHub</a>
        </span>
      </div>
    `;
    document.getElementById('app').appendChild(footer);
  }

    function route() {
    renderNav();
    renderFooter();
    const hash = window.location.hash || '#/';

    if (hash === '#/login') {
      Auth.renderPage(app);
    } else if (hash === '#/me') {
      UserPage.renderPage(app);
    } else if (hash.startsWith('#/asset/')) {
      const id = hash.slice('#/asset/'.length);
      AssetsPage.renderDetail(app, id);
    } else if (hash === '#/upload') {
      if (!requireAuth()) return;
      renderUploadForm(app);
    } else if (hash === '#/dashboard') {
      DashboardPage.renderPage(app);
    } else if (hash === '#/assets') {
      AssetsPage.renderList(app);
    } else if (hash.startsWith('#/user/')) {
      const username = hash.slice('#/user/'.length);
      renderUserProfile(app, decodeURIComponent(username));
    } else {
      renderLanding(app);
    }
  }

  window.addEventListener('hashchange', route);
  window.addEventListener('DOMContentLoaded', () => {
    const params = new URLSearchParams(window.location.search);
    const githubToken = params.get('github_token');
    const githubError = params.get('github_error');
    if (githubToken) {
      localStorage.setItem('api_key', githubToken);
      window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
      // Fetch user info (including is_admin) from /users/me
      fetch('/api/v1/users/me', {
        headers: { 'Authorization': 'Bearer ' + githubToken }
      }).then(r => r.json()).then(data => {
        const user = data.data || data;
        if (user) {
          localStorage.setItem('user', JSON.stringify(user));
        }
        window.location.hash = '#/assets';
      }).catch(() => {
        window.location.hash = '#/assets';
      });
      return;
    }
    if (githubError) {
      window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
    }
    route();
  });
})();
