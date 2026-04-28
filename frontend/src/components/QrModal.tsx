import { Modal, Button, message } from 'antd'
import { DownloadOutlined, ReloadOutlined } from '@ant-design/icons'
import { useEffect, useState } from 'react'
import { getAccessToken } from '../store/auth'
import { downloadWithAuth } from '../utils/download'

interface QrModalProps {
  open: boolean
  peerId: string | null
  peerName: string
  onClose: () => void
}

export default function QrModal({ open, peerId, peerName, onClose }: QrModalProps) {
  const [qrUrl, setQrUrl] = useState<string>('')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)

  useEffect(() => {
    if (open && peerId) {
      const token = getAccessToken()
      const authParam = token ? `?token=${token}` : ''
      setQrUrl(`/api/v1/wg/peers/${peerId}/qr${authParam}`)
      setLoading(true)
      setError(false)
    }
  }, [open, peerId])

  const handleDownload = () => {
    if (!peerId) return
    downloadWithAuth(`/api/v1/wg/peers/${peerId}/qr`, `${peerName}-qr.png`)
    message.success('QR-код скачивается')
  }

  const handleReload = () => {
    const token = getAccessToken()
    const authParam = token ? `?token=${token}` : ''
    const ts = Date.now()
    const separator = authParam ? '&' : '?'
    setQrUrl(`/api/v1/wg/peers/${peerId}/qr${authParam}${separator}_t=${ts}`)
    setLoading(true)
    setError(false)
  }

  return (
    <Modal
      title={`QR-код: ${peerName}`}
      open={open}
      onCancel={onClose}
      footer={
        <div style={{ display: 'flex', justifyContent: 'space-between' }}>
          <Button icon={<ReloadOutlined />} onClick={handleReload}>
            Обновить
          </Button>
          <div style={{ display: 'flex', gap: 8 }}>
            <Button onClick={onClose}>Закрыть</Button>
            <Button
              type="primary"
              icon={<DownloadOutlined />}
              onClick={handleDownload}
              disabled={error || loading}
            >
              Скачать
            </Button>
          </div>
        </div>
      }
      centered
      width={400}
    >
      <div style={{ textAlign: 'center', minHeight: 256 }}>
        {error ? (
          <div style={{ padding: '40px 0' }}>
            <p>Не удалось загрузить QR-код</p>
            <Button onClick={handleReload} icon={<ReloadOutlined />}>
              Попробовать снова
            </Button>
          </div>
        ) : (
          qrUrl && (
            <img
              src={qrUrl}
              alt="QR код"
              style={{ maxWidth: 300, maxHeight: 300 }}
              onLoad={() => setLoading(false)}
              onError={() => {
                setLoading(false)
                setError(true)
              }}
            />
          )
        )}
      </div>
    </Modal>
  )
}
