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
          <a href="#/">首页</a>
          <a href="#/assets">资产市场</a>
          ${loggedIn
            ? `<a href="#/me">个人中心</a>
               <span class="nav-user">@${Components.escHtml(user?.username || '')}</span>
               <button id="btn-logout" class="btn btn-sm btn-outline">退出</button>`
            : `<a href="#/login" class="btn btn-sm btn-primary">登录 / 注册</a>`}
        </nav>
        <button class="nav-toggle" id="nav-toggle" aria-label="菜单">&#x2630;</button>
      </div>
    `;
    document.getElementById('btn-logout')?.addEventListener('click', Auth.logout);
    document.getElementById('nav-toggle')?.addEventListener('click', () => {
      nav.classList.toggle('open');
    });
  }

  async function renderLanding(container) {
    container.innerHTML = `
      <div class="hero">
        <div class="hero-inner">
          <h1 class="hero-title">发现和分享 KodaClaw 技能与灵魂模板</h1>
          <p class="hero-sub">一个开放的社区，让你的 AI 伙伴变得更强大</p>
          <div class="hero-actions">
            <a href="#/assets" class="btn btn-primary btn-lg">浏览资产</a>
            <a href="https://github.com/koda-claw/kodaclaw-community" class="btn btn-outline btn-lg" target="_blank">GitHub</a>
          </div>
          <div id="landing-stats" class="hero-stats">${Components.spinner()}</div>
        </div>
      </div>
      <div class="landing-sections">
        <div class="landing-section">
          <div class="section-inner">
            <h2 class="section-title">&#x1F525; 热门推荐</h2>
            <div id="landing-hot" class="asset-grid">${Components.spinner()}</div>
          </div>
        </div>
        <div class="landing-section">
          <div class="section-inner">
            <h2 class="section-title">&#x2728; 最新上架</h2>
            <div id="landing-new" class="asset-grid">${Components.spinner()}</div>
          </div>
        </div>
        <div class="landing-section landing-cta">
          <div class="section-inner">
            <h2>加入我们</h2>
            <p>安装 KodaClaw 后，访问社区一键发现和安装技能与灵魂模板。</p>
            <div class="cmd-box" style="text-align:center;">curl https://community.ai-koda.com/skill.md | head -20</div>
          </div>
        </div>
      </div>
    `;

    try {
      const stats = await API.get('/public/stats', { public: true });
      document.getElementById('landing-stats').innerHTML = `
        <div class="stat"><span class="stat-num">${stats.assets || 0}</span><span class="stat-label">资产</span></div>
        <div class="stat"><span class="stat-num">${stats.users || 0}</span><span class="stat-label">用户</span></div>
        <div class="stat"><span class="stat-num">${stats.downloads || 0}</span><span class="stat-label">下载</span></div>
      `;
    } catch { document.getElementById('landing-stats').innerHTML = ''; }

    try {
      const data = await API.get('/public/skills?sort=downloads&page_size=4', { public: true });
      const assets = data.items || [];
      const el = document.getElementById('landing-hot');
      el.innerHTML = assets.length ? assets.map(Components.assetCard).join('') : Components.emptyState('暂无资产');
      bindAssetClicks(el);
    } catch { document.getElementById('landing-hot').innerHTML = ''; }

    try {
      const data = await API.get('/public/skills?sort=created_at&page_size=4', { public: true });
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
      window.location.hash = '#/assets';
      return;
    }
    if (githubError) {
      window.history.replaceState({}, document.title, window.location.pathname + window.location.hash);
    }
    route();
  });
})();
