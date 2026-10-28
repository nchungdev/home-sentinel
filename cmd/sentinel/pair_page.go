package main

const pairPage = `<!doctype html>
<html lang="vi">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Đăng nhập TV</title>
<style>
:root { color-scheme: dark; --bg: #0f1115; --card: #181b22; --line: #2a2f3a;
  --text: #eef0f5; --muted: #9aa1b0; --accent: #6366f1; --ok: #34d399; --bad: #f87171; }
* { box-sizing: border-box; }
body { margin: 0; min-height: 100vh; display: grid; place-items: center; padding: 16px;
  background: var(--bg); color: var(--text); font: 16px/1.5 system-ui, sans-serif; }
main { width: 100%; max-width: 380px; background: var(--card); border: 1px solid var(--line);
  border-radius: 18px; padding: 24px; }
h1 { font-size: 20px; margin: 0 0 4px; }
p { color: var(--muted); margin: 0 0 20px; }
.code { font: 600 32px/1 ui-monospace, monospace; letter-spacing: 6px; text-align: center;
  padding: 16px; border-radius: 12px; background: #0b0d11; margin-bottom: 20px; }
input { width: 100%; padding: 14px; border-radius: 12px; border: 1px solid var(--line);
  background: #0b0d11; color: var(--text); font-size: 16px; margin-bottom: 12px; }
button { width: 100%; padding: 14px; border: 0; border-radius: 12px; background: var(--accent);
  color: white; font-size: 16px; font-weight: 600; }
button:disabled { opacity: .5; }
.status { margin-top: 14px; min-height: 24px; text-align: center; }
.ok { color: var(--ok); } .bad { color: var(--bad); }
[hidden] { display: none; }
#done { text-align: center; }
#done p { margin: 0; }
.done-mark { width: 56px; height: 56px; margin: 4px auto 14px; border-radius: 50%;
  display: grid; place-items: center; background: var(--ok); color: #0b0d11;
  font-size: 30px; font-weight: 700; }
</style>
</head>
<body>
<main>
  <h1>Đăng nhập TV vào Home Sentinel</h1>
  <p>Kiểm tra mã dưới đây trùng với mã đang hiện trên TV.</p>
  <div class="code" id="code">——————</div>
  <form id="login" hidden>
    <input id="token" type="password" autocomplete="current-password"
      placeholder="Access token (AGENT_TOKEN)" required>
    <button type="submit">Đăng nhập</button>
  </form>
  <button id="approve" hidden>Cho phép TV này</button>
  <div class="status" id="status"></div>
  <section id="done" hidden>
    <div class="done-mark">✓</div>
    <h1>Đã đăng nhập TV</h1>
    <p>Bạn có thể đóng trang này.</p>
  </section>
</main>
<script>
const code = new URLSearchParams(location.search).get("code") || "";
const $ = (id) => document.getElementById(id);
$("code").textContent = code || "——————";

function say(text, kind) {
  $("status").textContent = text;
  $("status").className = "status " + (kind || "");
}

async function json(path, body) {
  const response = await fetch(path, {
    method: body ? "POST" : "GET",
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });
  return { ok: response.ok, status: response.status };
}

async function showStep() {
  const response = await fetch("/api/auth/status", { credentials: "same-origin" });
  const state = await response.json();
  const signedIn = !state.required || state.authorized;
  $("login").hidden = signedIn;
  $("approve").hidden = !signedIn;
}

$("login").addEventListener("submit", async (event) => {
  event.preventDefault();
  const result = await json("/api/auth/login", { token: $("token").value });
  if (!result.ok) return say("Token không đúng", "bad");
  say("");
  await showStep();
});

$("approve").addEventListener("click", async () => {
  $("approve").disabled = true;
  const result = await json("/api/pair/approve", { code });
  if (result.ok) return finish();
  $("approve").disabled = false;
  say(result.status === 404 ? "Mã đã hết hạn, hãy mở lại trên TV" : "Không duyệt được", "bad");
});

function finish() {
  history.replaceState(null, "", location.pathname);
  for (const id of ["code", "login", "approve", "status"]) $(id).hidden = true;
  document.querySelector("main > h1").hidden = true;
  document.querySelector("main > p").hidden = true;
  $("done").hidden = false;
  setTimeout(() => window.close(), 1500);
}

if (!code) say("Thiếu mã. Hãy quét lại QR trên TV.", "bad");
showStep();
</script>
</body>
</html>
`
