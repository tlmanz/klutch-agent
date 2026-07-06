import { useState } from 'react'
import { agent } from '../lib/agent'
import { Button, Segmented } from './primitives'
import { Modal, TextArea, TextInput } from './form'

// The first-run / re-connect wizard. Collects the backend URL and connects via a
// one-time pairing code or a pre-issued device token, in one consistent modal
// (replacing Fyne's dialog with mismatched inputs/buttons).
export function EnrollModal({
  open,
  onClose,
  defaultServer,
}: {
  open: boolean
  onClose: () => void
  defaultServer: string
}) {
  const [server, setServer] = useState(defaultServer)
  const [mode, setMode] = useState('code')
  const [code, setCode] = useState('')
  const [token, setToken] = useState('')
  const [busy, setBusy] = useState(false)
  const [err, setErr] = useState('')

  const connect = async () => {
    setErr('')
    if (mode === 'token' && !token.trim()) return setErr('Enter a device token.')
    if (mode === 'code' && !code.trim()) return setErr('Enter a pairing code.')
    setBusy(true)
    try {
      if (server && server !== defaultServer) await agent.setServer(server)
      if (mode === 'token') await agent.setToken(token.trim())
      else await agent.enroll(code.trim())
      onClose()
    } catch (e) {
      setErr(String(e))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Set up this agent"
      width={480}
      footer={
        <>
          <Button variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="primary" onClick={connect} disabled={busy}>
            {busy ? 'Connecting…' : 'Connect'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-4">
        <div>
          <label className="mb-1.5 block text-[13px] font-bold text-text">Backend server</label>
          <TextInput value={server} onChange={setServer} placeholder="https://api.example.com" />
        </div>
        <div>
          <label className="mb-1.5 block text-[13px] font-bold text-text">Connect using</label>
          <Segmented
            value={mode}
            onChange={setMode}
            items={[
              { key: 'code', label: 'Pairing code' },
              { key: 'token', label: 'Device token' },
            ]}
          />
        </div>
        {mode === 'code' ? (
          <div>
            <label className="mb-1.5 block text-[13px] font-bold text-text">Pairing code</label>
            <TextInput value={code} onChange={setCode} placeholder="one-time code from the dashboard" autoFocus />
          </div>
        ) : (
          <div>
            <label className="mb-1.5 block text-[13px] font-bold text-text">Device token</label>
            <TextArea value={token} onChange={setToken} placeholder="paste the device token" />
          </div>
        )}
        {err && <div className="text-[13px] font-semibold text-red-ink">{err}</div>}
      </div>
    </Modal>
  )
}
