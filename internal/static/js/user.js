// 个人中心
const UserPage = (() => {
  let relayRefreshInterval = null;

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
        <button class="tab-btn" data-tab="relay">Relay 中继</button>
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
      const configJson = JSON.stringify({
        relayUrl: 'wss://community.ai-koda.com/ws/relay',
        sharedSecret: inst.sharedSecret || inst.shared_secret || '',
      });
      const overlay = document.createElement('div');
      overlay.className = 'modal-overlay';
      overlay.style.display = 'flex';
      overlay.innerHTML = `
        <div class="modal-box relay-creation-modal">
          <h3 class="section-title">实例创建成功</h3>
          <p class="relay-creation-modal warning">⚠ 共享密钥仅显示一次，请立即保存！</p>
          <div class="secret-display">
            <div class="label">Account ID</div>
            <div>${Components.escHtml(inst.accountID || inst.account_id || '')}&nbsp;${relayCopyBtn(inst.accountID || inst.account_id || '', '复制')}</div>
            <div class="label" style="margin-top:0.6rem">Shared Secret</div>
            <div>${Components.escHtml(inst.sharedSecret || inst.shared_secret || '')}&nbsp;${relayCopyBtn(inst.sharedSecret || inst.shared_secret || '', '复制')}</div>
          </div>
          <p style="font-size:0.85rem;color:var(--text-muted,#94a3b8)">将以上信息填入 KodaClaw 的 Relay 频道配置中。</p>
          <div class="modal-actions">
            <button class="btn btn-primary" id="btn-copy-config">复制配置 JSON</button>
            <button class="btn btn-outline" id="btn-created-close">关闭</button>
          </div>
        </div>
      `;
      document.body.appendChild(overlay);
      setupRelayCopyBtns(overlay);

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
      const accountID = inst.accountID || inst.account_id || '';
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
          const accountID = inst.accountID || inst.account_id || '';
          const isOnline = inst.isOnline;
          const lastConn = inst.lastConnectedAt || inst.last_connected_at;
          const createdAt = inst.createdAt || inst.created_at;
          const statusClass = isOnline ? 'relay-online' : 'relay-offline';
          const statusLabel = isOnline ? '在线' : '离线';
          const configJson = JSON.stringify({ relayUrl: 'wss://community.ai-koda.com/ws/relay', accountId: accountID });

          html += `
            <div class="relay-card">
              <div class="relay-card-header">
                <span class="relay-card-name">${Components.escHtml(name)}</span>
                <span class="${statusClass}">● ${statusLabel}</span>
              </div>
              <div class="relay-card-meta">
                <span>Account ID: <code>${Components.escHtml(accountID)}</code></span>
                <button class="relay-copy-btn" data-copy="${Components.escHtml(accountID)}" title="复制 Account ID">复制 ID</button>
              </div>
              <div class="relay-card-meta">
                ${lastConn ? `<span>最近连接: ${new Date(lastConn).toLocaleString('zh-CN')}</span>` : '<span>尚未连接</span>'}
                &nbsp;·&nbsp;
                <span>创建于: ${createdAt ? new Date(createdAt).toLocaleDateString('zh-CN') : '未知'}</span>
              </div>
              <div class="relay-card-actions">
                <button class="btn btn-sm btn-relay-test-conn" data-id="${Components.escHtml(inst.id)}">测试连接</button>
                <button class="btn btn-sm btn-relay-test-webhook" data-id="${Components.escHtml(inst.id)}"${isOnline ? '' : ' disabled'}>发送测试消息</button>
                <button class="btn btn-sm btn-relay-copy-config" data-config="${Components.escHtml(configJson)}">复制配置</button>
                <button class="btn btn-sm btn-relay-regen" data-id="${Components.escHtml(inst.id)}">重新生成密钥</button>
                <button class="btn btn-sm btn-danger btn-relay-delete" data-id="${Components.escHtml(inst.id)}" data-name="${Components.escHtml(name)}">删除</button>
              </div>
            </div>
          `;
        });
      }

      html += `</div>`;
      el.innerHTML = html;

      setupRelayCopyBtns(el);

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

      el.querySelectorAll('.btn-relay-copy-config').forEach(btn => {
        btn.addEventListener('click', () => {
          navigator.clipboard.writeText(btn.dataset.config).then(() => {
            const orig = btn.textContent;
            btn.textContent = '已复制';
            setTimeout(() => { btn.textContent = orig; }, 1000);
          });
        });
      });

      el.querySelectorAll('.btn-relay-regen').forEach(btn => {
        btn.addEventListener('click', () => {
          const inst = list.find(i => i.id === btn.dataset.id);
          if (inst) showRegenerateModal(inst);
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
