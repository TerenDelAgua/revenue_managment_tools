# 🧡 TEREN Hotels – Revenue Management Tools

> **Flow systems. Soulful experiences.**

TEREN Hotels is a modern digital toolkit designed for small hotels to evolve from paper-based operations to elegant digital management. Built on the principle that software should respect its users, this suite provides seamless reservation and revenue management with zero friction.

---

## 🏔️ Core Features

- **Interactive Floor Map Builder:** Visually map out rooms, track real-time occupancy, and manage status seamlessly.
- **Executive Dashboard:** Clear, high-contrast revenue metrics and booking overviews.
- **Guest-First Booking System:** Frictionless reservation flows designed for speed and clarity.
- **Internationalization (i18n):** Native support for English, Spanish, and Japanese.

---

## 🛠️ The TEREN Stack

We choose technologies that align with our core values—minimal boilerplate, high performance, and explicit over implicit magic.

- **Backend:** Go (Golang) + Chi Router — *Fast, clean, no magic.*
- **Frontend:** SvelteKit 5 + TailwindCSS v4 — *Reactive by default, compiled for speed.*
- **Database:** PostgreSQL 16 — *Robust, standards-compliant, and durable.*

---

## 🚀 Getting Started

Follow these steps to run the TEREN Hotels suite locally.

### Prerequisites
- [Go](https://golang.org/doc/install) (1.21+)
- [Node.js](https://nodejs.org/) (20+) & [pnpm](https://pnpm.io/)
- [Docker](https://www.docker.com/)

### 1. Database Setup
The backend relies on PostgreSQL. A Docker configuration is provided.
```bash
cd backend
docker compose up -d
```
*(Migrations are applied automatically via `docker-entrypoint-initdb.d`)*

### 2. Start the Backend API
```bash
cd backend
go run cmd/api/main.go
```
The API will run at `http://localhost:8080`.

### 3. Start the Web Frontend
In a new terminal, set up the frontend workspace:
```bash
cd web
pnpm install
```

Create a `.env` file in the `web/` directory:
```env
VITE_API_URL=http://localhost:8080
```

Start the development server:
```bash
pnpm dev
```
The application will be available at `http://localhost:5173`.

---

## 🎨 Design Philosophy

Built in accordance with the **TEREN Brand Manifesto**:
- **Elegance Over Complexity:** If a feature adds complexity without clear user value, it doesn't belong.
- **Outdoor-First Design:** High contrast, earthy tones (`#F5F4F1` canvas, `#1C1917` text), and our signature **Sunrise Orange** (`#FF8C42`).
- **Respect for Time:** Fluid micro-animations, no unnecessary loading screens, no confusing navigation.

---

**License:** © TEREN. All rights reserved.  
**Philosophy:** Built with intention. Designed for flow. Owned by TEREN.
