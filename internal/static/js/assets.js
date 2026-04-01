// 资产列表和详情
const AssetsPage = (() => {

  // ---- 列表页 ----
  async function renderList(container) {
    container.innerHTML = `
      <div class="page-header">
        <h1 class="page-title">资产市场</h1>
        <p class="page-sub">发现并下载 SOUL 和 Skill 资产</p>
      </div>
      <div class="search-bar">
        <input id="search-q" type="text" placeholder="搜索资产名称、描述…" />
        <select id="filter-type">
          <option value="">全部类型</option>
          <option value="soul">SOUL</option>
          <option value="skill">SKILL</option>
        </select>
        <select id="filter-sort">
          <option value="created_at">最新发布</option>
          <option value="downloads">最多下载</option>
          <option value="rating">最高评分</option>
        </select>
        <button id="btn-search" class="btn btn-primary">搜索</button>
      </div>
      <div class="list-layout">
        <div class="list-main">
          <div id="tag-cloud" class="tag-cloud"></div>
          <div id="asset-list" class="asset-grid">${Components.spinner()}</div>
          <div id="pagination" class="pagination"></div>
        </div>
        <aside id="sidebar" class="sidebar">
          <div class="sidebar-section">
            <h3 class="sidebar-title">&#x1F525; 热门资产</h3>
            <div id="hot-assets">${Components.spinner()}</div>
          </div>
        </aside>
      </div>
    `;

    let page = 1;
    let q = '';
    let type = '';
    let sort = 'created_at';

    async function load() {
      const listEl = document.getElementById('asset-list');
      listEl.innerHTML = Components.spinner();
      try {
        let path = `/public/skills?page=${page}&page_size=20&sort=${sort}`;
        if (q) path += `&q=${encodeURIComponent(q)}`;
        if (type) path += `&type=${encodeURIComponent(type)}`;
        const data = await API.get(path, { public: true });
        const assets = data.items || data.assets || data.data || data || [];
        if (!assets.length) {
          listEl.innerHTML = Components.emptyState('暂无资产');
          document.getElementById('pagination').innerHTML = '';
          return;
        }
        listEl.innerHTML = assets.map(Components.assetCard).join('');
        listEl.querySelectorAll('.asset-card').forEach(card => {
          const target = () => {
            window.location.hash = '#/asset/' + encodeURIComponent(card.dataset.name || card.dataset.id);
          };
          card.addEventListener('click', target);
          card.addEventListener('keydown', e => { if (e.key === 'Enter') target(); });
        });
        const total = data.total || assets.length;
        const pages = Math.ceil(total / 20);
        const pagEl = document.getElementById('pagination');
        if (pages > 1) {
          let html = '';
          if (page > 1) html += `<button class="btn btn-sm" id="pg-prev">&#x2190; 上一页</button>`;
          html += `<span class="pg-info">第 ${page} / ${pages} 页</span>`;
          if (page < pages) html += `<button class="btn btn-sm" id="pg-next">下一页 &#x2192;</button>`;
          pagEl.innerHTML = html;
          pagEl.querySelector('#pg-prev')?.addEventListener('click', () => { page--; load(); });
          pagEl.querySelector('#pg-next')?.addEventListener('click', () => { page++; load(); });
        } else {
          pagEl.innerHTML = '';
        }
      } catch (err) {
        listEl.innerHTML = Components.errorBox(err.message);
      }
    }

    async function loadHotAssets() {
      const el = document.getElementById('hot-assets');
      try {
        const data = await API.get('/public/skills?sort=downloads&page_size=5', { public: true });
        const assets = data.items || [];
        if (!assets.length) { el.innerHTML = Components.emptyState('暂无数据'); return; }
        el.innerHTML = assets.map(a => {
          const rating = Number(a.avg_rating || 0);
          const stars = rating ? '&#x2605;'.repeat(Math.round(rating)) + '&#x2606;'.repeat(5 - Math.round(rating)) : '';
          return `<a class="hot-item" href="#/asset/${encodeURIComponent(a.name)}" data-name="${Components.escHtml(a.name)}">
            <span class="hot-name">${Components.escHtml(a.name)}</span>
            <span class="hot-meta">${stars} &middot; &#x2193;${a.install_count || 0}</span>
          </a>`;
        }).join('');
      } catch { el.innerHTML = ''; }
    }

    document.getElementById('btn-search').addEventListener('click', () => {
      q = document.getElementById('search-q').value.trim();
      type = document.getElementById('filter-type').value;
      sort = document.getElementById('filter-sort').value;
      page = 1;
      load();
    });
    document.getElementById('search-q').addEventListener('keydown', e => {
      if (e.key === 'Enter') document.getElementById('btn-search').click();
    });
    document.getElementById('filter-type').addEventListener('change', () => {
      document.getElementById('btn-search').click();
    });
    document.getElementById('filter-sort').addEventListener('change', () => {
      document.getElementById('btn-search').click();
    });

    // 热门标签
    try {
      const tagsData = await API.get('/public/skills', { public: true });
      const allAssets = tagsData.items || [];
      const tagSet = new Set();
      allAssets.forEach(a => (a.tags || []).forEach(t => tagSet.add(t)));
      const tags = [...tagSet].slice(0, 15);
      const cloud = document.getElementById('tag-cloud');
      if (tags.length) {
        cloud.innerHTML = tags.map(t =>
          `<span class="tag tag-clickable" data-tag="${Components.escHtml(t)}">${Components.escHtml(t)}</span>`
        ).join('');
        cloud.querySelectorAll('.tag-clickable').forEach(el => {
          el.addEventListener('click', () => {
            document.getElementById('search-q').value = el.dataset.tag;
            document.getElementById('btn-search').click();
          });
        });
      } else {
        cloud.innerHTML = '';
      }
    } catch { /* ignore */ }

    load();
    loadHotAssets();
  }

  // ---- 详情页 ----
  async function renderDetail(container, identifier) {
    container.innerHTML = `
      <div class="back-link"><a href="#/assets">&#x2190; 返回列表</a></div>
      <div id="asset-detail">${Components.spinner()}</div>
    `;

    try {
      let asset;
      let isPublic = false;

      try {
        const data = await API.get('/public/skills/' + identifier, { public: true });
        asset = data.asset || data;
        isPublic = true;
      } catch {
        try {
          const data = await API.get('/assets/' + identifier);
          asset = data.asset || data;
        } catch {
          throw new Error('资产不存在');
        }
      }

      const tags = (asset.tags || []).map(t => `<span class="tag">${Components.escHtml(t)}</span>`).join('');
      const assetType = asset.type || asset.asset_type;
      const typeLabel = assetType === 'soul' ? 'SOUL' : 'SKILL';
      const typeClass = assetType === 'soul' ? 'badge-soul' : 'badge-skill';
      const ratingVal = Number(asset.avg_rating || asset.average_rating || 0);
      const rating = ratingVal ? ratingVal.toFixed(1) : '\u2014';
      const stars = ratingVal ? '\u2605'.repeat(Math.round(ratingVal)) + '\u2606'.repeat(5 - Math.round(ratingVal)) : '\u2606\u2606\u2606\u2606\u2606';
      const assetName = asset.name || identifier;
      const installName = asset.name || identifier;

      document.getElementById('asset-detail').innerHTML = `
        <div class="detail-card">
          <div class="detail-header">
            <span class="badge ${typeClass}">${typeLabel}</span>
            <h1 class="detail-title">${Components.escHtml(assetName)}</h1>
            <div class="detail-meta">
              <span>\u4F5C\u8005\uFF1A@${Components.escHtml(asset.author_name || asset.author_id || '')}</span>
              <span>\u4E0B\u8F7D\uFF1A&#x2193; ${asset.install_count || asset.download_count || 0}</span>
              <span>\u8BC4\u5206\uFF1A${stars} ${rating}</span>
            </div>
          </div>

          <p class="detail-desc">${Components.escHtml(asset.description || '')}</p>
          <div class="detail-tags">${tags}</div>

          ${asset.skill_content ? (() => {
            const full = asset.skill_content;
            const preview = full.length > 500 ? full.slice(0, 500) : null;
            return `<div class="detail-preview">
              <pre id="skill-content-preview">${Components.escHtml(preview || full)}</pre>
              ${preview ? `<button class="btn btn-sm" id="btn-expand-content">\u5C55\u5F00\u5B8C\u6574\u5185\u5BB9</button>` : ''}
            </div>`;
          })() : ''}

          <div class="detail-actions">
            <button class="btn btn-primary" id="btn-download">\u4E0B\u8F7D</button>
            <button class="btn btn-outline" id="btn-fav">\u6536\u85CF</button>
          </div>

          <div class="install-cmd">
            <h3>&#x1F4E6; \u5FEB\u901F\u5B89\u88C5</h3>
            <p>\u5728\u4F60\u7684 KodaClaw \u7EC8\u7AEF\u4E2D\u6267\u884C\uFF1A</p>
            <div class="cmd-box" id="cmd-install">kc-community install ${Components.escHtml(installName)}</div>
            <button class="btn btn-sm btn-outline" id="btn-copy-cmd">\u590D\u5236\u547D\u4EE4</button>
          </div>
        </div>

        <div class="section">
          <h2 class="section-title">\u8BC4\u8BBA</h2>
          <div id="reviews-list">${Components.spinner()}</div>
        </div>

        <div class="section" id="review-form-section">
          <h2 class="section-title">\u53D1\u8868\u8BC4\u8BBA</h2>
          <form id="form-review">
            <div class="field">
              <label>\u8BC4\u5206</label>
              <div class="star-input">
                ${[1,2,3,4,5].map(i => `<input type="radio" name="rating" id="star${i}" value="${i}"><label for="star${i}">&#x2605;</label>`).join('')}
              </div>
            </div>
            <div class="field">
              <label>\u8BC4\u8BBA</label>
              <textarea name="comment" rows="3" placeholder="\u5206\u4EAB\u4F60\u7684\u4F7F\u7528\u4F53\u9A8C\u2026"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">\u63D0\u4EA4\u8BC4\u8BBA</button>
          </form>
          <div id="review-msg" class="msg"></div>
        </div>
      `;

      document.getElementById('btn-expand-content')?.addEventListener('click', () => {
        document.getElementById('skill-content-preview').textContent = asset.skill_content;
        document.getElementById('btn-expand-content').remove();
      });

      document.getElementById('btn-copy-cmd')?.addEventListener('click', () => {
        const cmd = document.getElementById('cmd-install').textContent;
        navigator.clipboard.writeText(cmd).then(() => {
          document.getElementById('btn-copy-cmd').textContent = '\u5DF2\u590D\u5236 \u2713';
          setTimeout(() => { document.getElementById('btn-copy-cmd').textContent = '\u590D\u5236\u547D\u4EE4'; }, 2000);
        });
      });

      if (!Auth.isLoggedIn()) {
        const formSection = document.getElementById('review-form-section');
        if (formSection) formSection.innerHTML = '<p style="color:#888;text-align:center;padding:16px;">\u767B\u5F55\u540E\u53EF\u53D1\u8868\u8BC4\u8BBA</p>';
      }

      document.getElementById('btn-download').addEventListener('click', (e) => {
        e.preventDefault();
        const downloadUrl = '/api/v1/public/skills/download/' + (asset.id || identifier);
        const key = localStorage.getItem('api_key');
        const headers = {};
        if (key) headers['Authorization'] = 'Bearer ' + key;

        fetch(downloadUrl, { headers })
          .then(res => {
            if (!res.ok) throw new Error('\u4E0B\u8F7D\u5931\u8D25 (' + res.status + ')');
            return res.blob();
          })
          .then(blob => {
            const url = URL.createObjectURL(blob);
            const a = document.createElement('a');
            a.href = url;
            a.download = assetName.replace(/[^a-zA-Z0-9\u4e00-\u9fff_-]/g, '_') + '.zip';
            a.click();
            URL.revokeObjectURL(url);
          })
          .catch(err => alert(err.message));
      });

      document.getElementById('btn-fav').addEventListener('click', async () => {
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        try {
          await API.post('/assets/' + (asset.id || identifier) + '/favorite', {});
          document.getElementById('btn-fav').textContent = '\u5DF2\u6536\u85CF \u2665';
        } catch (err) {
          alert(err.message);
        }
      });

      loadReviews(asset.id || identifier);

      document.getElementById('form-review')?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        const fd = new FormData(e.target);
        const msg = document.getElementById('review-msg');
        const ratingVal = parseInt(fd.get('rating'));
        if (!ratingVal) { msg.textContent = '\u8BF7\u9009\u62E9\u8BC4\u5206'; msg.className = 'msg error'; return; }
        try {
          await API.post('/assets/' + (asset.id || identifier) + '/reviews', {
            rating: ratingVal,
            comment: fd.get('comment') || '',
          });
          msg.textContent = '\u8BC4\u8BBA\u5DF2\u63D0\u4EA4\uFF01';
          msg.className = 'msg success';
          e.target.reset();
          loadReviews(asset.id || identifier);
        } catch (err) {
          msg.textContent = err.message;
          msg.className = 'msg error';
        }
      });

    } catch (err) {
      document.getElementById('asset-detail').innerHTML = Components.errorBox(err.message);
    }
  }

  async function loadReviews(id) {
    const el = document.getElementById('reviews-list');
    if (!el) return;
    try {
      let data;
      try {
        data = await API.get('/public/reviews/' + id, { public: true });
      } catch {
        data = await API.get('/assets/' + id + '/reviews');
      }
      const reviews = data.reviews || data.data || data || [];
      el.innerHTML = reviews.length
        ? reviews.map(Components.reviewCard).join('')
        : Components.emptyState('\u6682\u65E0\u8BC4\u8BBA\uFF0C\u6765\u5199\u7B2C\u4E00\u6761\u5427\uFF01');
    } catch {
      el.innerHTML = Components.emptyState('\u767B\u5F55\u540E\u53EF\u67E5\u770B\u8BC4\u8BBA');
    }
  }

  return { renderList, renderDetail };
})();
