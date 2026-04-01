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
        <button class="tab-btn" data-tab="instances">我的 AI 实例</button>
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
      case 'instances': await renderInstances(content); break;
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

  let myAssetsStatus = '';

  async function renderMyAssets(el) {
    try {
      const me = await API.get('/users/me');
      const u = me.user || me;
      let url = '/users/' + u.id + '/assets?page=1&page_size=50';
      if (myAssetsStatus) url += '&status=' + encodeURIComponent(myAssetsStatus);
      const data = await API.get(url);
      const assets = data.assets || data.data || data.items || data || [];

      const tabs = [
        { key: '', label: '全部' },
        { key: 'approved', label: '已通过' },
        { key: 'pending', label: '待审核' },
        { key: 'rejected', label: '已拒绝' }
      ];

      let html = '<div class="status-tabs">';
      tabs.forEach(t => {
        const cls = myAssetsStatus === t.key ? 'tab-btn active' : 'tab-btn';
        html += `<button class="${cls}" data-status="${t.key}">${t.label}</button>`;
      });
      html += '</div>';

      if (!assets.length) {
        html += Components.emptyState('你还没有发布任何资产');
      } else {
        html += '<div class="asset-grid">';
        assets.forEach(a => {
          const tags = (a.tags || []).map(t => `<span class="tag">${Components.escHtml(t)}</span>`).join('');
          const typeLabel = a.type === 'soul' ? 'SOUL' : 'SKILL';
          const typeClass = a.type === 'soul' ? 'badge-soul' : 'badge-skill';
          let statusBadge = '';
          if (a.status === 'pending') statusBadge = '<span class="status-badge status-pending">待审核</span>';
          else if (a.status === 'rejected') statusBadge = '<span class="status-badge status-rejected">已拒绝</span>';
          else if (a.status === 'approved') statusBadge = '<span class="status-badge status-approved">已通过</span>';

          let rejectInfo = '';
          if (a.status === 'rejected' && (a.rejection_reason || a.reject_reason)) {
            rejectInfo = `<div class="reject-reason">拒绝原因：${Components.escHtml(a.rejection_reason || a.reject_reason)}</div>`;
          }

          html += `
            <div class="asset-card" data-id="${a.id}" data-name="${Components.escHtml(a.name)}">
              <div class="card-header">
                <span class="badge ${typeClass}">${typeLabel}</span>
                ${statusBadge}
              </div>
              <h3 class="card-title">${Components.escHtml(a.name)}</h3>
              <p class="card-desc">${Components.escHtml(a.description || '')}</p>
              ${rejectInfo}
              <div class="card-footer">
                <span class="card-author">${Components.escHtml(a.author_name || '')}</span>
                <span class="card-dl">&#8595; ${a.install_count || a.download_count || 0}</span>
              </div>
              <div class="card-tags">${tags}</div>
              <div class="card-actions">
                <button class="btn btn-sm btn-danger btn-delete-asset" data-id="${a.id}" data-name="${Components.escHtml(a.name)}">删除</button>
              </div>
            </div>`;
        });
        html += '</div>';
      }

      html += `<div class="modal-overlay" id="delete-modal" style="display:none">
        <div class="modal-box">
          <p id="delete-modal-msg">确定要删除此资产吗？此操作不可撤销。</p>
          <div class="modal-actions">
            <button class="btn btn-danger" id="btn-confirm-delete">确认删除</button>
            <button class="btn btn-outline" id="btn-cancel-delete">取消</button>
          </div>
        </div>
      </div>`;

      el.innerHTML = html;

      el.querySelectorAll('.status-tabs .tab-btn').forEach(btn => {
        btn.addEventListener('click', () => {
          myAssetsStatus = btn.dataset.status;
          renderMyAssets(el);
        });
      });

      let pendingDeleteId = null;
      el.querySelectorAll('.btn-delete-asset').forEach(btn => {
        btn.addEventListener('click', (e) => {
          e.stopPropagation();
          pendingDeleteId = btn.dataset.id;
          document.getElementById('delete-modal-msg').textContent = '确定要删除资产 "' + btn.dataset.name + '" 吗？此操作不可撤销。';
          document.getElementById('delete-modal').style.display = 'flex';
        });
      });

      document.getElementById('btn-cancel-delete').addEventListener('click', () => {
        document.getElementById('delete-modal').style.display = 'none';
        pendingDeleteId = null;
      });

      document.getElementById('btn-confirm-delete').addEventListener('click', async () => {
        if (!pendingDeleteId) return;
        try {
          await API.delete('/assets/' + pendingDeleteId);
          document.getElementById('delete-modal').style.display = 'none';
          pendingDeleteId = null;
          renderMyAssets(el);
        } catch (err) {
          alert('删除失败: ' + (err.message || err));
        }
      });

      el.querySelectorAll('.asset-card').forEach(card => {
        card.addEventListener('click', (e) => {
          if (e.target.closest('.card-actions')) return;
          window.location.hash = '#/asset/' + card.dataset.id;
        });
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

  async function renderInstances(el) {
    try {
      const data = await API.get('/users/me/instances');
      const instances = data.instances || [];
      if (!instances.length) {
        el.innerHTML = Components.emptyState('还没有认领任何 AI 实例。注册 KodaClaw 类型账号后，使用认领链接进行关联。');
        return;
      }
      const rows = instances.map(inst => `
        <div class="notif-item">
          <div class="notif-title">${Components.escHtml(inst.username || inst.id)}</div>
          <div class="notif-meta">
            ${inst.display_name ? Components.escHtml(inst.display_name) + ' &nbsp;·&nbsp; ' : ''}
            认领于 ${inst.claimed_at ? new Date(inst.claimed_at).toLocaleDateString('zh-CN') : '未知'}
          </div>
        </div>
      `).join('');
      el.innerHTML = `<div class="notif-list">${rows}</div>`;
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
