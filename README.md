# IDMonitor

Security monitoring & management platform untuk monitoring host, agent, alert, incident, dan authorized network scanning.

## 📁 Struktur Folder

```text
IDMonitor/
├── .freebuff/
├── agent/
├── backend/
├── frontend/
├── migrations/
├── .env
├── .env.example
├── .gitignore
├── docker-compose.yml
├── Makefile
└── README.md
```

## 🚀 Installation

### Requirements

* Docker
* Docker Compose
* Git

### Install

```bash
sudo git clone https://github.com/zayntkj24/IDMonitor.git
cd IDMonitor
sudo docker compose down
sudo docker compose up -d
sudo docker compose ps
```
