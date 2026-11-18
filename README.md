# SimplePunchSystem 上班打卡系統

給 20 人以下公司使用的手機打卡系統。

- **老闆（管理者）**：建立／編輯／停用員工、設定與重設預設密碼、查看每位員工的上下班時間與打卡照片、計算單日／單週／單月工時、匯出 Excel／CSV。
- **員工**：第一次登入必須把預設密碼改成自己的；之後只能打卡與查看自己的紀錄，不能修改個人資料。上下班打卡都必須開相機拍照。

## 架構

```
手機瀏覽器 / 加入主畫面的 PWA
        │  HTTPS（Netlify 自動提供，相機需要 HTTPS）
        ▼
Netlify：frontend/ 靜態檔案
        │  /api/* 由 Netlify 轉發（同一個網址，cookie 同源）
        ▼
Render：Go 後端（backend/）+ SQLite + 打卡照片
```

| 目錄 | 說明 |
| --- | --- |
| `backend/` | Go 1.23 後端，標準庫 `net/http`、SQLite（`modernc.org/sqlite`，免 cgo）、bcrypt、excelize |
| `frontend/` | 手機優先的 PWA，原生 HTML/CSS/JS（ES modules），**不需要 Node.js、不需要建置** |
| `render.yaml` | Render Blueprint（後端部署設定） |
| `netlify.toml` | Netlify 設定（前端發佈與 `/api` 轉發） |

## 本機開發

只需要安裝 Go 1.23 以上。

```bash
cd backend
STATIC_DIR=../frontend DEMO_SEED=true go run ./cmd/server
```

用電腦瀏覽器開啟 http://localhost:8080 。`localhost` 允許使用相機，可以直接測試拍照打卡（Chrome 開發者工具可切換成手機尺寸）。

執行測試：

```bash
cd backend
go test ./...
```

### 環境變數

| 變數 | 預設 | 說明 |
| --- | --- | --- |
| `PORT` | `8080` | 監聽埠（Render 會自動提供） |
| `DATA_DIR` | `./data` | SQLite 資料庫與照片存放位置 |
| `TZ_NAME` | `Asia/Taipei` | 公司時區，工時以此時區計算日期 |
| `ADMIN_USERNAME` | `admin` | 老闆帳號 |
| `ADMIN_PASSWORD` | （空） | 老闆初始密碼。沒設定時：demo 模式用 `demo1234`；正式模式會隨機產生並印在 log |
| `DEMO_SEED` | `false` | `true` 時自動建立示範員工與兩週打卡資料，登入頁也會顯示示範帳號 |
| `COOKIE_SECURE` | `false` | 正式環境（HTTPS）請設 `true` |
| `STATIC_DIR` | （空） | 本機開發時讓 Go 直接提供前端檔案；部署到 Netlify 時留空 |

## 部署（Demo）

### 1. 推上 GitHub

Render 和 Netlify 都從 Git repo 部署，先把專案推到 GitHub（可以是私有 repo）。

### 2. Render：部署後端

1. 到 https://dashboard.render.com → **New** → **Blueprint**。
2. 選擇這個 repo，Render 會讀取 `render.yaml`，建立名為 `simple-punch-api` 的 Web Service（免費方案、新加坡機房）。
3. 部署完成後，記下服務網址，例如 `https://simple-punch-api.onrender.com`（如果名稱被用走了，Render 會加上後綴）。
4. 用瀏覽器開 `https://<你的網址>/api/health`，看到 `{"ok":true,"demo":true,...}` 就成功了。

### 3. Netlify：部署前端

1. 把 `netlify.toml` 裡的 `https://simple-punch-api.onrender.com` 換成第 2 步的實際網址，commit 並 push。
2. 到 Netlify → **Add new site** → **Import an existing project**，選同一個 repo。
3. 建置設定會自動讀 `netlify.toml`（publish 目錄 `frontend`，不需要建置指令），直接部署。
4. 用手機開啟 Netlify 給的網址（`https://xxx.netlify.app`）即可使用。

### 免費方案的限制（Demo 用）

- **資料不會永久保存**：Render 免費方案沒有永久磁碟，服務休眠、重啟或重新部署後資料都會清空。因為設定了 `DEMO_SEED=true`，每次啟動都會自動重建示範資料，所以隨時都能展示。
- **閒置會休眠**：15 分鐘沒人使用會休眠，下次開啟約需 30–60 秒喚醒。前端會顯示「伺服器喚醒中…」並自動重試，不會直接報錯。

### 示範帳號（`DEMO_SEED=true`）

| 角色 | 帳號 | 密碼 |
| --- | --- | --- |
| 老闆 | `admin` | `demo1234` |
| 員工 王小明 | `0911000001` | `demo1234` |
| 員工 李美華 | `0911000002` | `demo1234` |
| 員工 陳志強 | `0911000003` | `demo1234` |
| 員工 林新人（展示首次登入改密碼） | `0911000004` | `init1234` |

## 轉為正式使用

修改 `render.yaml`（檔案內已有註解說明）：

1. `plan: free` 改成 `plan: starter`（約 US$7／月，掛永久磁碟需要付費方案）。
2. 取消 `disk` 區塊的註解（1GB 掛在 `/var/data`）。
3. `DATA_DIR` 改成 `/var/data`，`DEMO_SEED` 改成 `"false"`。
4. 在 Render dashboard 的 Environment 設定 `ADMIN_PASSWORD`（老闆初始密碼，第一次登入時必須更改）。

程式碼不需要修改。Render 磁碟每天自動快照（保留 7 天）；也可以用 Render Shell 把 `/var/data` 打包下載備份。照片每張約 100–200KB，20 人每天打卡 2 次，一年約 1–2GB。

## 手機使用說明

- **iPhone（Safari）**：開啟網址 → 分享按鈕 → 「加入主畫面」，之後就像 App 一樣從主畫面開啟。第一次打卡時允許使用相機。
- **Android（Chrome）**：開啟網址 → 選單 → 「安裝應用程式」或「加到主畫面」。
- 如果相機權限被拒絕，打卡畫面會提供「開啟手機相機」按鈕改用系統相機拍照；也可以到手機設定重新允許此網站使用相機。

## 功能與規則

- **打卡**：伺服器自動判斷下一次該打上班還是下班；已上班不能再打上班，未上班不能打下班。打卡時間以**伺服器時間**為準，不採用手機時間。照片會加上時間浮水印。
- **忘記打下班卡**：上班超過 20 小時仍未下班，系統視為忘記打卡，該段不計工時並標示「未打下班卡」，員工可以重新打上班卡。20 小時內的未完成時段顯示為「上班中」。
- **工時計算**：每個上班卡與其後第一個下班卡配成一段，一天可以有多段（例如午休分段打卡）；跨夜班算在上班當天。週為週一到週日，月為自然月。
- **匯出**：Excel 包含「工時統計」與「打卡明細」兩個工作表；CSV 為打卡明細（UTF-8 BOM，Excel 直接開啟中文不會亂碼）。
- **安全**：密碼以 bcrypt 雜湊；session 為 HttpOnly cookie（伺服器只存雜湊）；登入失敗 5 次鎖 15 分鐘；停用員工或重設密碼時會立即登出該員工所有裝置；照片只有本人與老闆能看。

## API 一覽

| 方法 | 路徑 | 權限 |
| --- | --- | --- |
| GET | `/api/health` | 公開 |
| POST | `/api/auth/login`、`/api/auth/logout` | 公開／登入 |
| GET | `/api/me` | 登入 |
| POST | `/api/me/password` | 登入（含尚未改預設密碼） |
| GET | `/api/punch/status` | 員工 |
| POST | `/api/punch`（multipart：`type`、`photo`） | 員工 |
| GET | `/api/me/punches?from&to` | 員工 |
| GET | `/api/photos/{id}` | 本人或老闆 |
| GET/POST | `/api/admin/users` | 老闆 |
| GET/PUT | `/api/admin/users/{id}` | 老闆 |
| POST | `/api/admin/users/{id}/reset-password` | 老闆 |
| GET | `/api/admin/attendance?from&to&user_id` | 老闆 |
| GET | `/api/admin/hours?period=day\|week\|month&date&user_id` | 老闆 |
| GET | `/api/admin/export?format=xlsx\|csv&from&to&user_id` | 老闆 |

所有會改變資料的請求都必須帶 `X-Requested-With: fetch` 標頭（防 CSRF）。
