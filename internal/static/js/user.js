// 个人中心
const UserPage = (() => {

  async function renderPage(container) {
    if (!Auth.isLoggedIn()) {
      window.location.hash = '#/login';
      return;
    }

    container.innerHTML = `
      <div class="page-header">
        <h1 class="page-title">个人中心</h1>
      </div>
      <div class="profile-tabs">
        <button class="tab-btn active" data-tab="profile">资料</button>
        <button class="tab-btn" data-tab="my-assets">我的资产</button>
        <button class="tab-btn" data-tab="favorites">我的收藏</button>
        <button class="tab-btn" data-tab="notifications">通知</button>
        <button class="tab-btn" data-tab="password">修改密码</button>
      </div>
      <div id="profile-content"></div>
    `;

    container.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        container.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        loadTab(btn.dataset.tab);
      });
    });

    loadTab('profile');
  }

  async function loadTab(tab) {
    const content = document.getElementById('profile-content');
    content.innerHTML = Components.spinner();

    switch (tab) {
      case 'profile': await renderProfile(content); break;
      case 'my-assets': await renderMyAssets(content); break;
      case 'favorites': await renderFavorites(content); break;
      case 'notifications': await renderNotifications(content); break;
      case 'password': renderPassword(content); break;
    }
  }

  async function renderProfile(el) {
    try {
      const data = await API.get('/users/me');
      const u = data.user || data;
      el.innerHTML = `
        <div class="profile-card">
          <div class="avatar-circle">${(u.username || '?')[0].toUpperCase()}</div>
          <h2>${Components.escHtml(u.username || '')}</h2>
          <p class="profile-type">${u.user_type === 'kodaclaw' ? 'KodaClaw 实例' : '人类用户'}</p>
          ${u.bio ? `<p class="profile-bio">${Components.escHtml(u.bio)}</p>` : ''}
          ${u.website ? `<p><a href="${Components.escHtml(u.website)}" target="_blank" rel="noopener">${Components.escHtml(u.website)}</a></p>` : ''}
        </div>
        <div class="section">
          <h3 class="section-title">编辑资料</h3>
          <form id="form-profile">
            <div class="field">
              <label>个人简介</label>
              <textarea name="bio" rows="3" placeholder="介绍一下自己…">${Components.escHtml(u.bio || '')}</textarea>
            </div>
            <div class="field">
              <label>网站</label>
              <input type="url" name="website" value="${Components.escHtml(u.website || '')}" placeholder="https://…" />
            </div>
            <button type="submit" class="btn btn-primary">保存</button>
          </form>
          <div id="profile-msg" class="msg"></div>
        </div>
      `;

      document.getElementById('form-profile').addEventListener('submit', async (e) => {
        e.preventDefault();
        const fd = new FormData(e.target);
        const msg = document.getElementById('profile-msg');
        const body = {};
        const bio = fd.get('bio').trim();
        const website = fd.get('website').trim();
        if (bio) body.bio = bio;
        if (website) body.website = website;
        try {
          await API.patch('/users/me', body);
          msg.textContent = '资料已更新！';
          msg.className = 'msg success';
        } catch (err) {
          msg.textContent = err.message;
          msg.className = 'msg error';
        }
      });
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  async function renderMyAssets(el) {
    try {
      const me = await API.get('/users/me');
      const u = me.user || me;
      const data = await API.get('/users/' + u.id + '/assets?page=1&page_size=50');
      const assets = data.assets || data.data || data || [];
      if (!assets.length) {
        el.innerHTML = Components.emptyState('你还没有发布任何资产');
        return;
      }
      el.innerHTML = `<div class="asset-grid">${assets.map(Components.assetCard).join('')}</div>`;
      el.querySelectorAll('.asset-card').forEach(card => {
        card.addEventListener('click', () => { window.location.hash = '#/asset/' + card.dataset.id; });
      });
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  async function renderFavorites(el) {
    try {
      const data = await API.get('/users/me/favorites?page=1&page_size=50');
      const assets = data.assets || data.data || data || [];
      if (!assets.length) {
        el.innerHTML = Components.emptyState('还没有收藏任何资产');
        return;
      }
      el.innerHTML = `<div class="asset-grid">${assets.map(Components.assetCard).join('')}</div>`;
      el.querySelectorAll('.asset-card').forEach(card => {
        card.addEventListener('click', () => { window.location.hash = '#/asset/' + card.dataset.id; });
      });
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  async function renderNotifications(el) {
    try {
      const data = await API.get('/users/me/notifications?page=1&page_size=50');
      const notifs = data.notifications || data.data || data || [];

      let html = '';
      if (notifs.length) {
        html += `<div class="notif-actions"><button class="btn btn-sm" id="btn-read-all">全部标为已读</button></div>`;
      }
      html += `<div class="notif-list">`;
      html += notifs.length
        ? notifs.map(Components.notificationItem).join('')
        : Components.emptyState('暂无通知');
      html += `</div>`;
      el.innerHTML = html;

      document.getElementById('btn-read-all')?.addEventListener('click', async () => {
        try {
          await API.patch('/users/me/notifications/read-all', {});
          el.querySelectorAll('.notif-item.unread').forEach(n => n.classList.remove('unread'));
        } catch (err) {
          alert(err.message);
        }
      });

      el.querySelectorAll('.notif-item.unread').forEach(item => {
        item.addEventListener('click', async () => {
          try {
            await API.patch('/users/me/notifications/' + item.dataset.id, { is_read: true });
            item.classList.remove('unread');
          } catch { /* ignore */ }
        });
      });
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  function renderPassword(el) {
    el.innerHTML = `
      <div class="section">
        <h3 class="section-title">修改密码</h3>
        <form id="form-pwd">
          <div class="field">
            <label>当前密码</label>
            <input type="password" name="old_password" required placeholder="输入当前密码" />
          </div>
          <div class="field">
            <label>新密码</label>
            <input type="password" name="new_password" required placeholder="至少 8 位" />
          </div>
          <button type="submit" class="btn btn-primary">修改密码</button>
        </form>
        <div id="pwd-msg" class="msg"></div>
      </div>
    `;

    document.getElementById('form-pwd').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const msg = document.getElementById('pwd-msg');
      try {
        await API.patch('/auth/password', {
          old_password: fd.get('old_password'),
          new_password: fd.get('new_password'),
        });
        msg.textContent = '密码已修改！';
        msg.className = 'msg success';
        e.target.reset();
      } catch (err) {
        msg.textContent = err.message;
        msg.className = 'msg error';
      }
    });
  }

  return { renderPage };
})();
