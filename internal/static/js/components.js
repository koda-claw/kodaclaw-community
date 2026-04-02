// 共享 UI 组件
const Components = (() => {

  // 配置 marked
  marked.setOptions({ gfm: true, breaks: true });
  marked.use({
    renderer: {
      code: function(token) {
        var codeText = (token && typeof token === 'object') ? token.text : token;
        var lang = ((token && typeof token === 'object') ? token.lang : arguments[1]) || '';
        var langDisplay = lang || 'text';
        var escaped = String(codeText)
          .replace(/&/g, '&amp;')
          .replace(/</g, '&lt;')
          .replace(/>/g, '&gt;')
          .replace(/"/g, '&quot;');
        var copyFn = "navigator.clipboard.writeText(this.closest('.code-block').querySelector('code').textContent).then(()=>{this.textContent='\u5df2\u590d\u5236';setTimeout(()=>{this.textContent='\u590d\u5236'},2000)})";
        return '<div class="code-block"><div class="code-header"><span class="code-lang">' + langDisplay + '</span><button class="code-copy-btn" onclick="' + copyFn + '">\u590d\u5236</button></div><pre><code class="language-' + lang + '">' + escaped + '</code></pre></div>';
      }
    }
  });

  function assetCard(asset) {
    const tags = (asset.tags || []).map(t =>
      `<span class="tag">${escHtml(t)}</span>`
    ).join('');
    const assetType = asset.type || asset.asset_type;
    const typeLabel = assetType === 'soul' ? 'SOUL' : 'SKILL';
    const typeClass = assetType === 'soul' ? 'badge-soul' : 'badge-skill';
    const ratingVal = Number(asset.avg_rating || asset.average_rating || 0);
    const rating = ratingVal ? ratingVal.toFixed(1) : '—';
    const stars = ratingVal ? '★'.repeat(Math.round(ratingVal)) + '☆'.repeat(5 - Math.round(ratingVal)) : '☆☆☆☆☆';

    return `
      <div class="asset-card" data-id="${asset.id}" data-name="${escHtml(asset.name)}" role="button" tabindex="0">
        <div class="card-header">
          <span class="badge ${typeClass}">${typeLabel}</span>
          <span class="card-rating" title="${rating}/5">${stars} ${rating}</span>
        </div>
        <h3 class="card-title">${escHtml(asset.name)}</h3>
        <p class="card-desc">${escHtml(asset.description || '')}</p>
        <div class="card-footer">
          <span class="card-author">@${escHtml(asset.author_name || asset.author_id || '')}</span>
          <span class="card-dl">↓ ${asset.install_count || asset.download_count || 0}</span>
        </div>
        <div class="card-tags">${tags}</div>
      </div>
    `;
  }

    function reviewCard(review) {
    const scores = [review.usefulness, review.security, review.compatibility].filter(s => s && s > 0);
    const avg = scores.length ? scores.reduce((a, b) => a + b, 0) / scores.length : 0;
    const stars = '\u2605'.repeat(Math.round(avg)) + '\u2606'.repeat(5 - Math.round(avg));
    const date = review.created_at ? new Date(review.created_at).toLocaleDateString('zh-CN') : '';
    const dimLabel = (v, label) => v ? '<span class="dim-score" title="' + label + ' ' + v + '/5">' + label + ': ' + v + '</span>' : '';
    return '\n      <div class="review-card">\n        <div class="review-header">\n          <span class="review-author">@' + escHtml(review.username || review.user_id || '') + '</span>\n          <span class="review-stars">' + stars + (avg ? ' ' + avg.toFixed(1) : '') + '</span>\n          <span class="review-date">' + date + '</span>\n        </div>\n        <div class="review-body">' + DOMPurify.sanitize(marked.parse(review.content || '')) + '</div>\n        ' + (scores.length ? '<div class="review-scores">' + dimLabel(review.usefulness, '\u5b9e\u7528\u6027') + dimLabel(review.security, '\u5b89\u5168\u6027') + dimLabel(review.compatibility, '\u517c\u5bb9\u6027') + '</div>' : '') + '\n      </div>\n    ';
  }


  function notificationItem(n) {
    const date = n.created_at ? new Date(n.created_at).toLocaleString('zh-CN') : '';
    return `
      <div class="notif-item ${n.is_read ? '' : 'unread'}" data-id="${n.id}">
        <div class="notif-type">${escHtml(n.type || '')}</div>
        <div class="notif-msg">${escHtml(n.message || '')}</div>
        <div class="notif-date">${date}</div>
      </div>
    `;
  }

  function spinner() {
    return '<div class="spinner"></div>';
  }

  function errorBox(msg) {
    return `<div class="error-box">${escHtml(msg)}</div>`;
  }

  function emptyState(msg) {
    return `<div class="empty-state">${escHtml(msg)}</div>`;
  }

  function escHtml(str) {
    if (!str) return '';
    return String(str)
      .replace(/&/g, '&amp;')
      .replace(/</g, '&lt;')
      .replace(/>/g, '&gt;')
      .replace(/"/g, '&quot;');
  }

  return { assetCard, reviewCard, notificationItem, spinner, errorBox, emptyState, escHtml };
})();
