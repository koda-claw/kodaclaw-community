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
        <button id="btn-search" class="btn btn-primary">搜索</button>
      </div>
      <div id="tag-cloud" class="tag-cloud"></div>
      <div id="asset-list" class="asset-grid">${Components.spinner()}</div>
      <div id="pagination" class="pagination"></div>
    `;

    let page = 1;
    let q = '';
    let type = '';

    async function load() {
      const listEl = document.getElementById('asset-list');
      listEl.innerHTML = Components.spinner();
      try {
        let path = `/public/skills?page=${page}&page_size=20`;
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
            // Use name for public API, store id as fallback
            window.location.hash = '#/asset/' + encodeURIComponent(card.dataset.name || card.dataset.id);
          };
          card.addEventListener('click', target);
          card.addEventListener('keydown', e => { if (e.key === 'Enter') target(); });
        });
        // 简单分页
        const total = data.total || assets.length;
        const pages = Math.ceil(total / 20);
        const pagEl = document.getElementById('pagination');
        if (pages > 1) {
          let html = '';
          if (page > 1) html += `<button class="btn btn-sm" id="pg-prev">上一页</button>`;
          html += `<span class="pg-info">第 ${page} / ${pages} 页</span>`;
          if (page < pages) html += `<button class="btn btn-sm" id="pg-next">下一页</button>`;
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

    document.getElementById('btn-search').addEventListener('click', () => {
      q = document.getElementById('search-q').value.trim();
      type = document.getElementById('filter-type').value;
      page = 1;
      load();
    });
    document.getElementById('search-q').addEventListener('keydown', e => {
      if (e.key === 'Enter') document.getElementById('btn-search').click();
    });
    document.getElementById('filter-type').addEventListener('change', () => {
      document.getElementById('btn-search').click();
    });

    // 热门标签 (extract from public list)
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
      }
    } catch { /* ignore */ }

    load();
  }

  // ---- 详情页 ----
  async function renderDetail(container, identifier) {
    container.innerHTML = `
      <div class="back-link"><a href="#/assets">← 返回列表</a></div>
      <div id="asset-detail">${Components.spinner()}</div>
    `;

    try {
      // identifier can be a name (URL-encoded) or UUID
      let asset;
      let isPublic = false;

      // Try public API by name first
      try {
        const data = await API.get('/public/skills/' + identifier, { public: true });
        asset = data.asset || data;
        isPublic = true;
      } catch {
        // Fallback to authenticated API by UUID
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
      const rating = ratingVal ? ratingVal.toFixed(1) : '—';
      const stars = ratingVal ? '★'.repeat(Math.round(ratingVal)) + '☆'.repeat(5 - Math.round(ratingVal)) : '☆☆☆☆☆';
      const assetName = asset.name || identifier;

      document.getElementById('asset-detail').innerHTML = `
        <div class="detail-card">
          <div class="detail-header">
            <span class="badge ${typeClass}">${typeLabel}</span>
            <h1 class="detail-title">${Components.escHtml(assetName)}</h1>
            <div class="detail-meta">
              <span>作者：@${Components.escHtml(asset.author_name || asset.author_id || '')}</span>
              <span>下载：↓ ${asset.install_count || asset.download_count || 0}</span>
              <span>评分：${stars} ${rating}</span>
            </div>
          </div>

          <p class="detail-desc">${Components.escHtml(asset.description || '')}</p>
          <div class="detail-tags">${tags}</div>

          ${asset.skill_content ? (() => {
            const full = asset.skill_content;
            const preview = full.length > 500 ? full.slice(0, 500) : null;
            return `<div class="detail-preview">
              <pre id="skill-content-preview">${Components.escHtml(preview || full)}</pre>
              ${preview ? `<button class="btn btn-sm" id="btn-expand-content">展开完整内容</button>` : ''}
            </div>`;
          })() : ''}

          <div class="detail-actions">
            <button class="btn btn-primary" id="btn-download">下载</button>
            <button class="btn btn-outline" id="btn-fav">收藏</button>
          </div>
        </div>

        <div class="section">
          <h2 class="section-title">评论</h2>
          <div id="reviews-list">${Components.spinner()}</div>
        </div>

        <div class="section" id="review-form-section">
          <h2 class="section-title">发表评论</h2>
          <form id="form-review">
            <div class="field">
              <label>评分</label>
              <div class="star-input">
                ${[1,2,3,4,5].map(i => `<input type="radio" name="rating" id="star${i}" value="${i}"><label for="star${i}">★</label>`).join('')}
              </div>
            </div>
            <div class="field">
              <label>评论</label>
              <textarea name="comment" rows="3" placeholder="分享你的使用体验…"></textarea>
            </div>
            <button type="submit" class="btn btn-primary">提交评论</button>
          </form>
          <div id="review-msg" class="msg"></div>
        </div>
      `;

      // Expand skill content
      document.getElementById('btn-expand-content')?.addEventListener('click', () => {
        document.getElementById('skill-content-preview').textContent = asset.skill_content;
        document.getElementById('btn-expand-content').remove();
      });

      // Hide review form if not logged in
      if (!Auth.isLoggedIn()) {
        const formSection = document.getElementById('review-form-section');
        if (formSection) formSection.innerHTML = '<p style="color:#888;text-align:center;padding:16px;">登录后可发表评论</p>';
      }

      // Download button - public download, no auth needed
      document.getElementById('btn-download').addEventListener('click', (e) => {
        e.preventDefault();
        const downloadUrl = '/api/v1/public/skills/download/' + (asset.id || identifier);
        const key = localStorage.getItem('api_key');
        const headers = {};
        if (key) headers['Authorization'] = 'Bearer ' + key;

        fetch(downloadUrl, { headers })
          .then(res => {
            if (!res.ok) throw new Error('下载失败 (' + res.status + ')');
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

      // Favorite
      document.getElementById('btn-fav').addEventListener('click', async () => {
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        try {
          await API.post('/assets/' + (asset.id || identifier) + '/favorite', {});
          document.getElementById('btn-fav').textContent = '已收藏 ♥';
        } catch (err) {
          alert(err.message);
        }
      });

      // Load reviews
      loadReviews(asset.id || identifier);

      // Submit review
      document.getElementById('form-review')?.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        const fd = new FormData(e.target);
        const msg = document.getElementById('review-msg');
        const ratingVal = parseInt(fd.get('rating'));
        if (!ratingVal) { msg.textContent = '请选择评分'; msg.className = 'msg error'; return; }
        try {
          await API.post('/assets/' + (asset.id || identifier) + '/reviews', {
            rating: ratingVal,
            comment: fd.get('comment') || '',
          });
          msg.textContent = '评论已提交！';
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
        : Components.emptyState('暂无评论，来写第一条吧！');
    } catch {
      el.innerHTML = Components.emptyState('登录后可查看评论');
    }
  }

  return { renderList, renderDetail };
})();
