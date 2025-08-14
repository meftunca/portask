# Portask Admin UI

Modern React TypeScript dashboard for Portask message queue system.

## ⚡ Features

- **Real-time Monitoring**: WebSocket-based live updates
- **Modern UI**: Built with ShadcN UI components and Tailwind CSS
- **TypeScript**: Full type safety
- **Responsive Design**: Works on desktop and mobile
- **Dark/Light Theme**: System preference aware
- **Performance Optimized**: Vite build system with code splitting

## 🛠️ Tech Stack

- **Framework**: React 18 + TypeScript
- **Build Tool**: Vite
- **UI Library**: ShadcN UI (Radix UI + Tailwind CSS)
- **Routing**: React Router v6
- **State Management**: Zustand + React Query
- **HTTP Client**: Axios
- **Charts**: Recharts
- **Icons**: Lucide React

## 🚀 Getting Started

### Prerequisites

- Node.js 18+
- Bun (recommended) or npm/yarn

### Installation

```bash
# Navigate to admin UI directory
cd admin_ui

# Install dependencies with Bun (recommended)
bun install

# Or with npm
npm install

# Start development server
bun dev
# Or with npm
npm run dev
```

The dashboard will be available at `http://localhost:3000`

### Backend Integration

The UI connects to Portask backend on `http://localhost:8080` by default. Make sure your Portask server is running with the Fiber API enabled.

## 📁 Project Structure

```
admin_ui/
├── src/
│   ├── components/          # Reusable UI components
│   │   ├── ui/             # ShadcN UI components
│   │   ├── layout/         # Layout components
│   │   └── charts/         # Chart components
│   ├── pages/              # Route pages
│   ├── hooks/              # Custom hooks
│   ├── lib/                # Utilities and helpers
│   ├── api/                # API client
│   ├── store/              # State management
│   └── types/              # TypeScript definitions
├── public/                 # Static assets
└── dist/                   # Build output
```

## 🔧 Development

### Scripts

- `bun dev` - Start development server
- `bun run build` - Build for production
- `bun run preview` - Preview production build
- `bun run lint` - Run ESLint
- `bun run type-check` - Run TypeScript type checking

Alternative npm commands:
- `npm run dev` - Start development server
- `npm run build` - Build for production
- `npm run preview` - Preview production build
- `npm run lint` - Run ESLint
- `npm run type-check` - Run TypeScript type checking

### Environment Variables

Create a `.env` file in the `admin_ui` directory:

```env
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws
```

## 📊 Features Overview

### Dashboard
- System health overview
- Real-time metrics
- Connection statistics
- Memory usage charts

### Messages
- Browse messages by topic
- Real-time message streaming
- Message publishing interface
- Search and filtering

### Topics
- Topic management
- Create/delete topics
- Topic statistics
- Partition information

### Connections
- Active connections monitoring
- Connection details
- Network statistics

### Monitoring
- Performance metrics
- System resource usage
- Error tracking
- Custom alerts

### Settings
- System configuration
- JSON library selection
- Theme preferences
- API settings

## 🎨 UI Components

The dashboard uses ShadcN UI components for consistent design:

- **Navigation**: Sidebar with breadcrumbs
- **Data Display**: Tables, cards, badges
- **Forms**: Inputs, selects, textareas
- **Feedback**: Toasts, alerts, modals
- **Charts**: Line, bar, pie charts using Recharts

## 🔌 WebSocket Integration

Real-time updates are powered by WebSocket connections:

- Live message streaming
- Connection status updates
- System metrics updates
- Error notifications

## 🌙 Theme Support

- Light/Dark mode toggle
- System preference detection
- Persistent theme storage
- CSS variables for customization

## 📱 Responsive Design

- Mobile-first approach
- Breakpoint-based layouts
- Touch-friendly interactions
- Optimized for tablets and phones

## 🚀 Deployment

### Build for Production

```bash
# With Bun (recommended)
bun run build

# With npm
npm run build
```

The built files will be in the `dist/` directory.

### Serve Static Files

You can serve the built files using any static file server or integrate with your Portask backend.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## 📄 License

This project is part of the Portask system and follows the same license.
