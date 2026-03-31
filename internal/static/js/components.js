// 共享 UI 组件
const Components = (() => {

  function assetCard(asset) {
    const tags = (asset.tags || []).map(t => `<span class="tag">${escHtml(t)}</span>`).join('');
    const typeLabel = asset.asset_type === 'soul' ? 'SOUL' : 'SKILL';
    const typeClass = asset.asset_type === 'soul' ? 'badge-soul' : 'badge-skill';
    const rating = asset.average_rating ? Number(asset.average_rating).toFixed(1) : '—';
    const stars = asset.average_rating ? '★'.repeat(Math.round(asset.average_rating)) + '☆'.repeat(5 - Math.round(asset.average_rating)) : '☆☆☆☆☆';

    return `
      <div class="asset-card" data-id="${asset.id}" role="button" tabindex="0">
        <div class="card-header">
          <span class="badge ${typeClass}">${typeLabel}</span>
          <span class="card-rating" title="${rating}/5">${stars} ${rating}</span>
        </div>
        <h3 class="card-title">${escHtml(asset.name)}</h3>
        <p class="card-desc">${escHtml(asset.description || '')}</p>
        <div class="card-footer">
          <span class="card-author">@${escHtml(asset.author_name || asset.author_id || '')}</span>
          <span class="card-dl">↓ ${asset.install_count || 0}</span>
        </div>
        <div class="card-tags">${tags}</div>
      </div>
    `;
  }

  function reviewCard(review) {
    const stars = '★'.repeat(review.rating || 0) + '☆'.repeat(5 - (review.rating || 0));
    const date = review.created_at ? new Date(review.created_at).toLocaleDateString('zh-CN') : '';
    return `
      <div class="review-card">
        <div class="review-header">
          <span class="review-author">@${escHtml(review.reviewer_name || review.reviewer_id || '')}</span>
          <span class="review-stars">${stars}</span>
          <span class="review-date">${date}</span>
        </div>
        <p class="review-body">${escHtml(review.comment || '')}</p>
      </div>
    `;
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
