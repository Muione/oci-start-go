import axios from 'axios'

// Axios instance with cookie auth (satoken). Response interceptor unwraps the
// ApiResponse envelope ({success,message,data,code}) → returns data on success,
// rejects with Error(message) on failure or non-2xx. Keeps the cycle-free
// (no router/store imports) — the router guard handles 401 redirects.
const request = axios.create({ baseURL: '/', withCredentials: true, timeout: 30000 })

request.interceptors.response.use(
  (response) => {
    const b = response.data
    if (b && typeof b.success === 'boolean') {
      if (!b.success) return Promise.reject(new Error(b.message || 'error'))
      return b.data
    }
    return b
  },
  (error) => {
    const msg = error.response?.data?.message || error.message || '请求错误'
    return Promise.reject(new Error(msg))
  },
)

export default request
