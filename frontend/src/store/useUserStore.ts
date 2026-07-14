import { create } from 'zustand'
import client from '@/api/client'

interface UserState {
  username: string
  isLoggedIn: boolean
  token: string
  authChecked: boolean
  login: (username: string, token: string) => void
  logout: () => void
  setUsername: (username: string) => void
  checkAuth: () => Promise<void>
}

export const useUserStore = create<UserState>((set) => ({
  username: '',
  isLoggedIn: false,
  token: '',
  authChecked: false,
  login: (username, token) => set({ username, isLoggedIn: true, token, authChecked: true }),
  logout: () => set({ username: '', isLoggedIn: false, token: '', authChecked: true }),
  setUsername: (username) => set({ username }),
  checkAuth: async () => {
    try {
      const data = await client.get('/api/userInfo') as { username?: string; token?: string }
      if (data?.username) {
        set({ username: data.username, isLoggedIn: true, token: data.token ?? '', authChecked: true })
      } else {
        set({ isLoggedIn: false, authChecked: true })
      }
    } catch {
      set({ isLoggedIn: false, authChecked: true })
    }
  },
}))
