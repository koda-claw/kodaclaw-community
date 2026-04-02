// 管理员 Dashboard 页面
const DashboardPage = (() => {
  let chartInstance = null;

  async function renderPage(container) {
    if (!Auth.isLoggedIn() || !Auth.isAdmin()) {
      window.location.hash = '#/';
      return;
    }

    container.innerHTML = `
      <div class="dashboard-page">
        <div class="page-header">
          <h1 class="page-title"><i data-lucide="bar-chart-3" class="inline-icon"></i> 控制台</h1>
          <p class="page-sub">社区运营数据概览</p>
        </div>

        <div class="stats-grid" id="stats-grid">
          ${[1,2,3,4].map(() => `<div class="stat-card loading"><div class="stat-value">—</div><div class="stat-label">加载中…</div></div>`).join('')}
        </div>

        <div class="dashboard-row">
          <div class="card chart-container">
            <div class="chart-header">
              <h2 class="chart-title"><i data-lucide="trending-up" class="inline-icon"></i> 近期趋势</h2>
              <div class="chart-controls">
                <button class="btn btn-sm btn-outline trend-btn active" data-days="7">7 天</button>
                <button class="btn btn-sm btn-outline trend-btn" data-days="14">14 天</button>
                <button class="btn btn-sm btn-outline trend-btn" data-days="30">30 天</button>
              </div>
            </div>
            <div id="trend-chart" style="width:100%;height:280px;"></div>
          </div>

          <div class="card review-queue">
            <h2 class="chart-title"><i data-lucide="clock" class="inline-icon"></i> 审核队列</h2>
            <div id="review-queue-content">${Components.spinner()}</div>
          </div>
        </div>

        <div class="card">
          <h2 class="chart-title"><i data-lucide="clipboard-list" class="inline-icon"></i> 最近审核记录</h2>
          <div id="recent-reviews-content" class="recent-reviews">${Components.spinner()}</div>
        </div>
      </div>
    `;

    // Bind trend buttons
    container.querySelectorAll('.trend-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        container.querySelectorAll('.trend-btn').forEach(b => b.classList.remove('active'));
        btn.classList.add('active');
        loadTrends(parseInt(btn.dataset.days));
      });
    });

    loadStats();
    loadTrends(7);
    loadRecentReviews();
  }

  async function loadStats() {
    try {
      const data = await API.get('/admin/dashboard/stats');
      const grid = document.getElementById('stats-grid');
      if (!grid) return;
      grid.innerHTML = `
        <div class="stat-card">
          <div class="stat-icon"><i data-lucide="package" class="inline-icon"></i></div>
          <div class="stat-value">${data.total_assets ?? 0}</div>
          <div class="stat-label">总资产数</div>
          <div class="stat-sub">已上架 ${data.approved_assets ?? 0} / 待审核 <span class="${(data.pending_assets ?? 0) > 0 ? 'text-warning' : ''}">${data.pending_assets ?? 0}</span></div>
        </div>
        <div class="stat-card">
          <div class="stat-icon"><i data-lucide="users" class="inline-icon"></i></div>
          <div class="stat-value">${data.total_users ?? 0}</div>
          <div class="stat-label">KodaClaw 实例</div>
          <div class="stat-sub">活跃实例</div>
        </div>
        <div class="stat-card">
          <div class="stat-icon"><i data-lucide="download" class="inline-icon"></i><i data-lucide="" class="inline-icon"></i></div>
          <div class="stat-value">${data.total_downloads ?? 0}</div>
          <div class="stat-label">总下载量</div>
          <div class="stat-sub">累计安装次数</div>
        </div>
        <div class="stat-card ${(data.pending_versions ?? 0) > 0 ? 'stat-card--warn' : ''}">
          <div class="stat-icon"><i data-lucide="search" class="inline-icon"></i></div>
          <div class="stat-value ${(data.pending_versions ?? 0) > 0 ? 'text-warning' : ''}">${data.pending_versions ?? 0}</div>
          <div class="stat-label">待审核版本</div>
          <div class="stat-sub">
            ${(data.pending_versions ?? 0) > 0
              ? `<a href="#/admin-review" class="text-warning">立即处理 &rarr;</a>`
              : '暂无待审核'}
          </div>
        </div>
      `;
    } catch (e) {
      const grid = document.getElementById('stats-grid');
      if (grid) grid.innerHTML = `<div class="stat-card">${Components.errorBox('统计数据加载失败: ' + e.message)}</div>`;
    }
    refreshIcons();
  }

  async function loadTrends(days) {
    try {
      const data = await API.get(`/admin/dashboard/trends?days=${days}`);
      const items = data.data || [];
      renderChart(items);
    } catch (e) {
      const el = document.getElementById('trend-chart');
      if (el) el.innerHTML = Components.errorBox('趋势数据加载失败');
    }
  }

  function renderChart(items) {
    const el = document.getElementById('trend-chart');
    if (!el) return;

    if (typeof echarts === 'undefined') {
      el.innerHTML = '<p style="text-align:center;color:var(--text-muted);padding:2rem;">ECharts 未加载</p>';
      return;
    }

    if (chartInstance) {
      chartInstance.dispose();
    }
    chartInstance = echarts.init(el, 'dark');

    const dates = items.map(d => d.date);
    const newAssets = items.map(d => d.new_assets);
    const newUsers = items.map(d => d.new_users);
    const downloads = items.map(d => d.downloads);

    chartInstance.setOption({
      backgroundColor: 'transparent',
      tooltip: { trigger: 'axis' },
      legend: { data: ['新增资产', '新增实例', '下载量'], textStyle: { color: '#8b949e' } },
      grid: { left: '3%', right: '4%', bottom: '3%', containLabel: true },
      xAxis: {
        type: 'category',
        data: dates,
        axisLine: { lineStyle: { color: '#30363d' } },
        axisLabel: { color: '#8b949e', fontSize: 11 },
      },
      yAxis: {
        type: 'value',
        minInterval: 1,
        axisLine: { lineStyle: { color: '#30363d' } },
        splitLine: { lineStyle: { color: '#21262d' } },
        axisLabel: { color: '#8b949e' },
      },
      series: [
        {
          name: '新增资产',
          type: 'line',
          smooth: true,
          data: newAssets,
          itemStyle: { color: '#58a6ff' },
          areaStyle: { color: 'rgba(88,166,255,0.1)' },
        },
        {
          name: '新增实例',
          type: 'line',
          smooth: true,
          data: newUsers,
          itemStyle: { color: '#3fb950' },
          areaStyle: { color: 'rgba(63,185,80,0.1)' },
        },
        {
          name: '下载量',
          type: 'line',
          smooth: true,
          data: downloads,
          itemStyle: { color: '#f0883e' },
          areaStyle: { color: 'rgba(240,136,62,0.1)' },
        },
      ],
    });

    window.addEventListener('resize', () => { chartInstance && chartInstance.resize(); });
  }

  async function loadRecentReviews() {
    // Load review queue summary
    try {
      const data = await API.get('/admin/dashboard/stats');
      const el = document.getElementById('review-queue-content');
      if (!el) return;
      el.innerHTML = `
        <div class="queue-item">
          <span class="queue-label">待审核资产</span>
          <span class="queue-value ${(data.pending_assets ?? 0) > 0 ? 'text-warning' : 'text-muted'}">${data.pending_assets ?? 0}</span>
        </div>
        <div class="queue-item">
          <span class="queue-label">待审核版本</span>
          <span class="queue-value ${(data.pending_versions ?? 0) > 0 ? 'text-warning' : 'text-muted'}">${data.pending_versions ?? 0}</span>
        </div>
        <div class="queue-item">
          <span class="queue-label">已拒绝资产</span>
          <span class="queue-value text-muted">${data.rejected_assets ?? 0}</span>
        </div>
        ${(data.pending_assets > 0 || data.pending_versions > 0)
          ? `<a href="#/assets?status=pending" class="btn btn-sm btn-primary" style="margin-top:1rem;width:100%;text-align:center;">前往审核</a>`
          : ''}
      `;
    } catch {
      const el = document.getElementById('review-queue-content');
      if (el) el.innerHTML = Components.errorBox('加载失败');
    }

    // Load recent review records
    try {
      const data = await API.get('/admin/dashboard/recent-reviews?limit=20');
      const el = document.getElementById('recent-reviews-content');
      if (!el) return;
      const items = data.items || [];
      if (items.length === 0) {
        el.innerHTML = Components.emptyState('审核历史记录功能开发中，敬请期待');
        return;
      }
      el.innerHTML = items.map(item => `
        <div class="review-record">
          <span class="review-name">${Components.escHtml(item.name || '')}</span>
          <span class="badge badge-${item.status}">${item.status}</span>
          <span class="review-date">${item.updated_at ? new Date(item.updated_at).toLocaleDateString('zh-CN') : ''}</span>
        </div>
      `).join('');
    } catch {
      const el = document.getElementById('recent-reviews-content');
      if (el) el.innerHTML = Components.emptyState('审核历史记录功能开发中');
    }
  }

  return { renderPage };
})();
