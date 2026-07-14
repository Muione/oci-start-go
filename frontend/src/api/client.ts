import axios, { type AxiosRequestConfig } from 'axios'

// Axios instance with cookie auth (satoken). Response interceptor unwraps the
// ApiResponse envelope ({success,message,data,code}) → returns data on success,
// rejects with Error(message) on failure or non-2xx.
// On 401, dispatches 'auth:unauthorized' custom event for the router to handle.
const raw = axios.create({
  baseURL: '/',
  withCredentials: true,
  timeout: 30000,
})

raw.interceptors.response.use(
  (response) => {
    const b = response.data
    if (b && typeof b.success === 'boolean') {
      if (!b.success) return Promise.reject(new Error(b.message || 'error'))
      return b.data
    }
    return b
  },
  (error) => {
    if (error.response?.status === 401) {
      window.dispatchEvent(new CustomEvent('auth:unauthorized'))
    }
    const msg = error.response?.data?.message || error.message || '请求错误'
    return Promise.reject(new Error(msg))
  },
)

// Typed wrapper matching the 2-generic-param pattern used in the codebase:
//   client.get<unknown, ActualReturn>(url)
// The interceptor unwraps the envelope, so the 2nd generic is the real return type.
const client = {
  get: <_T = unknown, R = unknown>(url: string, config?: AxiosRequestConfig) =>
    raw.get(url, config) as unknown as Promise<R>,
  post: <_T = unknown, R = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    raw.post(url, data, config) as unknown as Promise<R>,
  put: <_T = unknown, R = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    raw.put(url, data, config) as unknown as Promise<R>,
  delete: <_T = unknown, R = unknown>(url: string, config?: AxiosRequestConfig) =>
    raw.delete(url, config) as unknown as Promise<R>,
  patch: <_T = unknown, R = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig) =>
    raw.patch(url, data, config) as unknown as Promise<R>,
}

export default client
