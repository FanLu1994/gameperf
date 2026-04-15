<template>
  <div class="dashboard">
    <!-- 新建采集 -->
    <el-card class="control-card">
      <template #header>
        <span>🚀 新建采集</span>
      </template>
      <el-form :inline="true" :model="form">
        <el-form-item label="标签">
          <el-input v-model="form.name" placeholder="如：v1.0优化前" style="width:180px" />
        </el-form-item>
        <el-form-item label="进程PID">
          <el-input-number v-model="form.pid" :min="1" placeholder="目标进程PID" />
        </el-form-item>
        <el-form-item label="采样间隔">
          <el-select v-model="form.interval" style="width:120px">
            <el-option label="500ms" value="500ms" />
            <el-option label="1秒" value="1s" />
            <el-option label="2秒" value="2s" />
            <el-option label="5秒" value="5s" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleCreate" :loading="creating">创建并开始采集</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 实时数据 -->
    <div v-if="currentSession" class="charts-row">
      <el-card class="chart-card">
        <template #header>
          <div class="chart-header">
            <span>📊 CPU 使用率</span>
            <span class="live-value" :class="cpuClass">{{ currentCPU }}%</span>
          </div>
        </template>
        <div ref="cpuChart" class="chart-container"></div>
      </el-card>

      <el-card class="chart-card">
        <template #header>
          <div class="chart-header">
            <span>💾 内存占用</span>
            <span class="live-value">{{ currentMem }} MB</span>
          </div>
        </template>
        <div ref="memChart" class="chart-container"></div>
      </el-card>

      <el-card class="chart-card">
        <template #header>
          <div class="chart-header">
            <span>🧵 线程数</span>
            <span class="live-value">{{ currentThreads }}</span>
          </div>
        </template>
        <div ref="threadChart" class="chart-container"></div>
      </el-card>
    </div>

    <!-- 控制栏 -->
    <div v-if="currentSession" class="action-bar">
      <el-tag type="success" size="large">采集中: {{ currentSession.name || currentSession.id }}</el-tag>
      <el-tag>已采集 {{ sampleCount }} 条</el-tag>
      <el-button type="danger" @click="handleStop">停止采集</el-button>
    </div>

    <!-- 最近会话 -->
    <el-card style="margin-top:20px">
      <template #header><span>📋 最近采集记录</span></template>
      <el-table :data="recentSessions" stripe style="width:100%" max-height="300">
        <el-table-column prop="name" label="标签" width="180" />
        <el-table-column prop="process" label="进程" width="120" />
        <el-table-column prop="status" label="状态" width="100">
          <template #default="{ row }">
            <el-tag :type="row.status === 'running' ? 'success' : 'info'" size="small">
              {{ row.status }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_time" label="开始时间" width="200" />
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button size="small" type="primary" @click="$router.push('/compare?highlight=' + row.id)">对比</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row.id)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, nextTick } from 'vue'
import * as echarts from 'echarts'
import { createSession, startCollect, stopCollect, listSessions, getSamples, deleteSession } from '../api'

const form = ref({ name: '', pid: null, interval: '1s' })
const creating = ref(false)
const currentSession = ref(null)
const recentSessions = ref([])
const sampleCount = ref(0)

const cpuChart = ref(null)
const memChart = ref(null)
const threadChart = ref(null)

let cpuInstance, memInstance, threadInstance
let pollTimer = null
let allSamples = []

const currentCPU = computed(() => {
  if (allSamples.length === 0) return '0.0'
  return allSamples[allSamples.length - 1].cpu.toFixed(1)
})

const currentMem = computed(() => {
  if (allSamples.length === 0) return '0.0'
  return allSamples[allSamples.length - 1].memory.toFixed(1)
})

const currentThreads = computed(() => {
  if (allSamples.length === 0) return 0
  return allSamples[allSamples.length - 1].threads
})

const cpuClass = computed(() => {
  const v = parseFloat(currentCPU.value)
  if (v > 80) return 'danger'
  if (v > 50) return 'warning'
  return 'normal'
})

function initCharts() {
  cpuInstance = echarts.init(cpuChart.value, 'dark')
  memInstance = echarts.init(memChart.value, 'dark')
  threadInstance = echarts.init(threadChart.value, 'dark')

  const baseOption = {
    backgroundColor: 'transparent',
    grid: { top: 20, right: 20, bottom: 30, left: 60 },
    xAxis: { type: 'category', data: [], axisLabel: { color: '#666' } },
    yAxis: { type: 'value', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
    series: [{ type: 'line', smooth: true, showSymbol: false, data: [], areaStyle: { opacity: 0.15 } }],
    tooltip: { trigger: 'axis' },
    animation: false,
  }

  cpuInstance.setOption({ ...baseOption, series: [{ ...baseOption.series[0], itemStyle: { color: '#00d4ff' }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(0,212,255,0.3)' }, { offset: 1, color: 'rgba(0,212,255,0)' }]) } }] })
  memInstance.setOption({ ...baseOption, series: [{ ...baseOption.series[0], itemStyle: { color: '#36d399' }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(54,211,153,0.3)' }, { offset: 1, color: 'rgba(54,211,153,0)' }]) } }] })
  threadInstance.setOption({ ...baseOption, series: [{ ...baseOption.series[0], itemStyle: { color: '#f59e0b' }, areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: 'rgba(245,158,11,0.3)' }, { offset: 1, color: 'rgba(245,158,11,0)' }]) } }] })
}

function updateCharts() {
  if (!cpuInstance) return

  const times = allSamples.map(s => {
    const elapsed = s.elapsed
    const min = Math.floor(elapsed / 60)
    const sec = Math.floor(elapsed % 60)
    return `${min}:${sec.toString().padStart(2, '0')}`
  })

  cpuInstance.setOption({
    xAxis: { data: times },
    series: [{ data: allSamples.map(s => s.cpu.toFixed(1)) }]
  })

  memInstance.setOption({
    xAxis: { data: times },
    series: [{ data: allSamples.map(s => s.memory.toFixed(1)) }]
  })

  threadInstance.setOption({
    xAxis: { data: times },
    series: [{ data: allSamples.map(s => s.threads) }]
  })
}

async function pollData() {
  if (!currentSession.value) return
  try {
    const { data } = await getSamples(currentSession.value.id)
    allSamples = data
    sampleCount.value = data.length
    updateCharts()
  } catch (e) {
    console.error('poll error', e)
  }
}

async function handleCreate() {
  if (!form.value.pid) {
    ElMessage.warning('请输入进程 PID')
    return
  }
  creating.value = true
  try {
    const { data: session } = await createSession({
      name: form.value.name,
      process: `PID:${form.value.pid}`
    })
    await startCollect(session.id, {
      pid: form.value.pid,
      interval: form.value.interval
    })
    currentSession.value = { ...session, status: 'running' }
    allSamples = []

    await nextTick()
    if (!cpuInstance) initCharts()

    pollTimer = setInterval(pollData, 2000)
    ElMessage.success('采集已开始')
  } catch (e) {
    ElMessage.error('启动失败: ' + (e.response?.data?.error || e.message))
  }
  creating.value = false
}

async function handleStop() {
  if (!currentSession.value) return
  try {
    await stopCollect(currentSession.value.id)
    clearInterval(pollTimer)
    currentSession.value = null
    ElMessage.success('采集已停止')
    loadSessions()
  } catch (e) {
    ElMessage.error('停止失败')
  }
}

async function handleDelete(id) {
  try {
    await deleteSession(id)
    ElMessage.success('已删除')
    loadSessions()
  } catch (e) {
    ElMessage.error('删除失败')
  }
}

async function loadSessions() {
  try {
    const { data } = await listSessions()
    recentSessions.value = data.slice(0, 20)
  } catch (e) {
    console.error(e)
  }
}

import { ElMessage } from 'element-plus'

onMounted(() => {
  loadSessions()
})

onUnmounted(() => {
  clearInterval(pollTimer)
  cpuInstance?.dispose()
  memInstance?.dispose()
  threadInstance?.dispose()
})
</script>

<style scoped>
.dashboard {}
.control-card { margin-bottom: 20px; }
.charts-row { display: grid; grid-template-columns: repeat(3, 1fr); gap: 16px; margin-bottom: 16px; }
.chart-card {}
.chart-header { display: flex; justify-content: space-between; align-items: center; }
.live-value { font-size: 20px; font-weight: 700; color: #00d4ff; }
.live-value.warning { color: #f59e0b; }
.live-value.danger { color: #ef4444; }
.live-value.normal { color: #36d399; }
.chart-container { height: 250px; }
.action-bar { display: flex; align-items: center; gap: 12px; padding: 12px 0; }
</style>
