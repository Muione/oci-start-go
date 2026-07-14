import client from './client'

export interface SshKey {
  id: number
  label: string
  fingerprint: string
}

export interface SshKeyAddPayload {
  label: string
  content: string
  passphrase?: string
}

export function sshKeyList(): Promise<SshKey[]> {
  return client.get<unknown, SshKey[]>('/ssh-keys')
}

export function sshKeyAdd(data: SshKeyAddPayload): Promise<unknown> {
  return client.post<unknown, unknown>('/ssh-keys', data)
}

export function sshKeyDelete(id: number): Promise<unknown> {
  return client.delete<unknown, unknown>(`/ssh-keys/${id}`)
}
