# 🛡️ IDMonitor

**IDMonitor** adalah platform monitoring dan security management yang dirancang untuk membantu mengelola **user, role, agent, host, monitoring, scanner, alert, dan incident** dalam satu sistem.

Project ini dikembangkan dengan fokus pada environment **Linux/server**, containerization menggunakan **Docker**, serta backend berbasis **Go**.

> 🚧 **Status:** Active Development
> Project masih dalam tahap pengembangan dan beberapa fitur dapat berubah pada versi berikutnya.

---

## ✨ Features

IDMonitor dirancang dengan beberapa komponen utama:

* 🔐 **Authentication**

  * Login menggunakan email dan password
  * Session management
  * JWT-based authentication
  * Password hashing
  * Two-Factor Authentication (TOTP)
  * Dukungan authenticator seperti Google Authenticator

* 👤 **User Management**

  * Membuat user
  * Mengelola user
  * Mengatur status user
  * Role-based access

* 🛡️ **Role & Authorization**

  * Role management
  * Pembatasan akses berdasarkan role
  * Middleware authorization

* 🖥️ **Host Management**

  * Management host/server
  * Informasi host
  * Monitoring target

* 🤖 **Agent Management**

  * Management agent
  * Monitoring agent
  * Integrasi agent dengan server

* 📊 **Monitoring**

  * Monitoring host dan agent
  * Health checking
  * Status monitoring

* 🔎 **Network Scanner**

  * Network/security scanning
  * Nmap integration
  * Scanner management

* 🚨 **Alert Management**

  * Pengelolaan alert
  * Monitoring event
  * Alert status

* 📋 **Incident Management**

  * Pencatatan incident
  * Incident tracking
  * Pengelolaan status incident

* ⚙️ **Background Worker**

  * Background processing menggunakan service worker terpisah

* 🐳 **Docker**

  * Backend container
  * Worker container
  * Frontend container
  * Database/service integration melalui Docker Compose

---

# 🏗️ Architecture

Secara umum IDMonitor terdiri dari beberapa komponen:

```text
                    ┌─────────────────────┐
                    │      Frontend       │
                    │    Web Interface    │
                    └──────────┬──────────┘
                               │
                               │ HTTP/API
                               ▼
                    ┌─────────────────────┐
                    │      Backend        │
                    │       Go API        │
                    └──────────┬──────────┘
                               │
                ┌──────────────┼──────────────┐
                │              │              │
                ▼              ▼              ▼
          ┌──────────┐   ┌──────────┐   ┌──────────┐
          │ Database │   │  Worker  │   │  Nmap    │
          │          │   │          │   │ Scanner  │
          └──────────┘   └──────────┘   └──────────┘
```

### Backend

Backend menangani:

* Authentication
* Authorization
* User management
* Role management
* Host management
* Agent management
* Monitoring
* Scanner
* Alert
* Incident
* API

### Worker

Worker digunakan untuk proses background yang tidak perlu dijalankan langsung oleh request HTTP.

### Frontend

Frontend menyediakan interface untuk berinteraksi dengan API IDMonitor.

### Database

Database digunakan untuk menyimpan data aplikasi seperti:

* Users
* Roles
* Sessions
* Hosts
* Agents
* Monitoring data
* Alerts
* Incidents
* Configuration

---

# 🧰 Tech Stack

## Backend

* Go
* Chi Router
* PostgreSQL
* JWT
* UUID
* TOTP
* bcrypt
* pgx

## Frontend

* React
* TypeScript
* Vite
* Tailwind CSS

## Infrastructure

* Docker
* Docker Compose
* Alpine Linux
* Nmap

---

# 📋 Requirements

Sebelum menjalankan IDMonitor, pastikan sistem sudah memiliki:

* Linux server/desktop
* Git
* Docker
* Docker Compose Plugin
* Internet connection

Cek instalasi:

```bash
git --version
docker --version
docker compose version
```

Contoh environment yang direkomendasikan:

```text
Linux
Docker Engine
Docker Compose v2
```

---

# 🚀 Installation

## 1. Clone Repository

Clone repository IDMonitor:

```bash
sudo git clone https://github.com/zayntkj24/IDMonitor.git
```

Masuk ke directory project:

```bash
cd IDMonitor
```

> **Catatan:** Setelah `git clone`, perintah Docker harus dijalankan dari directory `IDMonitor`.

---

# 🐳 Running with Docker

IDMonitor menggunakan Docker Compose untuk menjalankan service yang diperlukan.

## 2. Stop Existing Containers

Sebelum menjalankan versi terbaru, hentikan container yang mungkin masih berjalan:

```bash
sudo docker compose down
```

Perintah ini menghentikan dan menghapus container yang dibuat oleh Compose.

---

## 3. Start IDMonitor

Jalankan seluruh service dalam background:

```bash
sudo docker compose up -d
```

Flag `-d` membuat container berjalan di background sehingga terminal tetap dapat digunakan.

---

## 4. Check Container Status

Untuk memastikan seluruh service berjalan:

```bash
sudo docker compose ps
```

Contoh status yang diharapkan:

```text
NAME                         STATUS
idmonitor-backend            Up
idmonitor-frontend           Up
idmonitor-worker             Up
```

Nama container dapat berbeda tergantung konfigurasi Docker Compose.

---

# 🔍 Checking Logs

Jika ada masalah saat startup, cek log menggunakan:

```bash
sudo docker compose logs
```

Untuk melihat log backend:

```bash
sudo docker compose logs backend
```

Untuk melihat log frontend:

```bash
sudo docker compose logs frontend
```

Untuk melihat log worker:

```bash
sudo docker compose logs worker
```

Untuk mengikuti log secara realtime:

```bash
sudo docker compose logs -f
```

Atau hanya backend:

```bash
sudo docker compose logs -f backend
```

---

# 🔄 Restart IDMonitor

Jika ingin restart seluruh service:

```bash
sudo docker compose restart
```

Atau restart service tertentu:

```bash
sudo docker compose restart backend
```

---

# 🛑 Stop IDMonitor

Untuk menghentikan service:

```bash
sudo docker compose down
```

---

# 🔑 Default Login

Setelah IDMonitor berhasil dijalankan, gunakan akun administrator default berikut:

```text
Email    : admin@idmonitor.local
Password : Admin123!
```

> ⚠️ **PENTING:** Kredensial tersebut adalah akun default untuk instalasi/development. Segera ganti password setelah login pertama jika instance digunakan di environment nyata atau server yang dapat diakses orang lain.

Jangan menggunakan password default untuk production.

---

# 🔐 Authentication & Security

IDMonitor memiliki beberapa mekanisme security yang digunakan dalam sistem:

### Password Hashing

Password user tidak seharusnya disimpan dalam bentuk plaintext. Backend menggunakan hashing untuk penyimpanan password.

### JWT

JWT digunakan sebagai bagian dari mekanisme authentication API.

### Session Management

Session digunakan untuk mengelola login dan status authentication user.

### Two-Factor Authentication

IDMonitor mendukung TOTP-based 2FA.

Authenticator yang kompatibel dengan TOTP dapat digunakan, termasuk:

* Google Authenticator
* Aplikasi authenticator lainnya yang mendukung TOTP

---

# 🔎 Nmap Scanner

IDMonitor menyediakan komponen scanner yang menggunakan **Nmap** untuk melakukan network/security scanning.

Nmap dijalankan dari environment backend/container yang telah menyediakan binary Nmap.

Pastikan penggunaan scanner hanya dilakukan terhadap:

* Server milik sendiri
* Network yang mendapatkan izin
* Sistem yang memang berada dalam scope testing

Jangan melakukan scanning terhadap sistem pihak lain tanpa authorization.

---

# 📁 Project Structure

Struktur project secara umum:

```text
IDMonitor/
│
├── backend/
│   ├── internal/
│   │   ├── auth/
│   │   ├── database/
│   │   └── ...
│   │
│   ├── main.go
│   ├── worker.go
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
│
├── frontend/
│   ├── src/
│   ├── package.json
│   └── ...
│
├── docker-compose.yml
├── README.md
└── ...
```

Struktur aktual dapat berubah seiring perkembangan project.

---

# 🧪 Development

Untuk development backend secara lokal, pastikan Go sudah terinstall.

Cek:

```bash
go version
```

Download dependency:

```bash
go mod download
```

Rapikan dependency:

```bash
go mod tidy
```

Build backend:

```bash
go build ./...
```

Format source code:

```bash
gofmt -w .
```

Jika project memiliki test:

```bash
go test ./...
```

---

# 🐳 Build Docker Image

Untuk rebuild backend setelah perubahan source:

```bash
sudo docker compose build backend
```

Untuk rebuild tanpa menggunakan cache:

```bash
sudo docker compose build --no-cache backend
```

Kemudian jalankan kembali:

```bash
sudo docker compose up -d
```

---

# 🧹 Rebuild Full Stack

Jika ingin membangun ulang seluruh service:

```bash
sudo docker compose down
sudo docker compose build --no-cache
sudo docker compose up -d
```

Kemudian cek:

```bash
sudo docker compose ps
```

---

# 📡 Service Verification

Setelah container aktif, lakukan pengecekan:

```bash
sudo docker compose ps
```

Kemudian cek log:

```bash
sudo docker compose logs --tail=100
```

Jika backend memiliki health endpoint, endpoint tersebut dapat digunakan untuk memastikan API sudah merespons.

Contoh:

```text
GET /health
```

Endpoint aktual mengikuti konfigurasi API pada versi project yang sedang digunakan.

---

# ⚙️ Environment Configuration

Untuk deployment yang serius, gunakan environment variable untuk konfigurasi sensitif seperti:

* Database credentials
* JWT secret
* Session secret
* Application secret
* TOTP configuration
* Service configuration

**Jangan commit secret atau password production ke repository GitHub.**

Contoh `.env`:

```env
DATABASE_URL=your_database_connection
JWT_SECRET=change-this-secret
SESSION_SECRET=change-this-secret
```

> Nilai environment variable yang sebenarnya mengikuti konfigurasi `docker-compose.yml` dan source code project.

---

# 🛡️ Production Security Checklist

Sebelum IDMonitor digunakan pada production:

* [ ] Ganti password administrator default
* [ ] Gunakan password yang kuat
* [ ] Aktifkan 2FA
* [ ] Ganti JWT/session secret
* [ ] Jangan expose PostgreSQL langsung ke internet
* [ ] Gunakan HTTPS
* [ ] Batasi akses firewall
* [ ] Jalankan Docker dengan konfigurasi yang aman
* [ ] Jangan commit `.env`
* [ ] Backup database secara berkala
* [ ] Batasi akses scanner
* [ ] Pastikan Nmap hanya digunakan pada target yang authorized
* [ ] Update dependency secara berkala
* [ ] Periksa Docker image secara berkala

---

# 🐞 Troubleshooting

## Container tidak berjalan

Cek:

```bash
sudo docker compose ps
```

Kemudian:

```bash
sudo docker compose logs --tail=100
```

---

## Backend gagal start

Cek:

```bash
sudo docker compose logs backend
```

Jika masalah berkaitan dengan database, pastikan database/service terkait sudah berjalan.

---

## Frontend tidak bisa diakses

Cek:

```bash
sudo docker compose ps
```

Kemudian:

```bash
sudo docker compose logs frontend
```

Pastikan port yang digunakan tidak sedang dipakai service lain.

---

## Setelah update source code perubahan tidak terlihat

Rebuild image:

```bash
sudo docker compose down
sudo docker compose build --no-cache
sudo docker compose up -d
```

Kemudian:

```bash
sudo docker compose ps
```

---

# 📌 Quick Start

Kalau cuma mau langsung menjalankan IDMonitor:

```bash
sudo git clone https://github.com/zayntkj24/IDMonitor.git
cd IDMonitor

sudo docker compose down
sudo docker compose up -d
sudo docker compose ps
```

Kemudian login menggunakan:

```text
Email    : admin@idmonitor.local
Password : Admin123!
```

---

# 📜 License

License project mengikuti file `LICENSE` yang tersedia di repository.

Jika file `LICENSE` belum tersedia, tambahkan license sebelum project didistribusikan sebagai open-source.

---

# ⚠️ Disclaimer

IDMonitor dibuat untuk tujuan **monitoring, administration, security management, dan authorized security testing**.

Fitur scanner harus digunakan secara bertanggung jawab dan hanya terhadap sistem/network yang memang dimiliki atau telah mendapatkan izin untuk diuji.

Developer tidak bertanggung jawab atas penyalahgunaan software ini terhadap sistem yang tidak mendapatkan authorization.

---

# 👨‍💻 Development

IDMonitor masih terus dikembangkan.

Bug, improvement, feature request, dan kontribusi dapat dilakukan melalui repository GitHub:

**https://github.com/zayntkj24/IDMonitor**

Jika menemukan bug keamanan, hindari mempublikasikan detail vulnerability yang sensitif secara terbuka sebelum masalah tersebut ditangani.
