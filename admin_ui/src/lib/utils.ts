import { type ClassValue, clsx } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number, decimals = 2) {
  if (bytes === 0) return '0 Bytes'
  
  const k = 1024
  const dm = decimals < 0 ? 0 : decimals
  const sizes = ['Bytes', 'KB', 'MB', 'GB', 'TB', 'PB', 'EB', 'ZB', 'YB']
  
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  
  return parseFloat((bytes / Math.pow(k, i)).toFixed(dm)) + ' ' + sizes[i]
}

export function formatNumber(num: number) {
  return new Intl.NumberFormat().format(num)
}

export function formatDuration(ms: number) {
  const seconds = Math.floor(ms / 1000)
  const minutes = Math.floor(seconds / 60)
  const hours = Math.floor(minutes / 60)
  const days = Math.floor(hours / 24)

  if (days > 0) return `${days}d ${hours % 24}h`
  if (hours > 0) return `${hours}h ${minutes % 60}m`
  if (minutes > 0) return `${minutes}m ${seconds % 60}s`
  return `${seconds}s`
}

export function formatLatency(ms: number) {
  if (ms < 1) return `${Math.round(ms * 1000)}μs`
  if (ms < 1000) return `${Math.round(ms)}ms`
  return `${Math.round(ms / 1000)}s`
}

export function getStatusColor(status: string) {
  switch (status.toLowerCase()) {
    case 'healthy':
    case 'connected':
    case 'running':
      return 'text-green-600 bg-green-50 border-green-200'
    case 'warning':
    case 'degraded':
      return 'text-yellow-600 bg-yellow-50 border-yellow-200'
    case 'error':
    case 'disconnected':
    case 'failed':
      return 'text-red-600 bg-red-50 border-red-200'
    default:
      return 'text-gray-600 bg-gray-50 border-gray-200'
  }
}
