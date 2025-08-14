import axios from 'axios'

export const api = axios.create({
  baseURL: 'http://localhost:8080/api/v1', // Artık doğrudan backend'e istek atacak
  timeout: 10000,
})

export const apiBase = axios.create({
  baseURL: 'http://localhost:8080', // API istekleri için temel URL
  timeout: 10000,
})