import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App.tsx'
import { ReactQueryProvider } from './lib/react-query'

ReactDOM.createRoot(document.getElementById('root')!).render(
  <React.StrictMode>
    <ReactQueryProvider>
      <App />
    </ReactQueryProvider>
  </React.StrictMode>,
)
