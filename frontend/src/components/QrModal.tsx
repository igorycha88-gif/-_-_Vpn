import { Modal } from 'antd'
import { useEffect, useState } from 'react'

interface QrModalProps {
  open: boolean
  peerId: string | null
  peerName: string
  onClose: () => void
}

export default function QrModal({ open, peerId, peerName, onClose }: QrModalProps) {
  const [qrUrl, setQrUrl] = useState<string>('')

  useEffect(() => {
    if (open && peerId) {
      const token = localStorage.getItem('smarttraffic_tokens')
      let authParam = ''
      if (token) {
        try {
          const parsed = JSON.parse(token)
          authParam = `?token=${parsed.accessToken}`
        } catch {
          // ignore
        }
      }
      setQrUrl(`/api/v1/wg/peers/${peerId}/qr${authParam}`)
    }
  }, [open, peerId])

  return (
    <Modal
      title={`QR-код: ${peerName}`}
      open={open}
      onCancel={onClose}
      footer={null}
      centered
    >
      <div style={{ textAlign: 'center' }}>
        {qrUrl && (
          <img
            src={qrUrl}
            alt="QR код"
            style={{ maxWidth: 300, maxHeight: 300 }}
          />
        )}
      </div>
    </Modal>
  )
}
