<template>
  <div class="compare">
    <el-card>
      <template #header><span>📊 对比分析</span></template>
      
      <div class="selector">
        <el-select v-model="selectedIds" multiple placeholder="选择要对比的采集记录" style="width:600px">
          <el-option v-for="s in sessions" :key="s.id" :label="(s.name || s.id) + ' - ' + s.start_time" :value="s.id" />
        </el-select>
        <el-button type="primary" @click="loadCompare" :loading="loading" :disabled="selectedIds.length < 2">
          开始对比 ({{ selectedIds.length }}/{{ selectedIds.length >= 2 ? '✓' : '至少选2个' }})
        </el-button>
      </div>
    </el-card>

    <!-- 对比摘要表格 -->
    <el-card v-if="results.length > 0" style="margin-top:16px">
      <template #header><span>📋 对比摘要</span></template>
      <el-table :data="results" stripe>
        <el-table-column label="标签" width="180">
          <template #default="{ row }">
            <strong>{{ row.session?.name || row.session?.id }}</strong>
          </template>
        </el-table-column>
        <el-table-column label="时长" width="100">
          <template #default="{ row }">{{ row.duration?.toFixed(0) }}秒</template>
        </el-table-column>
        <el-table-column label="采样数" width="100">
          <template #default="{ row }">{{ row.sample_count }}</template>
        </el-table-column>
        <el-table-column label="CPU Avg/Max/P95" width="200">
          <template #default="{ row }">
            <span :class="metricClass(row.avg_cpu, 'cpu')">
              {{ row.avg_cpu?.toFixed(1) }}% / {{ row.max_cpu?.toFixed(1) }}% / {{ row.p95_cpu?.toFixed(1) }}%
            </span>
          </template>
        </el-table-column>
        <el-table-column label="内存 Avg/Max/P95" width="240">
          <template #default="{ row }">
            {{ row.avg_memory?.toFixed(0) }} / {{ row.max_memory?.toFixed(0) }} / {{ row.p95_memory?.toFixed(0) }} MB
          </template>
        </el-table-column>
        <el-table-column label="FPS Avg/Min" width="160">
          <template #default="{ row }">
            <span v-if="row.avg_fps">{{ row.avg_fps?.toFixed(1) }} / {{ row.min_fps?.toFixed(1) }}</span>
            <span v-else>-</span>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 对比图表 -->
    <div v-if="results.length > 0" class="compare-charts">
      <el-card>
        <template #header><span>CPU 使用率对比</span></template>
        <div ref="cpuCompare" class="chart-container"></div>
      </el-card>
      <el-card>
        <template #header><span>内存占用对比</span></template>
        <div ref="memCompare" class="chart-container"></div>
      </el-card>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted, nextTick } from 'vue'
import { useRoute } from 'vue-router'
import * as echarts from 'echarts'
import { listSessions, getSamples } from '../api'
import { ElMessage } from 'element-plus'

const route = useRoute()
const sessions = ref([])
const selectedIds = ref([])
const results = ref([])
const loading = ref(false)

const cpuCompare = ref(null)
const memCompare = ref(null)
let cpuInstance, memInstance

const colors = ['#00d4ff', '#36d399', '#f59e0b', '#ef4444', '#a855f7', '#ec4899']

function metricClass(value, type) {
  if (type === 'cpu') {
    if (value > 80) return 'metric-danger'
    if (value > 50) return 'metric-warning'
  }
  return ''
}

async function loadCompare() {
  if (selectedIds.value.length < 2) return
  loading.value = true
  try {
    const allData = []
    for (const id of selectedIds.value) {
      const { data } = await getSamples(id)
      const session = sessions.value.find(s => s.id === id)
      allData.push({ id, session, samples: data })
    }
    results.value = allData.map(d => ({
      session: d.session,
      sample_count: d.samples.length,
      duration: d.samples.length > 0 ? d.samples[d.samples.length - 1].elapsed : 0,
      avg_cpu: avg(d.samples.map(s => s.cpu)),
      max_cpu: max(d.samples.map(s => s.cpu)),
      p95_cpu: p95(d.samples.map(s => s.cpu)),
      avg_memory: avg(d.samples.map(s => s.memory)),
      max_memory: max(d.samples.map(s => s.memory)),
      p95_memory: p95(d.samples.map(s => s.memory)),
      avg_fps: avg(d.samples.filter(s => s.fps > 0).map(s => s.fps)),
      min_fps: min(d.samples.filter(s => s.fps > 0).map(s => s.fps)),
    }))

    await nextTick()
    drawCompareCharts(allData)
  } catch (e) {
    ElMessage.error('对比失败')
  }
  loading.value = false
}

function drawCompareCharts(allData) {
  if (!cpuInstance) {
    cpuInstance = echarts.init(cpuCompare.value, 'dark')
    memInstance = echarts.init(memCompare.value, 'dark')
  }

  // 使用 elapsed 秒数作为公共 x 轴
  const cpuSeries = []
  const memSeries = []

  allData.forEach((d, i) => {
    const name = d.session?.name || d.id
    cpuSeries.push({
      name, type: 'line', smooth: true, showSymbol: false,
      data: d.samples.map(s => [s.elapsed, s.cpu]),
      itemStyle: { color: colors[i % colors.length] },
    })
    memSeries.push({
      name, type: 'line', smooth: true, showSymbol: false,
      data: d.samples.map(s => [s.elapsed, s.memory]),
      itemStyle: { color: colors[i % colors.length] },
    })
  })

  const baseOpt = {
    backgroundColor: 'transparent',
    tooltip: { trigger: 'axis' },
    legend: { textStyle: { color: '#999' } },
    grid: { top: 40, right: 20, bottom: 30, left: 60 },
    xAxis: { type: 'value', name: '秒', axisLabel: { color: '#666' } },
    yAxis: { type: 'value', axisLabel: { color: '#666' }, splitLine: { lineStyle: { color: '#1a1a3a' } } },
  }

  cpuInstance.setOption({ ...baseOpt, legend: { ...baseOpt.legend, data: allData.map(d => d.session?.name || d.id) }, series: cpuSeries })
  memInstance.setOption({ ...baseOpt, legend: { ...baseOpt.legend, data: allData.map(d => d.session?.name || d.id) }, series: memSeries })
}

function avg(arr) { return arr.length ? arr.reduce((a, b) => a + b, 0) / arr.length : 0 }
function max(arr) { return arr.length ? Math.max(...arr) : 0 }
function min(arr) { return arr.length ? Math.min(...arr) : 0 }
function p95(arr) {
  if (!arr.length) return 0
  const sorted = [...arr].sort((a, b) => a - b)
  return sorted[Math.floor(sorted.length * 0.95)]
}

onMounted(async () => {
  const { data } = await listSessions()
  sessions.value = data

  // 从 query 参数自动选中
  const highlight = route.query.highlight
  if (highlight) {
    selectedIds.value = [highlight]
  }
})
</script>

<style scoped>
.selector { display: flex; gap: 12px; align-items: center; }
.compare-charts { display: grid; grid-template-columns: 1fr 1fr; gap: 16px; margin-top: 16px; }
.chart-container { height: 350px; }
.metric-danger { color: #ef4444; font-weight: 700; }
.metric-warning { color: #f59e0b; }
</style>
