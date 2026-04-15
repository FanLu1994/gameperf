import axios from 'axios'

const api = axios.create({ baseURL: '/api', timeout: 30000 })

export const createSession = (data) => api.post('/sessions', data)
export const listSessions = () => api.get('/sessions')
export const getSession = (id) => api.get(`/sessions/${id}`)
export const deleteSession = (id) => api.delete(`/sessions/${id}`)
export const startCollect = (id, data) => api.post(`/sessions/${id}/start`, data)
export const stopCollect = (id) => api.post(`/sessions/${id}/stop`)
export const getSamples = (id) => api.get(`/sessions/${id}/samples`)
export const getSummary = (id) => api.get(`/sessions/${id}/summary`)
export const injectSample = (id, data) => api.post(`/sessions/${id}/samples`, data)
export const getFrameAnalysis = (id) => api.get(`/sessions/${id}/frame-analysis`)
export const getSystemInfo = (id) => api.get(`/sessions/${id}/system`)
export const compareSessions = (ids) => api.get(`/compare?ids=${ids.join(',')}`)
export const getServerInfo = () => api.get('/info')

// Android
export const listAndroidDevices = () => api.get('/android/devices')
export const listAndroidPackages = (deviceId) => api.get(`/android/packages${deviceId ? '?device_id=' + deviceId : ''}`)

// iOS
export const checkIOSPrereqs = () => api.get('/ios/check')
export const listIOSDevicesAPI = () => api.get('/ios/devices')
export const listIOSAppsAPI = (deviceId) => api.get(`/ios/apps${deviceId ? '?device_id=' + deviceId : ''}`)

export default api
