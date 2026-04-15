<template>
  <div class="dashboard">
    <!-- 新建采集 -->
    <el-card class="control-card">
      <template #header><span>🚀 新建采集</span></template>
      <el-form :inline="true" :model="form" label-width="80px">
        <el-form-item label="标签">
          <el-input v-model="form.name" placeholder="如：v1.0优化前" style="width:160px" />
        </el-form-item>
        <el-form-item label="进程PID">
          <el-input-number v-model="form.pid" :min="1" />
        </el-form-item>
        <el-form-item label="采样间隔">
          <el-select v-model="form.interval" style="width:110px">
            <el-option label="500ms" value="500ms" />
            <el-option label="1秒" value="1s" />
            <el-option label="2秒" value="2s" />
            <el-option label="5秒" value="5s" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button type="primary" @click="handleCreate" :loading="creating">开始采集</el-button>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 实时数据 -->
    <template v-if="currentSession">
      <!-- 状态栏 -->
      <div class="status-bar">
        <el-tag type="success" size="large">🟢 采集中: {{ currentSession.name || currentSession.id }}</el-tag>
        <el-tag>采样 {{ sampleCount }} 条</el-tag>
        <el-tag type="warning" v-if="latestCPU > 80">⚠️ CPU高负载</el-tag>
        <el-button type="danger" size="small" @click="handleStop">⏹ 停止</el-button>
      </div>

      <!-- CPU & Memory -->
      <div class="charts-row-2">
        <el-card>
          <template #header>
            <div class="chart-header"><span>🖥️ CPU</span><span class="lv" :class="cpuClass">{{ latestCPU.toFixed(1) }}%</span></div>
          </template>
          <div ref="cpuChart" class="chart-sm"></div>
        </el-card>
        <el-card>
          <template #header>
            <div class="chart-header"><span>💾 内存</span><span class="lv">{{ latestMem.toFixed(1) }} MB</span></div>
          </template>
          <div ref="memChart" class="chart-sm"></div>
        </el-card>
      </div>

      <!-- GPU -->
      <div class="charts-row-2" v-if="hasGPU">
        <el-card>
          <template #header>
            <div class="chart-header"><span>🎮 GPU</span><span class="lv">{{ latestGPU.toFixed(1) }}%  {{ latestGPUTemp > 0 ? latestGPUTemp.toFixed(0) + '℃' : '' }}</span></div>
          </template>
          <div ref="gpuChart" class="chart-sm"></div>
        </el-card>
        <el-card>
          <template #header>
            <div class="chart-header"><span>📊 GPU 显存</span><span class="lv">{{ latestGPUMem.toFixed(0) }} / {{ latestGPUMemTotal.toFixed(0) }} MB</span></div>
          </template>
          <div ref="gpuMemChart" class="chart-sm"></div>
        </el-card>
      </div>

      <!-- FPS & Frame Time -->
      <div class="charts-row-2" v-if="hasFPS">
        <el-card>
          <template #header>
            <div class="chart-header"><span>🎞️ FPS</span><span class="lv" :class="fpsClass">{{ latestFPS.toFixed(1) }}</span></div>
          </template>
          <div ref="fpsChart" class="chart-sm"></div>
        </el-card>
        <el-card>
          <template #header>
            <div class="chart-header"><span>⏱️ 帧时间</span><span class="lv">{{ latestFT.toFixed(2) }} ms</span></div>
          </template>
          <div ref="ftChart" class="chart-sm"></div>
        </el-card>
      </div>

      <!-- Disk & Network -->
      <div class="charts-row-2">
        <el-card>
          <template #header>
            <div class="chart-header"><span>💿 磁盘 I/O</span><span class="lv">读 {{ formatBPS(latestDiskRead) }} / 写 {{ formatBPS(latestDiskWrite) }}</span></div>
          </template>
          <div ref="diskChart" class="chart-sm"></div>
        </el-card>
        <el-card>
          <template #header>
            <div class="chart-header"><span>🌐 网络 I/O</span><span class="lv">↑ {{ formatBPS(latestNetSent) }} ↓ {{ formatBPS(latestNetRecv) }}</span></div>
          </template>
          <div ref="netChart" class="chart-sm"></div>
        </el-card>
      </div>

      <!-- Threads & Handles -->
      <div class="charts-row-2">
        <el-card>
          <template #header>
            <div class="chart-header"><span>🧵 线程数</span><span class="lv">{{ latestThreads }}</span></div>
          </template>
          <div ref="threadChart" class="chart-xs"></div>
        </el-card>
        <el-card>
          <template #header>
            <div class="chart-header"><span>🔑 句柄数</span><span class="lv">{{ latestHandles }}</span></div>
          </template>
          <div ref="handleChart" class="chart-xs"></div>
        </el-card>
      </div>
    </template>

    <!-- 最近记录 -->
    <el-card style="margin-top:20px">
      <template #header><span>📋 最近记录</span></template>
      <el-table :data="recentSessions" stripe max-height="300">
        <el-table-column prop="name" label="标签" width="160" />
        <el-table-column prop="status" label="状态" width="90">
          <template #default="{ row }">
            <el-tag :type="row.status === 'running' ? 'success' : 'info'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="start_time" label="时间" width="200" />
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
import { ElMessage } from 'element-plus'

const form = ref({ name: '', pid: null, interval: '1s' })
const creating = ref(false)
const currentSession = ref(null)
const recentSessions = ref([])
const sampleCount = ref(0)
const allSamples = ref([])

// chart refs
const cpuChart = ref(null), memChart = ref(null), gpuChart = ref(null), gpuMemChart = ref(null)
const fpsChart = ref(null), ftChart = ref(null), diskChart = ref(null), netChart = ref(null)
const threadChart = ref(null), handleChart = ref(null)

let charts = {}
let pollTimer = null

// computed latest values
const latestCPU = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].cpu : 0)
const latestMem = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].memory : 0)
const latestGPU = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].gpu_util : 0)
const latestGPUTemp = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].gpu_temp : 0)
const latestGPUMem = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].gpu_mem : 0)
const latestGPUMemTotal = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].gpu_mem_total : 0)
const latestFPS = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].fps : 0)
const latestFT = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].frame_time : 0)
const latestDiskRead = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].disk_read_bps : 0)
const latestDiskWrite = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].disk_write_bps : 0)
const latestNetSent = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].net_sent_bps : 0)
const latestNetRecv = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].net_recv_bps : 0)
const latestThreads = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].threads : 0)
const latestHandles = computed(() => allSamples.value.length ? allSamples.value[allSamples.value.length - 1].handle_count : 0)

const hasGPU = computed(() => allSamples.value.some(s => s.gpu_util > 0))
const hasFPS = computed(() => allSamples.value.some(s => s.fps > 0))

const cpuClass = computed(() => { const v = latestCPU.value; return v > 80 ? 'danger' : v > 50 ? 'warning' : 'good' })
const fpsClass = computed(() => { const v = latestFPS.value; return v < 30 ? 'danger' : v < 60 ? 'warning' : 'good' })

function formatBPS(bps) {
  if (!bps) return '0 B/s'
  if (bps > 1048576) return (bps / 1048576).toFixed(1) + ' MB/s'
  if (bps > 1024) return (bps / 1024).toFixed(1) + ' KB/s'
  return bps.toFixed(0) + ' B/s'
}

function makeChartOption(color, unit) {
  return {
    backgroundColor: 'transparent',
    grid: { top: 10, right: 15, bottom: 25, left: 55 },
    xAxis: { type: 'category', data: [], axisLabel: { color: '#555', fontSize: 10 } },
    yAxis: { type: 'value', axisLabel: { color: '#555', fontSize: 10, formatter: `{value}${unit}` }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
    series: [{ type: 'line', smooth: true, showSymbol: false, data: [],
      itemStyle: { color },
      areaStyle: { color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [{ offset: 0, color: color + '40' }, { offset: 1, color: color + '00' }]) }
    }],
    tooltip: { trigger: 'axis', valueFormatter: v => v + unit },
    animation: false,
  }
}

function initCharts() {
  charts.cpu = echarts.init(cpuChart.value, 'dark')
  charts.mem = echarts.init(memChart.value, 'dark')
  charts.gpu = echarts.init(gpuChart.value, 'dark')
  charts.gpuMem = echarts.init(gpuMemChart.value, 'dark')
  charts.fps = echarts.init(fpsChart.value, 'dark')
  charts.ft = echarts.init(ftChart.value, 'dark')
  charts.disk = echarts.init(diskChart.value, 'dark')
  charts.net = echarts.init(netChart.value, 'dark')
  charts.thread = echarts.init(threadChart.value, 'dark')
  charts.handle = echarts.init(handleChart.value, 'dark')

  charts.cpu.setOption(makeChartOption('#00d4ff', '%'))
  charts.mem.setOption(makeChartOption('#36d399', 'MB'))
  charts.gpu.setOption(makeChartOption('#a855f7', '%'))
  charts.gpuMem.setOption(makeChartOption('#f472b6', 'MB'))
  charts.fps.setOption(makeChartOption('#22d3ee', ''))
  charts.ft.setOption(makeChartOption('#fb923c', 'ms'))
  charts.disk.setOption({
    backgroundColor: 'transparent',
    grid: { top: 30, right: 15, bottom: 25, left: 55 },
    legend: { data: ['读取', '写入'], textStyle: { color: '#888', fontSize: 10 }, top: 0 },
    xAxis: { type: 'category', data: [], axisLabel: { color: '#555', fontSize: 10 } },
    yAxis: { type: 'value', axisLabel: { color: '#555', fontSize: 10, formatter: v => formatBPS(v) }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
    series: [
      { name: '读取', type: 'line', smooth: true, showSymbol: false, data: [], itemStyle: { color: '#60a5fa' } },
      { name: '写入', type: 'line', smooth: true, showSymbol: false, data: [], itemStyle: { color: '#f87171' } },
    ],
    tooltip: { trigger: 'axis', valueFormatter: v => formatBPS(v) },
    animation: false,
  })
  charts.net.setOption({
    backgroundColor: 'transparent',
    grid: { top: 30, right: 15, bottom: 25, left: 55 },
    legend: { data: ['上传', '下载'], textStyle: { color: '#888', fontSize: 10 }, top: 0 },
    xAxis: { type: 'category', data: [], axisLabel: { color: '#555', fontSize: 10 } },
    yAxis: { type: 'value', axisLabel: { color: '#555', fontSize: 10, formatter: v => formatBPS(v) }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
    series: [
      { name: '上传', type: 'line', smooth: true, showSymbol: false, data: [], itemStyle: { color: '#34d399' } },
      { name: '下载', type: 'line', smooth: true, showSymbol: false, data: [], itemStyle: { color: '#818cf8' } },
    ],
    tooltip: { trigger: 'axis', valueFormatter: v => formatBPS(v) },
    animation: false,
  })
  charts.thread.setOption(makeChartOption('#f59e0b', ''))
  charts.handle.setOption(makeChartOption('#e879f9', ''))
}

function updateCharts() {
  if (!charts.cpu) return
  const s = allSamples.value
  const times = s.map(x => { const e = x.elapsed; return `${Math.floor(e/60)}:${String(Math.floor(e%60)).padStart(2,'0')}` })

  charts.cpu.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.cpu.toFixed(1)) }] })
  charts.mem.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.memory.toFixed(1)) }] })
  charts.gpu.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.gpu_util.toFixed(1)) }] })
  charts.gpuMem.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.gpu_mem.toFixed(0)) }] })
  charts.fps.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.fps.toFixed(1)) }] })
  charts.ft.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.frame_time.toFixed(2)) }] })
  charts.disk.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.disk_read_bps.toFixed(0)) }, { data: s.map(x => x.disk_write_bps.toFixed(0)) }] })
  charts.net.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.net_sent_bps.toFixed(0)) }, { data: s.map(x => x.net_recv_bps.toFixed(0)) }] })
  charts.thread.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.threads) }] })
  charts.handle.setOption({ xAxis: { data: times }, series: [{ data: s.map(x => x.handle_count) }] })
}

async function pollData() {
  if (!currentSession.value) return
  try {
    const { data } = await getSamples(currentSession.value.id)
    allSamples.value = data
    sampleCount.value = data.length
    updateCharts()
  } catch (e) { console.error('poll', e) }
}

async function handleCreate() {
  if (!form.value.pid) { ElMessage.warning('请输入 PID'); return }
  creating.value = true
  try {
    const { data: session } = await createSession({ name: form.value.name, pid: form.value.pid })
    await startCollect(session.id, { pid: form.value.pid, interval: form.value.interval })
    currentSession.value = { ...session, status: 'running' }
    allSamples.value = []
    await nextTick()
    if (!charts.cpu) initCharts()
    pollTimer = setInterval(pollData, 2000)
    ElMessage.success('采集已开始')
  } catch (e) {
    ElMessage.error('启动失败: ' + (e.response?.data?.error || e.message))
  }
  creating.value = false
}

async function handleStop() {
  if (!currentSession.value) return
  await stopCollect(currentSession.value.id)
  clearInterval(pollTimer)
  currentSession.value = null
  ElMessage.success('已停止')
  loadSessions()
}

async function handleDelete(id) {
  await deleteSession(id)
  ElMessage.success('已删除')
  loadSessions()
}

async function loadSessions() {
  try { const { data } = await listSessions(); recentSessions.value = data.slice(0, 20) } catch {}
}

onMounted(loadSessions)
onUnmounted(() => {
  clearInterval(pollTimer)
  Object.values(charts).forEach(c => c?.dispose())
})
</script>

<style scoped>
.status-bar { display: flex; align-items: center; gap: 10px; margin: 12px 0; flex-wrap: wrap; }
.charts-row-2 { display: grid; grid-template-columns: 1fr 1fr; gap: 12px; margin-bottom: 12px; }
.chart-header { display: flex; justify-content: space-between; align-items: center; }
.lv { font-size: 18px; font-weight: 700; color: #00d4ff; }
.lv.warning { color: #f59e0b; }
.lv.danger { color: #ef4444; }
.lv.good { color: #36d399; }
.chart-sm { height: 200px; }
.chart-xs { height: 140px; }
</style>
