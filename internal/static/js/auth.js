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
