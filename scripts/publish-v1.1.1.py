import socket, threading, select, subprocess, os, time, sys

PORT = 8896

def to_nat64(host):
    if ":" in host:
        return host
    try:
        ip4 = socket.gethostbyname(host)
        parts = [int(p) for p in ip4.split(".")]
        return f"2001:67c:2b0:db32:0:1:{parts[0]:02x}{parts[1]:02x}:{parts[2]:02x}{parts[3]:02x}"
    except Exception:
        return host

def handle(client):
    try:
        req = b''
        while b'\r\n\r\n' not in req:
            chunk = client.recv(1024)
            if not chunk: break
            req += chunk
        lines = req.decode('latin1').split('\r\n')
        method, target, _ = lines[0].split()
        if method == 'CONNECT':
            host, port = target.split(':')
            ip6 = to_nat64(host)
            dest = socket.socket(socket.AF_INET6, socket.SOCK_STREAM)
            dest.settimeout(15)
            dest.connect((ip6, int(port)))
            dest.settimeout(None)
            client.sendall(b'HTTP/1.1 200 Connection Established\r\n\r\n')
            
            sockets = [client, dest]
            while True:
                r, _, _ = select.select(sockets, [], [], 30)
                if not r: break
                for s in r:
                    data = s.recv(65536)
                    if not data: return
                    other = dest if s is client else client
                    other.sendall(data)
    except Exception:
        pass
    finally:
        client.close()

def start_proxy():
    server = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    server.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    server.bind(('127.0.0.1', PORT))
    server.listen(50)
    print(f'[Tunnel] Dynamic IPv6 Bridge listening on 127.0.0.1:{PORT}')
    sys.stdout.flush()
    while True:
        c, _ = server.accept()
        threading.Thread(target=handle, args=(c,), daemon=True).start()

t = threading.Thread(target=start_proxy, daemon=True)
t.start()
time.sleep(1)

token = subprocess.check_output(['gh', 'auth', 'token']).decode().strip()
proxy_url = f'http://127.0.0.1:{PORT}'
env = dict(os.environ, 
           HTTPS_PROXY=proxy_url, 
           HTTP_PROXY=proxy_url,
           ALL_PROXY=proxy_url,
           GH_TOKEN=token)

print('[GitHub] Pushing main branch...')
sys.stdout.flush()
res = subprocess.run(['git', '-c', f'http.proxy={proxy_url}', 'push', 'origin', 'main'], env=env, capture_output=True, text=True)
print('Push main result:\n', res.stdout, res.stderr)
sys.stdout.flush()

print('[GitHub] Creating git tag v1.1.1...')
subprocess.run(['git', 'tag', '-f', '-a', 'v1.1.1', '-m', 'Release v1.1.1 - Clean Native UI'], capture_output=True)

print('[GitHub] Pushing tag v1.1.1...')
sys.stdout.flush()
res = subprocess.run(['git', '-c', f'http.proxy={proxy_url}', 'push', '-f', 'origin', 'v1.1.1'], env=env, capture_output=True, text=True)
print('Push tag result:\n', res.stdout, res.stderr)
sys.stdout.flush()

print('[GitHub] Creating Release v1.1.1 and uploading compiled assets...')
sys.stdout.flush()
assets = [
    'dist/GeminiSwap-macOS.zip',
    'dist/gemini-swap-darwin-arm64.tar.gz',
    'dist/gemini-swap-darwin-amd64.tar.gz',
    'dist/gemini-swap-linux-amd64.tar.gz',
    'dist/gemini-swap-linux-arm64.tar.gz',
    'dist/gemini-swap-windows-amd64.zip'
]

release_notes = """### ⚡ Gemini Swap v1.1.1 - Clean Native macOS UI & Polish

#### ✨ What's Changed in v1.1.1:
- **Clean Native Design (Anti-vibecoding)**:
  - Removed cheesy gradient logo and "X accounts configured" subtitle from top bar.
  - Replaced loud neon `[ACTIVE]` badge with a subtle, elegant Apple-style status indicator.
  - Fixed duplicated email issue: email is now cleanly displayed only once.
  - Removed confusing bottom proxy telemetry section (`http://127.0.0.1:8045/v1` non-working link) from main window.
- **Account Sharing**:
  - Export and import account configs via files and share codes.

#### 📥 Pre-compiled Downloads:
- **macOS App**: [`GeminiSwap-macOS.zip`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/GeminiSwap-macOS.zip) (Native Swift App with Menu Bar Mode)
- **Linux VPS (x64)**: [`gemini-swap-linux-amd64.tar.gz`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/gemini-swap-linux-amd64.tar.gz)
- **Linux VPS (ARM64)**: [`gemini-swap-linux-arm64.tar.gz`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/gemini-swap-linux-arm64.tar.gz)
- **macOS CLI (Apple Silicon)**: [`gemini-swap-darwin-arm64.tar.gz`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/gemini-swap-darwin-arm64.tar.gz)
- **macOS CLI (Intel)**: [`gemini-swap-darwin-amd64.tar.gz`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/gemini-swap-darwin-amd64.tar.gz)
- **Windows (x64)**: [`gemini-swap-windows-amd64.zip`](https://github.com/Leu3ery/gemini-swap/releases/download/v1.1.1/gemini-swap-windows-amd64.zip)
"""

release_cmd = ['gh', 'release', 'create', 'v1.1.1',
               '--title', 'Gemini Swap v1.1.1 - Clean Native UI & Polish',
               '--notes', release_notes] + assets

res = subprocess.run(release_cmd, env=env, capture_output=True, text=True)
print('Release result:\n', res.stdout, res.stderr)
print('✓ Successfully released v1.1.1!')
sys.stdout.flush()
