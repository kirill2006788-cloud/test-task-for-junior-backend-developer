# 📋 test-task-for-junior-backend-developer

<p align="center">
  <b>Task Service API • Go • Recurrence Support</b>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.21+-00ADD8?style=for-the-badge&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/License-MIT-green?style=for-the-badge" alt="License">
  <img src="https://img.shields.io/badge/API-REST-brightgreen?style=for-the-badge" alt="API">
</p>

---

> Backend task service API with recurrence support — test task for junior developers.

---

## ✨ Features

- ✅ **RESTful API** design
- ✅ **Task CRUD** operations
- ✅ **Recurrence support** (cron-like patterns)
- ✅ **SQLite** database
- ✅ **Unit tests**
- ✅ **Clean architecture**

---

## 🛠 Tech Stack

| Component | Technology |
|-----------|------------|
| **Language** | Go 1.21+ |
| **Database** | SQLite |
| **API** | RESTful |
| **Testing** | Go testing |

---

## 📂 Project Structure

```
test-task-for-junior-backend-developer/
├── cmd/                   # Entry points
│   └── server/
│
├── internal/              # Internal packages
│   ├── handler/          # HTTP handlers
│   ├── service/          # Business logic
│   ├── repository/       # Data access
│   └── model/            # Data models
│
├── migrations/            # Database migrations
├── go.mod                # Go modules
└── README.md
```

---

## 🚀 Getting Started

```bash
# Clone repository
git clone https://github.com/kirill2006788-cloud/test-task-for-junior-backend-developer.git
cd test-task-for-junior-backend-developer

# Run server
go run cmd/server/main.go

# Run tests
go test ./...
```

---

## 📡 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/tasks` | List all tasks |
| POST | `/tasks` | Create new task |
| GET | `/tasks/:id` | Get task by ID |
| PUT | `/tasks/:id` | Update task |
| DELETE | `/tasks/:id` | Delete task |

---

## 📄 License

MIT License - see [LICENSE](LICENSE) for details.

---

<div align="center">
  <p>Built with ❤️ by <a href="https://github.com/kirill2006788-cloud">Kirill</a></p>
</div>
