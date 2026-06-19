# reverse-tunnel — 內網穿透 / 反向隧道

讓一台位於 NAT/防火牆後面、沒有公網 IP 的主機（**client**），主動連到一台有公網域名的伺服器（**server**），
由 server 在公網開放端口，把外部流量透過隧道轉發回 client 的本機服務。

跟 `ngrok` / `frp` / `ssh -R` 同一類東西，用 Go 寫成單一執行檔，一份代碼跨 **Linux / macOS / Windows**。
轉發任意 **TCP**，所以 Minecraft（25565）、SSH、資料庫、自訂協定都能用。

提供**命令列**與**原生視窗 GUI**（Fyne）兩種版本，server 與 client 都各有 GUI。

> ⚠️ **只在你擁有或已獲明確授權的主機上使用。** 把別人的機器在未授權情況下對外開放是違法的。

```
外部玩家 ──► server(公網 example.com:25565) ══隧道══► client(你家主機) ──► 本機 127.0.0.1:25565 (Minecraft)
```

## 運作原理

1. client 主動向 server 的控制端口（預設 7000）發起一條**控制連線**，附上密鑰與想開放的遠端端口。
2. server 驗證密鑰後，在公網開放該端口。
3. 每當有訪客連進公網端口，server 透過控制連線通知 client。
4. client 回連一條**資料連線**到 server，並同時連上本機目標服務，雙向轉發資料。

因為連線都是 client「主動向外」發起的，所以 client 不需要任何公網 IP 或開放入站端口，
能穿透 NAT 與大多數防火牆。

## 編譯

需要安裝 Go 1.21+。

### 命令列版（純 Go，交叉編譯零負擔）

```bash
# 只編譯目前這台機器用的版本
go build -o server ./cmd/server
go build -o client ./cmd/client

# 一次交叉編譯出全部平台（輸出在 dist/）
./build.sh
```

`dist/` 內的檔名格式為 `server-<os>-<arch>` / `client-<os>-<arch>`，Windows 版帶 `.exe`。

### GUI 版（Fyne 原生視窗，需要 CGO）

在「目前這台機器」上原生編譯最簡單：

```bash
go build -o server-gui ./cmd/server-gui
go build -o client-gui ./cmd/client-gui
```

GUI 用了 Fyne，需要 CGO 與系統的圖形函式庫，**無法**像命令列版那樣從一台機器直接交叉編譯到其他作業系統。
要產出三大平台的 GUI 執行檔，請用官方的 [`fyne-cross`](https://github.com/fyne-io/fyne-cross)（底層用 Docker 準備各平台工具鏈）：

```bash
go install github.com/fyne-io/fyne-cross@latest

# 範例：替 client GUI 產出各平台版本
fyne-cross windows -arch=amd64 ./cmd/client-gui
fyne-cross linux   -arch=amd64 ./cmd/client-gui
fyne-cross darwin  -arch=amd64,arm64 ./cmd/client-gui
# server GUI 同理，把路徑換成 ./cmd/server-gui
```

各平台原生編譯所需的系統相依（GCC、OpenGL 等）詳見 Fyne 官方文件
<https://docs.fyne.io/started/>。

## 使用（GUI）

1. 在公網伺服器上開 **server GUI**，填「監聽位址」（例如 `:7000`）與「密鑰」，按**啟動**。
2. 在你家主機開 **client GUI**，填 server 位址、相同的密鑰、本機服務（例如 `127.0.0.1:25565`）、遠端端口，按**啟動**。
3. 兩邊視窗下方都會即時顯示日誌；按**停止**或關閉視窗即中止。

### macOS 第一次開啟

Release 的 macOS GUI 是 **universal `.app`**（Intel + Apple Silicon 通用），下載 `*-darwin-universal.app.zip`，解壓得到 `Tunnel Client.app` / `Tunnel Server.app`。

因為沒有付費的 Apple 開發者簽章/公證，第一次開啟會被 Gatekeeper 攔。任選一種方式放行（只需做一次）：

- **右鍵打開**：在 Finder 對 `.app` 按右鍵 →「打開」→ 再點一次「打開」。
- 或在系統設定 → **隱私權與安全性**，點「仍要打開」。
- 或用終端機移除隔離標記：

  ```bash
  xattr -dr com.apple.quarantine "Tunnel Client.app"
  ```

`.app` 已做 ad-hoc 簽章，所以不會再出現「已損毀」的訊息。

## 使用（命令列）

### 1. 在公網伺服器（example.com）上跑 server

```bash
./server -bind :7000 -token 換成你自己的長密鑰
```

- `-bind`：控制與資料連線的監聽位址，預設 `:7000`。
- `-token`：共享密鑰，client 必須提供相同值（**必填**）。

請確認雲端防火牆 / 安全群組同時放行 **7000**（控制）與你要開放的端口（例如 **25565**）。

### 2. 在你家的主機上跑 client

以 Minecraft 為例（本機 25565 開放到公網 25565）：

```bash
# Linux / macOS
./client -server example.com:7000 -token 換成你自己的長密鑰 \
         -local 127.0.0.1:25565 -remote-port 25565
```

```powershell
# Windows
.\client-windows-amd64.exe -server example.com:7000 -token 換成你自己的長密鑰 `
         -local 127.0.0.1:25565 -remote-port 25565
```

- `-server`：server 的位址（**必填**）。
- `-token`：與 server 相同的密鑰（**必填**）。
- `-local`：要被穿透的本機服務位址，預設 `127.0.0.1:25565`。
- `-remote-port`：要在 server 公網開放的端口，預設 `25565`。

### 3. 連線

朋友用 `example.com:25565` 連你的 Minecraft 伺服器即可。
換其他服務只要改 `-local` 與 `-remote-port`，例如穿透 SSH：

```bash
./client -server example.com:7000 -token ... -local 127.0.0.1:22 -remote-port 2222
```

client 斷線後會每 5 秒自動重連。

## 安全建議

- `-token` 用一段夠長的隨機字串（例如 `openssl rand -hex 24`）。
- 本工具的隧道內容**未加密**。若要保護敏感流量，請穿透本身已加密的服務（SSH、HTTPS），
  或把 server↔client 之間再包一層（例如先用 WireGuard / SSH 隧道）。
- 在 server 上用防火牆只放行真正需要的端口。

## 專案結構

```
cmd/server/main.go        server 命令列版（公網端，開放端口、轉發）
cmd/client/main.go        client 命令列版（內網端，主動外連、接本機服務）
cmd/server-gui/main.go    server 原生視窗 GUI（Fyne）
cmd/client-gui/main.go    client 原生視窗 GUI（Fyne）
internal/protocol/        server 與 client 共用的訊息格式
internal/tunnelserver/    server 核心邏輯（CLI 與 GUI 共用）
internal/tunnelclient/    client 核心邏輯（CLI 與 GUI 共用）
build.sh                  命令列版的交叉編譯腳本
```
