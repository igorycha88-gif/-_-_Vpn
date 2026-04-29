import { getAccessToken } from '../store/auth'

export function downloadWithAuth(url: string, filename: string): void {
  const token = getAccessToken()
  const link = document.createElement('a')
  const separator = url.includes('?') ? '&' : '?'
  link.href = `${url}${separator}token=${token}`
  link.download = filename
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
}
