import { defineStore } from 'pinia'
import request from '../utils/request'

interface UserInfo {
  username: string
  role: string
}

export const useUserStore = defineStore('user', {
  state: () => ({
    username: '' as string,
    role: '' as string,
  }),
  actions: {
    async fetchUserInfo() {
      const data = (await request.get('/api/userInfo')) as unknown as UserInfo
      if (data) {
        this.username = data.username || ''
        this.role = data.role || ''
      }
    },
    clear() {
      this.username = ''
      this.role = ''
    },
  },
})
