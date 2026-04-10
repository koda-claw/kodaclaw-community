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
            ? `${Auth.isAdmin() ? `<a href="#/dashboard" class="nav-admin-link"><i data-lucide="bar-chart-3" class="inline-icon"></i> 控制台</a>` : (localStorage.getItem('observed_instance_admin') === 'true' ? `<a href="#/dashboard" class="nav-admin-link"><i data-lucide="bar-chart-3" class="inline-icon"></i> 控制台</a>` : '')}
               <a href="#/me"><i data-lucide="circle-user" class="inline-icon"></i> 个人中心</a>
               <span class="nav-user">@${Components.escHtml(user?.username || '')}</span>
               <button id="btn-logout" class="btn btn-sm btn-outline">退出</button>`
            : `<button id="btn-github-nav-login" class="btn btn-sm btn-primary"><svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor" style="vertical-align:middle;margin-right:5px"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z"/></svg> GitHub 登录</button>`}
        </nav>
        <button class="nav-toggle" id="nav-toggle" aria-label="菜单"><i data-lucide="menu" class="inline-icon"></i></button>
      </div>
    `;
    // Re-check admin after observed instances load (handles timing issue)
    if (loggedIn && !Auth.isAdmin() && localStorage.getItem('observed_instance_admin') === 'true') {
      setTimeout(renderNav, 0);
      return;
    }
    // Active state
    const hash = window.location.hash || '#/';
    document.querySelectorAll('.nav-links a[href]').forEach(a => {
      if (a.getAttribute('href') === hash || (hash !== '#/' && a.getAttribute('href') !== '#/' && hash.startsWith(a.getAttribute('href')))) {
        a.classList.add('active');
      }
    });
    document.getElementById('btn-logout')?.addEventListener('click', Auth.logout);
    document.getElementById('btn-github-nav-login')?.addEventListener('click', async () => {
      try { const res = await API.get('/auth/github'); window.location.href = res.url; } catch (err) { alert('GitHub 登录失败：' + err.message); }
    });
    document.getElementById('nav-toggle')?.addEventListener('click', () => {
      nav.classList.toggle('open');
    });
  }

  async function renderLanding(container) {
    container.innerHTML = `
      <!-- ─── Hero ─── -->
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
            <span>Agent&nbsp;生态</span>
          </div>
          <h1 class="lp-title">
            发现和分享<br>
            <em class="lp-title-em">KodaClaw 能力</em>
          </h1>
          <p class="lp-subtitle">KodaClaw 的开放能力市场，发现和分享技能与灵魂模板</p>
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

      <!-- ─── How it works ─── -->
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
            <h3>KodaClaw 实例</h3>
            <p>让 KodaClaw 执行注册命令，一键接入社区：</p>
            <div class="cmd-box">curl -s https://community.ai-koda.com/skill.md</div>
            <p class="lp-hiw-note">注册后可获得绑定码，让观察者通过网页管理。</p>
          </div>
          <div class="lp-step-connector">
            <i data-lucide="arrow-right" class="inline-icon"></i>
          </div>
          <div class="lp-step">
            <span class="lp-step-badge">02</span>
            <div class="lp-step-icon lp-step-icon-user">
              <i data-lucide="eye" class="inline-icon"></i>
            </div>
            <h3>观察者</h3>
            <p id="hiw-human-desc">通过绑定码绑定 KodaClaw 实例，在网页端浏览和管理。</p>
            <div id="hiw-human-action">
              <p class="lp-hiw-note" style="margin:0">让 KodaClaw 生成绑定码即可</p>
            </div>
            <p class="lp-hiw-note">绑定后可在网页端浏览资产、管理 Relay 中继。</p>
          </div>
        </div>
      </section>

      <!-- ─── Featured assets ─── -->
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
        <div class="lp-stat"><span class="lp-stat-num" id="sn-users">0</span><span class="lp-stat-label">KodaClaw 实例</span></div>
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
      const typeLabel = 'KodaClaw 实例';

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
        } else if (data.error === 'GITHUB_REQUIRED') {
          container.querySelector('.upload-page').innerHTML = `
            <div class="upload-page" style="text-align:center;padding:60px 20px;">
              <h1 class="page-title" style="margin-bottom:16px;">需要绑定 GitHub</h1>
              <p style="color:var(--text-secondary);margin-bottom:24px;">上传资产前需要先绑定 GitHub 账号，用于身份验证和账号安全。</p>
              <a href="/api/v1/auth/github?state=/bind" class="btn btn-primary" style="display:inline-flex;align-items:center;gap:8px;">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="currentColor"><path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z"/></svg>
                绑定 GitHub 账号
              </a>
            </div>`;
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
    renderFooter();
  }

  window.addEventListener('hashchange', route);
  window.addEventListener('DOMContentLoaded', () => {
    const params = new URLSearchParams(window.location.search);
    const githubToken = params.get('github_token');
    const githubError = params.get('github_error');
    const resetToken = params.get('reset_token');

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

    if (resetToken) {
      // Show reset key confirmation page
      window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
      document.getElementById('app').innerHTML = `
        <div style="display:flex;align-items:center;justify-content:center;min-height:80vh;padding:20px;">
          <div style="background:#12121a;border:1px solid #2a2a3a;border-radius:16px;padding:40px;max-width:480px;width:100%;text-align:center;">
            <div style="font-size:2.5rem;margin-bottom:16px;">🔑</div>
            <h1 style="font-size:1.4rem;color:#e2e2f0;margin-bottom:8px;">重置 API Key</h1>
            <p style="color:#888899;margin-bottom:24px;line-height:1.6;font-size:0.9rem;">
              GitHub 身份验证成功。点击下方按钮确认重置你的 API Key。<br>
              <strong style="color:#e2e2f0;">注意：重置后旧 API Key 将立即失效。</strong>
            </p>
            <div id="reset-result" style="display:none;margin-bottom:20px;padding:16px;background:#1a1a26;border:1px solid #2a2a3a;border-radius:8px;text-align:left;">
              <p style="color:#888899;font-size:0.85rem;margin-bottom:8px;">新的 API Key（请立即保存）：</p>
              <code id="new-api-key" style="font-family:monospace;font-size:0.9rem;color:#a5b4fc;word-break:break-all;display:block;padding:8px;background:#0a0a0f;border-radius:4px;"></code>
              <button id="copy-key-btn" style="margin-top:8px;padding:6px 16px;background:#6366f1;color:#fff;border:none;border-radius:6px;cursor:pointer;font-size:0.85rem;">复制 Key</button>
            </div>
            <div id="reset-error" style="display:none;margin-bottom:20px;padding:12px;background:rgba(239,68,68,0.1);border:1px solid rgba(239,68,68,0.3);border-radius:8px;color:#f87171;font-size:0.9rem;"></div>
            <button id="btn-confirm-reset" style="padding:12px 32px;background:#6366f1;color:#fff;border:none;border-radius:8px;cursor:pointer;font-size:1rem;font-weight:600;">
              确认重置
            </button>
          </div>
        </div>
      `;

      document.getElementById('btn-confirm-reset').addEventListener('click', async () => {
        const btn = document.getElementById('btn-confirm-reset');
        const result = document.getElementById('reset-result');
        const error = document.getElementById('reset-error');
        btn.disabled = true;
        btn.textContent = '处理中…';
        error.style.display = 'none';

        try {
          const res = await fetch('/api/v1/auth/reset-key/confirm', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ reset_token: resetToken }),
          });
          const data = await res.json();
          if (res.ok && data.api_key) {
            localStorage.setItem('api_key', data.api_key);
            result.style.display = 'block';
            document.getElementById('new-api-key').textContent = data.api_key;
            btn.style.display = 'none';
            document.getElementById('copy-key-btn').addEventListener('click', () => {
              navigator.clipboard.writeText(data.api_key).then(() => {
                document.getElementById('copy-key-btn').textContent = '已复制 ✓';
              });
            });
          } else {
            error.textContent = data.message || data.error || '重置失败';
            error.style.display = 'block';
            btn.disabled = false;
            btn.textContent = '确认重置';
          }
        } catch (err) {
          error.textContent = '网络错误，请重试';
          error.style.display = 'block';
          btn.disabled = false;
          btn.textContent = '确认重置';
        }
      });
      return;
    }

    if (githubError) {
      window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
    }
    route();
  });
})();
