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
        let path = `/assets?page=${page}&page_size=20`;
        if (q) path += `&q=${encodeURIComponent(q)}`;
        if (type) path += `&asset_type=${type}`;
        const data = await API.get(path);
        const assets = data.assets || data.data || data || [];
        if (!assets.length) {
          listEl.innerHTML = Components.emptyState('暂无资产');
          document.getElementById('pagination').innerHTML = '';
          return;
        }
        listEl.innerHTML = assets.map(Components.assetCard).join('');
        listEl.querySelectorAll('.asset-card').forEach(card => {
          card.addEventListener('click', () => {
            window.location.hash = '#/asset/' + card.dataset.id;
          });
          card.addEventListener('keydown', e => {
            if (e.key === 'Enter') window.location.hash = '#/asset/' + card.dataset.id;
          });
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

    // 搜索按钮
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

    // 热门标签
    try {
      const tagsData = await API.get('/tags/popular');
      const tags = tagsData.tags || tagsData || [];
      const cloud = document.getElementById('tag-cloud');
      if (tags.length) {
        cloud.innerHTML = tags.map(t => {
          const name = typeof t === 'string' ? t : (t.name || t.tag);
          return `<span class="tag tag-clickable" data-tag="${Components.escHtml(name)}">${Components.escHtml(name)}</span>`;
        }).join('');
        cloud.querySelectorAll('.tag-clickable').forEach(el => {
          el.addEventListener('click', () => {
            document.getElementById('search-q').value = el.dataset.tag;
            document.getElementById('btn-search').click();
          });
        });
      }
    } catch { /* 标签云加载失败不影响主列表 */ }

    load();
  }

  // ---- 详情页 ----
  async function renderDetail(container, id) {
    container.innerHTML = `
      <div class="back-link"><a href="#/assets">← 返回列表</a></div>
      <div id="asset-detail">${Components.spinner()}</div>
    `;

    try {
      const data = await API.get('/assets/' + id);
      const asset = data.asset || data;
      const tags = (asset.tags || []).map(t => `<span class="tag">${Components.escHtml(t)}</span>`).join('');
      const typeLabel = asset.asset_type === 'soul' ? 'SOUL' : 'SKILL';
      const typeClass = asset.asset_type === 'soul' ? 'badge-soul' : 'badge-skill';
      const rating = asset.average_rating ? Number(asset.average_rating).toFixed(1) : '—';
      const stars = asset.average_rating ? '★'.repeat(Math.round(asset.average_rating)) + '☆'.repeat(5 - Math.round(asset.average_rating)) : '☆☆☆☆☆';

      document.getElementById('asset-detail').innerHTML = `
        <div class="detail-card">
          <div class="detail-header">
            <span class="badge ${typeClass}">${typeLabel}</span>
            <h1 class="detail-title">${Components.escHtml(asset.name)}</h1>
            <div class="detail-meta">
              <span>作者：@${Components.escHtml(asset.author_name || asset.author_id || '')}</span>
              <span>下载：↓ ${asset.install_count || 0}</span>
              <span>评分：${stars} ${rating}</span>
            </div>
          </div>

          <p class="detail-desc">${Components.escHtml(asset.description || '')}</p>
          <div class="detail-tags">${tags}</div>

          ${asset.content_preview ? `<div class="detail-preview"><pre>${Components.escHtml(asset.content_preview)}</pre></div>` : ''}

          <div class="detail-actions">
            <a href="/api/v1/assets/${id}/download" class="btn btn-primary" id="btn-download">下载</a>
            <button class="btn btn-outline" id="btn-fav">收藏</button>
          </div>
        </div>

        <div class="section">
          <h2 class="section-title">评论</h2>
          <div id="reviews-list">${Components.spinner()}</div>
        </div>

        <div class="section">
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

      // 下载按钮加上 auth header
      document.getElementById('btn-download').addEventListener('click', (e) => {
        e.preventDefault();
        const key = localStorage.getItem('api_key');
        if (!key) { window.location.hash = '#/login'; return; }
        // 用 fetch 下载
        fetch('/api/v1/assets/' + id + '/download', {
          headers: { 'Authorization': 'Bearer ' + key }
        }).then(res => {
          if (!res.ok) throw new Error('下载失败');
          return res.blob();
        }).then(blob => {
          const url = URL.createObjectURL(blob);
          const a = document.createElement('a');
          a.href = url;
          a.download = asset.name + '.yaml';
          a.click();
          URL.revokeObjectURL(url);
        }).catch(err => alert(err.message));
      });

      // 收藏
      document.getElementById('btn-fav').addEventListener('click', async () => {
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        try {
          await API.post('/assets/' + id + '/favorite', {});
          document.getElementById('btn-fav').textContent = '已收藏 ♥';
        } catch (err) {
          alert(err.message);
        }
      });

      // 加载评论
      loadReviews(id);

      // 发表评论
      document.getElementById('form-review').addEventListener('submit', async (e) => {
        e.preventDefault();
        if (!Auth.isLoggedIn()) { window.location.hash = '#/login'; return; }
        const fd = new FormData(e.target);
        const msg = document.getElementById('review-msg');
        const rating = parseInt(fd.get('rating'));
        if (!rating) { msg.textContent = '请选择评分'; msg.className = 'msg error'; return; }
        try {
          await API.post('/assets/' + id + '/reviews', {
            rating,
            comment: fd.get('comment') || '',
          });
          msg.textContent = '评论已提交！';
          msg.className = 'msg success';
          e.target.reset();
          loadReviews(id);
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
      const data = await API.get('/assets/' + id + '/reviews');
      const reviews = data.reviews || data.data || data || [];
      el.innerHTML = reviews.length
        ? reviews.map(Components.reviewCard).join('')
        : Components.emptyState('暂无评论，来写第一条吧！');
    } catch (err) {
      el.innerHTML = Components.errorBox(err.message);
    }
  }

  return { renderList, renderDetail };
})();
