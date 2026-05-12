import axios from 'axios'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

export interface InstanceConfig {
  id?: string
  name: string
  iec104_port: number
  xlsx_file: string
  enabled?: boolean
  http_enabled?: boolean
  http_port?: number
}

export interface InstanceStats {
  uptime_seconds: number
  total_points: number
  client_connected: boolean
  interrogations: number
  controls: number
  spontaneous: number
}

export interface InstanceState {
  id: string
  name: string
  iec104_port: number
  xlsx_file: string
  enabled: boolean
  http_enabled?: boolean
  http_port?: number
  status: 'running' | 'stopped' | 'error'
  stats?: InstanceStats
  error?: string
}

export interface GlobalStatus {
  version: string
  mode: string
  configured: number
  running: number
  stopped: number
  max: number
}

// Instance CRUD
export async function listInstances(): Promise<InstanceState[]> {
  const res = await http.get('/instances')
  return res.data.instances
}

export async function createInstance(cfg: InstanceConfig): Promise<void> {
  await http.post('/instances', cfg)
}

export async function getInstance(id: string): Promise<InstanceState> {
  const res = await http.get(`/instances/${id}`)
  return res.data
}

export async function updateInstance(id: string, cfg: InstanceConfig): Promise<void> {
  await http.put(`/instances/${id}`, cfg)
}

export async function deleteInstance(id: string): Promise<void> {
  await http.delete(`/instances/${id}`)
}

// Instance control
export async function startInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/start`)
}

export async function stopInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/stop`)
}

export async function restartInstance(id: string): Promise<void> {
  await http.post(`/instances/${id}/restart`)
}

// File upload
export async function uploadExcel(file: File): Promise<string> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post('/upload', form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data.filename
}

// Global status
export async function getStatus(): Promise<GlobalStatus> {
  const res = await http.get('/status')
  return res.data
}

// Uploaded files list
export async function listFiles(): Promise<{ name: string; size: number; modtime: string }[]> {
  const res = await http.get('/files')
  return res.data.files
}
