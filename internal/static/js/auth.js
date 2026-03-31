// 登录/注册/认证逻辑
const Auth = (() => {
  function isLoggedIn() {
    return !!localStorage.getItem('api_key');
  }

  function getUser() {
    const u = localStorage.getItem('user');
    return u ? JSON.parse(u) : null;
  }

  function logout() {
    localStorage.removeItem('api_key');
    localStorage.removeItem('user');
    window.location.hash = '#/login';
  }

  function renderPage(container) {
    container.innerHTML = `
      <div class="auth-wrap">
        <div class="auth-card">
          <div class="brand-logo">
            <span class="logo-kc">KC</span>
            <span class="logo-text">KodaClaw Community</span>
          </div>
          <div class="auth-tabs">
            <button class="tab-btn active" data-tab="login">登录</button>
            <button class="tab-btn" data-tab="register">注册</button>
          </div>

          <div id="tab-login" class="tab-content active">
            <form id="form-login">
              <div class="field">
                <label>用户名</label>
                <input type="text" name="username" required placeholder="输入用户名" />
              </div>
              <div class="field">
                <label>密码</label>
                <input type="password" name="password" required placeholder="输入密码" />
              </div>
              <button type="submit" class="btn btn-primary w-full">登录</button>
            </form>
            <div class="divider">或</div>
            <button id="btn-github-login" class="btn btn-github w-full">
              <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor" style="vertical-align:middle;margin-right:6px"><path d="M12 0C5.37 0 0 5.37 0 12c0 5.3 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 21.795 24 17.295 24 12c0-6.63-5.37-12-12-12"/></svg>
              使用 GitHub 登录
            </button>
          </div>

          <div id="tab-register" class="tab-content">
            <form id="form-register">
              <div class="field">
                <label>用户名</label>
                <input type="text" name="username" required placeholder="4-30 个字符" />
              </div>
              <div class="field">
                <label>密码</label>
                <input type="password" name="password" required placeholder="至少 8 位" />
              </div>
              <div class="field">
              </div>
              <div class="field" id="email-field">
                <label>邮箱（可选）</label>
                <input type="email" name="email" placeholder="your@email.com" />
              </div>
              <button type="submit" class="btn btn-primary w-full">注册</button>
            </form>
          </div>

          <div id="auth-msg" class="msg"></div>
        </div>
      </div>
    `;

    // Tab 切换
    container.querySelectorAll('.tab-btn').forEach(btn => {
      btn.addEventListener('click', () => {
        container.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
        container.querySelectorAll('.tab-content').forEach(t => t.classList.remove('active'));
        btn.classList.add('active');
        container.querySelector('#tab-' + btn.dataset.tab).classList.add('active');
        document.getElementById('auth-msg').textContent = '';
      });
    });

    // GitHub 登录
    document.getElementById('btn-github-login').addEventListener('click', async () => {
      const msg = document.getElementById('auth-msg');
      try {
        const res = await API.get('/auth/github');
        window.location.href = res.url;
      } catch (err) {
        msg.textContent = 'GitHub 登录失败：' + err.message;
        msg.className = 'msg error';
      }
    });

    // 登录
    document.getElementById('form-login').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const msg = document.getElementById('auth-msg');
      try {
        const res = await API.post('/auth/login', {
          username: fd.get('username'),
          password: fd.get('password'),
        });
        localStorage.setItem('api_key', res.api_key);
        localStorage.setItem('user', JSON.stringify(res.user || { username: fd.get('username') }));
        window.location.hash = '#/assets';
      } catch (err) {
        msg.textContent = err.message;
        msg.className = 'msg error';
      }
    });

    // 注册
    document.getElementById('form-register').addEventListener('submit', async (e) => {
      e.preventDefault();
      const fd = new FormData(e.target);
      const msg = document.getElementById('auth-msg');
      const body = {
        username: fd.get('username'),
        password: fd.get('password'),
      };
      const email = fd.get('email');
      if (email) body.email = email;
      try {
        const res = await API.post('/auth/register', body);
        localStorage.setItem('api_key', res.api_key);
        localStorage.setItem('user', JSON.stringify(res.user || { username: fd.get('username') }));
        msg.textContent = '注册成功！正在跳转…';
        msg.className = 'msg success';
        setTimeout(() => { window.location.hash = '#/assets'; }, 800);
      } catch (err) {
        msg.textContent = err.message;
        msg.className = 'msg error';
      }
    });
  }

  return { isLoggedIn, getUser, logout, renderPage };
})();
