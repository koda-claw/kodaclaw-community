// 个人中心
const UserPage = (() => {
  let relayRefreshInterval = null;

  async function renderPage(container) {
    if (!Auth.isLoggedIn()) {
      window.location.hash = '#/login';
      return;
    }

    // SVG icons for each nav item
    const icons = {
      profile: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"/><path d="M4 20c0-4 3.6-7 8-7s8 3 8 7"/></svg>`,
      password: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="11" width="18" height="11" rx="2"/><path d="M7 11V7a5 5 0 0 1 10 0v4"/></svg>`,
      'my-assets': `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/></svg>`,
      favorites: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><polygon points="12 2 15.09 8.26 22 9.27 17 14.14 18.18 21.02 12 17.77 5.82 21.02 7 14.14 2 9.27 8.91 8.26 12 2"/></svg>`,
      notifications: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 8A6 6 0 0 0 6 8c0 7-3 9-3 9h18s-3-2-3-9"/><path d="M13.73 21a2 2 0 0 1-3.46 0"/></svg>`,
      instances: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/><circle cx="9" cy="10" r="1" fill="currentColor"/><circle cx="15" cy="10" r="1" fill="currentColor"/><path d="M9 13h6"/></svg>`,
      relay: `<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>`,
    };

    // Fetch user info for sidebar profile
    let sidebarUser = { username: '…', user_type: 'human', bio: '' };
    try {
      const d = await API.get('/users/me');
      sidebarUser = d.user || d;
    } catch (_) {}

    const typeLabel = sidebarUser.user_type === 'kodaclaw' ? 'KodaClaw 实例' : '人类用户';
    const typeCls = sidebarUser.user_type === 'kodaclaw' ? 'badge-soul' : 'badge-muted';
    const initial = (sidebarUser.username || '?')[0].toUpperCase();
    const bioHtml = sidebarUser.bio
      ? `<p class="sidebar-bio">${Components.escHtml(sidebarUser.bio)}</p>`
      : '';

    container.innerHTML = `
      <div class="user-page">
        <aside class="user-sidebar">
          <div class="user-sidebar-profile">
            <div class="avatar-circle sidebar-avatar">${initial}</div>
            <h2 class="sidebar-username">${Components.escHtml(sidebarUser.username || '')}</h2>
            <span class="badge ${typeCls} sidebar-type-badge">${typeLabel}</span>
            ${bioHtml}
          </div>
          <nav class="user-sidebar-nav">
            <span class="nav-group-label">账户</span>
            <button class="nav-item active" data-tab="profile">${icons.profile} 资料</button>
            <button class="nav-item" data-tab="password">${icons.password} 修改密码</button>
            <span class="nav-group-label">内容</span>
            <button class="nav-item" data-tab="my-assets">${icons['my-assets']} 我的资产</button>
            <button class="nav-item" data-tab="favorites">${icons.favorites} 我的收藏</button>
            <span class="nav-group-label">系统</span>
            <button class="nav-item" data-tab="notifications">${icons.notifications} 通知</button>
            <button class="nav-item" data-tab="instances">${icons.instances} 我的 AI 实例</button>
            <button class="nav-item" data-tab="relay">${icons.relay} Relay 中继</button>
          </nav>
          <div class="sidebar-footer">
            <button class="btn btn-outline btn-sm sidebar-logout" id="btn-sidebar-logout">退出登录</button>
          </div>
        </aside>
        <main class="user-content">
          <div id="profile-content"></div>
        </main>
      </div>
    `;

    // Sidebar nav click
    container.querySelectorAll('.nav-item').forEach(btn => {
      btn.addEventListener('click', () => {
        container.querySelectorAll('.nav-item').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        loadTab(btn.dataset.tab);
      });
    });

    // Logout button
    document.getElementById('btn-sidebar-logout')?.addEventListener('click', () => {
      Auth.logout();
      window.location.hash = '#/login';
    });

    loadTab('profile');
  }

  async function loadTab(tab) {
    if (relayRefreshInterval) {
      clearInterval(relayRefreshInterval);
      relayRefreshInterval = null;
    }
    const content = document.getElementById('profile-content');
    content.innerHTML = Components.spinner();

    switch (tab) {
      case 'profile': await renderProfile(content); break;
      case 'my-assets': await renderMyAssets(content); break;
      case 'favorites': await renderFavorites(content); break;
      case 'notifications': await renderNotifications(content); break;
      case 'instances': await renderInstances(content); break;
      case 'relay': await renderRelay(content); break;
      case 'password': renderPassword(content); break;
    }
  }

  async function renderProfile(el) {
    try {
      const data = await API.get('/users/me');
      const u = data.user || data;
      // Show profile card only on mobile (sidebar hidden), hide on desktop via CSS
      el.innerHTML = `
        <div class="profile-card profile-card-mobile">
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
        el.innerHTML = Components.emptyState('还没有绑定任何 KodaClaw 实例。');
        return;
      }
      const rows = instances.map(inst => `
        <div class="notif-item">
          <div class="notif-title">${Components.escHtml(inst.username || inst.id)}</div>
          <div class="notif-meta">
            ${inst.display_name ? Components.escHtml(inst.display_name) + ' &nbsp;·&nbsp; ' : ''}
            绑定于 ${inst.bound_at ? new Date(inst.bound_at).toLocaleDateString('zh-CN') : '未知'}
          </div>
        </div>
      `).join('');
      el.innerHTML = `<div class="notif-list">${rows}</div>`;
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  function relayCopyBtn(text, label) {
    return `<button class="relay-copy-btn" data-copy="${Components.escHtml(text)}" title="复制">${label || '复制'}</button>`;
  }

  function setupRelayCopyBtns(container) {
    container.querySelectorAll('.relay-copy-btn[data-copy]').forEach(btn => {
      btn.addEventListener('click', () => {
        const text = btn.dataset.copy;
        navigator.clipboard.writeText(text).then(() => {
          const orig = btn.textContent;
          btn.textContent = '已复制';
          setTimeout(() => { btn.textContent = orig; }, 1000);
        });
      });
    });
  }

  async function renderRelay(el) {
    if (relayRefreshInterval) {
      clearInterval(relayRefreshInterval);
      relayRefreshInterval = null;
    }
    const refresh = () => renderRelay(el);

    function showToast(message) {
      const toast = document.createElement('div');
      toast.style.cssText = 'position:fixed;bottom:1.5rem;left:50%;transform:translateX(-50%);background:#1e293b;color:#f1f5f9;padding:0.6rem 1.2rem;border-radius:0.5rem;z-index:9999;font-size:0.9rem;box-shadow:0 2px 8px rgba(0,0,0,0.3)';
      toast.textContent = message;
      document.body.appendChild(toast);
      setTimeout(() => toast.remove(), 3000);
    }

    function showCreateKeyModal(instId, onCreated) {
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">添加 Webhook 密钥</h3>
          <div class="field">
            <label>密钥名称</label>
            <input type="text" id="key-name-input" placeholder="例如: 生产环境" required />
          </div>
          <div class="field">
            <label>过期时间（可选）</label>
            <input type="date" id="key-expires-input" />
          </div>
          <div id="key-create-err"></div>
          <div class="modal-actions">
            <button class="btn btn-primary" id="btn-key-create-confirm">创建</button>
            <button class="btn btn-outline" id="btn-key-create-cancel">取消</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);
      if (typeof lucide !== 'undefined') lucide.createIcons();

      overlay.querySelector('#btn-key-create-cancel').addEventListener('click', () => overlay.remove());
      overlay.querySelector('#btn-key-create-confirm').addEventListener('click', async () => {
        const keyName = overlay.querySelector('#key-name-input').value.trim();
        const expiresAt = overlay.querySelector('#key-expires-input').value;
        const errEl = overlay.querySelector('#key-create-err');
        if (!keyName) { errEl.innerHTML = Components.errorBox('请输入密钥名称'); return; }
        const body = { keyName };
        if (expiresAt) body.expiresAt = expiresAt;
        try {
          const result = await API.createRelayKey(instId, body);
          overlay.remove();
          showNewKeyModal(result, onCreated);
        } catch (err) {
          errEl.innerHTML = Components.errorBox(err.message);
        }
      });
    }

    function showNewKeyModal(key, onClose) {
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">密钥已创建</h3>
          <p class="relay-creation-modal warning">⚠ 密钥值仅显示一次，请立即保存！</p>
          <div class="secret-display">
            <div class="label">密钥名称</div>
            <div>${Components.escHtml(key.keyName || '')}</div>
            <div class="label" style="margin-top:0.6rem">密钥值</div>
            <div style="display:flex;align-items:center;gap:0.5rem;flex-wrap:wrap">
              <span style="word-break:break-all">${Components.escHtml(key.keyValue || '')}</span>
              ${relayCopyBtn(key.keyValue || '', '复制')}
            </div>
          </div>
          <p style="font-size:0.85rem;color:var(--text-muted,#94a3b8)">使用此密钥作为 HMAC 签名的 secret，在 Webhook 请求头携带 X-Relay-Signature。</p>
          <div class="modal-actions">
            <button class="btn btn-outline" id="btn-newkey-close">关闭</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);
      setupRelayCopyBtns(overlay);
      if (typeof lucide !== 'undefined') lucide.createIcons();
      overlay.querySelector('#btn-newkey-close').addEventListener('click', () => {
        overlay.remove();
        if (onClose) onClose();
      });
    }

    async function loadAndRenderKeys(instId, keysBodyEl, keysCountEl) {
      keysBodyEl.innerHTML = '<p style="color:var(--text-muted,#94a3b8);font-size:0.85rem;padding:0.5rem 0">加载中...</p>';
      try {
        const data = await API.listRelayKeys(instId);
        const keys = Array.isArray(data) ? data : (data.keys || []);
        if (keysCountEl) keysCountEl.textContent = `Webhook 密钥 (${keys.length})`;
        if (!keys.length) {
          keysBodyEl.innerHTML = '<p style="color:var(--text-muted,#94a3b8);font-size:0.85rem;padding:0.5rem 0">暂无密钥，点击"+ 添加"创建</p>';
          return;
        }
        keysBodyEl.innerHTML = keys.map(k => {
          const now = new Date();
          const expired = k.expiresAt && new Date(k.expiresAt) < now;
          let statusCls = 'relay-key-status-active', statusLabel = '活跃';
          if (expired) { statusCls = 'relay-key-status-expired'; statusLabel = '已过期'; }
          else if (!k.isActive) { statusCls = 'relay-key-status-disabled'; statusLabel = '已禁用'; }
          return `
            <div class="relay-key-card" data-key-id="${Components.escHtml(k.id)}">
              <div class="relay-key-left">
                <i data-lucide="key" class="inline-icon" style="color:var(--accent,#8B5CF6)"></i>
                <span class="relay-key-name">${Components.escHtml(k.keyName || k.id)}</span>
                <span class="relay-key-status ${statusCls}">${statusLabel}</span>
              </div>
              <div class="relay-key-value-wrap">
                <code class="relay-key-value">${Components.escHtml(k.keyPrefix || '')}...</code>
                <button class="relay-copy-btn btn-relay-key-copy" data-copy="${Components.escHtml(k.keyPrefix || '')}..." title="复制前缀">
                  <i data-lucide="copy" class="inline-icon"></i>
                </button>
              </div>
              <div class="relay-key-actions">
                <button class="relay-copy-btn btn-relay-key-toggle" data-key-id="${Components.escHtml(k.id)}" data-active="${k.isActive ? '1' : '0'}" title="${k.isActive ? '禁用' : '启用'}">
                  <i data-lucide="${k.isActive ? 'toggle-right' : 'toggle-left'}" class="inline-icon" style="color:${k.isActive ? '#22c55e' : '#64748b'};font-size:1.3em"></i>
                </button>
                <button class="relay-copy-btn btn-relay-key-delete" data-key-id="${Components.escHtml(k.id)}" title="删除密钥">
                  <i data-lucide="trash-2" class="inline-icon" style="color:#ef4444"></i>
                </button>
              </div>
            </div>
          `;
        }).join('');
        if (typeof lucide !== 'undefined') lucide.createIcons();
        setupRelayCopyBtns(keysBodyEl);

        keysBodyEl.querySelectorAll('.btn-relay-key-toggle').forEach(btn => {
          btn.addEventListener('click', async () => {
            const keyId = btn.dataset.keyId;
            const isActive = btn.dataset.active === '1';
            try {
              await API.toggleRelayKey(instId, keyId, { isActive: !isActive });
              loadAndRenderKeys(instId, keysBodyEl, keysCountEl);
            } catch (err) { showToast('操作失败: ' + err.message); }
          });
        });

        keysBodyEl.querySelectorAll('.btn-relay-key-delete').forEach(btn => {
          btn.addEventListener('click', async () => {
            if (!confirm('确定删除此密钥？此操作不可撤销。')) return;
            try {
              await API.deleteRelayKey(instId, btn.dataset.keyId);
              loadAndRenderKeys(instId, keysBodyEl, keysCountEl);
            } catch (err) { showToast('删除失败: ' + err.message); }
          });
        });
      } catch (err) {
        keysBodyEl.innerHTML = Components.errorBox(err.message);
      }
    }

    function showCreateModal() {
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">创建 Relay 实例</h3>
          <div class="field">
            <label>实例名称</label>
            <input type="text" id="relay-name-input" placeholder="例如: 我的 KodaClaw" required />
          </div>
          <div id="relay-create-err"></div>
          <div class="modal-actions">
            <button class="btn btn-primary" id="btn-relay-create-confirm">创建</button>
            <button class="btn btn-outline" id="btn-relay-create-cancel">取消</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);

      overlay.querySelector('#btn-relay-create-cancel').addEventListener('click', () => {
        overlay.remove();
      });

      overlay.querySelector('#btn-relay-create-confirm').addEventListener('click', async () => {
        const nameInput = overlay.querySelector('#relay-name-input');
        const errEl = overlay.querySelector('#relay-create-err');
        const name = nameInput.value.trim();
        if (!name) {
          errEl.innerHTML = Components.errorBox('请输入实例名称');
          return;
        }
        try {
          const result = await API.createRelayInstance({ instanceName: name });
          overlay.remove();
          showCreatedModal(result);
        } catch (err) {
          errEl.innerHTML = Components.errorBox(err.message);
        }
      });
    }

    function showCreatedModal(inst) {
      const sharedSecret = inst.sharedSecret || inst.shared_secret || '';
      const webhookSecret = inst.webhookSecret || inst.webhook_secret || '';
      const accountId = inst.accountId || inst.account_id || '';
      const configJson = JSON.stringify({
        relayUrl: 'wss://community.ai-koda.com/ws/relay',
        sharedSecret,
      });
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">实例创建成功</h3>
          <div class="secret-display">
            <div class="label">Account ID</div>
            <div>${Components.escHtml(accountId)}&nbsp;${relayCopyBtn(accountId, '复制')}</div>
            <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px dashed rgba(255,255,255,0.1)">
              <div class="label">Shared Secret <span style="font-size:0.75rem;color:#f59e0b">（WebSocket 连接用）</span></div>
              <div style="display:flex;align-items:center;gap:0.5rem;flex-wrap:wrap">${Components.escHtml(sharedSecret)}&nbsp;${relayCopyBtn(sharedSecret, '复制')}</div>
              <p style="font-size:0.75rem;color:#f59e0b;margin:0.25rem 0 0">⚠ 仅显示一次，请立即保存</p>
            </div>
            ${webhookSecret ? `
            <div style="margin-top:0.8rem;padding-top:0.8rem;border-top:1px dashed rgba(255,255,255,0.1)">
              <div class="label">Webhook Secret <span style="font-size:0.75rem;color:#f59e0b">（首个 Webhook 密钥）</span></div>
              <div style="display:flex;align-items:center;gap:0.5rem;flex-wrap:wrap">${Components.escHtml(webhookSecret)}&nbsp;${relayCopyBtn(webhookSecret, '复制')}</div>
              <p style="font-size:0.75rem;color:#f59e0b;margin:0.25rem 0 0">⚠ 仅显示一次，请立即保存</p>
            </div>` : ''}
          </div>
          <p style="font-size:0.85rem;color:var(--text-muted,#94a3b8);margin-top:0.5rem">将以上信息填入 KodaClaw 的 Relay 频道配置中。</p>
          <div class="modal-actions">
            <button class="btn btn-primary" id="btn-copy-config">复制配置 JSON</button>
            <button class="btn btn-outline" id="btn-created-close">关闭</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);
      setupRelayCopyBtns(overlay);
      if (typeof lucide !== 'undefined') lucide.createIcons();

      overlay.querySelector('#btn-copy-config').addEventListener('click', () => {
        navigator.clipboard.writeText(configJson).then(() => {
          const btn = overlay.querySelector('#btn-copy-config');
          btn.textContent = '已复制';
          setTimeout(() => { btn.textContent = '复制配置 JSON'; }, 1000);
        });
      });

      overlay.querySelector('#btn-created-close').addEventListener('click', () => {
        overlay.remove();
        refresh();
      });
    }

    function showDeleteModal(inst) {
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box">
          <p>确定要删除实例 "<strong>${Components.escHtml(inst.instanceName || inst.instance_name || inst.id)}</strong>" 吗？此操作不可撤销。</p>
          <div class="modal-actions">
            <button class="btn btn-danger" id="btn-relay-del-confirm">确认删除</button>
            <button class="btn btn-outline" id="btn-relay-del-cancel">取消</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);

      overlay.querySelector('#btn-relay-del-cancel').addEventListener('click', () => overlay.remove());
      overlay.querySelector('#btn-relay-del-confirm').addEventListener('click', async () => {
        try {
          await API.deleteRelayInstance(inst.id);
          overlay.remove();
          refresh();
        } catch (err) {
          alert('删除失败: ' + err.message);
        }
      });
    }

    function showTestConnectionModal(inst) {
      const accountID = inst.accountId || inst.account_id || '';
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">测试连接</h3>
          <div class="field">
            <label>Account ID</label>
            <input type="text" id="tc-account-id" value="${Components.escHtml(accountID)}" readonly />
          </div>
          <div class="field">
            <label>Shared Secret</label>
            <input type="password" id="tc-secret" placeholder="输入 Shared Secret" />
          </div>
          <div id="tc-result"></div>
          <div class="modal-actions">
            <button class="btn btn-primary" id="btn-tc-test">测试</button>
            <button class="btn btn-outline" id="btn-tc-close">关闭</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);

      overlay.querySelector('#btn-tc-close').addEventListener('click', () => overlay.remove());
      overlay.querySelector('#btn-tc-test').addEventListener('click', async () => {
        const secret = overlay.querySelector('#tc-secret').value.trim();
        const resultEl = overlay.querySelector('#tc-result');
        if (!secret) {
          resultEl.innerHTML = Components.errorBox('请输入 Shared Secret');
          return;
        }
        const btn = overlay.querySelector('#btn-tc-test');
        btn.disabled = true;
        btn.textContent = '测试中...';
        try {
          const res = await API.testRelayConnection({ accountId: accountID, sharedSecret: secret });
          if (!res.ok) {
            resultEl.innerHTML = `<p style="color:#ef4444">❌ ${Components.escHtml(res.message || '密钥不匹配')}</p>`;
          } else if (res.online) {
            resultEl.innerHTML = `<p style="color:#22c55e">✅ 认证成功，实例在线</p>`;
          } else {
            resultEl.innerHTML = `<p style="color:#f59e0b">⚠ 认证成功，但实例当前不在线</p>`;
          }
        } catch (err) {
          resultEl.innerHTML = Components.errorBox(err.message);
        }
        btn.disabled = false;
        btn.textContent = '测试';
      });
    }

    function showRegenerateModal(inst) {
      const name = inst.instanceName || inst.instance_name || inst.id;
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box">
          <p>重新生成密钥后，旧的密钥将<strong>立即失效</strong>，KodaClaw 实例会被断开。确定吗？</p>
          <p style="font-size:0.85rem;color:var(--text-muted,#94a3b8)">实例: ${Components.escHtml(name)}</p>
          <div class="modal-actions">
            <button class="btn btn-danger" id="btn-regen-confirm">确认重新生成</button>
            <button class="btn btn-outline" id="btn-regen-cancel">取消</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);

      overlay.querySelector('#btn-regen-cancel').addEventListener('click', () => overlay.remove());
      overlay.querySelector('#btn-regen-confirm').addEventListener('click', async () => {
        const btn = overlay.querySelector('#btn-regen-confirm');
        btn.disabled = true;
        btn.textContent = '生成中...';
        try {
          const result = await API.regenerateRelaySecret(inst.id);
          overlay.remove();
          showNewSecretModal(result);
        } catch (err) {
          alert('生成失败: ' + err.message);
          btn.disabled = false;
          btn.textContent = '确认重新生成';
        }
      });
    }

    function showNewSecretModal(result) {
      const accountID = result.accountId || result.account_id || '';
      const secret = result.sharedSecret || result.shared_secret || '';
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">新密钥已生成</h3>
          <p class="relay-creation-modal warning">⚠ 新密钥仅显示一次，请立即保存！</p>
          <div class="secret-display">
            <div class="label">Account ID</div>
            <div>${Components.escHtml(accountID)}&nbsp;${relayCopyBtn(accountID, '复制')}</div>
            <div class="label" style="margin-top:0.6rem">新 Shared Secret</div>
            <div>${Components.escHtml(secret)}&nbsp;${relayCopyBtn(secret, '复制')}</div>
          </div>
          <p style="font-size:0.85rem;color:var(--text-muted,#94a3b8)">请在 KodaClaw 的 Relay 频道配置中更新此密钥。</p>
          <div class="modal-actions">
            <button class="btn btn-outline" id="btn-newsecret-close">关闭</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);
      setupRelayCopyBtns(overlay);

      overlay.querySelector('#btn-newsecret-close').addEventListener('click', () => {
        overlay.remove();
        refresh();
      });
    }

    try {
      const instances = await API.getRelayInstances();
      const list = Array.isArray(instances) ? instances : (instances.instances || []);

      let html = `
        <div class="section">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:1rem">
            <h3 class="section-title" style="margin:0">Relay 中继实例</h3>
            <button class="btn btn-primary btn-sm" id="btn-relay-create">+ 创建实例</button>
          </div>
      `;

      if (!list.length) {
        html += Components.emptyState('还没有 Relay 实例。创建一个后，在 KodaClaw 中配置中继连接。');
      } else {
        list.forEach(inst => {
          const name = inst.instanceName || inst.instance_name || inst.id;
          const accountID = inst.accountId || inst.account_id || '';
          const isOnline = inst.isOnline;
          const lastConn = inst.lastConnectedAt || inst.last_connected_at;
          const createdAt = inst.createdAt || inst.created_at;
          const statusClass = isOnline ? 'relay-online' : 'relay-offline';
          const statusLabel = isOnline ? '在线' : '离线';
          const configJson = JSON.stringify({ relayUrl: 'wss://community.ai-koda.com/ws/relay', accountId: accountID });
          const webhookUrl = `https://community.ai-koda.com/api/v1/webhook/incoming/${inst.id}`;
          const curlExample = `timestamp=$(date +%s)
body='{"text":"hello"}'
sig=$(printf '%s' "$timestamp.$body" | openssl dgst -sha256 -hmac "YOUR_WEBHOOK_KEY" | awk '{print $2}')
curl -X POST "${webhookUrl}" \\
  -H "Content-Type: application/json" \\
  -H "X-Relay-Timestamp: $timestamp" \\
  -H "X-Relay-Signature: $sig" \\
  -d "$body"`;
          const safeId = Components.escHtml(inst.id);

          html += `
            <div class="relay-card" data-inst-id="${safeId}">
              <!-- Section 1: 连接状态 -->
              <div class="relay-section">
                <div class="relay-section-header" data-target="conn-${safeId}">
                  <span class="relay-section-title">
                    <i data-lucide="plug" class="inline-icon"></i> 连接
                  </span>
                  <i data-lucide="chevron-down" class="inline-icon chevron"></i>
                </div>
                <div class="relay-section-body" id="conn-${safeId}">
                  <div class="relay-card-header">
                    <span class="relay-card-name">${Components.escHtml(name)}</span>
                    <span class="${statusClass}">● ${statusLabel}</span>
                  </div>
                  <div class="relay-card-meta">
                    <span>Account ID: <code>${Components.escHtml(accountID)}</code></span>
                    <button class="relay-copy-btn" data-copy="${Components.escHtml(accountID)}" title="复制 Account ID">
                      <i data-lucide="copy" class="inline-icon"></i> 复制
                    </button>
                  </div>
                  <div class="relay-card-meta">
                    ${lastConn ? `<span>最近连接: ${new Date(lastConn).toLocaleString('zh-CN')}</span>` : '<span>尚未连接</span>'}
                    &nbsp;·&nbsp;
                    <span>创建于: ${createdAt ? new Date(createdAt).toLocaleDateString('zh-CN') : '未知'}</span>
                  </div>
                  <div class="relay-card-actions">
                    <button class="btn btn-sm btn-relay-test-conn" data-id="${safeId}">
                      <i data-lucide="radio" class="inline-icon"></i> 测试连接
                    </button>
                    <button class="btn btn-sm btn-relay-copy-config" data-config="${Components.escHtml(configJson)}">
                      <i data-lucide="copy" class="inline-icon"></i> 复制配置
                    </button>
                    <button class="btn btn-sm btn-relay-regen" data-id="${safeId}">重新生成密钥</button>
                    <button class="btn btn-sm btn-danger btn-relay-delete" data-id="${safeId}" data-name="${Components.escHtml(name)}">
                      <i data-lucide="trash-2" class="inline-icon"></i> 删除
                    </button>
                  </div>
                </div>
              </div>

              <!-- Section 2: Webhook 端点 -->
              <div class="relay-section">
                <div class="relay-section-header" data-target="wh-${safeId}">
                  <span class="relay-section-title">
                    <i data-lucide="webhook" class="inline-icon"></i> Webhook 端点
                  </span>
                  <i data-lucide="chevron-down" class="inline-icon chevron"></i>
                </div>
                <div class="relay-section-body" id="wh-${safeId}">
                  <div class="relay-card-meta" style="flex-direction:column;align-items:flex-start;gap:0.3rem">
                    <div style="display:flex;align-items:flex-start;gap:0.5rem;width:100%">
                      <code style="word-break:break-all;flex:1;font-size:0.82rem">${Components.escHtml(webhookUrl)}</code>
                      <button class="relay-copy-btn" data-copy="${Components.escHtml(webhookUrl)}" title="复制 Webhook URL" style="flex-shrink:0">
                        <i data-lucide="copy" class="inline-icon"></i>
                      </button>
                    </div>
                    <span style="font-size:0.8rem;color:var(--text-muted,#94a3b8)">向此 URL 发送任意 JSON，需携带 HMAC 签名验证</span>
                  </div>
                  <div style="margin-top:0.5rem">
                    <button class="btn btn-sm btn-relay-curl-toggle" data-id="${safeId}">
                      <i data-lucide="terminal" class="inline-icon"></i> curl 示例
                    </button>
                    <pre id="curl-${safeId}" style="display:none;margin:0.5rem 0 0;font-size:0.78rem;background:var(--surface-alt,#1e293b);color:#e2e8f0;padding:0.6rem;border-radius:0.4rem;overflow-x:auto;white-space:pre-wrap">${Components.escHtml(curlExample)}</pre>
                  </div>
                  <div class="relay-card-actions">
                    <button class="btn btn-sm btn-relay-test-webhook" data-id="${safeId}"${isOnline ? '' : ' disabled'}>
                      <i data-lucide="send" class="inline-icon"></i> 发送测试消息
                    </button>
                  </div>
                </div>
              </div>

              <!-- Section 3: 密钥管理 -->
              <div class="relay-section relay-section-last">
                <div class="relay-section-header" data-target="keys-${safeId}">
                  <span class="relay-section-title">
                    <i data-lucide="key-round" class="inline-icon"></i>
                    <span class="relay-keys-count" data-inst="${safeId}">Webhook 密钥</span>
                  </span>
                  <div style="display:flex;align-items:center;gap:0.5rem">
                    <button class="btn btn-sm btn-relay-key-add" data-id="${safeId}" style="padding:0.2rem 0.6rem">
                      <i data-lucide="plus" class="inline-icon"></i> 添加
                    </button>
                    <i data-lucide="chevron-down" class="inline-icon chevron"></i>
                  </div>
                </div>
                <div class="relay-section-body" id="keys-${safeId}">
                  <p style="color:var(--text-muted,#94a3b8);font-size:0.85rem;padding:0.5rem 0">加载中...</p>
                </div>
              </div>
            </div>
          `;
        });
      }

      html += `</div>`;
      el.innerHTML = html;

      setupRelayCopyBtns(el);
      if (typeof lucide !== 'undefined') lucide.createIcons();

      // 折叠面板 toggle
      el.querySelectorAll('.relay-section-header').forEach(header => {
        header.addEventListener('click', (e) => {
          if (e.target.closest('.btn-relay-key-add')) return;
          const targetId = header.dataset.target;
          const body = targetId ? el.querySelector(`#${targetId}`) : header.nextElementSibling;
          const chevron = header.querySelector('.chevron');
          if (!body) return;
          const isOpen = body.style.display !== 'none';
          body.style.display = isOpen ? 'none' : '';
          if (chevron) chevron.style.transform = isOpen ? 'rotate(0deg)' : 'rotate(180deg)';
        });
      });

      // 加载每个实例的密钥列表
      list.forEach(inst => {
        const safeId = inst.id;
        const keysBodyEl = el.querySelector(`#keys-${safeId}`);
        const keysCountEl = el.querySelector(`.relay-keys-count[data-inst="${safeId}"]`);
        if (keysBodyEl) loadAndRenderKeys(safeId, keysBodyEl, keysCountEl);
      });

      el.querySelector('#btn-relay-create').addEventListener('click', showCreateModal);

      el.querySelectorAll('.btn-relay-delete').forEach(btn => {
        btn.addEventListener('click', () => {
          const inst = list.find(i => i.id === btn.dataset.id);
          if (inst) showDeleteModal(inst);
        });
      });

      el.querySelectorAll('.btn-relay-test-conn').forEach(btn => {
        btn.addEventListener('click', () => {
          const inst = list.find(i => i.id === btn.dataset.id);
          if (inst) showTestConnectionModal(inst);
        });
      });

      el.querySelectorAll('.btn-relay-test-webhook').forEach(btn => {
        btn.addEventListener('click', async () => {
          btn.disabled = true;
          try {
            const res = await API.testRelayWebhook(btn.dataset.id);
            showToast(res.ok ? '✅ 测试消息已发送，请检查 KodaClaw 是否收到' : '⚠ 实例不在线，请先建立 Relay 连接');
          } catch (err) {
            showToast('❌ 发送失败: ' + err.message);
          }
          btn.disabled = false;
        });
      });

      el.querySelectorAll('.btn-relay-curl-toggle').forEach(btn => {
        btn.addEventListener('click', () => {
          const pre = el.querySelector(`#curl-${btn.dataset.id}`);
          if (!pre) return;
          pre.style.display = pre.style.display === 'none' ? 'block' : 'none';
        });
      });

      el.querySelectorAll('.btn-relay-copy-config').forEach(btn => {
        btn.addEventListener('click', () => {
          navigator.clipboard.writeText(btn.dataset.config).then(() => {
            const orig = btn.innerHTML;
            btn.textContent = '已复制';
            setTimeout(() => { btn.innerHTML = orig; }, 1000);
          });
        });
      });

      el.querySelectorAll('.btn-relay-regen').forEach(btn => {
        btn.addEventListener('click', () => {
          const inst = list.find(i => i.id === btn.dataset.id);
          if (inst) showRegenerateModal(inst);
        });
      });

      el.querySelectorAll('.btn-relay-key-add').forEach(btn => {
        btn.addEventListener('click', (e) => {
          e.stopPropagation();
          const instId = btn.dataset.id;
          const keysBodyEl = el.querySelector(`#keys-${instId}`);
          const keysCountEl = el.querySelector(`.relay-keys-count[data-inst="${instId}"]`);
          showCreateKeyModal(instId, () => loadAndRenderKeys(instId, keysBodyEl, keysCountEl));
        });
      });

      relayRefreshInterval = setInterval(refresh, 10000);

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
