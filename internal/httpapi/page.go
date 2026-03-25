package httpapi

const homePageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Weixin Bridge Console</title>
  <style>
    :root {
      --bg: #f3efe5;
      --panel: rgba(255, 251, 243, 0.96);
      --ink: #1f2a1f;
      --muted: #566156;
      --accent: #1d7a43;
      --accent-soft: #d9efdf;
      --warn: #b2522a;
      --warn-soft: #f7dfd4;
      --border: rgba(31, 42, 31, 0.12);
      --shadow: 0 20px 50px rgba(39, 48, 39, 0.12);
      --radius: 20px;
    }

    * { box-sizing: border-box; }
    body {
      margin: 0;
      font-family: "SF Pro Display", "PingFang SC", "Helvetica Neue", sans-serif;
      color: var(--ink);
      background:
        radial-gradient(circle at top left, rgba(29, 122, 67, 0.16), transparent 28%),
        radial-gradient(circle at top right, rgba(178, 82, 42, 0.12), transparent 30%),
        linear-gradient(180deg, #faf6ee 0%, #f2ecdf 100%);
      min-height: 100vh;
    }

    .shell {
      max-width: 1180px;
      margin: 0 auto;
      padding: 32px 20px 48px;
    }

    .hero {
      display: grid;
      gap: 14px;
      margin-bottom: 24px;
    }

    .eyebrow {
      font-size: 12px;
      letter-spacing: 0.18em;
      text-transform: uppercase;
      color: var(--accent);
      font-weight: 700;
    }

    h1 {
      margin: 0;
      font-size: clamp(30px, 4vw, 48px);
      line-height: 1.05;
    }

    .hero p {
      margin: 0;
      max-width: 760px;
      color: var(--muted);
      font-size: 16px;
      line-height: 1.6;
    }

    .grid {
      display: grid;
      grid-template-columns: 1.2fr 0.9fr;
      gap: 20px;
    }

    .stack {
      display: grid;
      gap: 20px;
    }

    .panel {
      background: var(--panel);
      border: 1px solid var(--border);
      border-radius: var(--radius);
      box-shadow: var(--shadow);
      padding: 20px;
      backdrop-filter: blur(10px);
    }

    .panel h2 {
      margin: 0 0 8px;
      font-size: 20px;
    }

    .panel p.lead {
      margin: 0 0 18px;
      color: var(--muted);
      line-height: 1.5;
    }

    .stats {
      display: grid;
      grid-template-columns: repeat(2, minmax(0, 1fr));
      gap: 12px;
    }

    .stat {
      border: 1px solid var(--border);
      border-radius: 16px;
      padding: 14px;
      background: rgba(255,255,255,0.48);
    }

    .stat label {
      display: block;
      color: var(--muted);
      font-size: 12px;
      margin-bottom: 8px;
      text-transform: uppercase;
      letter-spacing: 0.08em;
    }

    .stat .value {
      font-size: 15px;
      line-height: 1.5;
      word-break: break-word;
    }

    .actions {
      display: flex;
      flex-wrap: wrap;
      gap: 10px;
      margin-top: 18px;
    }

    button {
      appearance: none;
      border: 0;
      border-radius: 999px;
      padding: 12px 18px;
      font-size: 14px;
      font-weight: 700;
      cursor: pointer;
      transition: transform 140ms ease, opacity 140ms ease, box-shadow 140ms ease;
    }

    button:hover { transform: translateY(-1px); }
    button:disabled { opacity: 0.55; cursor: wait; transform: none; }

    .primary {
      background: var(--accent);
      color: #fff;
      box-shadow: 0 12px 24px rgba(29, 122, 67, 0.22);
    }

    .secondary {
      background: #fff;
      color: var(--ink);
      border: 1px solid var(--border);
    }

    .danger {
      background: var(--warn);
      color: #fff;
      box-shadow: 0 12px 24px rgba(178, 82, 42, 0.2);
    }

    .pill {
      display: inline-flex;
      align-items: center;
      gap: 8px;
      padding: 8px 12px;
      border-radius: 999px;
      font-size: 13px;
      font-weight: 700;
      background: rgba(255,255,255,0.75);
      border: 1px solid var(--border);
    }

    .pill.ok { background: var(--accent-soft); color: var(--accent); }
    .pill.warn { background: var(--warn-soft); color: var(--warn); }

    .qr-wrap {
      display: grid;
      justify-items: center;
      gap: 14px;
      min-height: 360px;
      border: 1px dashed rgba(31, 42, 31, 0.18);
      border-radius: 24px;
      padding: 20px;
      background:
        linear-gradient(135deg, rgba(255,255,255,0.82), rgba(255,255,255,0.58));
    }

    .qr-wrap img {
      max-width: min(100%, 320px);
      max-height: 320px;
      border-radius: 20px;
      border: 10px solid #fff;
      background: #fff;
      box-shadow: 0 16px 40px rgba(31, 42, 31, 0.12);
    }

    .qr-wrap canvas {
      max-width: min(100%, 320px);
      max-height: 320px;
      border-radius: 20px;
      border: 10px solid #fff;
      background: #fff;
      box-shadow: 0 16px 40px rgba(31, 42, 31, 0.12);
    }

    .qr-placeholder {
      align-self: center;
      color: var(--muted);
      text-align: center;
      line-height: 1.7;
      padding: 26px;
    }

    .hint {
      color: var(--muted);
      font-size: 14px;
      line-height: 1.6;
      text-align: center;
    }

    .message {
      margin-top: 18px;
      padding: 14px 16px;
      border-radius: 16px;
      background: rgba(255,255,255,0.72);
      border: 1px solid var(--border);
      color: var(--ink);
      line-height: 1.6;
      min-height: 56px;
    }

    .message.error {
      background: var(--warn-soft);
      color: var(--warn);
      border-color: rgba(178, 82, 42, 0.18);
    }

    details {
      margin-top: 18px;
      border: 1px solid var(--border);
      border-radius: 18px;
      padding: 14px 16px;
      background: rgba(255,255,255,0.56);
    }

    details summary {
      cursor: pointer;
      font-weight: 700;
    }

    pre {
      margin: 12px 0 0;
      padding: 14px;
      border-radius: 14px;
      background: #1f2520;
      color: #def5df;
      overflow: auto;
      font-size: 12px;
      line-height: 1.55;
    }

    @media (max-width: 920px) {
      .grid {
        grid-template-columns: 1fr;
      }

      .stats {
        grid-template-columns: 1fr;
      }
    }
  </style>
</head>
<body>
  <div class="shell">
    <section class="hero">
      <div class="eyebrow">Agent Weixin Bridge</div>
      <h1>微信桥接控制台</h1>
      <p>在这个页面里直接完成二维码登录、扫码状态轮询、桥接状态查看与消息轮询控制。适合本机或内网环境下快速把微信机器人接到 runner。</p>
    </section>

    <section class="grid">
      <div class="stack">
        <article class="panel">
          <h2>服务与账号状态</h2>
          <p class="lead">这里展示 bridge 健康状态、当前微信账号绑定情况、轮询状态以及最近一次收发消息和错误。</p>
          <div class="stats">
            <div class="stat"><label>服务健康</label><div class="value" id="healthStatus">检查中...</div></div>
            <div class="stat"><label>微信已配置</label><div class="value" id="configuredStatus">检查中...</div></div>
            <div class="stat"><label>当前轮询</label><div class="value" id="pollingStatus">检查中...</div></div>
            <div class="stat"><label>账号 ID</label><div class="value" id="accountId">-</div></div>
            <div class="stat"><label>最近收消息</label><div class="value" id="lastInboundAt">-</div></div>
            <div class="stat"><label>最近发消息</label><div class="value" id="lastOutboundAt">-</div></div>
            <div class="stat"><label>最近错误</label><div class="value" id="lastError">-</div></div>
            <div class="stat"><label>扫码提示</label><div class="value" id="loginHint">点击“生成二维码”开始。</div></div>
          </div>
          <div class="actions">
            <button class="secondary" id="refreshStatusBtn">刷新状态</button>
            <button class="primary" id="startPollBtn">开始轮询</button>
            <button class="danger" id="stopPollBtn">停止轮询</button>
          </div>
        </article>

        <article class="panel">
          <h2>调试面板</h2>
          <p class="lead">这里保留最近一次关键接口的原始 JSON 响应，方便排障和对照后端返回。</p>
          <details open>
            <summary>login/start 响应</summary>
            <pre id="loginStartDebug">尚未调用</pre>
          </details>
          <details>
            <summary>login/status 响应</summary>
            <pre id="loginStatusDebug">尚未调用</pre>
          </details>
          <details>
            <summary>account 响应</summary>
            <pre id="accountDebug">尚未调用</pre>
          </details>
        </article>
      </div>

      <div class="stack">
        <article class="panel">
          <h2>微信二维码登录</h2>
          <p class="lead">点击生成二维码后，用微信扫码；扫码后页面会自动轮询状态，并在确认成功后刷新账号信息。</p>
          <div class="actions" style="margin-top:0">
            <button class="primary" id="startLoginBtn">生成二维码</button>
            <span class="pill" id="sessionBadge">当前无 session</span>
          </div>
          <div class="qr-wrap" style="margin-top:18px">
            <img id="qrImage" alt="微信登录二维码" style="display:none">
            <canvas id="qrCanvas" width="320" height="320" style="display:none"></canvas>
            <div class="qr-placeholder" id="qrPlaceholder">
              还没有二维码。<br>
              点击上方按钮后，这里会直接显示微信扫码图。
            </div>
            <div class="hint">
              <div>会话：<span id="sessionKey">-</span></div>
              <div>过期时间：<span id="expiresAt">-</span></div>
              <div>扫码状态：<span id="loginState">未开始</span></div>
              <div>二维码源：<span id="qrSourceType">-</span></div>
            </div>
          </div>
          <div class="message" id="messageBox">页面加载后会自动刷新 bridge 状态。二维码 session 不会在刷新后保留。</div>
        </article>
      </div>
    </section>
  </div>

  <script src="/assets/qrcode.js"></script>
  <script>
    (function () {
      var state = {
        sessionKey: '',
        qrCodeUrl: '',
        loginTimer: null,
        qrRenderToken: 0,
        lastResponses: {
          loginStart: null,
          loginStatus: null,
          account: null
        }
      };

      var els = {
        healthStatus: document.getElementById('healthStatus'),
        configuredStatus: document.getElementById('configuredStatus'),
        pollingStatus: document.getElementById('pollingStatus'),
        accountId: document.getElementById('accountId'),
        lastInboundAt: document.getElementById('lastInboundAt'),
        lastOutboundAt: document.getElementById('lastOutboundAt'),
        lastError: document.getElementById('lastError'),
        loginHint: document.getElementById('loginHint'),
        messageBox: document.getElementById('messageBox'),
        qrImage: document.getElementById('qrImage'),
        qrCanvas: document.getElementById('qrCanvas'),
        qrPlaceholder: document.getElementById('qrPlaceholder'),
        sessionBadge: document.getElementById('sessionBadge'),
        sessionKey: document.getElementById('sessionKey'),
        expiresAt: document.getElementById('expiresAt'),
        loginState: document.getElementById('loginState'),
        qrSourceType: document.getElementById('qrSourceType'),
        loginStartDebug: document.getElementById('loginStartDebug'),
        loginStatusDebug: document.getElementById('loginStatusDebug'),
        accountDebug: document.getElementById('accountDebug'),
        startLoginBtn: document.getElementById('startLoginBtn'),
        refreshStatusBtn: document.getElementById('refreshStatusBtn'),
        startPollBtn: document.getElementById('startPollBtn'),
        stopPollBtn: document.getElementById('stopPollBtn')
      };

      function setMessage(text, isError) {
        els.messageBox.textContent = text;
        els.messageBox.className = isError ? 'message error' : 'message';
      }

      function setSessionBadge(text, kind) {
        els.sessionBadge.textContent = text;
        els.sessionBadge.className = kind ? 'pill ' + kind : 'pill';
      }

      function setBusy(button, busy) {
        button.disabled = busy;
      }

      function formatTime(value) {
        if (!value) return '-';
        var date = new Date(value);
        if (isNaN(date.getTime())) return String(value);
        return date.toLocaleString();
      }

      function pretty(value) {
        if (!value) return '尚未调用';
        return JSON.stringify(value, null, 2);
      }

      function compactBase64(value) {
        return String(value || '').replace(/\s+/g, '');
      }

      function isLikelyBase64Image(value) {
        var normalized = compactBase64(value);
        if (!normalized || normalized.length < 80 || normalized.length % 4 !== 0) {
          return false;
        }
        return /^[A-Za-z0-9+/]+=*$/.test(normalized);
      }

      function isHTTPURL(value) {
        try {
          var parsed = new URL(value, window.location.href);
          return parsed.protocol === 'http:' || parsed.protocol === 'https:';
        } catch (error) {
          return false;
        }
      }

      function isLiteAppQRCodeURL(value) {
        if (!isHTTPURL(value)) {
          return false;
        }
        try {
          var parsed = new URL(value, window.location.href);
          return parsed.hostname === 'liteapp.weixin.qq.com' && parsed.pathname.indexOf('/q/') === 0;
        } catch (error) {
          return false;
        }
      }

      function classifyQRCodeValue(raw) {
        var value = String(raw || '').trim();
        if (!value) return 'empty';
        if (value.indexOf('data:') === 0) return 'data-url';
        if (isLikelyBase64Image(value)) return 'base64-image';
        if (isHTTPURL(value)) {
          return isLiteAppQRCodeURL(value) ? 'liteapp-url' : 'http-url';
        }
        return 'unsupported';
      }

      function clearQrMedia() {
        els.qrImage.onload = null;
        els.qrImage.onerror = null;
        els.qrImage.removeAttribute('src');
        els.qrImage.style.display = 'none';
        var context = els.qrCanvas.getContext('2d');
        context.clearRect(0, 0, els.qrCanvas.width, els.qrCanvas.height);
        els.qrCanvas.style.display = 'none';
      }

      function beginQrRender() {
        state.qrRenderToken += 1;
        clearQrMedia();
        els.qrPlaceholder.style.display = 'block';
        return state.qrRenderToken;
      }

      function isActiveQrRender(renderToken) {
        return renderToken === state.qrRenderToken;
      }

      function failQrRender(renderToken, message) {
        if (!isActiveQrRender(renderToken)) {
          return;
        }
        clearQrMedia();
        els.qrPlaceholder.style.display = 'block';
        els.qrSourceType.textContent = 'render-error';
        setMessage(message, true);
      }

      function finishImageRender(renderToken, sourceType) {
        if (!isActiveQrRender(renderToken)) {
          return;
        }
        els.qrImage.style.display = 'block';
        els.qrPlaceholder.style.display = 'none';
        els.qrSourceType.textContent = sourceType;
        setMessage('二维码已就绪，请使用微信扫码并确认。', false);
      }

      function finishCanvasRender(renderToken, sourceType) {
        if (!isActiveQrRender(renderToken)) {
          return;
        }
        els.qrCanvas.style.display = 'block';
        els.qrPlaceholder.style.display = 'none';
        els.qrSourceType.textContent = sourceType;
        setMessage('二维码已就绪，请使用微信扫码并确认。', false);
      }

      function loadImageQRCode(renderToken, src, sourceType, fallback) {
        els.qrPlaceholder.innerHTML = '二维码加载中...<br>如果长时间无变化，请查看下方调试面板。';
        els.qrSourceType.textContent = sourceType;
        els.qrImage.onload = function () {
          finishImageRender(renderToken, sourceType);
        };
        els.qrImage.onerror = function () {
          if (!isActiveQrRender(renderToken)) {
            return;
          }
          if (typeof fallback === 'function') {
            fallback();
            return;
          }
          failQrRender(renderToken, '二维码图片加载失败，请查看调试面板中的 qrCodeUrl 原始值。');
        };
        els.qrImage.src = src;
      }

      function renderCanvasQRCode(renderToken, text) {
        try {
          if (typeof qrcode !== 'function') {
            throw new Error('local renderer unavailable');
          }
          var qr = qrcode(0, 'M');
          qr.addData(text, 'Byte');
          qr.make();

          var moduleCount = qr.getModuleCount();
          var margin = 20;
          var targetSize = 320;
          var cellSize = Math.max(4, Math.floor((targetSize - margin * 2) / moduleCount));
          var actualSize = moduleCount * cellSize;
          var canvasSize = actualSize + margin * 2;
          var context = els.qrCanvas.getContext('2d');

          els.qrCanvas.width = canvasSize;
          els.qrCanvas.height = canvasSize;
          context.fillStyle = '#ffffff';
          context.fillRect(0, 0, canvasSize, canvasSize);
          context.save();
          context.translate(margin, margin);
          qr.renderTo2dContext(context, cellSize);
          context.restore();

          finishCanvasRender(renderToken, 'generated-from-url');
        } catch (error) {
          failQrRender(renderToken, '二维码本地生成失败，请查看调试面板中的 qrCodeUrl 原始值。');
        }
      }

      function renderQRCodeValue(sessionKey, rawValue) {
        if (!sessionKey) {
          els.qrSourceType.textContent = 'missing-session';
          beginQrRender();
          setMessage('接口已返回 session 异常，无法加载二维码。', true);
          return;
        }

        var value = String(rawValue || '').trim();
        var kind = classifyQRCodeValue(value);
        var renderToken = beginQrRender();

        if (kind === 'empty') {
          failQrRender(renderToken, '接口未返回 qrCodeUrl，无法渲染二维码。');
          return;
        }

        if (kind === 'data-url') {
          loadImageQRCode(renderToken, value, 'direct-image');
          return;
        }

        if (kind === 'base64-image') {
          loadImageQRCode(renderToken, 'data:image/png;base64,' + compactBase64(value), 'direct-image');
          return;
        }

        if (kind === 'liteapp-url') {
          renderCanvasQRCode(renderToken, value);
          return;
        }

        if (kind === 'http-url') {
          loadImageQRCode(renderToken, value, 'direct-image', function () {
            renderCanvasQRCode(renderToken, value);
          });
          return;
        }

        failQrRender(renderToken, '当前 qrCodeUrl 不是可渲染的图片或链接，请查看调试面板原始值。');
      }

      function updateDebug() {
        els.loginStartDebug.textContent = pretty(state.lastResponses.loginStart);
        els.loginStatusDebug.textContent = pretty(state.lastResponses.loginStatus);
        els.accountDebug.textContent = pretty(state.lastResponses.account);
      }

      async function requestJson(url, options) {
        var response = await fetch(url, options || {});
        var data;
        try {
          data = await response.json();
        } catch (error) {
          throw new Error('接口未返回 JSON: ' + response.status);
        }
        if (!response.ok) {
          throw new Error(data && data.error ? data.error : '请求失败: ' + response.status);
        }
        return data;
      }

      async function refreshHealth() {
        try {
          var health = await requestJson('/healthz');
          els.healthStatus.textContent = health.ok ? '正常' : '异常';
        } catch (error) {
          els.healthStatus.textContent = '异常';
          setMessage(error.message, true);
        }
      }

      async function refreshAccount() {
        try {
          var account = await requestJson('/api/weixin/account');
          state.lastResponses.account = account;
          updateDebug();
          els.configuredStatus.textContent = account.configured ? '已配置' : '未配置';
          els.pollingStatus.textContent = account.polling ? '运行中' : '未运行';
          els.accountId.textContent = account.accountId || '-';
          els.lastInboundAt.textContent = formatTime(account.lastInboundAt);
          els.lastOutboundAt.textContent = formatTime(account.lastOutboundAt);
          els.lastError.textContent = account.lastError || '-';
        } catch (error) {
          setMessage(error.message, true);
        }
      }

      function stopLoginPolling() {
        if (state.loginTimer) {
          clearTimeout(state.loginTimer);
          state.loginTimer = null;
        }
      }

      function scheduleLoginPolling() {
        stopLoginPolling();
        state.loginTimer = setTimeout(function () {
          refreshLoginStatus(false);
        }, 2000);
      }

      async function startLogin() {
        stopLoginPolling();
        setBusy(els.startLoginBtn, true);
        try {
          var payload = await requestJson('/api/weixin/login/start', { method: 'POST' });
          state.sessionKey = payload.sessionKey || '';
          state.qrCodeUrl = payload.qrCodeUrl || '';
          state.lastResponses.loginStart = payload;
          state.lastResponses.loginStatus = null;
          updateDebug();

          els.sessionKey.textContent = payload.sessionKey || '-';
          els.expiresAt.textContent = formatTime(payload.expiresAt);
          els.loginState.textContent = '等待扫码';
          els.loginHint.textContent = '二维码已生成，请用微信扫码。';
          setSessionBadge('session 已生成', 'ok');
          renderQRCodeValue(state.sessionKey, state.qrCodeUrl);
          scheduleLoginPolling();
        } catch (error) {
          els.qrSourceType.textContent = 'request-error';
          beginQrRender();
          setMessage(error.message, true);
          setSessionBadge('生成失败', 'warn');
        } finally {
          setBusy(els.startLoginBtn, false);
        }
      }

      async function refreshLoginStatus(manual) {
        if (!state.sessionKey) {
          if (manual) {
            setMessage('当前没有登录 session，请先点击“生成二维码”。', true);
          }
          return;
        }
        try {
          var payload = await requestJson('/api/weixin/login/status?sessionKey=' + encodeURIComponent(state.sessionKey));
          state.lastResponses.loginStatus = payload;
          updateDebug();
          els.loginState.textContent = payload.status || '-';
          els.loginHint.textContent = payload.message || '-';

          if (payload.status === 'confirmed') {
            stopLoginPolling();
            setSessionBadge('已连接', 'ok');
            setMessage(payload.message || '与微信连接成功。', false);
            await refreshAccount();
            return;
          }

          if (payload.status === 'expired') {
            stopLoginPolling();
            setSessionBadge('二维码过期', 'warn');
            setMessage(payload.message || '二维码已过期，请重新生成。', true);
            return;
          }

          if (payload.status === 'scaned') {
            setSessionBadge('已扫码', 'ok');
            setMessage(payload.message || '已扫码，请在微信中确认。', false);
          } else {
            setSessionBadge('等待扫码', '');
            setMessage(payload.message || '等待扫码。', false);
          }

          scheduleLoginPolling();
        } catch (error) {
          stopLoginPolling();
          setMessage(error.message, true);
          setSessionBadge('状态检查失败', 'warn');
        }
      }

      async function startPolling() {
        setBusy(els.startPollBtn, true);
        try {
          await requestJson('/api/weixin/poll/start', { method: 'POST' });
          setMessage('消息轮询已启动。', false);
          await refreshAccount();
        } catch (error) {
          setMessage(error.message, true);
        } finally {
          setBusy(els.startPollBtn, false);
        }
      }

      async function stopPolling() {
        setBusy(els.stopPollBtn, true);
        try {
          await requestJson('/api/weixin/poll/stop', { method: 'POST' });
          setMessage('消息轮询已停止。', false);
          await refreshAccount();
        } catch (error) {
          setMessage(error.message, true);
        } finally {
          setBusy(els.stopPollBtn, false);
        }
      }

      els.startLoginBtn.addEventListener('click', startLogin);
      els.refreshStatusBtn.addEventListener('click', function () {
        refreshHealth();
        refreshAccount();
        refreshLoginStatus(true);
      });
      els.startPollBtn.addEventListener('click', startPolling);
      els.stopPollBtn.addEventListener('click', stopPolling);

      refreshHealth();
      refreshAccount();
      updateDebug();
    })();
  </script>
</body>
</html>
`
