import { useNavigate } from 'react-router-dom'
import { useUserStore } from '@/store/useUserStore'
import client from '@/api/client'

export function useAuth() {
  const navigate = useNavigate()
  const { login: storeLogin, logout: storeLogout } = useUserStore()

  const login = async (
    username: string,
    password: string,
    preLoginToken: string,
    extra?: { rememberMe?: boolean; turnstileToken?: string; mfaCode?: string },
  ) => {
    const data = (await client.post('/api/login', {
      preLoginToken,
      username,
      password,
      rememberMe: extra?.rememberMe ?? false,
      turnstileToken: extra?.turnstileToken,
      mfaCode: extra?.mfaCode,
    })) as { username?: string; token?: string }
    storeLogin(data.username ?? username, data.token ?? '')
    return data
  }

  const logout = async () => {
    try {
      await client.post('/api/logout')
    } finally {
      storeLogout()
      navigate('/login')
    }
  }

  const fetchUserInfo = async () => {
    const data = (await client.get('/api/userInfo')) as { username?: string; token?: string }
    if (data?.username) {
      storeLogin(data.username, data.token ?? '')
    }
    return data
  }

  return { login, logout, fetchUserInfo }
}
