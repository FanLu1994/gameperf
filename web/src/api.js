import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
  timeout: 30000
})

// 会话管理
export const createSession = (data) => api.post('/sessions', data)
export const listSessions = () => api.get('/sessions')
export const getSession = (id) => api.get(`/sessions/${id}`)
export const deleteSession = (id) => api.delete(`/sessions/${id}`)

// 采集控制
export const startCollect = (id, data) => api.post(`/sessions/${id}/start`, data)
export const stopCollect = (id) => api.post(`/sessions/${id}/stop`)

// 数据查询
export const getSamples = (id) => api.get(`/sessions/${id}/samples`)
export const getSummary = (id) => api.get(`/sessions/${id}/summary`)
export const injectSample = (id, data) => api.post(`/sessions/${id}/samples`, data)

// 对比分析
export const compareSessions = (ids) => api.get(`/compare?ids=${ids.join(',')}`)

export default api
