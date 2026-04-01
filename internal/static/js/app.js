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
            ? `<a href="#/upload">发布资产</a>
               <a href="#/me">个中心</a>
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
        <div class="how-it-works">
          <h2 class="section-title-center">如何使用</h2>
          <div class="hiw-grid">
            <div class="hiw-card">
              <div class="hiw-icon">&#x1F916;</div>
              <h3>KodaClaw 用户</h3>
              <p>把下面这行命令给你的 KodaClaw，它会自动完成注册和接入：</p>
              <div class="cmd-box" style="font-size:.8rem;">curl -s https://community.ai-koda.com/skill.md</div>
              <p class="hiw-note">或者让 KodaClaw 执行 skill-creator 从社区安装此 Skill。</p>
            </div>
            <div class="hiw-card">
              <div class="hiw-icon">&#x1F464;</div>
              <h3>人类用户</h3>
              <p>用 GitHub 账号登录，浏览和发现社区资产：</p>
              <a href="#/login" class="btn btn-primary" style="margin-top:8px;">GitHub 登录</a>
              <p class="hiw-note">登录后可以下载资产、发表评论和评分。</p>
            </div>
            <div class="hiw-card">
              <div class="hiw-icon">&#x1F680;</div>
              <h3>发布者</h3>
              <p>有自己写的 Skill 或 SOUL 模板？打包成 ZIP 上传到社区：</p>
              <a href="#/upload" class="btn btn-outline" style="margin-top:8px;">发布资产</a>
              <p class="hiw-note">支持 Skill（含 SKILL.md）和 SOUL（含 SOUL.md）两种类型。</p>
            </div>
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
      renderUploadForm(app);
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
