import axios from 'axios'

const http = axios.create({
  baseURL: '/api/v1',
  timeout: 10000,
})

export interface ModbusConfig {
  port?: number
  byte_order?: string
  slave_id?: number
}

export interface InstanceConfig {
  id?: string
  name: string
  iec104_port: number
  xlsx_file: string
  enabled?: boolean
  http_enabled?: boolean
  http_port?: number
  protocol?: string
  modbus_config?: ModbusConfig
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
  protocol?: string
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

export interface PointSnapshot {
  ioa: number
  name: string
  point_type: string
  value: number
  bool_value: boolean
  int_value: number
  updated_at: string
  unit: string
  function_code?: number
  register_address?: number
  byte_order?: string
}

export interface PointsResponse {
  points: PointSnapshot[]
  refreshed_at: string
}

export interface StrategyParams {
   start_value?: number
   step?: number
   period_ms?: number
   max_value?: number
   min_value?: number
   max_value_r?: number
   decimal_places?: number
   csv_file?: string
   time_format?: string
   time_unit?: string
   csv_column_map?: string
   para_a?: string
   para_b?: string
   init_soc?: number
   rated_cap?: number
   power_ioa?: number
   integral_ms?: number
   init_energy?: number
   stat_type?: number
   energy_power_ioa?: number
   energy_period_ms?: number
   follow_ao_ioa?: number
   api_init_value?: number
   custom_ioas?: string
   custom_formula?: string
  }

export interface AutoChangeConfig {
  ioa: number
  strategy: string
  enabled: boolean
  params: StrategyParams
  updated_at: string
}

export interface BatchAutoChangeRequest {
  ioas: number[]
  config: {
    strategy: string
    enabled: boolean
    params: StrategyParams
  }
}

export async function getPoints(instanceId: string): Promise<PointsResponse> {
  const res = await http.get(`/instances/${instanceId}/points`)
  return res.data
}

export async function readPoint(instanceId: string, ioa: number): Promise<PointSnapshot> {
  const res = await http.get(`/instances/${instanceId}/points/${ioa}`)
  return res.data
}

export async function setPointValue(instanceId: string, ioa: number, value: any): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/${ioa}`, value)
  return res.data
}

export async function getAutoChange(instanceId: string, ioa: number): Promise<AutoChangeConfig> {
  const res = await http.get(`/instances/${instanceId}/points/auto-change/${ioa}`)
  return res.data
}

export async function setAutoChange(instanceId: string, ioa: number, cfg: any): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/auto-change/${ioa}`, cfg)
  return res.data
}

export async function deleteAutoChange(instanceId: string, ioa: number): Promise<any> {
  const res = await http.delete(`/instances/${instanceId}/points/auto-change/${ioa}`)
  return res.data
}

export async function batchAutoChange(instanceId: string, req: BatchAutoChangeRequest): Promise<any> {
  const res = await http.put(`/instances/${instanceId}/points/auto-change/batch`, req)
  return res.data
}

export async function exportAutoConfig(instanceId: string): Promise<Blob> {
  const res = await http.get(`/instances/${instanceId}/points/auto-change/export`, { responseType: 'blob' })
  return res.data
}

export async function importAutoConfig(instanceId: string, file: File): Promise<any> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/instances/${instanceId}/points/auto-change/import`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}

export async function exportPointsCSV(instanceId: string): Promise<Blob> {
  const res = await http.get(`/instances/${instanceId}/points/export`, { responseType: 'blob' })
  return res.data
}

export async function uploadCSV(instanceId: string, file: File): Promise<any> {
  const form = new FormData()
  form.append('file', file)
  const res = await http.post(`/instances/${instanceId}/upload-csv`, form, {
    headers: { 'Content-Type': 'multipart/form-data' },
  })
  return res.data
}

export async function getProtocols(): Promise<string[]> {
  const res = await http.get('/protocols')
  return res.data.protocols
}
